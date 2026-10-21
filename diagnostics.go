package main

import (
	"bytes"
	"os"
	"regexp"
	"strings"
)

// CLI diagnostics are bounded even if a script logs unbounded page data.
type cappedBuffer struct {
	data      []byte
	limit     int
	discarded int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.limit == 0 {
		b.limit = 8 * 1024 * 1024
	}
	b.data = append(b.data, p...)
	if len(b.data) > b.limit {
		drop := len(b.data) - b.limit
		b.discarded += drop
		b.data = append([]byte(nil), b.data[drop:]...)
	}
	return n, nil
}
func (b *cappedBuffer) String() string { return strings.ToValidUTF8(string(b.data), "�") }
func (b *cappedBuffer) Bytes() []byte  { return bytes.ToValidUTF8(b.data, []byte("�")) }
func conciseCLIError(detail string) string {
	if os.Getenv("EGO_DSH_DEBUG") == "1" {
		return runePrefix(detail, 12000)
	}
	lines := strings.Split(strings.ReplaceAll(detail, "\r\n", "\n"), "\n")
	causes := []string{}
	safety := []string{}
	other := []string{}
	errorLine := regexp.MustCompile(`(?i)(?:^|\s)(?:\w*Error:|failed to|task space|page label|timed out|timeout|cannot |no managed)`)
	for _, line := range lines {
		v := strings.TrimSpace(line)
		if v == "" || strings.HasPrefix(v, "at ") || strings.Contains(v, "nodejs process exited") {
			continue
		}
		if strings.Contains(v, "executionStopped") || strings.Contains(v, "mayHaveLateEffects") || strings.Contains(v, "pageResponsive") {
			safety = append(safety, v)
			continue
		}
		if errorLine.MatchString(v) {
			causes = append(causes, v)
		} else if !strings.Contains(v, "file:///") {
			other = append(other, v)
		}
	}
	if len(causes) == 0 {
		causes = other
	}
	if len(causes) > 2 {
		causes = causes[:2]
	}
	result := strings.Join(append(causes, safety...), "\n")
	if result == "" {
		result = "Ego returned no usable result; enable EGO_DSH_DEBUG=1 for diagnostics."
	}
	if strings.Contains(result, "Failed to connect to ego_cli bootstrap") {
		result += "\nUse the configured MCP connector outside the workspace; direct CLI may require host Full Access. Do not retry blocked sandbox operations via another route."
	}
	return runePrefix(result, 2400)
}
