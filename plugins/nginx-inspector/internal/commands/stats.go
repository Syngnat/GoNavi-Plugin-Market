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
)

// accessLogLine matches the nginx "combined" format plus the optional
// upstream timing fields many setups append. Custom log formats still
// contribute the leading fields when they keep the same prefix.
var accessLogLine = regexp.MustCompile(
	`^(\S+) \S+ (\S+) \[([^\]]+)\] "((?:[^"\\]|\\.)*)" (\d{3}) (\d+|-) "((?:[^"\\]|\\.)*)" "((?:[^"\\]|\\.)*)"(?: (\S+) (\S+))?`)

const nginxTimeLayout = "02/Jan/2006:15:04:05 -0700"

// CountEntry is one aggregated bucket; AvgMs only appears for paths.
type CountEntry struct {
	Value string   `json:"value"`
	Count int      `json:"count"`
	AvgMs *float64 `json:"avgMs,omitempty"`
}

// AccessStats is the access.stats response.
type AccessStats struct {
	Path          string       `json:"path"`
	WindowBytes   int64        `json:"windowBytes"`
	Total         int          `json:"total"`
	Parsed        int          `json:"parsed"`
	Unparsed      int          `json:"unparsed"`
	Statuses      []CountEntry `json:"statuses"`
	Classes       []CountEntry `json:"classes"`
	Methods       []CountEntry `json:"methods"`
	TopIPs        []CountEntry `json:"topIPs"`
	TopPaths      []CountEntry `json:"topPaths"`
	BytesTotal    int64        `json:"bytesTotal"`
	TimeFrom      string       `json:"timeFrom,omitempty"`
	TimeTo        string       `json:"timeTo,omitempty"`
	AvgUpstreamMs *float64     `json:"avgUpstreamMs,omitempty"`
	MaxUpstreamMs *float64     `json:"maxUpstreamMs,omitempty"`
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
	logPath, perr := resolveLogPath(info, "access", input.Path)
	if perr != nil {
		return nil, perr
	}
	window := input.Bytes
	if window <= 0 {
		window = 20 << 20
	}
	if window > 100<<20 {
		window = 100 << 20
	}
	result, perr := r.pool.Run(ctx, profile, "tail -c "+strconv.FormatInt(window, 10)+" "+sshx.ShellQuote(logPath), 25*time.Second, window+(1<<20))
	if perr != nil {
		return nil, perr
	}
	if result.ExitCode != 0 {
		return nil, rpcerr.E(rpcerr.CodeFileUnreadable, "access log is not readable: "+logPath,
			map[string]string{"stderr": firstLine(result.Stderr)})
	}
	return analyzeAccessLog(logPath, window, result.Stdout), nil
}

func analyzeAccessLog(path string, window int64, content string) *AccessStats {
	stats := &AccessStats{Path: path, WindowBytes: window}
	statuses := map[string]int{}
	classes := map[string]int{}
	methods := map[string]int{}
	ips := map[string]int{}
	paths := map[string]int{}
	pathTimeSum := map[string]float64{}
	pathTimeCount := map[string]int{}
	var upstreamSum float64
	var upstreamCount int
	var maxUpstream float64
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
		if url != "" {
			clean := url
			if idx := strings.IndexAny(clean, "?#"); idx >= 0 {
				clean = clean[:idx]
			}
			paths[clean]++
			if rt, ok := parseResponseTime(m[10]); ok {
				pathTimeSum[clean] += rt
				pathTimeCount[clean]++
			}
		}
		if m[6] != "-" {
			if n, err := strconv.ParseInt(m[6], 10, 64); err == nil && n > 0 {
				stats.BytesTotal += n
			}
		}
		if rt, ok := parseResponseTime(m[10]); ok {
			upstreamSum += rt
			upstreamCount++
			if rt > maxUpstream {
				maxUpstream = rt
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
	if upstreamCount > 0 {
		avg := upstreamSum / float64(upstreamCount) * 1000
		stats.AvgUpstreamMs = &avg
		maxMs := maxUpstream * 1000
		stats.MaxUpstreamMs = &maxMs
	}
	return stats
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

func unescapeLog(s string) string {
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(s)
}
