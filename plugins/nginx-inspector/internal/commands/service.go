package commands

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
)

// ServiceStatus is the service.status response.
type ServiceStatus struct {
	Found    bool   `json:"found"`
	Version  string `json:"version,omitempty"`
	Binary   string `json:"binary,omitempty"`
	Systemd  bool   `json:"systemd"`
	Docker   bool   `json:"docker"`
	State    string `json:"state"` // active|inactive|failed|unknown
	PID      int    `json:"pid,omitempty"`
	Uptime   string `json:"uptime,omitempty"`
	PIDFile  string `json:"pidFile,omitempty"`
	Hostname string `json:"hostname,omitempty"`
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
		if out, perr := r.pool.Output(ctx, profile, "systemctl is-active nginx 2>&1", 10*time.Second); perr == nil {
			status.State = strings.TrimSpace(out)
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

// service.reload runs nginx -t first and refuses to reload a broken config.
// The UI must send confirm:true after its own explicit confirmation dialog.
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
		return nil, rpcerr.E(rpcerr.CodeReloadBlocked, "nginx -t reported configuration errors; reload was refused",
			map[string]any{"messages": validation.Messages})
	}

	var command, method string
	switch input.Method {
	case "signal":
		command, method = info.quotedBinary()+" -s reload", "signal"
	case "systemd":
		command, method = "systemctl reload nginx", "systemd"
	default:
		if info.Systemd {
			command, method = "systemctl reload nginx", "systemd"
		} else {
			command, method = info.quotedBinary()+" -s reload", "signal"
		}
	}
	result, perr := r.pool.Run(ctx, profile, command+" 2>&1", 20*time.Second, 0)
	if perr != nil {
		return nil, perr
	}
	if result.ExitCode != 0 {
		return nil, rpcerr.E(rpcerr.CodeExecFailed, "reload command failed with exit code "+strconv.Itoa(result.ExitCode),
			map[string]string{"command": command, "stderr": strings.TrimSpace(result.Stdout + " " + result.Stderr)})
	}
	return map[string]any{
		"method": method, "command": command,
		"output": strings.TrimSpace(result.Stdout), "validated": true,
	}, nil
}
