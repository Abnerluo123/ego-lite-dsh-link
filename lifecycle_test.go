package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Agents used to guess space ids, which produced "task space not found: 1" and
// retries. A wrong id must name the ids that actually exist.
func TestResolveSpaceRejectsUnknownIdWithHint(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		return json.RawMessage(`[{"id":12,"name":"dsh-ego-12","ownership":"agent"}]`), nil
	})
	s := &server{spaceID: 12, taskName: "dsh-ego-12"}
	if _, err := s.resolveSpace("1"); err == nil ||
		!strings.Contains(err.Error(), "space 1 does not exist") ||
		!strings.Contains(err.Error(), "current space is 12") ||
		!strings.Contains(err.Error(), "ego_lite_spaces") {
		t.Fatalf("unexpected error: %v", err)
	}
	if id, err := s.resolveSpace("dsh-ego-12"); err != nil || id != 12 {
		t.Fatalf("name lookup failed: id=%d err=%v", id, err)
	}
	if id, err := s.resolveSpace(""); err != nil || id != 12 {
		t.Fatalf("empty id should resolve to the current space: id=%d err=%v", id, err)
	}
	if _, err := s.resolveSpace("nope"); err == nil || !strings.Contains(err.Error(), "no space is named") {
		t.Fatalf("unknown name should fail clearly: %v", err)
	}
}

// When the registry cannot be read, a numeric id is still used: the finish
// receipt validates it, and blocking here would break a working finish.
func TestResolveSpaceFallsBackWhenRegistryUnavailable(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		return nil, errors.New("registry down")
	})
	s := &server{}
	if id, err := s.resolveSpace("9"); err != nil || id != 9 {
		t.Fatalf("expected fallback to 9, got id=%d err=%v", id, err)
	}
}

