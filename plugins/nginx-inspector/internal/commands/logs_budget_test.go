package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// A 512 KiB byte window of 100-byte lines holds far more than the 1000-line
// cap, so the response must be trimmed locally and stay well under the one MiB
// host frame limit. The first line of the byte window is usually torn and must
// be dropped.
func TestLogsTailStaysUnderFrameBudget(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 8000; i++ {
		fmt.Fprintf(&b, "line-%05d %s\n", i, strings.Repeat("x", 87))
	}
	data := []byte(b.String())
	// fileCommands does not model shell redirection; drop it before matching.
	base := fileCommands(map[string][]byte{"/var/log/nginx/access.log": data})
	handler := func(command string) (string, string, int) {
		return base(strings.ReplaceAll(command, " 2>/dev/null", ""))
	}
	host, port := startFakeSSH(t, handler)
	runtime, connectionID := newFakeRuntime(t, host, port)
	result, err := runtime.logsTail(context.Background(), connectionArgs(t, connectionID,
		"kind", "access", "lines", 1000))
	if err != nil {
		t.Fatalf("logs.tail: %v", err)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(payload) >= 1<<20 {
		t.Fatalf("response is %d bytes, at or above the one MiB frame limit", len(payload))
	}
	response := result.(map[string]any)
	if truncated, _ := response["truncated"].(bool); !truncated {
		t.Fatalf("truncated = false, the window and the line cap both cut data")
	}
	if scanned, _ := response["scanned"].(int); scanned != 1000 {
		t.Fatalf("scanned = %d, want 1000", scanned)
	}
	if start, _ := response["windowStartLine"].(int64); start != 7001 {
		t.Fatalf("windowStartLine = %d, want 7001", start)
	}
	lines, _ := response["lines"].([]string)
	if len(lines) != 1000 {
		t.Fatalf("lines = %d, want 1000", len(lines))
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "line-") || len(line) != 98 {
			t.Fatalf("torn line in tail response: %q", line)
		}
	}
}
