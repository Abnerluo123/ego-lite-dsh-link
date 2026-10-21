package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *server) spaceRef() string {
	if s.spaceID > 0 {
		return strconv.Itoa(s.spaceID)
	}
	return quote(s.taskName)
}
func (s *server) taskSource(body string) string {
	return browserHelpers + "\nconst task=await taskSpace(" + s.spaceRef() + ");const value=await(async()=>{" + body + "})();console.log(" + quote(resultMarker) + "+(JSON.stringify(value)??'null'));"
}
func (s *server) snapshotOptions(a map[string]string) (any, error) {
	opts := map[string]any{}
	if a["scope"] != "" {
		switch a["scope"] {
		case "viewport", "full_page", "subtree":
			opts["scope"] = a["scope"]
		default:
			return nil, errors.New("scope must be viewport, full_page or subtree")
		}
	}
	if a["root"] != "" {
		opts["root"] = a["root"]
	}
	if a["scope"] == "subtree" && a["root"] == "" {
		return nil, errors.New("subtree scope requires root ref")
	}
	limit := s.snapshotLimit()
	if a["maxChars"] != "" {
		var err error
		limit, err = intArg("maxChars", a["maxChars"], 200, 200000)
		if err != nil {
			return nil, err
		}
	}
	encoded, _ := json.Marshal(opts)
	value, err := s.run(s.pagePrefix()+"const snapshot=await page.snapshot("+string(encoded)+");return {page:page.label,url:await page.url(),title:await page.title(),snapshot:typeof snapshot==='string'?snapshot:JSON.stringify(snapshot)};", 90*time.Second)
	if err != nil {
		return nil, err
	}
	m := value.(map[string]any)
	if text, ok := m["snapshot"].(string); ok {
		m["snapshot"] = truncateSnapshot(text, limit)
	}
	s.reconcileResult(m)
	return m, nil
}

func runStandalone(body string, timeout time.Duration) (any, error) {
	source := browserHelpers + "\nconst value=await(async()=>{" + body + "})();console.log(" + quote(resultMarker) + "+(JSON.stringify(value)??'null'));"
	data, err := runner(source, timeout)
	if err != nil {
		return nil, err
	}
	var value any
	if err = json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("invalid Ego response: %w", err)
	}
	return value, nil
}

func (s *server) acceptSpace(value any) (any, error) {
	result, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("invalid task-space response")
	}
	id, ok := result["spaceId"].(float64)
	name, _ := result["name"].(string)
	ownership, _ := result["ownership"].(string)
	if !ok || id < 1 || name == "" || ownership == "" {
		return nil, errors.New("task-space response is missing id, name or ownership")
	}
	s.lastCompletion = nil
	s.spaceID = int(id)
	s.taskName = name
	s.ownership = ownership
	s.page = ""
	s.lastTabs = nil
	if tabs, ok := result["tabs"].([]any); ok {
		s.reconcileTabs(tabs)
	}
	result["page"] = s.page
	result["needsClaim"] = ownership != "agent"
	result["completionRequired"] = ownership == "agent"
	result["finishHint"] = "Before final success answer call ego_lite_finish; keep:\"all\" retains result tabs while ending takeover."
	return result, nil
}
func (s *server) refreshSpace() (any, error) {
	value, err := runStandalone("return await listTaskSpaces();", 30*time.Second)
	if err != nil {
		return nil, err
	}
	spaces, ok := value.([]any)
	if !ok {
		return nil, errors.New("invalid task-space list")
	}
	for _, v := range spaces {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		id, _ := m["id"].(float64)
		if (s.spaceID > 0 && int(id) == s.spaceID) || (s.spaceID == 0 && name == s.taskName) {
			ownership, _ := m["ownership"].(string)
			s.ownership = ownership
			result := map[string]any{"spaceId": int(id), "taskSpaceName": name, "ownership": ownership, "needsClaim": ownership != "agent", "status": "resumed", "page": s.page}
			if ownership != "agent" {
				result["status"] = "needs-claim"
				return result, nil
			}
			tabs, err := s.run("return (await task.tabs()).map(t=>({label:t.label,url:t.url,title:t.title,active:t.active,openedBy:t.openedBy}));", 30*time.Second)
			if err != nil {
				return nil, err
			}
			if items, ok := tabs.([]any); ok {
				s.reconcileTabs(items)
				result["tabs"] = items
				result["page"] = s.page
			}
			return result, nil
		}
	}
	return map[string]any{"spaceId": s.spaceID, "ownership": "missing", "needsClaim": false, "status": "missing", "nextAction": "Start a new goal explicitly; do not replay previous writes."}, nil
}
func (s *server) reconcileTabs(tabs []any) {
	labels := []string{}
	active := ""
	for _, v := range tabs {
		if m, ok := v.(map[string]any); ok {
			if label, ok := m["label"].(string); ok && label != "" {
				labels = append(labels, label)
				if m["active"] == true {
					active = label
				}
			}
		}
	}
	exists := false
	for _, label := range labels {
		if label == s.page {
			exists = true
		}
	}
	if !exists {
		previous := s.page
		s.page = active
		if s.page == "" && len(labels) > 0 {
			s.page = labels[0]
		}
		if previous != s.page {
			s.recoveredFrom = previous
		}
	}
	s.lastTabs = labels
}
func (s *server) reconcileResult(m map[string]any) {
	// State-returning actions provide the actual recovered page. Keep it in sync.
	if label, ok := m["page"].(string); ok && label != "" && label != s.page {
		s.recoveredFrom = s.page
		s.page = label
	}
	if tabs, ok := m["__tabs"].([]any); ok {
		exists := false
		labels := []string{}
		for _, v := range tabs {
			if label, ok := v.(string); ok && label != "" {
				labels = append(labels, label)
				if label == s.page {
					exists = true
				}
			}
		}
		if !exists {
			previous := s.page
			s.page = ""
			if len(labels) > 0 {
				s.page = labels[0]
			}
			m["pageRecoveredFrom"] = previous
			m["current"] = s.page
		}
	}
	if s.recoveredFrom != "" {
		m["pageRecoveredFrom"] = s.recoveredFrom
		s.recoveredFrom = ""
	}
}

