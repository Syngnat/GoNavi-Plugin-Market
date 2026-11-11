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

// Stats windows are streamed: the access log is read chunk by chunk and
// aggregated on the fly, so a 100 MB window costs no more memory than a 20 MB
// one. The caps below bound remote work and the time a statistics request can
// take, and the response reports the bytes actually analyzed.
const (
	defaultStatsWindow = 20 << 20
	maxStatsWindow     = 100 << 20
	// maxBuckets bounds the distinct values kept per dimension (IPs, paths,
	// statuses...). A pathological log with millions of unique paths must not
	// grow the plugin process without limit; anything past the cap lands in the
	// "(other)" bucket so the totals still add up.
	maxBuckets = 200000
	otherLabel = "(other)"
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
// a larger window was read. Truncated marks a window that was cut short by the
// transport cap or by the command output limit. Source and Note carry the same
// provenance the logs page shows (config / nginx -V / explicit, plus caveats
// such as "access_log off").
type AccessStats struct {
	Path            string       `json:"path"`
	Source          string       `json:"source,omitempty"`
	Note            string       `json:"note,omitempty"`
	WindowBytes     int64        `json:"windowBytes"`
	RequestedBytes  int64        `json:"requestedBytes"`
	AnalyzedBytes   int64        `json:"analyzedBytes"`
	Truncated       bool         `json:"truncated"`
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
	// Prefer the merged configuration over the compile-time default from
	// "nginx -V": a site that sets its own access_log would otherwise have its
	// statistics computed from an empty default file while the logs page shows
	// the real one.
	target, perr := r.logTarget(ctx, input.ConnectionID, info, "access", input.Path)
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
	if target.Path == "" {
		// The configured target cannot be read (off, syslog, variable path):
		// report the reason instead of analyzing an unrelated default file.
		stats := emptyAccessStats("", requested, false)
		stats.Source = target.Source
		stats.Note = target.Note
		return stats, nil
	}
	logPath := target.Path
	if compressed, ok := r.looksGzip(ctx, profile, logPath); ok && compressed {
		gzipStats := emptyAccessStats(logPath, requested, true)
		gzipStats.Source = target.Source
		gzipStats.Note = target.Note
		return gzipStats, nil
	}
	// The window is streamed and aggregated line by line, so its size is not
	// capped by the per-command output limit. Reading exactly `requested` bytes
	// from the tail keeps the newest part of the log, which is what a log
	// window means; a shorter file is simply analyzed in full.
	command := "tail -c " + strconv.FormatInt(requested, 10) + " " + sshx.ShellQuote(logPath)
	acc := newAccessAccumulator(logPath, requested)
	stream, perr := r.pool.Stream(ctx, profile, command, statsTimeout(requested), requested, acc.write)
	if perr != nil {
		// A transport failure before any byte arrived is worth one retry on a
		// fresh connection; the accumulator is restarted so no line is counted
		// twice. Once data has been consumed, a retry could double-count.
		if acc.stats.AnalyzedBytes > 0 {
			return nil, perr
		}
		acc = newAccessAccumulator(logPath, requested)
		stream, perr = r.pool.Stream(ctx, profile, command, statsTimeout(requested), requested, acc.write)
		if perr != nil {
			return nil, perr
		}
	}
	if stream.ExitCode != 0 && stream.ExitCode != 141 {
		// 141 is SIGPIPE: the transport stopped reading, not a missing file.
		return nil, rpcerr.E(rpcerr.CodeFileUnreadable, "access log is not readable: "+logPath)
	}
	stats := acc.finish()
	stats.Source = target.Source
	stats.Note = target.Note
	stats.Truncated = stream.Truncated
	return stats, nil
}

// statsTimeout scales the remote deadline with the window size. A 100 MB read
// over a slow link needs far longer than the 20 s default, while a small window
// must still fail fast.
func statsTimeout(window int64) time.Duration {
	seconds := 20 + window/(2<<20) // 1 s per 2 MiB on top of 20 s
	if seconds > 180 {
		seconds = 180
	}
	return time.Duration(seconds) * time.Second
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

// accessAccumulator aggregates access log lines incrementally. write accepts
// arbitrary chunks from the remote stream, keeps the trailing partial line for
// the next call and feeds complete lines to add, so a window never has to be
// held in memory as a whole.
type accessAccumulator struct {
	stats *AccessStats
	rest  []byte

	statuses map[string]int
	classes  map[string]int
	methods  map[string]int
	ips      map[string]int
	paths    map[string]int
	// ipOther and pathOther collect values beyond maxBuckets so the totals
	// still add up without letting the maps grow without limit.
	ipOther   int
	pathOther int

	pathTimeSum   map[string]float64
	pathTimeCount map[string]int

	requestSum    float64
	requestCount  int
	requestMax    float64
	upstreamSum   float64
	upstreamCount int
	upstreamMax   float64

	firstTime, lastTime time.Time
}

func newAccessAccumulator(path string, requested int64) *accessAccumulator {
	return &accessAccumulator{
		stats:         emptyAccessStats(path, requested, false),
		statuses:      map[string]int{},
		classes:       map[string]int{},
		methods:       map[string]int{},
		ips:           map[string]int{},
		paths:         map[string]int{},
		pathTimeSum:   map[string]float64{},
		pathTimeCount: map[string]int{},
	}
}

// boundedCount increments counter for value, folding values beyond maxBuckets
// into the caller's overflow counter. Statuses, classes and methods have tiny
// cardinality and are counted directly.
func boundedCount(counter map[string]int, value string, other *int) {
	if _, ok := counter[value]; ok || len(counter) < maxBuckets {
		counter[value]++
		return
	}
	*other++
}

func (a *accessAccumulator) write(chunk []byte) {
	a.stats.AnalyzedBytes += int64(len(chunk))
	data := chunk
	if len(a.rest) > 0 {
		// A partial line at the end of the previous chunk is completed here.
		joined := make([]byte, 0, len(a.rest)+len(chunk))
		joined = append(joined, a.rest...)
		joined = append(joined, chunk...)
		data = joined
		a.rest = nil
	}
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] != '\n' {
			continue
		}
		a.add(strings.TrimSuffix(string(data[start:i]), "\r"))
		start = i + 1
	}
	if start < len(data) {
		a.rest = append(a.rest[:0], data[start:]...)
	}
}

