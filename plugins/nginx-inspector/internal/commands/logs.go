package commands

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// resolveLogPath picks the configured default when the caller passes none.
func resolveLogPath(info *HostInfo, kind, explicit string) (string, *plugin.Error) {
	if explicit != "" {
		if !strings.HasPrefix(explicit, "/") || len(explicit) > 4096 || strings.ContainsAny(explicit, "\x00\n\r") {
			return "", rpcerr.E(rpcerr.CodeInvalidArgs, "log path must be absolute")
		}
		return explicit, nil
	}
	switch kind {
	case "access":
		if info.HTTPLogPath != "" {
			return info.HTTPLogPath, nil
		}
	case "error", "":
		if info.ErrorLogPath != "" {
			return info.ErrorLogPath, nil
		}
	}
	return "", rpcerr.E(rpcerr.CodeInvalidArgs, "no default log path is known; pass an explicit path")
}

func (r *Runtime) logsTail(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
		Kind         string `json:"kind,omitempty"` // error|access
		Path         string `json:"path,omitempty"`
		Lines        int    `json:"lines,omitempty"`
		Filter       string `json:"filter,omitempty"`
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
	logPath, perr := resolveLogPath(info, input.Kind, input.Path)
	if perr != nil {
		return nil, perr
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

	var sizeBytes int64 = -1
	if out, perr := r.pool.Output(ctx, profile, "wc -c < "+sshx.ShellQuote(logPath)+" 2>/dev/null", 10*time.Second); perr == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64); err == nil {
			sizeBytes = n
		}
	}
	result, perr := r.pool.Run(ctx, profile, "tail -n "+strconv.Itoa(lines)+" "+sshx.ShellQuote(logPath), 20*time.Second, 0)
	if perr != nil {
		return nil, perr
	}
	if result.ExitCode != 0 {
		return nil, rpcerr.E(rpcerr.CodeFileUnreadable, "log file is not readable: "+logPath,
			map[string]string{"stderr": firstLine(result.Stderr)})
	}
	all := splitLines(result.Stdout)
	selected := all
	if filter != nil {
		selected = nil
		for _, line := range all {
			if filter.MatchString(line) {
				selected = append(selected, line)
			}
		}
	}
	return map[string]any{
		"path": logPath, "lines": selected, "matched": len(selected), "scanned": len(all),
		"truncated": result.Truncated, "sizeBytes": sizeBytes,
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
	}](args)
	if perr != nil {
		return nil, perr
	}
	if strings.TrimSpace(input.Pattern) == "" || len(input.Pattern) > 256 {
		return nil, rpcerr.E(rpcerr.CodeInvalidArgs, "pattern must be 1-256 characters (POSIX extended regex)")
	}
	info, perr := r.ensureInfo(ctx, input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	profile, perr := r.profile(input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	logPath, perr := resolveLogPath(info, input.Kind, input.Path)
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
	command := "tail -c " + strconv.FormatInt(window, 10) + " " + sshx.ShellQuote(logPath) +
		" | grep -a -n -E -m " + strconv.Itoa(maxMatches+1) + " -e " + sshx.ShellQuote(input.Pattern)
	result, perr := r.pool.Run(ctx, profile, command, 25*time.Second, 0)
	if perr != nil {
		return nil, perr
	}
	if result.ExitCode != 0 && result.ExitCode != 1 {
		// grep exit 1 means "no matches", which is a valid answer.
		return nil, rpcerr.E(rpcerr.CodeExecFailed, "remote search failed with exit code "+strconv.Itoa(result.ExitCode),
			map[string]string{"stderr": firstLine(result.Stderr)})
	}
	type match struct {
		Line int    `json:"line"`
		Text string `json:"text"`
	}
	matches := []match{}
	more := false
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
		matches = append(matches, match{Line: number, Text: line[idx+1:]})
	}
	return map[string]any{
		"path": logPath, "matches": matches, "more": more,
		"scannedBytes": window, "exitCode": result.ExitCode,
	}, nil
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
