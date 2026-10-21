package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartRefreshDoesNotInventAgentOwnership(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		return json.RawMessage(`[{"id":5,"name":"course","ownership":"user"}]`), nil
	})
	s := &server{taskName: "course", spaceID: 5}
	v, e := s.start()
	if e != nil {
		t.Fatal(e)
	}
	m := v.(map[string]any)
	if m["needsClaim"] != true || m["ownership"] != "user" {
		t.Fatal(m)
	}
}
func TestMacroStopsAtFailureWithoutReplay(t *testing.T) {
	calls := 0
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		calls++
		return json.RawMessage(`{"page":"p1","title":"ok"}`), nil
	})
	s := &server{taskName: "space"}
	v, e := s.macro(`[{"tool":"ego_lite_evaluate","arguments":{"expression":"1"}},{"tool":"missing"},{"tool":"ego_lite_evaluate","arguments":{"expression":"2"}}]`)
	if e != nil {
		t.Fatal(e)
	}
	if calls != 1 || v.(map[string]any)["failedIndex"] != 1 {
		t.Fatal(v, calls)
	}
}
func TestOversizedNonStringResultIsArtifactEnvelope(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"items": strings.Repeat("x", maxResult+20)})
	})
	s := &server{taskName: "space"}
	v := s.call("ego_lite_cdp", map[string]string{"method": "Runtime.evaluate"})
	text := v["content"].([]map[string]any)[0]["text"].(string)
	var envelope map[string]any
	if json.Unmarshal([]byte(text), &envelope) != nil || envelope["truncated"] != true {
		t.Fatal("invalid envelope")
	}
	if path, ok := envelope["artifact"].(string); ok {
		if _, e := os.Stat(path); e != nil {
			t.Fatal(e)
		}
	}
}
func TestLocalFileValidationAndEscaping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file # 中文.html")
	os.WriteFile(path, []byte("ok"), 0600)
	u := fileURL(path)
	if !strings.Contains(u, "%23") || strings.Contains(u, " ") {
		t.Fatal(u)
	}
	if _, e := (&server{}).localOpen("relative.pdf"); e == nil {
		t.Fatal("relative path accepted")
	}
}
func TestFrameAndScreenshotSchemasAcceptObjectArguments(t *testing.T) {
	a, e := stringifyArguments(map[string]json.RawMessage{"clip": json.RawMessage(`{"x":0,"y":0,"width":20,"height":10}`), "steps": json.RawMessage(`[]`)})
	if e != nil || !json.Valid([]byte(a["clip"])) {
		t.Fatal(a, e)
	}
}
