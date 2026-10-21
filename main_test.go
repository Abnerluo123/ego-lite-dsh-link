package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateURL(t *testing.T) {
	good := []string{"https://example.com", "http://localhost:3000/x?a=1", " https://a.b/c "}
	for _, raw := range good {
		if _, err := validateURL(raw); err != nil {
			t.Errorf("validateURL(%q) unexpected error: %v", raw, err)
		}
	}
	bad := []string{"", "file:///etc/passwd", "javascript:alert(1)", "//example.com", "https://user:pass@example.com", "https://"}
	for _, raw := range bad {
		if _, err := validateURL(raw); err == nil {
			t.Errorf("validateURL(%q) expected error", raw)
		}
	}
}

func TestValidateSelectorTextKey(t *testing.T) {
	if _, err := validateSelector("ref=1"); err != nil {
		t.Fatalf("selector rejected: %v", err)
	}
	if _, err := validateSelector("  "); err == nil {
		t.Fatal("blank selector accepted")
	}
	if _, err := validateSelector(strings.Repeat("x", 513)); err == nil {
		t.Fatal("long selector accepted")
	}
	if _, err := validateText(strings.Repeat("x", maxText+1)); err == nil {
		t.Fatal("long text accepted")
	}
	if _, err := validateKey("ControlOrMeta+Enter"); err != nil {
		t.Fatalf("chord rejected: %v", err)
	}
	if _, err := validateKey("Enter;process.exit(1)"); err == nil {
		t.Fatal("code-like key accepted")
	}
}

func TestTaskSourceEscapesTaskName(t *testing.T) {
	script := taskSource("return 1;", `name";process.exit(1);//`)
	if !strings.Contains(script, `taskSpace("name\";process.exit(1);//")`) {
		t.Fatalf("task name not escaped: %s", script)
	}
	if strings.Count(script, "process.exit(1)") != 1 {
		t.Fatalf("unexpected executable text in script: %s", script)
	}
}

func TestActionScriptsDeclarePageOnce(t *testing.T) {
	cases := []string{
		`const page=task.page("p1");await page.click(` + quote("ref=1") + ");" + (&server{}).stateBody("p1"),
		`const page=task.page("p1");await page.fill(` + quote("#q") + "," + quote("x") + ");" + (&server{}).stateBody("p1"),
		`const page=task.page("p1");await page.press(` + quote("#q") + "," + quote("Enter") + ");" + (&server{}).stateBody("p1"),
	}
	for _, body := range cases {
		script := taskSource(body, "space")
		if strings.Count(script, "const page=task.page(") != 1 {
			t.Fatalf("page declared more than once: %s", script)
		}
		if strings.Count(script, `const statePage=(await __ego.pick(task,"p1")).page;`) != 1 {
			t.Fatalf("statePage declaration missing: %s", script)
		}
	}
	if strings.Contains((&server{}).stateBody("p1"), "const page=task.page") {
		t.Fatal("stateBody redeclares page")
	}
}

func TestFindMarkerPrefersParsablePayload(t *testing.T) {
	payload, ok := findMarker("", resultMarker+`{"spaceId":4}`+"\n")
	if !ok || string(payload) != `{"spaceId":4}` {
		t.Fatalf("stderr marker not found: %v %s", ok, payload)
	}
	if _, ok := findMarker("no marker here", ""); ok {
		t.Fatal("false positive marker")
	}
	if _, ok := findMarker(resultMarker+"not-json", ""); ok {
		t.Fatal("invalid JSON accepted as marker")
	}
}

func withFakeRunner(t *testing.T, fn func(string, time.Duration) (json.RawMessage, error)) {
	t.Helper()
	original := runner
	runner = fn
	t.Cleanup(func() { runner = original })
}

func TestStartThenNavigateUsesSameSpace(t *testing.T) {
	var seen []string
	withFakeRunner(t, func(script string, _ time.Duration) (json.RawMessage, error) {
		seen = append(seen, script)
		if strings.Contains(script, "taskSpace(") && strings.Contains(script, "{spaceId") {
			return json.RawMessage(`{"spaceId":7,"name":"space","ownership":"agent","tabs":[{"label":"p1","active":true}]}`), nil
		}
		return json.RawMessage(`{"page":"p1","url":"https://example.com/","title":"Example","snapshot":"root"}`), nil
	})
	s := &server{}
	if _, err := s.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	value, err := s.navigate("https://example.com")
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	result := value.(map[string]any)
	if result["title"] != "Example" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(seen) != 2 || !strings.Contains(seen[1], "taskSpace(7)") {
		t.Fatal("navigate did not reuse the started numeric space id")
	}
}

