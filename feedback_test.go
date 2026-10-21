package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestExplicitSnapshotOverridesBriefOnlyForObservation(t *testing.T) {
	var source string
	withFakeRunner(t, func(script string, _ time.Duration) (json.RawMessage, error) {
		source = script
		return json.RawMessage(`{"page":"p1","snapshot":"root\n button @1"}`), nil
	})
	s := &server{brief: true, taskName: "space"}
	value, err := s.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(source, "statePage.snapshot()") || value.(map[string]any)["snapshot"] == nil || !s.brief {
		t.Fatal("explicit snapshot lost tree or changed action defaults")
	}
}
func TestSpacesWorksBeforeStart(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		return json.RawMessage(`{"spaces":[{"id":5,"ownership":"user"}]}`), nil
	})
	if _, err := (&server{}).spaces(); err != nil {
		t.Fatal(err)
	}
}
func TestClaimRejectsIncompleteResponseWithoutStateMutation(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) { return json.RawMessage(`{"spaceId":5}`), nil })
	s := &server{taskName: "old", page: "p2"}
	if _, err := s.claimSpace("5"); err == nil {
		t.Fatal("invalid claim response accepted")
	}
	if s.taskName != "old" || s.page != "p2" {
		t.Fatal("failed claim mutated active state")
	}
}
func TestClaimSelectsExistingPageInsteadOfP1(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		return json.RawMessage(`{"spaceId":5,"name":"course","ownership":"agent","tabs":[{"label":"p7","active":true}]}`), nil
	})
	s := &server{}
	if _, err := s.claimSpace("5"); err != nil {
		t.Fatal(err)
	}
	if s.page != "p7" || s.spaceID != 5 {
		t.Fatalf("wrong claim state: %#v", s)
	}
}
func TestEvaluateDoesNotInspectArrowSubstring(t *testing.T) {
	withFakeRunner(t, func(source string, _ time.Duration) (json.RawMessage, error) {
		if !strings.Contains(source, "__ego.evaluate(page,") {
			t.Fatal("missing expression evaluation helper")
		}
		return json.RawMessage(`null`), nil
	})
	for _, expr := range []string{`new Promise(r=>r(3))`, `[1,2].map(x=>x+1)`, `"=>"`, `async function(){return 1}`, `(()=>4)()`} {
		if _, err := (&server{taskName: "space"}).evaluate(expr); err != nil {
			t.Fatal(err)
		}
	}
}
func TestKeyboardInsertUsesDocumentedMethod(t *testing.T) {
	withFakeRunner(t, func(source string, _ time.Duration) (json.RawMessage, error) {
		if !strings.Contains(source, "page.keyboard.insertText(") {
			t.Fatal("wrong keyboard API")
		}
		return json.RawMessage(`{"page":"p1"}`), nil
	})
	if _, err := (&server{taskName: "space"}).keyboard("insert", "", "测试"); err != nil {
		t.Fatal(err)
	}
}
func TestFetchPreservesBodyAndDoesNotReadCSRF(t *testing.T) {
	withFakeRunner(t, func(source string, _ time.Duration) (json.RawMessage, error) {
		// The embedded helper contains no cookie logic either.
		if strings.Contains(source, "bili_jct") || strings.Contains(source, "document.cookie") {
			t.Fatal("unexpected CSRF injection")
		}
		if !strings.Contains(source, `\"x\":1`) {
			t.Fatal("JSON body missing")
		}
		return json.RawMessage(`{"status":200,"body":"false","json":false}`), nil
	})
	v, err := (&server{taskName: "space"}).fetchOptions(map[string]string{"url": "/api", "method": "POST", "body": `{"x":1}`})
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["json"] != false {
		t.Fatal("false JSON lost")
	}
}
func TestCompactResultRemainsValidJSON(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"data": "data:image/png;base64," + strings.Repeat("A", 300000)})
	})
	s := &server{taskName: "space"}
	result := s.call("ego_lite_evaluate", map[string]string{"expression": "1"})
	text := result["content"].([]map[string]any)[0]["text"].(string)
	if !json.Valid([]byte(text)) || len(text) > maxResult {
		t.Fatal("invalid or unbounded result")
	}
}
func TestTruncateSnapshotKeepsUTF8AndWholeLines(t *testing.T) {
	text := "root\n  text 中文中文\n  button hello\n"
	out := truncateSnapshot(text, 15)
	if !utf8.ValidString(out) || !strings.Contains(out, "line boundary") {
		t.Fatal(out)
	}
}
func TestCLIOutputBoundedAndPreservesSafetyMetadata(t *testing.T) {
	var b cappedBuffer
	b.limit = 30
	b.Write([]byte(strings.Repeat("x", 100)))
	if len(b.data) != 30 || b.discarded != 70 {
		t.Fatal("unbounded buffer")
	}
	msg := cleanCLIError("banner\nError: stopped\n at file:///x\nexecutionStopped:true mayHaveLateEffects:true\nego's nodejs process exited with code 1")
	if !strings.Contains(msg, "Error: stopped") || !strings.Contains(msg, "mayHaveLateEffects") || strings.Contains(msg, "file:///") {
		t.Fatal(msg)
	}
}
func TestFileChooserRejectsMissingTriggerRatherThanChoosingLastInput(t *testing.T) {
	if _, err := (&server{}).fileChooser("", "unused", ""); err == nil {
		t.Fatal("missing trigger allowed")
	}
}
func TestScreenshotRejectsInvalidGeometryAndRelativePath(t *testing.T) {
	s := &server{taskName: "space"}
	for _, args := range []map[string]string{{"saveAs": "relative.png"}, {"clip": `{"x":0,"y":0,"width":0,"height":5}`}, {"scale": "0"}} {
		if _, err := s.screenshotOptions(args); err == nil {
			t.Fatal("invalid screenshot accepted")
		}
	}
}
func TestReconcileClosedActivePage(t *testing.T) {
	s := &server{page: "p2"}
	s.reconcileTabs([]any{map[string]any{"label": "p3", "active": true}})
	if s.page != "p3" {
		t.Fatal(s.page)
	}
}