func TestFinishKeepAllVerifiesThenClearsState(t *testing.T) {
	var source string
	withFakeRunner(t, func(script string, _ time.Duration) (json.RawMessage, error) {
		source = script
		return json.RawMessage(`{"spaceId":7,"status":"finished","kept":["p1"],"agentControlReleased":true,"verification":"space-registry"}`), nil
	})
	s := &server{spaceID: 7, taskName: "qa", page: "p1", ownership: "agent"}
	v, e := s.finishWith("all")
	if e != nil {
		t.Fatal(e)
	}
	if v.(map[string]any)["agentControlReleased"] != true || s.taskName != "" || s.spaceID != 0 {
		t.Fatal(v, s)
	}
	if !strings.Contains(source, "requested=\"all\"") || !strings.Contains(source, "await task.finish({keep:keep})") || !strings.Contains(source, "after=await listTaskSpaces()") {
		t.Fatal("missing keep-all or verification")
	}
	finish := strings.Index(source, "await task.finish")
	tail := source[finish:]
	if strings.Contains(tail, "task.page(") {
		t.Fatal("Page operation after finish")
	}
}
func TestFinishRepeatedDoesNotDispatchAgain(t *testing.T) {
	n := 0
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		n++
		return json.RawMessage(`{"spaceId":7,"status":"finished","agentControlReleased":true}`), nil
	})
	s := &server{spaceID: 7, taskName: "qa"}
	s.finish()
	s.finish()
	if n != 1 {
		t.Fatal("finish replayed", n)
	}
}
func TestFinishErrorRetainsRecoveryIdentity(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) { return nil, errors.New("transport failure") })
	s := &server{spaceID: 7, taskName: "qa", page: "p1"}
	if _, e := s.finish(); e == nil {
		t.Fatal("failure hidden")
	}
	if s.spaceID != 7 || s.page != "p1" {
		t.Fatal("identity lost")
	}
}
func TestFinishInvalidReceiptDoesNotClaimSuccess(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		return json.RawMessage(`{"spaceId":8,"status":"finished"}`), nil
	})
	s := &server{spaceID: 7, taskName: "qa"}
	if _, e := s.finish(); e == nil {
		t.Fatal("wrong space accepted")
	}
	if s.spaceID != 7 {
		t.Fatal("identity mutated")
	}
}
func TestFinishUnknownVerificationDropsInvalidPageHandles(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		return json.RawMessage(`{"spaceId":7,"status":"finished","agentControlReleased":null,"verification":"unavailable"}`), nil
	})
	s := &server{spaceID: 7, taskName: "qa", page: "p1"}
	v, e := s.finish()
	if e != nil {
		t.Fatal(e)
	}
	if s.page != "" || s.taskName != "" || v.(map[string]any)["agentControlReleased"] != nil {
		t.Fatal("invalid handles or invented release", v)
	}
}
func TestFinishExplicitOtherSpacePreservesActiveTask(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		return json.RawMessage(`{"spaceId":8,"status":"finished","agentControlReleased":true}`), nil
	})
	s := &server{spaceID: 7, taskName: "qa", page: "p2"}
	if _, e := s.completeSpace("8", ""); e != nil {
		t.Fatal(e)
	}
	if s.spaceID != 7 || s.page != "p2" {
		t.Fatal("unrelated local state cleared")
	}
}
func TestFinishNoTaskDoesNotInventObservedRelease(t *testing.T) {
	v, e := (&server{}).finish()
	if e != nil || v.(map[string]any)["agentControlReleased"] != nil {
		t.Fatal(v, e)
	}
}
func TestKeepParserRejectsInvalidBeforeDispatch(t *testing.T) {
	for _, raw := range []string{"all,p1", "p1;bad", "p12345"} {
		if _, e := parseKeep(raw); e == nil {
			t.Fatal(raw)
		}
	}
	v, e := parseKeep("p1,p1 p2")
	if e != nil || v != `["p1","p2"]` {
		t.Fatal(v, e)
	}
}
func TestControlStatusWorksAfterFinishWithoutStartingTask(t *testing.T) {
	withFakeRunner(t, func(script string, _ time.Duration) (json.RawMessage, error) {
		if strings.Contains(script, "const task=await taskSpace") {
			t.Fatal("status started task")
		}
		if !strings.Contains(script, "id===7") {
			t.Fatal("last completed id missing")
		}
		return json.RawMessage(`{"agentControlReleased":true}`), nil
	})
	s := &server{lastCompletion: map[string]any{"spaceId": float64(7)}}
	if _, e := s.controlStatus(""); e != nil {
		t.Fatal(e)
	}
}
func TestIdleHandoffIsOptInAndNonDestructive(t *testing.T) {
	t.Setenv("EGO_DSH_IDLE_HANDOFF_SECONDS", "")
	if idleHandoffFromEnv() != 0 {
		t.Fatal("not opt in")
	}
	t.Setenv("EGO_DSH_IDLE_HANDOFF_SECONDS", "600")
	if idleHandoffFromEnv() != 10*time.Minute {
		t.Fatal("wrong duration")
	}
	for _, v := range []string{"1", "bad", "-1", "999999"} {
		t.Setenv("EGO_DSH_IDLE_HANDOFF_SECONDS", v)
		if idleHandoffFromEnv() != 0 {
			t.Fatal(v)
		}
	}
	calls := 0
	withFakeRunner(t, func(script string, _ time.Duration) (json.RawMessage, error) {
		calls++
		if !strings.Contains(script, "task.handOff()") || strings.Contains(script, "task.finish(") {
			t.Fatal("idle closes task")
		}
		return json.RawMessage(`{"status":"handed-off"}`), nil
	})
	now := time.Now()
	s := &server{spaceID: 7, taskName: "qa", ownership: "agent", lastActivity: now.Add(-11 * time.Minute), idleHandoff: 10 * time.Minute}
	s.maybeHandoffIdle(now)
	if calls != 1 || s.ownership != "user" || s.taskName == "" {
		t.Fatal("wrong handoff state")
	}
	s.maybeHandoffIdle(now.Add(time.Hour))
	if calls != 1 {
		t.Fatal("repeated user takeover")
	}
}
func TestIdleDoesNotInterruptActiveCallOrRecentActivity(t *testing.T) {
	withFakeRunner(t, func(_ string, _ time.Duration) (json.RawMessage, error) {
		t.Fatal("unexpected idle handoff")
		return nil, nil
	})
	now := time.Now()
	s := &server{spaceID: 7, taskName: "qa", ownership: "agent", idleHandoff: time.Minute, lastActivity: now}
	s.maybeHandoffIdle(now)
	s.lastActivity = now.Add(-time.Hour)
	s.mu.Lock()
	s.maybeHandoffIdle(now)
	s.mu.Unlock()
}
func TestExportCompletionSDK(t *testing.T) {
	raw := os.Getenv("ZERO_EGO_QA_SPACE")
	if raw == "" {
		t.Skip("opt-in actual SDK completion source export")
	}
	s := &server{spaceID: 3, taskName: "qa"}
	dir := filepath.Join(".gotmp", "completion-sdk")
	os.MkdirAll(dir, 0700)
	cases := []struct {
		name string
		fn   func() (any, error)
	}{{"finish", func() (any, error) { return s.completeSpace(raw, "all") }}, {"status", func() (any, error) { return s.controlStatus(raw) }}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withFakeRunner(t, func(source string, _ time.Duration) (json.RawMessage, error) {
				i := strings.LastIndex(source, "console.log(")
				if i < 0 {
					t.Fatal("missing result")
				}
				source = source[:i] + "return value;"
				if e := os.WriteFile(filepath.Join(dir, c.name+".mjs"), []byte(source), 0600); e != nil {
					t.Fatal(e)
				}
				return json.RawMessage(`{"spaceId":3,"status":"finished","agentControlReleased":true}`), nil
			})
			if _, e := c.fn(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
