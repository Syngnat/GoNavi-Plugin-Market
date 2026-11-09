package commands

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/ngxconf"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// logTarget is the resolved log file plus where the path came from and any
// caveat the UI should show (off, syslog, variable, fallback, allowlist skip).
type logTarget struct {
	Path   string
	Source string
	Note   string
	Level  string
}

// Response budgets. The host rejects a window invoke payload above one MiB
// (and the protocol frame is one MiB too), so logs must stay well below it
// even after JSON escaping. Both limits bound the bytes read from the remote.
const (
	maxLogTailBytes   = 512 << 10
	maxLogSearchBytes = 512 << 10
	maxMatchTextBytes = 4 << 10
)

var errorLogLevels = map[string]int{
	"debug": 0, "info": 1, "notice": 2, "warn": 3,
	"error": 4, "crit": 5, "alert": 6, "emerg": 7,
}

// logPath resolves only the file path. stats uses it; logs uses logTarget.
func (r *Runtime) logPath(ctx context.Context, connectionID string, info *HostInfo, kind, explicit string) (string, *plugin.Error) {
	if explicit != "" {
		return r.resolveAllowedPath(ctx, connectionID, explicit)
	}
	if kind == "access" {
		if info.HTTPLogPath != "" {
			return info.HTTPLogPath, nil
		}
		return "", rpcerr.E(rpcerr.CodeInvalidArgs, "no default access log path is known; pass an explicit path")
	}
	if info.ErrorLogPath != "" {
		return info.ErrorLogPath, nil
	}
	return "", rpcerr.E(rpcerr.CodeInvalidArgs, "no default error log path is known; pass an explicit path")
}

// logTarget prefes the merged nginx configuration over "nginx -V" defaults
// because -V only reports the compile-time paths, which are frequently stale.
func (r *Runtime) logTarget(ctx context.Context, connectionID string, info *HostInfo, kind, explicit string) (logTarget, *plugin.Error) {
	if explicit != "" {
		resolved, perr := r.resolveAllowedPath(ctx, connectionID, explicit)
		if perr != nil {
			return logTarget{}, perr
		}
		return logTarget{Path: resolved, Source: "explicit"}, nil
	}
	name := "error_log"
	if kind == "access" {
		name = "access_log"
	}
	if loaded, perr := r.loadTree(ctx, connectionID); perr == nil {
		if directive := configuredLogDirective(loaded.directives, name); directive != nil {
			target := logTargetFromDirective(info, directive, kind)
			if target.Path != "" || target.Note != "" {
				return target, nil
			}
		}
	}
	// Fall back to the compile-time defaults from "nginx -V".
	if kind == "access" && info.HTTPLogPath != "" {
		return logTarget{Path: info.HTTPLogPath, Source: "nginx-v"}, nil
	}
	if kind != "access" && info.ErrorLogPath != "" {
		return logTarget{Path: info.ErrorLogPath, Source: "nginx-v"}, nil
	}
	return logTarget{}, rpcerr.E(rpcerr.CodeInvalidArgs, "no log path is known; pass an explicit path")
}

// configuredLogDirective returns the http-level directive when present and
// falls back to a global one. Server-level access_log entries are ignored so
// the answer matches nginx's own default for requests without an override.
func configuredLogDirective(directives []ngxconf.Directive, name string) *ngxconf.Directive {
	for i := range directives {
		if directives[i].Name != "http" {
			continue
		}
		if found := firstDirective(directives[i].Block, name); found != nil {
			return found
		}
	}
	return firstDirective(directives, name)
}

func firstDirective(directives []ngxconf.Directive, name string) *ngxconf.Directive {
	for i := range directives {
		if directives[i].Name == name && len(directives[i].Args) > 0 {
			return &directives[i]
		}
	}
	return nil
}

func logTargetFromDirective(info *HostInfo, directive *ngxconf.Directive, kind string) logTarget {
	raw := strings.TrimSpace(directive.Args[0].Value)
	target := logTarget{Source: "config"}
	if kind != "access" && len(directive.Args) > 1 {
		level := strings.ToLower(strings.TrimSpace(directive.Args[1].Value))
		if _, ok := errorLogLevels[level]; ok {
			target.Level = level
		}
	}
	switch {
	case raw == "off":
		target.Note = "log is disabled (off) in the nginx configuration"
		return target
	case strings.HasPrefix(raw, "syslog:"):
		target.Note = "log is sent to syslog; read it through the system journal instead"
		return target
	case raw == "stderr":
		target.Note = "log is sent to stderr and is only available through the process supervisor"
		return target
	case raw == "memory:" || raw == "memory":
		target.Note = "log is kept in memory by nginx and is not readable from disk"
		return target
	case strings.Contains(raw, "$"):
		target.Note = "log path contains a variable and cannot be resolved statically"
		return target
	}
	// Absolute paths from the remote configuration get the same allowlist
	// as explicit arguments: a hostile access_log value must not turn the
	// tail command into a generic file reader.
	resolved, perr := allowedRelativePath(info, raw)
	if perr != nil {
		target.Note = "configured log path is not allowed: " + perr.Message
		return target
	}
	target.Path = resolved
	return target
}

