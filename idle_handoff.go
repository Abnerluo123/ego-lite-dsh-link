package main

import (
	"os"
	"strconv"
	"time"
)

// Optional protection for abandoned takeovers. A quiet interval is not proof of
// completion, so this only hands control to the user and never closes pages.
// Disabled by default: model reasoning and non-browser work may take a long time.
func idleHandoffFromEnv() time.Duration {
	raw := os.Getenv("EGO_DSH_IDLE_HANDOFF_SECONDS")
	if raw == "" || raw == "0" {
		return 0
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 60 || seconds > 86400 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
func (s *server) watchIdle(stop <-chan struct{}) {
	if s.idleHandoff == 0 {
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			s.maybeHandoffIdle(now)
		}
	}
}
func (s *server) maybeHandoffIdle(now time.Time) {
	// Never interrupt a running browser operation, even if it exceeds the timeout.
	if !s.mu.TryLock() {
		return
	}
	defer s.mu.Unlock()
	if s.idleHandoff == 0 || s.taskName == "" || s.spaceID == 0 || s.ownership != "agent" || s.lastActivity.IsZero() || now.Sub(s.lastActivity) < s.idleHandoff {
		return
	}
	s.lastActivity = now
	_, err := s.handoff()
	if err != nil {
		s.log("idle-handoff-error", err.Error())
		return
	}
	s.log("idle-handoff", "Control handed to user; pages kept; task not finished")
}
