package commands

import (
	"strings"
	"testing"
)

const sampleAccessLog = `203.0.113.7 - - [24/Oct/2026:10:00:01 +0800] "GET /api/users?page=2 HTTP/1.1" 200 1534 "https://example.com/" "Mozilla/5.0" 0.045 0.045, 0.044
203.0.113.7 - - [24/Oct/2026:10:00:02 +0800] "POST /api/login HTTP/1.1" 401 96 "-" "curl/8.0" 0.010 0.009
198.51.100.3 - - [24/Oct/2026:10:00:03 +0800] "GET /assets/app.js HTTP/1.1" 304 - "-" "Mozilla/5.0" 0.002 0.002
203.0.113.7 - - [24/Oct/2026:10:00:04 +0800] "GET /api/users HTTP/1.1" 500 512 "-" "Mozilla/5.0" 1.200 1.199
garbage line that never parses
203.0.113.7 - - [24/Oct/2026:10:00:05 +0800] "GET /api/users HTTP/1.1" 200 1200 "-" "Mozilla/5.0"
`

func TestAnalyzeAccessLog(t *testing.T) {
	stats := analyzeAccessLog("/var/log/nginx/access.log", 4096, sampleAccessLog)
	if stats.Total != 6 || stats.Parsed != 5 || stats.Unparsed != 1 {
		t.Fatalf("total/parsed/unparsed = %d/%d/%d", stats.Total, stats.Parsed, stats.Unparsed)
	}
	if got := countOf(stats.Statuses, "200"); got != 2 {
		t.Fatalf("200 count = %d", got)
	}
	if got := countOf(stats.Classes, "5xx"); got != 1 {
		t.Fatalf("5xx count = %d", got)
	}
	if got := countOf(stats.Methods, "GET"); got != 4 {
		t.Fatalf("GET count = %d", got)
	}
	if countOf(stats.TopIPs, "203.0.113.7") != 4 {
		t.Fatalf("top IPs wrong")
	}
	if countOf(stats.TopPaths, "/api/users") != 3 {
		t.Fatalf("top paths wrong")
	}
	if stats.BytesTotal != 1534+96+512+1200 {
		t.Fatalf("bytesTotal = %d", stats.BytesTotal)
	}
	if stats.AvgUpstreamMs == nil || *stats.AvgUpstreamMs < 313 || *stats.AvgUpstreamMs > 315 {
		t.Fatalf("avg upstream = %v", stats.AvgUpstreamMs)
	}
	if stats.MaxUpstreamMs == nil || *stats.MaxUpstreamMs < 1199 || *stats.MaxUpstreamMs > 1201 {
		t.Fatalf("max upstream = %v", stats.MaxUpstreamMs)
	}
	if stats.TimeFrom == "" || stats.TimeTo == "" || stats.TimeFrom >= stats.TimeTo {
		t.Fatalf("time window = %s..%s", stats.TimeFrom, stats.TimeTo)
	}
	if stats.Path == "/var/log/nginx/access.log" && stats.WindowBytes != 4096 {
		t.Fatalf("path/window wrong")
	}
	// /api/users appears with upstream times twice: 0.045 + 1.200 → avg ≈ 622.5ms
	found := false
	for _, entry := range stats.TopPaths {
		if entry.Value == "/api/users" && entry.AvgMs != nil {
			found = true
			if *entry.AvgMs < 622 || *entry.AvgMs > 623 {
				t.Fatalf("/api/users avg = %v", *entry.AvgMs)
			}
		}
	}
	if !found {
		t.Fatalf("/api/users avg missing")
	}
}

func TestAnalyzeAccessLogEmpty(t *testing.T) {
	stats := analyzeAccessLog("/log", 0, "")
	if stats.Total != 0 || stats.Statuses == nil || stats.TopPaths == nil {
		t.Fatalf("empty log should yield zeroed but initialized stats")
	}
}

func countOf(entries []CountEntry, value string) int {
	for _, entry := range entries {
		if entry.Value == value {
			return entry.Count
		}
	}
	return 0
}

func TestSplitAddress(t *testing.T) {
	cases := []struct {
		in   string
		host string
		port int
		ok   bool
	}{
		{"127.0.0.1:8080", "127.0.0.1", 8080, true},
		{"backend.internal:80", "backend.internal", 80, true},
		{"[::1]:9090", "::1", 9090, true},
		{"[::1]", "::1", 80, true},
		{"localhost", "localhost", 80, true},
		{"unix:/tmp/sock", "", 0, false},
		{"$upstream:80", "", 0, false},
		{"", "", 0, false},
		{"host:notaport", "", 0, false},
	}
	for _, c := range cases {
		host, port, ok := splitAddress(c.in)
		if ok != c.ok || host != c.host || port != c.port {
			t.Errorf("splitAddress(%q) = %q,%d,%v want %q,%d,%v", c.in, host, port, ok, c.host, c.port, c.ok)
		}
	}
}

func TestValidateMessageParsing(t *testing.T) {
	lines := []string{
		"nginx: the configuration file /etc/nginx/nginx.conf syntax is ok",
		"nginx: configuration file /etc/nginx/nginx.conf test is successful",
	}
	runtime := &Runtime{}
	_ = runtime
	for _, line := range lines {
		if nginxTestMessage.MatchString(line) {
			t.Fatalf("success line %q should not match error pattern", line)
		}
	}
	errorLine := `nginx: [emerg] unknown directive "foo" in /etc/nginx/conf.d/a.conf:12`
	m := nginxTestMessage.FindStringSubmatch(errorLine)
	if m == nil || m[1] != "emerg" {
		t.Fatalf("emerg line not parsed: %v", m)
	}
	loc := nginxTestLocation.FindStringSubmatch(strings.TrimSpace(m[2]))
	if loc == nil || loc[2] != "/etc/nginx/conf.d/a.conf" || loc[3] != "12" {
		t.Fatalf("location not parsed: %v", loc)
	}
}