func runePrefix(text string, limit int) string {
	r := []rune(text)
	if len(r) <= limit {
		return text
	}
	return string(r[:limit])
}
func truncateSnapshot(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	prefix := runePrefix(text, limit)
	if i := strings.LastIndex(prefix, "\n"); i > 0 {
		prefix = prefix[:i]
	}
	return prefix + "\n...[snapshot truncated at line boundary; increase maxChars or use subtree/full_page scope]"
}
func compactValue(value any) any {
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "data:") && strings.Contains(v, ";base64,") && len(v) > 160 {
			return map[string]any{"type": "data-url", "preview": runePrefix(v, 100), "characters": utf8.RuneCountInString(v), "truncated": true}
		}
		if utf8.RuneCountInString(v) > 12000 {
			return map[string]any{"preview": runePrefix(v, 2000), "characters": utf8.RuneCountInString(v), "truncated": true}
		}
		return v
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = compactValue(x)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, x := range v {
			out[k] = compactValue(x)
		}
		return out
	default:
		return value
	}
}
func saveResultArtifact(encoded []byte) (string, error) {
	dir := filepath.Join(os.TempDir(), "zero-ego-results")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "result-*.json")
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.Write(encoded)
	return f.Name(), err
}

func (s *server) domBox(a map[string]string) (any, error) {
	selector, err := validateSelector(a["selector"])
	if err != nil {
		return nil, err
	}
	opts := map[string]any{"selector": selector, "scroll": a["scroll"] != "false", "frame": a["frame"]}
	if a["nth"] != "" {
		n, e := intArg("nth", a["nth"], -1, 100000)
		if e != nil {
			return nil, e
		}
		opts["nth"] = n
	}
	if a["visible"] != "" {
		v, e := strconv.ParseBool(a["visible"])
		if e != nil {
			return nil, e
		}
		opts["visible"] = v
	}
	encoded, _ := json.Marshal(opts)
	return s.run(s.pagePrefix()+"return await __ego.resolve(page,"+string(encoded)+");", 30*time.Second)
}
func (s *server) frameEvaluate(a map[string]string) (any, error) {
	expr := strings.TrimSpace(a["expression"])
	if expr == "" || len(expr) > maxScript {
		return nil, errors.New("expression must contain 1..20000 characters")
	}
	return s.run(s.pagePrefix()+"return await __ego.evaluate(page,"+quote(expr)+","+quote(a["frame"])+");", 90*time.Second)
}
func (s *server) fetchOptions(a map[string]string) (any, error) {
	raw := strings.TrimSpace(a["url"])
	if raw == "" {
		return nil, errors.New("url is required")
	}
	if !strings.HasPrefix(raw, "/") {
		if _, err := validateURL(raw); err != nil {
			return nil, err
		}
	}
	if a["csrf"] != "" {
		return nil, errors.New("csrf is no longer injected; supply the exact request body or headers explicitly")
	}
	opts := map[string]any{"credentials": "include"}
	if a["method"] != "" {
		opts["method"] = strings.ToUpper(a["method"])
	}
	if a["body"] != "" {
		opts["body"] = a["body"]
	}
	if a["credentials"] != "" {
		switch a["credentials"] {
		case "include", "omit", "same-origin":
			opts["credentials"] = a["credentials"]
		default:
			return nil, errors.New("invalid credentials option")
		}
	}
	if a["headers"] != "" {
		var h map[string]string
		if err := json.Unmarshal([]byte(a["headers"]), &h); err != nil {
			return nil, errors.New("headers must be a JSON object of strings")
		}
		opts["headers"] = h
	}
	if a["saveAs"] != "" {
		if !filepath.IsAbs(a["saveAs"]) {
			return nil, errors.New("saveAs must be absolute")
		}
		opts["saveAs"] = filepath.ToSlash(a["saveAs"])
	}
	encoded, _ := json.Marshal(opts)
	return s.run(s.pagePrefix()+"const res=await page.fetch("+quote(raw)+","+string(encoded)+");let parsed;try{parsed=JSON.parse(res.body)}catch{};return {...res,...(parsed===undefined?{}:{json:parsed})};", 120*time.Second)
}
func (s *server) screenshotOptions(a map[string]string) (any, error) {
	save := a["saveAs"]
	if save == "" {
		save = filepath.Join(os.TempDir(), "zero-ego-shots", fmt.Sprintf("shot-%d.png", time.Now().UnixNano()))
	}
	if !filepath.IsAbs(save) {
		return nil, errors.New("saveAs must be an absolute PNG path")
	}
	if a["fullPage"] == "true" && (a["clip"] != "" || a["selector"] != "") {
		return nil, errors.New("fullPage cannot be combined with clip or selector")
	}
	if a["selector"] != "" && a["clip"] != "" {
		return nil, errors.New("selector and clip are mutually exclusive")
	}
	options := map[string]any{"path": filepath.ToSlash(save)}
	if a["fullPage"] == "true" {
		options["fullPage"] = true
	}
	if a["clip"] != "" {
		var clip map[string]float64
		if err := json.Unmarshal([]byte(a["clip"]), &clip); err != nil {
			return nil, errors.New("clip must be JSON {x,y,width,height}")
		}
		if clip["width"] <= 0 || clip["height"] <= 0 {
			return nil, errors.New("clip width/height must be positive")
		}
		options["clip"] = clip
	}
	if a["scale"] != "" {
		scale, e := strconv.ParseFloat(a["scale"], 64)
		if e != nil || scale < 0.1 || scale > 4 {
			return nil, errors.New("scale must be 0.1..4")
		}
		options["scaleFactor"] = scale
	}
	encoded, _ := json.Marshal(options)
	body := s.pagePrefix() + "let options=" + string(encoded) + ";" +
		`if(` + strconv.FormatBool(a["selector"] != "") + `){const box=await __ego.resolve(page,{selector:` + quote(a["selector"]) + `,scroll:true});options.clip=box.rect;}` +
		`let restoredWindow=false;let saved;
 try {const win=await page.cdp('Browser.getWindowForTarget',{targetId:page.targetId});if(win.bounds?.windowState==='minimized'){await page.cdp('Browser.setWindowBounds',{windowId:win.windowId,bounds:{windowState:'normal'}});restoredWindow=true;}}catch{}
 try {if(options.scaleFactor||options.clip){throw new Error('explicit CDP capture')} saved=await page.screenshot(options);}
 catch(error){
  if(/minimized/i.test(error.message)){
   const win=await page.cdp('Browser.getWindowForTarget',{targetId:page.targetId});
   await page.cdp('Browser.setWindowBounds',{windowId:win.windowId,bounds:{windowState:'normal'}});restoredWindow=true;
  }else if(!options.scaleFactor&&!options.clip){throw error;}
  const params={format:'png',captureBeyondViewport:!!options.fullPage||!!options.clip};
  const metrics=await page.cdp('Page.getLayoutMetrics');const viewport=metrics.cssVisualViewport||metrics.visualViewport;
  const dpr=await page.evaluate(()=>window.devicePixelRatio||1);
  if(options.clip){params.clip=__ego.documentClip(options.clip,viewport,options.scaleFactor||1,dpr);}
  else {const r=options.fullPage?(metrics.cssContentSize||metrics.contentSize):viewport;params.clip={x:r.x||r.pageX||0,y:r.y||r.pageY||0,width:r.width||r.clientWidth,height:r.height||r.clientHeight,scale:(options.scaleFactor||1)/dpr};}
  const result=await page.cdp('Page.captureScreenshot',params);
  const fs=await import('node:fs/promises');await fs.mkdir(` + quote(filepath.ToSlash(filepath.Dir(save))) + `,{recursive:true});await fs.writeFile(options.path,Buffer.from(result.data,'base64'));saved=options.path;
 }
 return {path:saved,restoredWindow};`
	return s.run(body, 90*time.Second)
}
func (s *server) localOpen(raw string) (any, error) {
	if !filepath.IsAbs(raw) {
		return nil, errors.New("path must be absolute")
	}
	info, err := os.Stat(raw)
	if err != nil || info.IsDir() {
		return nil, errors.New("local file not found")
	}
	switch strings.ToLower(filepath.Ext(raw)) {
	case ".pdf", ".png", ".jpg", ".jpeg", ".webp", ".txt", ".html", ".htm":
	default:
		return nil, errors.New("supported local files: PDF, images, text, HTML")
	}
	if s.taskName == "" {
		if _, err = s.start(); err != nil {
			return nil, err
		}
	}
	u := fileURL(raw)
	return s.runState(s.pagePrefix()+"await page.goto("+quote(u)+");"+s.stateBody(s.activePage()), 90*time.Second)
}
func fileURL(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// References to helpers keep source archive builds self contained.
