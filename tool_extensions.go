package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func init() {
	for i := range tools {
		t := &tools[i]
		t.description = strings.ReplaceAll(t.description, "then return a snapshot", "then return compact page state (snapshot is explicit)")
		switch t.name {
		case "ego_lite_snapshot":
			t.properties = map[string]any{"scope": map[string]any{"type": "string", "enum": []string{"viewport", "full_page", "subtree"}}, "root": map[string]any{"type": "string"}, "maxChars": map[string]any{"type": "integer", "minimum": 200, "maximum": 200000}}
			t.description = "Explicitly read the accessibility tree even in brief mode. Supports viewport/full_page/subtree scope, root ref and Unicode-safe maxChars at complete lines."
			t.handler = func(s *server, a map[string]string) (any, error) { return s.snapshotOptions(a) }
		case "ego_lite_start":
			t.properties["spaceId"] = map[string]any{"type": "integer", "minimum": 1}
			t.handler = func(s *server, a map[string]string) (any, error) {
				if a["spaceId"] != "" {
					id, e := intArg("spaceId", a["spaceId"], 1, 1<<31-1)
					if e != nil {
						return nil, e
					}
					value, e := runStandalone("const t=await taskSpace("+strconv.Itoa(id)+");return {spaceId:t.spaceId,name:t.name,ownership:t.ownership,tabs:(await t.tabs()).map(t=>({label:t.label,url:t.url,title:t.title,active:t.active}))};", 90*time.Second)
					if e != nil {
						return nil, e
					}
					return s.acceptSpace(value)
				}
				return s.start()
			}
		case "ego_lite_finish":
			t.description = "REQUIRED final browser step on successful completion: end the task and release agent takeover. Default closes agent pages; keep:\"all\" retains results. Check agentControlReleased; false/null is not verified release. Optional spaceId addresses one known agent-owned space after restart, never all spaces."
			t.properties["spaceId"] = map[string]any{"type": "integer", "minimum": 1}
			t.properties["keep"] = map[string]any{"type": "string", "maxLength": 1024, "description": "Empty/default closes agent pages; all keeps every managed result page; or comma-separated labels."}
			t.handler = func(s *server, a map[string]string) (any, error) { return s.completeSpace(a["spaceId"], a["keep"]) }
		case "ego_lite_close_page":
			t.description += " Closing pages does not finish the task or exit agent mode; call ego_lite_finish at completion."
		case "ego_lite_handoff":
			t.description = "Temporarily return the current space to the user without closing pages. Use for user action or an interrupted task, not successful completion; then end the agent turn."
		case "ego_lite_screenshot":
			t.properties = map[string]any{
				"selector": map[string]any{"type": "string", "maxLength": 512},
				"clip":     map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "number"}, "y": map[string]any{"type": "number"}, "width": map[string]any{"type": "number"}, "height": map[string]any{"type": "number"}}, "required": []string{"x", "y", "width", "height"}},
				"scale":    map[string]any{"type": "number", "minimum": 0.1, "maximum": 4}, "fullPage": map[string]any{"type": "boolean"}, "saveAs": map[string]any{"type": "string", "maxLength": 4096},
			}
			t.description = "Save a PNG: viewport, selector crop, clip {x,y,width,height}, numeric scale, fullPage and saveAs. Restores a minimized window before retrying. Returns a path, never base64."
			t.handler = func(s *server, a map[string]string) (any, error) { return s.screenshotOptions(a) }
		case "ego_lite_evaluate":
			t.properties["frame"] = map[string]any{"type": "string"}
			t.description = "Evaluate an expression once; if its value is a function, invoke it once. Awaits Promises. Optional frame is a frame id/name from frames; out-of-process frames may require native snapshot refs."
			t.handler = func(s *server, a map[string]string) (any, error) { return s.frameEvaluate(a) }
		case "ego_lite_fetch":
			t.properties["saveAs"] = map[string]any{"type": "string", "maxLength": 4096}
			delete(t.properties, "csrf")
			t.description = "Use the documented page.fetch transparently, with credentials include by default. Relative URLs and binary saveAs supported; no automatic CSRF/body mutation."
			t.handler = func(s *server, a map[string]string) (any, error) { return s.fetchOptions(a) }
		case "ego_lite_box":
			t.properties["frame"] = map[string]any{"type": "string"}
			t.properties["nth"] = map[string]any{"type": "integer", "minimum": -1}
			t.properties["visible"] = map[string]any{"type": "boolean"}
			t.properties["scroll"] = map[string]any{"type": "boolean"}
			t.description = "Measure one CSS/text/href/XPath target after scrolling. Ambiguity fails; nth is zero-based (-1 last). Frame rects are local to that frame. Snapshot refs must use native actions."
			t.handler = func(s *server, a map[string]string) (any, error) { return s.domBox(a) }
		case "ego_lite_dialog":
			t.description = "Handle a JS dialog. With trigger, capture its receipt/event and accept/dismiss; without trigger, invoke the native handler directly without querying the modal page."
		case "ego_lite_run":
			t.description += " Uses ESM: await import('node:fs/promises'), not require(). Node code has host permissions; use only authorized absolute paths."
		case "ego_lite_filechooser":
			t.description = "Arm the official page.waitForFileChooser, click trigger, then setFiles in one call. Trigger is required; existing inputs (including hidden) use upload."
			t.required = []string{"trigger", "paths"}
		case "ego_lite_wait_function":
			t.properties["timeoutMs"].(map[string]any)["maximum"] = 300000
			t.description = "Wait until a page predicate is truthy, then return matched:true (no snapshot)."
		case "ego_lite_network_start", "ego_lite_network_stop":
			t.description = "Legacy split capture is not reliable across independent CLI processes. Use ego_lite_network_capture for enable/action/drain in one call."
		}
		// Keep Ego's canonical resolver for ordinary operations. Only explicit DOM
		// filters/frame selection use the shared resolver and a unique temporary tag.
		switch t.name {
		case "ego_lite_click", "ego_lite_fill", "ego_lite_press", "ego_lite_hover", "ego_lite_focus", "ego_lite_dblclick", "ego_lite_select", "ego_lite_upload":
			t.properties["frame"] = map[string]any{"type": "string"}
			t.properties["nth"] = map[string]any{"type": "integer", "minimum": -1}
			t.properties["visible"] = map[string]any{"type": "boolean"}
			original := t.handler
			t.handler = func(s *server, a map[string]string) (any, error) {
				copyArgs := make(map[string]string, len(a))
				for k, v := range a {
					copyArgs[k] = v
				}
				if a["frame"] != "" || a["nth"] != "" || a["visible"] != "" {
					opts := map[string]any{"selector": a["selector"], "frame": a["frame"], "tag": fmt.Sprintf("t%d", time.Now().UnixNano())}
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
					if _, e := s.run(s.pagePrefix()+"return await __ego.resolve(page,"+string(encoded)+");", 30*time.Second); e != nil {
						return nil, e
					}
					copyArgs["selector"] = "[data-zero-ego-target=\"" + opts["tag"].(string) + "\"]"
				}
				return original(s, copyArgs)
			}
		}
	}
	tools = append(tools,
		tool{name: "ego_lite_control_status", description: "Check whether a task space still has agent ownership; list remaining agent spaces without claiming, closing or creating one. spaceId is optional and accepts an id or exact name; omit it for the current (or last finished) space. Ids are never reused, so never guess one. This is registry status, not a global browser exit guarantee.", properties: map[string]any{"spaceId": map[string]any{"type": []string{"integer", "string"}, "description": "space id or exact space name"}}, handler: func(s *server, a map[string]string) (any, error) { return s.controlStatus(a["spaceId"]) }},
		tool{name: "ego_lite_frames", description: "List frame ids, names, URLs and parents. Use ids for frame evaluate/filtering; native snapshot refs remain preferred for actions in out-of-process frames.", properties: map[string]any{}, handler: func(s *server, _ map[string]string) (any, error) {
			return s.run(s.pagePrefix()+"return await __ego.frames(page);", 30*time.Second)
		}},
		tool{name: "ego_lite_open_local", description: "Open an explicitly requested absolute local PDF, image, text or HTML path. This is deliberate file access; normal navigate remains HTTP(S).", properties: map[string]any{"path": map[string]any{"type": "string", "maxLength": 4096}}, required: []string{"path"}, handler: func(s *server, a map[string]string) (any, error) { return s.localOpen(a["path"]) }},
		tool{name: "ego_lite_do", description: "Run a bounded sequential macro of existing tools. Stops at the first failure and returns completed steps plus failed index; never replays earlier steps automatically.", properties: map[string]any{"steps": map[string]any{"type": "array", "maxItems": 30, "items": map[string]any{"type": "object", "properties": map[string]any{"tool": map[string]any{"type": "string"}, "arguments": map[string]any{"type": "object"}}, "required": []string{"tool"}}}}, required: []string{"steps"}, handler: func(s *server, a map[string]string) (any, error) { return s.macro(a["steps"]) }})
}
func (s *server) macro(raw string) (any, error) {
	var steps []struct {
		Tool      string                     `json:"tool"`
		Arguments map[string]json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(raw), &steps); err != nil || len(steps) == 0 || len(steps) > 30 {
		return nil, errors.New("steps must contain 1..30 actions")
	}
	results := []any{}
	for i, step := range steps {
		if step.Tool == "ego_lite_do" || step.Tool == "ego_lite_run" {
			return map[string]any{"completed": results, "failedIndex": i, "error": "nested macros and raw scripts are not supported; use run directly"}, nil
		}
		var selected *tool
		for j := range tools {
			if tools[j].name == step.Tool {
				selected = &tools[j]
				break
			}
		}
		if selected == nil {
			return map[string]any{"completed": results, "failedIndex": i, "error": "unknown tool"}, nil
		}
		args, err := stringifyArguments(step.Arguments)
		if err == nil {
			for k := range args {
				if _, ok := selected.properties[k]; !ok {
					err = errors.New("unknown argument: " + k)
				}
			}
			for _, k := range selected.required {
				if _, ok := args[k]; !ok {
					err = errors.New("missing argument: " + k)
				}
			}
		}
		var result any
		if err == nil {
			result, err = selected.handler(s, args)
		}
		if err != nil {
			return map[string]any{"completed": results, "failedIndex": i, "tool": step.Tool, "error": err.Error(), "replaySafe": false}, nil
		}
		results = append(results, map[string]any{"tool": step.Tool, "result": result})
	}
	return map[string]any{"completed": results, "count": len(results)}, nil
}
