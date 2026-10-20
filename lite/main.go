// ego-dsh lite: a deliberately small Ego Lite-only MCP connector.
//
// Differences from the full build:
//   - 12 tools instead of 49; no Edge/CDP generalism, no downloads/uploads.
//   - Actions return {page,url,title} instead of an accessibility tree, so a
//     typical action costs a handful of tokens instead of thousands.
//   - ego_lite_run remains as the escape hatch for anything not covered.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	serverName = "ego-lite-lite"
	version    = "1.0.0"
	marker     = "__EGO_DSH_RESULT__"
	maxLine    = 1_000_000
)

var (
	mu      sync.Mutex
	taskName string
	page    = "p1"
)

func q(s string) string { b, _ := json.Marshal(s); return string(b) }

func cliPath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("EGO_BROWSER_CLI")); configured != "" {
		return configured, nil
	}
	resolved, err := exec.LookPath("ego-browser")
	if err != nil {
		return "", errors.New("ego-browser not found; set EGO_BROWSER_CLI")
	}
	return resolved, nil
}

func invoke(source string, timeout time.Duration) (json.RawMessage, error) {
	cli, err := cliPath()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, "nodejs")
	cmd.Stdin = strings.NewReader(source)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	_ = cmd.Run()
	for _, stream := range []string{out.String(), errb.String()} {
		lines := strings.Split(stream, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimRight(lines[i], "\r")
			if strings.HasPrefix(line, marker) {
				payload := []byte(strings.TrimSpace(strings.TrimPrefix(line, marker)))
				if json.Valid(payload) {
					return payload, nil
				}
			}
		}
	}
	detail := strings.TrimSpace(errb.String())
	if detail == "" {
		detail = strings.TrimSpace(out.String())
	}
	if detail == "" {
		detail = "ego-browser returned no result"
	}
	return nil, errors.New(detail)
}