func (r *Runtime) logsTail(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
		Kind         string `json:"kind,omitempty"` // error|access
		Path         string `json:"path,omitempty"`
		Lines        int    `json:"lines,omitempty"`
		Filter       string `json:"filter,omitempty"`
		Level        string `json:"level,omitempty"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	kind := normalizeLogKind(input.Kind)
	if input.Level != "" {
		level := strings.ToLower(input.Level)
		if _, ok := errorLogLevels[level]; !ok {
			return nil, rpcerr.E(rpcerr.CodeInvalidArgs, "level must be one of debug, info, notice, warn, error, crit, alert, emerg")
		}
		input.Level = level
	}
	info, perr := r.ensureInfo(ctx, input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	profile, perr := r.profile(input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	target, perr := r.logTarget(ctx, input.ConnectionID, info, kind, input.Path)
	if perr != nil {
		return nil, perr
	}
	if target.Path == "" {
		return map[string]any{
			"kind": kind, "path": "", "source": target.Source, "note": target.Note,
			"lines": []string{}, "matched": 0, "scanned": 0, "truncated": false,
			"sizeBytes": int64(-1), "totalLines": int64(-1), "windowStartLine": int64(0),
			"level": target.Level, "filter": input.Filter,
		}, nil
	}
	lines := input.Lines
	if lines <= 0 {
		lines = 200
	}
	if lines > 1000 {
		lines = 1000
	}
	filter, perr := compileFilter(input.Filter)
	if perr != nil {
		return nil, perr
	}
	totalLines, sizeBytes := r.logFileStats(ctx, profile, target.Path)
	// Read a byte-bounded suffix instead of "tail -n lines": one pathological
	// line can exceed the frame on its own, and the newest bytes are what a
	// log tail is for. Lines are then trimmed from the end locally.
	result, perr := r.pool.Run(ctx, profile,
		"tail -c "+strconv.Itoa(maxLogTailBytes)+" "+sshx.ShellQuote(target.Path),
		20*time.Second, maxLogTailBytes)
	if perr != nil {
		return nil, perr
	}
	if result.ExitCode != 0 {
		return nil, rpcerr.E(rpcerr.CodeFileUnreadable, "log file is not readable: "+target.Path,
			map[string]string{"stderr": firstLine(result.Stderr)})
	}
	raw := result.Stdout
	// When the byte window starts in the middle of a line, drop that partial
	// first line rather than showing a torn record.
	frontCut := int64(len(raw)) >= maxLogTailBytes
	if frontCut {
		if newline := strings.IndexByte(raw, '\n'); newline >= 0 {
			raw = raw[newline+1:]
		}
	}
	all := splitLines(raw)
	shortened := false
	if len(all) > lines {
		all = all[len(all)-lines:]
		shortened = true
	}
	truncated := frontCut || shortened
	selected := make([]string, 0, len(all))
	for _, line := range all {
		if filter != nil && !filter.MatchString(line) {
			continue
		}
		if input.Level != "" && kind == "error" {
			if _, rank, ok := errorLineLevel(line); !ok || rank < errorLogLevels[input.Level] {
				continue
			}
		}
		selected = append(selected, line)
	}
	note := target.Note
	if input.Level != "" && target.Level != "" && errorLogLevels[input.Level] < errorLogLevels[target.Level] {
		note = strings.TrimSpace(note + " configured error_log level is " + target.Level + "; lower levels are not written")
	}
	windowStartLine := int64(0)
	if totalLines >= 0 && int64(len(all)) <= totalLines {
		windowStartLine = totalLines - int64(len(all)) + 1
	}
	return map[string]any{
		"kind": kind, "path": target.Path, "source": target.Source, "note": note,
		"lines": selected, "matched": len(selected), "scanned": len(all),
		"truncated": truncated, "sizeBytes": sizeBytes, "totalLines": totalLines,
		"windowStartLine": windowStartLine, "level": target.Level, "filter": input.Filter,
	}, nil
}

func (r *Runtime) logsSearch(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
		Kind         string `json:"kind,omitempty"`
		Path         string `json:"path,omitempty"`
		Pattern      string `json:"pattern"`
		Bytes        int64  `json:"bytes,omitempty"`
		MaxMatches   int    `json:"maxMatches,omitempty"`
		Level        string `json:"level,omitempty"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	if strings.TrimSpace(input.Pattern) == "" || len(input.Pattern) > 256 {
		return nil, rpcerr.E(rpcerr.CodeInvalidArgs, "pattern must be 1-256 characters (POSIX extended regex)")
	}
	kind := normalizeLogKind(input.Kind)
	info, perr := r.ensureInfo(ctx, input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	profile, perr := r.profile(input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	target, perr := r.logTarget(ctx, input.ConnectionID, info, kind, input.Path)
	if perr != nil {
		return nil, perr
	}
	window := input.Bytes
	if window <= 0 {
		window = 10 << 20
	}
	if window > 50<<20 {
		window = 50 << 20
	}
	maxMatches := input.MaxMatches
	if maxMatches <= 0 {
		maxMatches = 100
	}
	if maxMatches > 500 {
		maxMatches = 500
	}
	if target.Path == "" {
		return map[string]any{
			"kind": kind, "path": "", "source": target.Source, "note": target.Note,
			"matches": []any{}, "more": false, "scannedBytes": int64(0), "windowBytes": int64(0),
			"fileSizeBytes": int64(-1), "windowStartOffset": int64(0), "exitCode": 0,
		}, nil
	}
	fileSize := int64(-1)
	if out, perr := r.pool.Output(ctx, profile, "wc -c < "+sshx.ShellQuote(target.Path)+" 2>/dev/null", 10*time.Second); perr == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64); err == nil {
			fileSize = n
		}
	}
	command := "tail -c " + strconv.FormatInt(window, 10) + " " + sshx.ShellQuote(target.Path) +
		" | grep -a -n -E -m " + strconv.Itoa(maxMatches+1) + " -e " + sshx.ShellQuote(input.Pattern)
	result, perr := r.pool.Run(ctx, profile, command, 25*time.Second, maxLogSearchBytes)
	if perr != nil {
		return nil, perr
	}
	if result.ExitCode != 0 && result.ExitCode != 1 {
		return nil, rpcerr.E(rpcerr.CodeExecFailed, "remote search failed with exit code "+strconv.Itoa(result.ExitCode),
			map[string]string{"stderr": firstLine(result.Stderr)})
	}
	type match struct {
		WindowLine int    `json:"windowLine"`
		Line       int    `json:"line"` // deprecated alias of windowLine
		Text       string `json:"text"`
	}
	matches := []match{}
	more := result.Truncated
	used := 0
	for _, line := range splitLines(result.Stdout) {
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		number, err := strconv.Atoi(line[:idx])
		if err != nil {
			continue
		}
		if len(matches) >= maxMatches {
			more = true
			break
		}
		text := line[idx+1:]
		if len(text) > maxMatchTextBytes {
			// Cut on a byte boundary, then drop a torn UTF-8 tail.
			text = strings.ToValidUTF8(text[:maxMatchTextBytes], "") + "…"
			more = true
		}
		if used+len(text) > maxLogSearchBytes {
			// The frame budget applies to the whole response, not one match.
			more = true
			break
		}
		used += len(text)
		matches = append(matches, match{WindowLine: number, Line: number, Text: text})
	}
	scanned := window
	if fileSize >= 0 && fileSize < scanned {
		scanned = fileSize
	}
	windowStart := int64(0)
	if fileSize > scanned {
		windowStart = fileSize - scanned
	}
	return map[string]any{
		"kind": kind, "path": target.Path, "source": target.Source, "note": target.Note,
		"matches": matches, "more": more, "scannedBytes": scanned, "windowBytes": scanned,
		"fileSizeBytes": fileSize, "windowStartOffset": windowStart, "exitCode": result.ExitCode,
	}, nil
}