// add counts one request line.
func (a *accessAccumulator) add(line string) {
	if line == "" {
		return
	}
	stats := a.stats
	stats.Total++
	m := accessLogLine.FindStringSubmatch(line)
	if m == nil {
		stats.Unparsed++
		return
	}
	stats.Parsed++
	boundedCount(a.ips, m[1], &a.ipOther)
	status := m[5]
	a.statuses[status]++
	switch {
	case strings.HasPrefix(status, "2"):
		a.classes["2xx"]++
	case strings.HasPrefix(status, "3"):
		a.classes["3xx"]++
	case strings.HasPrefix(status, "4"):
		a.classes["4xx"]++
	case strings.HasPrefix(status, "5"):
		a.classes["5xx"]++
	default:
		a.classes["other"]++
	}
	method, url := splitRequest(unescapeLog(m[4]))
	if method != "" {
		a.methods[method]++
	}
	requestTime, hasRequest := parseResponseTime(m[9])
	upstreamTime, hasUpstream := parseResponseTime(m[10])
	if hasRequest {
		a.requestSum += requestTime
		a.requestCount++
		if requestTime > a.requestMax {
			a.requestMax = requestTime
		}
	}
	if hasUpstream {
		a.upstreamSum += upstreamTime
		a.upstreamCount++
		if upstreamTime > a.upstreamMax {
			a.upstreamMax = upstreamTime
		}
	}
	if url != "" {
		clean := url
		if idx := strings.IndexAny(clean, "?#"); idx >= 0 {
			clean = clean[:idx]
		}
		// Bounding the counter keeps a log with millions of unique URLs from
		// growing the process without limit; the overflow is shown as
		// "(other)" so the response totals still add up.
		boundedCount(a.paths, clean, &a.pathOther)
		// A per-path latency prefers the upstream duration and falls back
		// to request_time when the log format has no upstream column.
		if hasUpstream {
			a.pathTimeSum[clean] += upstreamTime
			a.pathTimeCount[clean]++
		} else if hasRequest {
			a.pathTimeSum[clean] += requestTime
			a.pathTimeCount[clean]++
		}
	}
	if m[6] != "-" {
		if n, err := strconv.ParseInt(m[6], 10, 64); err == nil && n > 0 {
			stats.BytesTotal += n
		}
	}
	if ts, err := time.Parse(nginxTimeLayout, m[3]); err == nil {
		if a.firstTime.IsZero() || ts.Before(a.firstTime) {
			a.firstTime = ts
		}
		if a.lastTime.IsZero() || ts.After(a.lastTime) {
			a.lastTime = ts
		}
	}
}

// finish flushes the trailing unterminated line and renders the aggregates.
func (a *accessAccumulator) finish() *AccessStats {
	if len(a.rest) > 0 {
		line := strings.TrimSuffix(string(a.rest), "\r")
		a.rest = nil
		a.add(line)
	}
	stats := a.stats
	stats.WindowBytes = stats.AnalyzedBytes
	if a.ipOther > 0 {
		a.ips[otherLabel] += a.ipOther
	}
	if a.pathOther > 0 {
		a.paths[otherLabel] += a.pathOther
	}
	stats.Statuses = topCounts(a.statuses, 20, nil, nil)
	stats.Classes = topCounts(a.classes, 8, nil, nil)
	stats.Methods = topCounts(a.methods, 10, nil, nil)
	stats.TopIPs = topCounts(a.ips, 20, nil, nil)
	stats.TopPaths = topCounts(a.paths, 20, a.pathTimeSum, a.pathTimeCount)
	if !a.firstTime.IsZero() {
		stats.TimeFrom = a.firstTime.UTC().Format(time.RFC3339)
		stats.TimeTo = a.lastTime.UTC().Format(time.RFC3339)
	}
	stats.RequestTime = newTimingStats(a.requestSum, a.requestCount, a.requestMax)
	stats.UpstreamTime = newTimingStats(a.upstreamSum, a.upstreamCount, a.upstreamMax)
	stats.AvgUpstreamMs = stats.UpstreamTime.AvgMs
	stats.MaxUpstreamMs = stats.UpstreamTime.MaxMs
	return stats
}

// analyzeAccessLog aggregates an in-memory log. It is the reference
// implementation the streaming path must match, and keeps the unit tests
// readable.
func analyzeAccessLog(path string, requested int64, content string) *AccessStats {
	acc := newAccessAccumulator(path, requested)
	acc.write([]byte(content))
	return acc.finish()
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