func run(body string, timeout time.Duration) (any, error) {
	if taskName == "" {
		if _, err := start(); err != nil {
			return nil, err
		}
	}
	source := "const task=await taskSpace(" + q(taskName) + ");" +
		"const value=await (async()=>{" + body + "})();" +
		"console.log(" + q(marker) + "+(JSON.stringify(value)??'null'));"
	payload, err := invoke(source, timeout)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func prefix() string { return "const page=task.page(" + q(page) + ");" }

func state() string {
	return "return {page:page.label,url:await page.url(),title:await page.title()};"
}

func start() (any, error) {
	if taskName != "" {
		return map[string]any{"page": page, "status": "already-started"}, nil
	}
	name := fmt.Sprintf("dsh-lite-%d", time.Now().UnixNano())
	source := "const task=await taskSpace(" + q(name) + ");" +
		"console.log(" + q(marker) + "+JSON.stringify({spaceId:task.spaceId}));"
	if _, err := invoke(source, 90*time.Second); err != nil {
		return nil, err
	}
	taskName, page = name, "p1"
	return map[string]any{"page": "p1", "status": "started"}, nil
}

func action(body string, timeout time.Duration) (any, error) {
	return run(prefix()+body+state(), timeout)
}

// ---------- tools ----------

type tool struct {
	name, desc string
	props      map[string]any
	required   []string
	fn         func(map[string]string) (any, error)
}

var tools = []tool{
	{"open", "Open an http(s) URL in the managed page. Starts a task space if needed.",
		map[string]any{"url": map[string]any{"type": "string"}}, []string{"url"},
		func(a map[string]string) (any, error) {
			url := strings.TrimSpace(a["url"])
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				return nil, errors.New("only http(s) URLs are allowed")
			}
			return action("await page.goto("+q(url)+");await page.waitForLoadState();", 120*time.Second)
		}},
	{"snapshot", "Return the accessibility snapshot (full=false gives just page/url/title).",
		map[string]any{"full": map[string]any{"type": "string", "enum": []string{"true", "false"}}}, nil,
		func(a map[string]string) (any, error) {
			if a["full"] != "true" {
				return action("", 60*time.Second)
			}
			return run(prefix()+"const s=await page.snapshot();"+
				"return {page:page.label,url:await page.url(),title:await page.title(),snapshot:s};", 90*time.Second)
		}},
	{"click", "Click one ref or selector.", map[string]any{"selector": map[string]any{"type": "string"}}, []string{"selector"},
		func(a map[string]string) (any, error) { return action("await page.click("+q(a["selector"])+");", 90*time.Second) }},
	{"fill", "Fill an input.", map[string]any{"selector": map[string]any{"type": "string"}, "text": map[string]any{"type": "string"}}, []string{"selector", "text"},
		func(a map[string]string) (any, error) {
			return action("await page.fill("+q(a["selector"])+","+q(a["text"])+");", 90*time.Second)
		}},
	{"press", "Press a key on one selector.", map[string]any{"selector": map[string]any{"type": "string"}, "key": map[string]any{"type": "string"}}, []string{"selector", "key"},
		func(a map[string]string) (any, error) {
			return action("await page.press("+q(a["selector"])+","+q(a["key"])+");", 90*time.Second)
		}},
	{"eval", "Evaluate JS in the page and return the JSON result.",
		map[string]any{"expression": map[string]any{"type": "string"}}, []string{"expression"},
		func(a map[string]string) (any, error) {
			expr := a["expression"]
			if strings.Contains(expr, "=>") || strings.HasPrefix(strings.TrimSpace(expr), "function") {
				expr = "(" + expr + ")()"
			}
			return run(prefix()+"return await page.evaluate("+q(expr)+");", 90*time.Second)
		}},
	{"wait", "Wait for a selector to be visible.", map[string]any{"selector": map[string]any{"type": "string"}, "timeoutMs": map[string]any{"type": "integer"}}, []string{"selector"},
		func(a map[string]string) (any, error) {
			timeout := 15000
			if a["timeoutMs"] != "" {
				parsed, err := strconv.Atoi(a["timeoutMs"])
				if err != nil || parsed < 100 || parsed > 120000 {
					return nil, errors.New("timeoutMs must be 100..120000")
				}
				timeout = parsed
			}
			return action("await page.waitForSelector("+q(a["selector"])+",{state:'visible',timeout:"+strconv.Itoa(timeout)+"});", time.Duration(timeout)*time.Millisecond+60*time.Second)
		}},
	{"scroll", "Scroll the wheel by dy pixels (negative scrolls up).", map[string]any{"dy": map[string]any{"type": "integer"}, "x": map[string]any{"type": "integer"}, "y": map[string]any{"type": "integer"}}, []string{"dy"},
		func(a map[string]string) (any, error) {
			dy, err := strconv.Atoi(a["dy"])
			if err != nil || dy < -20000 || dy > 20000 {
				return nil, errors.New("dy must be -20000..20000")
			}
			x, y := 500, 500
			if a["x"] != "" { x, _ = strconv.Atoi(a["x"]) }
			if a["y"] != "" { y, _ = strconv.Atoi(a["y"]) }
			return action("await page.mouse.move("+strconv.Itoa(x)+","+strconv.Itoa(y)+");await page.mouse.wheel(0,"+strconv.Itoa(dy)+");", 60*time.Second)
		}},
	{"upload", "Set files on a file input; paths are absolute, one per line.", map[string]any{"selector": map[string]any{"type": "string"}, "paths": map[string]any{"type": "string"}}, []string{"selector", "paths"},
		func(a map[string]string) (any, error) {
			lines := strings.Split(a["paths"], "\n")
			files := make([]string, 0, len(lines))
			for _, line := range lines {
				item := strings.TrimSpace(line)
				if item == "" { continue }
				abs, err := filepath.Abs(item)
				if err != nil { return nil, err }
				if _, err := os.Stat(abs); err != nil { return nil, errors.New("file not found: " + abs) }
				files = append(files, filepath.ToSlash(abs))
			}
			if len(files) == 0 { return nil, errors.New("paths must contain at least one file") }
			encoded, _ := json.Marshal(files)
			return action("await page.setInputFiles("+q(a["selector"])+","+string(encoded)+");", 120*time.Second)
		}},
	{"dialog", "Accept or dismiss the open JS dialog.", map[string]any{"action": map[string]any{"type": "string"}, "text": map[string]any{"type": "string"}}, []string{"action"},
		func(a map[string]string) (any, error) {
			if a["action"] == "accept" {
				return action("await page.acceptDialog("+q(a["text"])+");", 60*time.Second)
			}
			if a["action"] == "dismiss" {
				return action("await page.dismissDialog();", 60*time.Second)
			}
			return nil, errors.New("action must be accept or dismiss")
		}},
	{"download", "Click a selector that starts a download and save it to saveAs (absolute path).", map[string]any{"selector": map[string]any{"type": "string"}, "saveAs": map[string]any{"type": "string"}}, []string{"selector", "saveAs"},
		func(a map[string]string) (any, error) {
			path := strings.TrimSpace(a["saveAs"])
			if path == "" { return nil, errors.New("saveAs is required") }
			abs, err := filepath.Abs(path)
			if err != nil { return nil, err }
			body := "const dlPromise=page.waitForEvent('download',{timeout:60000});" +
				"await page.click(" + q(a["selector"]) + ");" +
				"const dl=await dlPromise;await dl.saveAs(" + q(filepath.ToSlash(abs)) + ");return {path:" + q(filepath.ToSlash(abs)) + ",filename:dl.suggestedFilename()};"
			return run(prefix()+body, 120*time.Second)
		}},
	{"tabs", "List tabs in the task space.", map[string]any{}, nil,
		func(map[string]string) (any, error) {
			return run("const tabs=await task.tabs();return tabs.map(t=>({label:t.label,title:t.title,url:t.url,active:t.active}));", 60*time.Second)
		}},
	{"switch", "Point the tools at another page label.", map[string]any{"page": map[string]any{"type": "string"}}, []string{"page"},
		func(a map[string]string) (any, error) {
			target := strings.TrimSpace(a["page"])
			value, err := run("const tabs=await task.tabs();return tabs.map(t=>t.label);", 60*time.Second)
			if err != nil {
				return nil, err
			}
			labels, _ := value.([]any)
			for _, label := range labels {
				if label == target {
					page = target
					return action("", 60*time.Second)
				}
			}
			return nil, errors.New("page " + target + " is not open")
		}},
	{"screenshot", "Capture a PNG of the page and return its path.", map[string]any{}, nil,
		func(map[string]string) (any, error) {
			dir := os.TempDir()
			path := dir + string(os.PathSeparator) + "ego-lite-" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".png"
			body := prefix() + "const saved=await page.screenshot({path:" + q(strings.ReplaceAll(path, "\\", "/")) + "});return {path:saved};"
			return run(body, 90*time.Second)
		}},
	{"run", "Run a script against the task space with the full Ego API; page and task are in scope.",
		map[string]any{"script": map[string]any{"type": "string"}}, []string{"script"},
		func(a map[string]string) (any, error) {
			return run(prefix()+a["script"], 180*time.Second)
		}},
	{"finish", "Finish the task space.", map[string]any{}, nil,
		func(map[string]string) (any, error) {
			if taskName == "" {
				return map[string]any{"status": "not-started"}, nil
			}
			source := "const task=await taskSpace(" + q(taskName) + ");" +
				"const spaceId=task.spaceId;await task.finish({keep:[]});" +
				"console.log(" + q(marker) + "+JSON.stringify({spaceId,status:'finished'}));"
			payload, err := invoke(source, 90*time.Second)
			if err != nil {
				return nil, err
			}
			taskName = ""
			var value any
			_ = json.Unmarshal(payload, &value)
			return value, nil
		}},
}

