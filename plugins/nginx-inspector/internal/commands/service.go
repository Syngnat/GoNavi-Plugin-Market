package commands

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
)

// ServiceStatus is the service.status response.
type ServiceStatus struct {
	Found    bool   `json:"found"`
	Version  string `json:"version,omitempty"`
	Binary   string `json:"binary,omitempty"`
	Systemd  bool   `json:"systemd"`
	Docker   bool   `json:"docker"`
	Unit     string `json:"unit,omitempty"`
	State    string `json:"state"` // active|inactive|failed|unknown
	PID      int    `json:"pid,omitempty"`
	Uptime   string `json:"uptime,omitempty"`
	PIDFile  string `json:"pidFile,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

// serviceUnitCandidates lists the unit names used by the common nginx
// distributions, including OpenResty and Tengine packaging.
var serviceUnitCandidates = []string{
	"nginx.service", "openresty.service", "tengine.service",
}

func (r *Runtime) serviceStatus(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
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
	status := &ServiceStatus{
		Found: info.Found, Version: info.Version, Binary: info.Binary,
		Systemd: info.Systemd, Docker: info.Docker, State: "unknown",
		PIDFile: info.PIDPath, Hostname: info.Hostname,
	}
	if info.Systemd {
		if unit := r.detectServiceUnit(ctx, profile, info); unit != "" {
			status.Unit = unit
			if out, perr := r.pool.Output(ctx, profile, "systemctl is-active "+sshx.ShellQuote(unit)+" 2>&1", 10*time.Second); perr == nil {
				status.State = strings.TrimSpace(out)
			}
		}
	}
	if info.PIDPath != "" {
		if out, perr := r.pool.Output(ctx, profile, "cat "+sshx.ShellQuote(info.PIDPath)+" 2>/dev/null", 5*time.Second); perr == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(out)); err == nil && pid > 0 {
				status.PID = pid
				if out, perr := r.pool.Output(ctx, profile, "ps -o etime= -p "+strconv.Itoa(pid)+" 2>/dev/null", 5*time.Second); perr == nil {
					status.Uptime = strings.TrimSpace(out)
				}
			}
		}
	}
	if status.State == "unknown" && status.PID > 0 {
		status.State = "active"
	}
	if status.State == "unknown" && !info.Systemd {
		status.State = "inactive"
	}
	return status, nil
}

// detectServiceUnit picks the systemd unit that actually exists. "nginx" is
// not universally the unit name: OpenResty ships openresty.service and some
// distributions use the binary name instead.
func (r *Runtime) detectServiceUnit(ctx context.Context, profile store.Profile, info *HostInfo) string {
	if !info.Systemd {
		return ""
	}
	candidates := make([]string, 0, len(serviceUnitCandidates)+1)
	if base := filepath.Base(info.Binary); base != "" && base != "nginx" {
		candidates = append(candidates, base+".service")
	}
	candidates = append(candidates, serviceUnitCandidates...)
	seen := map[string]bool{}
	var script strings.Builder
	script.WriteString("for u in")
	for _, unit := range candidates {
		if unit == "" || seen[unit] {
			continue
		}
		seen[unit] = true
		script.WriteString(" ")
		script.WriteString(sshx.ShellQuote(unit))
	}
	script.WriteString("; do if systemctl cat \"$u\" >/dev/null 2>&1; then printf '%s' \"$u\"; exit 0; fi; done")
	out, perr := r.pool.Output(ctx, profile, script.String(), 10*time.Second)
	if perr != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// service.reload runs nginx -t first and refuses to reload a broken config.
// The UI must send confirm:true after its own explicit confirmation dialog.
// status is "applied" only when the reload command succeeded AND the error log
// has no emerg/alert entry at or after the reload start; otherwise "unknown".
func (r *Runtime) serviceReload(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
		Confirm      bool   `json:"confirm"`
		Method       string `json:"method,omitempty"` // auto|signal|systemd
	}](args)
	if perr != nil {
		return nil, perr
	}
	if !input.Confirm {
		return nil, rpcerr.E(rpcerr.CodeReloadConfirm, "reload requires an explicit confirmation")
	}
	info, perr := r.ensureInfo(ctx, input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	profile, perr := r.profile(input.ConnectionID)
	if perr != nil {
		return nil, perr
	}

	validation, perr := r.validateConfig(ctx, input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	if !validation.Ok {
		if permissionValidationFailure(validation.Messages) {
			return nil, rpcerr.E(rpcerr.CodeReloadBlocked,
				"nginx -t could not run with this SSH user's permissions; this is not a configuration error",
				map[string]any{"reason": "permission", "messages": validation.Messages})
		}
		return nil, rpcerr.E(rpcerr.CodeReloadBlocked, "nginx -t reported configuration errors; reload was refused",
			map[string]any{"messages": validation.Messages})
	}

	unit := ""
	method := input.Method
	if method == "" {
		method = "auto"
	}
	if method == "auto" {
		if info.Systemd {
			unit = r.detectServiceUnit(ctx, profile, info)
		}
		if unit != "" {
			method = "systemd"
		} else {
			method = "signal"
		}
	} else if method == "systemd" && unit == "" {
		unit = r.detectServiceUnit(ctx, profile, info)
		if unit == "" {
			unit = "nginx.service"
		}
	}

	startedAt := ""
	if out, perr := r.pool.Output(ctx, profile, "date '+%Y/%m/%d %H:%M:%S'", 5*time.Second); perr == nil {
		startedAt = strings.TrimSpace(out)
	}

	var command, output, stderr string
	fallbackUsed := false
	if method == "systemd" {
		command = "systemctl reload " + sshx.ShellQuote(unit)
	} else {
		command = info.quotedBinary() + " -s reload"
	}
	result, runErr := r.pool.RunOnce(ctx, profile, command+" 2>&1", 20*time.Second, 0)
	if runErr == nil && result.ExitCode == 0 {
		output = strings.TrimSpace(result.Stdout)
	} else {
		// A systemd reload can fail when the unit name is wrong or when the
		// service manager is degraded; falling back to a signal often works.
		if method == "systemd" {
			if runErr != nil {
				stderr = runErr.Message
			} else {
				stderr = strings.TrimSpace(result.Stdout + " " + result.Stderr)
			}
			fallbackUsed = true
			method = "signal"
			command = info.quotedBinary() + " -s reload"
			result, runErr = r.pool.RunOnce(ctx, profile, command+" 2>&1", 20*time.Second, 0)
		}
		if runErr == nil && result.ExitCode == 0 {
			output = strings.TrimSpace(result.Stdout)
		} else {
			detail := stderr
			if runErr != nil {
				detail = strings.TrimSpace(detail + " " + runErr.Message)
			} else {
				detail = strings.TrimSpace(detail + " " + result.Stdout + " " + result.Stderr)
			}
			data := map[string]any{"command": command, "output": strings.TrimSpace(detail)}
			if reloadPermissionFailure(detail) {
				data["reason"] = "permission"
				return nil, rpcerr.E(rpcerr.CodeExecFailed,
					"reload needs root privileges on the remote host; this plugin does not use sudo", data)
			}
			code := 0
			if result.ExitCode != 0 {
				code = result.ExitCode
			}
			data["exitCode"] = code
			return nil, rpcerr.E(rpcerr.CodeExecFailed, "reload command failed", data)
		}
	}

	// Verify the reload through the error log: a zero exit code only means the
	// signal/manager accepted the request, not that nginx re-read its config.
	errorLogPath, errorLogChecked, alerts := r.postReloadAlerts(ctx, input.ConnectionID, info, profile, startedAt)
	status := "applied"
	if len(alerts) > 0 || !errorLogChecked {
		status = "unknown"
	}
	return map[string]any{
		"method": method, "command": command, "output": output, "validated": true,
		"unit": unit, "status": status, "fallbackUsed": fallbackUsed,
		"startedAt": startedAt, "errorLogPath": errorLogPath,
		"errorLogChecked": errorLogChecked, "errorLogMatches": alerts,
	}, nil
}

func (r *Runtime) postReloadAlerts(ctx context.Context, connectionID string, info *HostInfo, profile store.Profile, startedAt string) (string, bool, []string) {
	target, perr := r.logTarget(ctx, connectionID, info, "error", "")
	if perr != nil || target.Path == "" {
		return "", false, []string{}
	}
	out, perr := r.pool.Output(ctx, profile, "tail -n 300 "+sshx.ShellQuote(target.Path)+" 2>/dev/null", 10*time.Second)
	if perr != nil {
		return target.Path, false, []string{}
	}
	return target.Path, true, postReloadAlerts(out, startedAt)
}

// postReloadAlerts returns [emerg]/[alert] lines written at or after the
// reload start. Timestamps use nginx's YYYY/MM/DD HH:MM:SS layout, so a
// lexicographic comparison of the first 19 bytes is exact.
func postReloadAlerts(content, startedAt string) []string {
	matches := []string{}
	for _, line := range splitLines(content) {
		if len(line) < 19 {
			continue
		}
		timestamp := line[:19]
		if timestamp[4] != '/' || timestamp[7] != '/' || timestamp[10] != ' ' {
			continue
		}
		if startedAt != "" && timestamp < startedAt {
			continue
		}
		level, _, ok := errorLineLevel(line)
		if ok && (level == "emerg" || level == "alert") {
			matches = append(matches, line)
			if len(matches) >= 20 {
				break
			}
		}
	}
	return matches
}

func permissionValidationFailure(messages []ValidationMessage) bool {
	for _, message := range messages {
		if looksLikePermissionFailure(message.Message) {
			return true
		}
	}
	return false
}

func reloadPermissionFailure(output string) bool {
	return looksLikePermissionFailure(output)
}

func looksLikePermissionFailure(text string) bool {
	lower := strings.ToLower(text)
	for _, marker := range []string{
		"permission denied", "operation not permitted", "access denied",
		"must be run as root", "must have privileges", "(13:", "authentication is required",
		"interactive authentication", "not authorized",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
