package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Export the actual generated handler scripts for SDK-level browser verification
// when this host's confined shell cannot reach Ego IPC directly.
func TestExportSDKScenarios(t *testing.T) {
	raw := os.Getenv("ZERO_EGO_QA_SPACE")
	if raw == "" {
		t.Skip("opt-in browser SDK fixture export")
	}
	id, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.Abs(filepath.Join(".gotmp", "sdk-scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(dir, 0700)
	empty := filepath.Join(dir, "empty.txt")
	os.WriteFile(empty, nil, 0600)
	local := filepath.Join(dir, "local 中文 file.html")
	os.WriteFile(local, []byte("<html><title>Local fixture</title><h1>Local file</h1></html>"), 0600)
	cases := []struct {
		name, tool string
		args       map[string]string
	}{
		{"local", "ego_lite_open_local", map[string]string{"path": local}},
		{"recover", "ego_lite_navigate", map[string]string{"url": "https://example.com"}},
		{"binary-fetch", "ego_lite_fetch", map[string]string{"url": "/robots.txt", "saveAs": filepath.Join(dir, "robots.txt")}},
		{"snapshot", "ego_lite_snapshot", map[string]string{"scope": "full_page"}},
		{"expression", "ego_lite_evaluate", map[string]string{"expression": "new Promise(r=>r([1,2].map(x=>x+1)))"}},
		{"dead", "ego_lite_click", map[string]string{"selector": "#dead"}},
		{"checkbox", "ego_lite_click", map[string]string{"selector": "#check"}},
		{"upload", "ego_lite_upload", map[string]string{"selector": "#file", "paths": empty}},
		{"shadow-upload", "ego_lite_upload", map[string]string{"selector": "#shadowfile", "paths": empty}},
		{"chooser", "ego_lite_filechooser", map[string]string{"trigger": "#choose", "paths": empty}},
		{"insert", "ego_lite_keyboard", map[string]string{"action": "insert", "text": "中文 fixture"}},
		{"frame", "ego_lite_evaluate", map[string]string{"frame": "fixture", "expression": "window.appState"}},
		{"clip", "ego_lite_screenshot", map[string]string{"clip": `{"x":0,"y":0,"width":200,"height":100}`, "scale": "0.5", "saveAs": filepath.Join(dir, "clip.png")}},
		{"selector", "ego_lite_screenshot", map[string]string{"selector": "#live", "saveAs": filepath.Join(dir, "selector.png")}},
		{"full", "ego_lite_screenshot", map[string]string{"fullPage": "true", "saveAs": filepath.Join(dir, "full.png")}},
		{"restore", "ego_lite_screenshot", map[string]string{"saveAs": filepath.Join(dir, "restored.png")}},
		{"dialog", "ego_lite_dialog", map[string]string{"trigger": "#dialog", "action": "accept", "text": "accepted", "timeoutMs": "3000"}},
		{"existing-dialog", "ego_lite_dialog", map[string]string{"action": "dismiss", "timeoutMs": "3000"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withFakeRunner(t, func(source string, _ time.Duration) (json.RawMessage, error) {
				i := strings.LastIndex(source, "console.log(")
				if i < 0 {
					t.Fatal("missing result boundary")
				}
				source = source[:i] + "return value;"
				if err := os.WriteFile(filepath.Join(dir, c.name+".mjs"), []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				return json.RawMessage(`{"page":"p1","snapshot":"root"}`), nil
			})
			s := &server{taskName: "qa", spaceID: id, page: "p1", brief: true, autoSwitch: true}
			if c.name == "recover" {
				s.page = "p999"
			}
			for _, tool := range tools {
				if tool.name == c.tool {
					if _, err := tool.handler(s, c.args); err != nil {
						t.Fatal(err)
					}
					return
				}
			}
			t.Fatal("tool missing")
		})
	}
}