// ---------- protocol ----------

type req struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func failure(err error) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}
}

func handle(raw []byte) map[string]any {
	var r req
	if err := json.Unmarshal(raw, &r); err != nil || r.JSONRPC != "2.0" || r.Method == "" {
		return map[string]any{"jsonrpc": "2.0", "id": r.ID, "error": map[string]any{"code": -32600, "message": "Invalid Request"}}
	}
	if len(r.ID) == 0 {
		return nil
	}
	reply := func(result any) map[string]any { return map[string]any{"jsonrpc": "2.0", "id": r.ID, "result": result} }
	switch r.Method {
	case "initialize":
		return reply(map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": serverName, "version": version},
			"instructions":    "Small Ego Lite connector: 12 tools, actions return page/url/title. Use run for anything else.",
		})
	case "ping":
		return reply(map[string]any{})
	case "tools/list":
		list := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			schema := map[string]any{"type": "object", "properties": t.props, "additionalProperties": false}
			if len(t.required) > 0 {
				schema["required"] = t.required
			}
			list = append(list, map[string]any{"name": t.name, "description": t.desc, "inputSchema": schema})
		}
		return reply(map[string]any{"tools": list})
	case "tools/call":
		var p struct {
			Name      string                     `json:"name"`
			Arguments map[string]json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(r.Params, &p); err != nil {
			return map[string]any{"jsonrpc": "2.0", "id": r.ID, "error": map[string]any{"code": -32602, "message": "Invalid params"}}
		}
		args := map[string]string{}
		for k, v := range p.Arguments {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				s = strings.Trim(string(v), `"`)
			}
			args[k] = s
		}
		var selected *tool
		for i := range tools {
			if tools[i].name == p.Name {
				selected = &tools[i]
			}
		}
		if selected == nil {
			return reply(failure(errors.New("unknown tool " + p.Name)))
		}
		mu.Lock()
		value, err := selected.fn(args)
		mu.Unlock()
		if err != nil {
			return reply(failure(err))
		}
		text, _ := json.Marshal(value)
		return reply(map[string]any{"content": []map[string]any{{"type": "text", "text": string(text)}}, "isError": false})
	}
	return map[string]any{"jsonrpc": "2.0", "id": r.ID, "error": map[string]any{"code": -32601, "message": "Method not found"}}
}

func main() {
	reader := bufio.NewReaderSize(os.Stdin, 64*1024)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for {
		line, err := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			if resp := handle(bytes.TrimSpace(line)); resp != nil {
				data, _ := json.Marshal(resp)
				out.Write(data)
				out.WriteByte('\n')
				out.Flush()
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}
	}
	if taskName != "" {
		source := "const task=await taskSpace(" + q(taskName) + ");await task.finish({keep:[]});"
		_, _ = invoke(source, 30*time.Second)
	}
}