func normalizeLogKind(kind string) string {
	if kind == "access" {
		return "access"
	}
	return "error"
}

func errorLineLevel(line string) (string, int, bool) {
	open := strings.Index(line, "[")
	if open < 0 {
		return "", 0, false
	}
	closeIdx := strings.Index(line[open:], "]")
	if closeIdx < 0 {
		return "", 0, false
	}
	level := strings.ToLower(line[open+1 : open+closeIdx])
	rank, ok := errorLogLevels[level]
	if !ok {
		return "", 0, false
	}
	return level, rank, true
}

// logFileStats reads total line and byte counts in one round trip.
func (r *Runtime) logFileStats(ctx context.Context, profile store.Profile, logPath string) (int64, int64) {
	out, perr := r.pool.Output(ctx, profile, "wc -l -c < "+sshx.ShellQuote(logPath)+" 2>/dev/null", 10*time.Second)
	if perr != nil {
		return -1, -1
	}
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return -1, -1
	}
	lines, errLines := strconv.ParseInt(fields[0], 10, 64)
	bytes, errBytes := strconv.ParseInt(fields[1], 10, 64)
	if errLines != nil || errBytes != nil {
		return -1, -1
	}
	return lines, bytes
}

func splitLines(s string) []string {
	trimmed := strings.TrimSuffix(s, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}