func TestNavigateRejectsBadURLWithoutRunningCLI(t *testing.T) {
	called := false
	withFakeRunner(t, func(string, time.Duration) (json.RawMessage, error) {
		called = true
		return nil, errors.New("must not run")
	})
	s := &server{taskName: "space"}
	if _, err := s.navigate("file:///secret"); err == nil {
		t.Fatal("bad URL accepted")
	}
	if called {
		t.Fatal("CLI ran for an invalid URL")
	}
}

func TestSnapshotWithoutTask(t *testing.T) {
	s := &server{}
	if _, err := s.snapshot(); err == nil || !strings.Contains(err.Error(), "start an Ego Lite browser task first") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProtocolInitializeAndToolsList(t *testing.T) {
	s := &server{}
	initResp := s.handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28"}}`))
	if initResp == nil || initResp.Error != nil {
		t.Fatalf("initialize failed: %#v", initResp)
	}
	info := initResp.Result.(map[string]any)["serverInfo"].(map[string]any)
	if info["name"] != serverName {
		t.Fatalf("unexpected server name: %v", info["name"])
	}
	listResp := s.handle([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	list := listResp.Result.(map[string]any)["tools"].([]map[string]any)
	if len(list) != 56 {
		t.Fatalf("expected 56 tools, got %d", len(list))
	}
	names := map[string]bool{}
	for _, item := range list {
		names[item["name"].(string)] = true
	}
	for _, want := range []string{
		"ego_lite_run", "ego_lite_screenshot", "ego_lite_tabs", "ego_lite_wait_for",
		"ego_lite_evaluate", "ego_lite_cdp", "ego_lite_download", "ego_lite_upload",
		"ego_lite_new_page", "ego_lite_dialog", "ego_lite_handoff", "ego_lite_keyboard",
		"ego_lite_network_start", "ego_lite_network_stop", "ego_lite_state",
	} {
		if !names[want] {
			t.Fatalf("missing tool %s", want)
		}
	}
}

func TestNotificationHasNoResponse(t *testing.T) {
	s := &server{}
	if resp := s.handle([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); resp != nil {
		t.Fatalf("notification produced a response: %#v", resp)
	}
}

func TestCallRejectsUnknownAndExtraArguments(t *testing.T) {
	s := &server{}
	unknown := s.call("browser_cdp", map[string]string{})
	if unknown["isError"] != true {
		t.Fatal("unknown tool not rejected")
	}
	extra := s.call("ego_lite_navigate", map[string]string{"url": "https://example.com", "command": "whoami"})
	if extra["isError"] != true {
		t.Fatal("extra argument not rejected")
	}
	missing := s.call("ego_lite_fill", map[string]string{"selector": "#q"})
	if missing["isError"] != true {
		t.Fatal("missing argument not rejected")
	}
}

func TestRunScriptWrapsTaskAndPage(t *testing.T) {
	var seen string
	withFakeRunner(t, func(script string, _ time.Duration) (json.RawMessage, error) {
		seen = script
		return json.RawMessage(`{"title":"Example","count":3}`), nil
	})
	s := &server{taskName: "space"}
	value, err := s.runScript("return await page.evaluate(() => document.title);")
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	result := value.(map[string]any)
	if result["title"] != "Example" {
		t.Fatalf("unexpected value: %#v", result)
	}
	if !strings.Contains(seen, `const page=(await __ego.pick(task,"p1")).page;`) || !strings.Contains(seen, "page.evaluate") {
		t.Fatalf("script not wrapped correctly: %s", seen)
	}
}

func TestRunScriptRejectsEmptyAndOversized(t *testing.T) {
	s := &server{taskName: "space"}
	if _, err := s.runScript("   "); err == nil {
		t.Fatal("empty script accepted")
	}
	if _, err := s.runScript(strings.Repeat("x", maxScript+1)); err == nil {
		t.Fatal("oversized script accepted")
	}
}

func TestSwitchPageRejectsUnknownAndAcceptsOpenTab(t *testing.T) {
	withFakeRunner(t, func(script string, _ time.Duration) (json.RawMessage, error) {
		if strings.Contains(script, "statePage") {
			return json.RawMessage(`{"page":"p2","url":"https://search.example/","title":"results","snapshot":"root"}`), nil
		}
		return json.RawMessage(`[{"label":"p1","active":true},{"label":"p2","title":"results"}]`), nil
	})
	s := &server{taskName: "space"}
	if _, err := s.switchPage("p9"); err == nil {
		t.Fatal("unknown page accepted")
	}
	value, err := s.switchPage("p2")
	if err != nil {
		t.Fatalf("switchPage: %v", err)
	}
	if s.activePage() != "p2" {
		t.Fatalf("active page not updated: %s", s.activePage())
	}
	if value.(map[string]any)["page"] != "p2" {
		t.Fatalf("unexpected snapshot: %#v", value)
	}
}

func TestSwitchPageRejectsMalformedLabel(t *testing.T) {
	s := &server{taskName: "space"}
	if _, err := s.switchPage("page2"); err == nil {
		t.Fatal("malformed label accepted")
	}
}

func TestApplyTabsFollowsSingleNewTab(t *testing.T) {
	s := &server{autoSwitch: true, page: "p1", lastTabs: []string{"p1"}}
	result := map[string]any{"__tabs": []any{"p1", "p2"}}
	if got := s.applyTabs(result); got != "p2" {
		t.Fatalf("expected switch to p2, got %q", got)
	}
	if s.page != "p2" {
		t.Fatalf("active page not updated: %s", s.page)
	}
	if _, ok := result["__tabs"]; ok {
		t.Fatal("__tabs leaked into the result")
	}
	if result["autoSwitchedTo"] != "p2" {
		t.Fatalf("missing autoSwitchedTo: %#v", result)
	}
}

func TestApplyTabsIgnoresMultipleNewTabs(t *testing.T) {
	s := &server{autoSwitch: true, page: "p1", lastTabs: []string{"p1"}}
	result := map[string]any{"__tabs": []any{"p1", "p2", "p3"}}
	if got := s.applyTabs(result); got != "" {
		t.Fatalf("should not auto-switch on two new tabs, got %q", got)
	}
	if s.page != "p1" {
		t.Fatalf("active page changed to %s", s.page)
	}
}

func TestApplyTabsRespectsAutoSwitchOff(t *testing.T) {
	s := &server{autoSwitch: false, page: "p1", lastTabs: []string{"p1"}}
	result := map[string]any{"__tabs": []any{"p1", "p2"}}
	if got := s.applyTabs(result); got != "" || s.page != "p1" {
		t.Fatalf("autoSwitch off should not switch: %q %s", got, s.page)
	}
}

func TestBriefModeSkipsSnapshotAndTruncates(t *testing.T) {
	s := &server{brief: true}
	briefBody := s.stateBody("p1")
	if strings.Contains(briefBody, "statePage.snapshot()") {
		t.Fatalf("brief body still snapshots: %s", briefBody)
	}
	if !strings.Contains(briefBody, "brief:true") {
		t.Fatalf("brief marker missing: %s", briefBody)
	}

	full := &server{maxChars: 500}
	truncated := full.state(map[string]any{"snapshot": strings.Repeat("x", 900)})
	text := truncated["snapshot"].(string)
	if len(text) > 650 || !strings.Contains(text, "[snapshot truncated") {
		t.Fatalf("snapshot not truncated: len=%d", len(text))
	}
}

func TestOutputConfigValidates(t *testing.T) {
	s := &server{}
	if _, err := s.outputConfig("maybe", "", ""); err == nil {
		t.Fatal("invalid brief accepted")
	}
	if _, err := s.outputConfig("", "10", ""); err == nil {
		t.Fatal("too-small maxChars accepted")
	}
	value, err := s.outputConfig("true", "800", "false")
	if err != nil {
		t.Fatalf("outputConfig: %v", err)
	}
	cfg := value.(map[string]any)
	if cfg["brief"] != true || cfg["maxChars"] != 800 || cfg["autoSwitch"] != false {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestServeRoundTrip(t *testing.T) {
	s := &server{}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	}, "\n") + "\n"
	var out strings.Builder
	if err := s.serve(strings.NewReader(input), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %q", len(lines), out.String())
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if first["id"].(float64) != 1 {
		t.Fatalf("unexpected first response: %v", first)
	}
}

func TestNonASCIIRoundTrip(t *testing.T) {
	withFakeRunner(t, func(string, time.Duration) (json.RawMessage, error) {
		return json.RawMessage(`{"page":"p1","title":"新标签页","snapshot":"مرحبا"}`), nil
	})
	s := &server{taskName: "space"}
	value, err := s.snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), "新标签页") || !strings.Contains(string(encoded), "مرحبا") {
		t.Fatalf("non-ASCII content lost: %s", encoded)
	}
}
