package commands

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
)

// accessLogLine matches the nginx "combined" format plus up to two optional
// trailing fields. Two trailing fields are the common "$request_time
// $upstream_response_time" pair; one trailing field is treated as
// request_time because that is by far the most common single addition.
var accessLogLine = regexp.MustCompile(
	`^(\S+) \S+ (\S+) \[([^\]]+)\] "((?:[^"\\]|\\.)*)" (\d{3}) (\d+|-) "((?:[^"\\]|\\.)*)" "((?:[^"\\]|\\.)*)"(?: (\S+))?(?: (\S+))?`)

const nginxTimeLayout = "02/Jan/2006:15:04:05 -0700"

const (
	defaultStatsWindow = 20 << 20
	maxStatsWindow     = 100 << 20
)

// CountEntry is one aggregated bucket; AvgMs only appears for paths.
type CountEntry struct {
	Value string   `json:"value"`
	Count int      `json:"count"`
	AvgMs *float64 `json:"avgMs,omitempty"`
}

// TimingStats summarizes one timing column. Samples is always present so the
// UI can distinguish "no timing data in this log format" from "zero".
type TimingStats struct {
	AvgMs   *float64 `json:"avgMs,omitempty"`
	MaxMs   *float64 `json:"maxMs,omitempty"`
	Samples int      `json:"samples"`
}

// AccessStats is the access.stats response. WindowBytes is the number of
// bytes actually analyzed (not the requested window); RequestedBytes and
// AnalyzedBytes make the transport cap visible instead of silently pretending
// a 20 MiB window was read.
type AccessStats struct {
	Path            string       `json:"path"`
	WindowBytes     int64        `json:"windowBytes"`
	RequestedBytes  int64        `json:"requestedBytes"`
	AnalyzedBytes   int64        `json:"analyzedBytes"`
	Total           int          `json:"total"`
	Parsed          int          `json:"parsed"`
	Unparsed        int          `json:"unparsed"`
	Statuses        []CountEntry `json:"statuses"`
	Classes         []CountEntry `json:"classes"`
	Methods         []CountEntry `json:"methods"`
	TopIPs          []CountEntry `json:"topIPs"`
	TopPaths        []CountEntry `json:"topPaths"`
	BytesTotal      int64        `json:"bytesTotal"`
	TimeFrom        string       `json:"timeFrom,omitempty"`
	TimeTo          string       `json:"timeTo,omitempty"`
	RequestTime     TimingStats  `json:"requestTime"`
	UpstreamTime    TimingStats  `json:"upstreamTime"`
	AvgUpstreamMs   *float64     `json:"avgUpstreamMs,omitempty"`
	MaxUpstreamMs   *float64     `json:"maxUpstreamMs,omitempty"`
	GzipUnsupported bool         `json:"gzipUnsupported,omitempty"`
}

