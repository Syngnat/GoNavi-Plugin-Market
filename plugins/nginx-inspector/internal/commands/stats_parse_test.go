package commands

import (
	"strings"
	"testing"
)

func TestUnescapeLog(t *testing.T) {
	cases := map[string]string{
		`GET /a\x22b HTTP/1.1`: `GET /a"b HTTP/1.1`,
		`/c\x5cx`:              `/c\x`,
		`/a\\b`:                `/a\b`,
		`/a\qb`:                `/a\qb`,
	}
	for input, want := range cases {
		if got := unescapeLog(input); got != want {
			t.Errorf("unescapeLog(%q) = %q want %q", input, got, want)
		}
	}
}

func TestParseResponseTime(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"0.045", 0.045, true},
		{"0.045, 0.044", 0.045, true},
		{" 1.200 ", 1.2, true},
		{"-", 0, false},
		{"", 0, false},
		{"abc", 0, false},
		{"-0.1", 0, false},
	}
	for _, c := range cases {
		got, ok := parseResponseTime(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseResponseTime(%q) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestAnalyzeAccessLogSingleTimingColumn(t *testing.T) {
	line := `203.0.113.9 - - [24/Oct/2026:11:00:00 +0800] "GET /one HTTP/1.1" 200 10 "-" "curl" 0.200`
	stats := analyzeAccessLog("/log", 1024, line+string(rune(10)))
	if stats.Parsed != 1 || stats.Total != 1 {
		t.Fatalf("parsed = %d total = %d", stats.Parsed, stats.Total)
	}
	if stats.RequestTime.Samples != 1 || stats.UpstreamTime.Samples != 0 {
		t.Fatalf("timing = request:%d upstream:%d", stats.RequestTime.Samples, stats.UpstreamTime.Samples)
	}
	if stats.AvgUpstreamMs != nil {
		t.Fatalf("single timing column must not invent upstream timing")
	}
	if len(stats.TopPaths) != 1 || stats.TopPaths[0].AvgMs == nil || *stats.TopPaths[0].AvgMs < 199 || *stats.TopPaths[0].AvgMs > 201 {
		t.Fatalf("per-path latency = %+v", stats.TopPaths)
	}
}

func TestAnalyzeAccessLogAllBucketsInitialized(t *testing.T) {
	stats := analyzeAccessLog("/log", 0, "")
	for name, entries := range map[string][]CountEntry{
		"statuses": stats.Statuses,
		"classes":  stats.Classes,
		"methods":  stats.Methods,
		"topIPs":   stats.TopIPs,
		"topPaths": stats.TopPaths,
	} {
		if entries == nil {
			t.Errorf("%s must be an empty slice, not nil", name)
		}
	}
}

func TestAnalyzeAccessLogEscapedRequest(t *testing.T) {
	line := `203.0.113.9 - - [24/Oct/2026:11:00:00 +0800] "GET /a\x22b HTTP/1.1" 200 10 "-" "curl"`
	stats := analyzeAccessLog("/log", 1024, line+string(rune(10)))
	if stats.Parsed != 1 {
		t.Fatalf("escaped line did not parse: %+v", stats.Unparsed)
	}
	if len(stats.TopPaths) != 1 || !strings.Contains(stats.TopPaths[0].Value, `/a"b`) {
		t.Fatalf("top paths = %+v", stats.TopPaths)
	}
}