func (r *Runtime) accessStats(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
		Path         string `json:"path,omitempty"`
		Bytes        int64  `json:"bytes,omitempty"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	info, perr := r.ensureInfo(ctx, input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	profile, perr := r.profile(input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	logPath, perr := r.logPath(ctx, input.ConnectionID, info, "access", input.Path)
	if perr != nil {
		return nil, perr
	}
	requested := input.Bytes
	if requested <= 0 {
		requested = defaultStatsWindow
	}
	if requested > maxStatsWindow {
		requested = maxStatsWindow
	}
	if compressed, ok := r.looksGzip(ctx, profile, logPath); ok && compressed {
		return emptyAccessStats(logPath, requested, true), nil
	}
	analysisLimit := requested + (1 << 20)
	if analysisLimit > sshx.MaxOutputBytes {
		analysisLimit = sshx.MaxOutputBytes
	}
	// tail must not ask for more than the transport can deliver: with a
	// 20 MiB request the old code read 20 MiB, kept the FIRST 2 MiB of it
	// and silently analyzed the oldest part of the window. Reading exactly
	// analysisLimit keeps the newest bytes, which is what a tail means.
	readWindow := analysisLimit
	result, perr := r.pool.Run(ctx, profile,
		"tail -c "+strconv.FormatInt(readWindow, 10)+" "+sshx.ShellQuote(logPath),
		25*time.Second, analysisLimit)
	if perr != nil {
		return nil, perr
	}
	if result.ExitCode != 0 {
		return nil, rpcerr.E(rpcerr.CodeFileUnreadable, "access log is not readable: "+logPath,
			map[string]string{"stderr": firstLine(result.Stderr)})
	}
	return analyzeAccessLog(logPath, requested, result.Stdout), nil
}

// looksGzip inspects the first two bytes; a gzip access log cannot be parsed
// as text and must be reported as unsupported rather than as 100% unparsed.
func (r *Runtime) looksGzip(ctx context.Context, profile store.Profile, logPath string) (bool, bool) {
	out, perr := r.pool.Output(ctx, profile,
		"head -c 2 "+sshx.ShellQuote(logPath)+" 2>/dev/null | od -An -tx1", 10*time.Second)
	if perr != nil {
		return false, false
	}
	return strings.Contains(strings.ToLower(out), "1f 8b"), true
}

func analyzeAccessLog(path string, requested int64, content string) *AccessStats {
	stats := emptyAccessStats(path, requested, false)
	stats.AnalyzedBytes = int64(len(content))
	stats.WindowBytes = stats.AnalyzedBytes

	statuses := map[string]int{}
	classes := map[string]int{}
	methods := map[string]int{}
	ips := map[string]int{}
	paths := map[string]int{}
	pathTimeSum := map[string]float64{}
	pathTimeCount := map[string]int{}
	var requestSum float64
	var requestCount int
	var requestMax float64
	var upstreamSum float64
	var upstreamCount int
	var upstreamMax float64
	var firstTime, lastTime time.Time

	for _, line := range splitLines(content) {
		stats.Total++
		m := accessLogLine.FindStringSubmatch(line)
		if m == nil {
			stats.Unparsed++
			continue
		}
		stats.Parsed++
		ips[m[1]]++
		status := m[5]
		statuses[status]++
		switch {
		case strings.HasPrefix(status, "2"):
			classes["2xx"]++
		case strings.HasPrefix(status, "3"):
			classes["3xx"]++
		case strings.HasPrefix(status, "4"):
			classes["4xx"]++
		case strings.HasPrefix(status, "5"):
			classes["5xx"]++
		default:
			classes["other"]++
		}
		method, url := splitRequest(unescapeLog(m[4]))
		if method != "" {
			methods[method]++
		}
		requestTime, hasRequest := parseResponseTime(m[9])
		upstreamTime, hasUpstream := parseResponseTime(m[10])
		if hasRequest {
			requestSum += requestTime
			requestCount++
			if requestTime > requestMax {
				requestMax = requestTime
			}
		}
		if hasUpstream {
			upstreamSum += upstreamTime
			upstreamCount++
			if upstreamTime > upstreamMax {
				upstreamMax = upstreamTime
			}
		}
		if url != "" {
			clean := url
			if idx := strings.IndexAny(clean, "?#"); idx >= 0 {
				clean = clean[:idx]
			}
			paths[clean]++
			// A per-path latency prefers the upstream duration and falls back
			// to request_time when the log format has no upstream column.
			if hasUpstream {
				pathTimeSum[clean] += upstreamTime
				pathTimeCount[clean]++
			} else if hasRequest {
				pathTimeSum[clean] += requestTime
				pathTimeCount[clean]++
			}
		}
		if m[6] != "-" {
			if n, err := strconv.ParseInt(m[6], 10, 64); err == nil && n > 0 {
				stats.BytesTotal += n
			}
		}
		if ts, err := time.Parse(nginxTimeLayout, m[3]); err == nil {
			if firstTime.IsZero() || ts.Before(firstTime) {
				firstTime = ts
			}
			if lastTime.IsZero() || ts.After(lastTime) {
				lastTime = ts
			}
		}
	}
	stats.Statuses = topCounts(statuses, 20, nil, nil)
	stats.Classes = topCounts(classes, 8, nil, nil)
	stats.Methods = topCounts(methods, 10, nil, nil)
	stats.TopIPs = topCounts(ips, 20, nil, nil)
	stats.TopPaths = topCounts(paths, 20, pathTimeSum, pathTimeCount)
	if !firstTime.IsZero() {
		stats.TimeFrom = firstTime.UTC().Format(time.RFC3339)
		stats.TimeTo = lastTime.UTC().Format(time.RFC3339)
	}
	stats.RequestTime = newTimingStats(requestSum, requestCount, requestMax)
	stats.UpstreamTime = newTimingStats(upstreamSum, upstreamCount, upstreamMax)
	stats.AvgUpstreamMs = stats.UpstreamTime.AvgMs
	stats.MaxUpstreamMs = stats.UpstreamTime.MaxMs
	return stats
}

func emptyAccessStats(path string, requested int64, gzip bool) *AccessStats {
	return &AccessStats{
		Path: path, RequestedBytes: requested, GzipUnsupported: gzip,
		Statuses: []CountEntry{}, Classes: []CountEntry{}, Methods: []CountEntry{},
		TopIPs: []CountEntry{}, TopPaths: []CountEntry{},
	}
}

func newTimingStats(sum float64, count int, max float64) TimingStats {
	if count == 0 {
		return TimingStats{}
	}
	avg := sum / float64(count) * 1000
	maxMs := max * 1000
	return TimingStats{AvgMs: &avg, MaxMs: &maxMs, Samples: count}
}

func splitRequest(request string) (method, url string) {
	fields := strings.Fields(request)
	if len(fields) >= 2 {
		return fields[0], fields[1]
	}
	if len(fields) == 1 {
		return "", fields[0]
	}
	return "", ""
}

// parseResponseTime reads the first value of a request_time or
// upstream_response_time field; multi-value entries look like "0.04, 0.03".
func parseResponseTime(field string) (float64, bool) {
	if field == "" || field == "-" {
		return 0, false
	}
	if idx := strings.Index(field, ","); idx >= 0 {
		field = field[:idx]
	}
	rt, err := strconv.ParseFloat(strings.TrimSpace(field), 64)
	if err != nil || rt < 0 {
		return 0, false
	}
	return rt, true
}

func topCounts(counter map[string]int, limit int, timeSum map[string]float64, timeCount map[string]int) []CountEntry {
	entries := make([]CountEntry, 0, len(counter))
	for value, count := range counter {
		entry := CountEntry{Value: value, Count: count}
		if timeSum != nil {
			if sum, ok := timeSum[value]; ok {
				if n := timeCount[value]; n > 0 {
					avg := sum / float64(n) * 1000
					entry.AvgMs = &avg
				}
			}
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].Value < entries[j].Value
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

// unescapeLog decodes nginx's default escape=default encoding, which writes
// quotes, backslashes and control/high bytes as \xHH (for example \x22 for a
// double quote). Only the request/referrer/user-agent strings need it.
func unescapeLog(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		switch s[i+1] {
		case 'x':
			if i+3 < len(s) {
				if value, err := strconv.ParseUint(s[i+2:i+4], 16, 8); err == nil {
					b.WriteByte(byte(value))
					i += 3
					continue
				}
			}
			b.WriteByte(s[i])
		case '\\':
			b.WriteByte('\\')
			i++
		case '"':
			b.WriteByte('"')
			i++
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
