package commands

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// HostInfo is the discovered nginx installation of one remote machine.
type HostInfo struct {
	Found        bool   `json:"found"`
	Binary       string `json:"binary,omitempty"`
	Version      string `json:"version,omitempty"`
	Prefix       string `json:"prefix,omitempty"`
	ConfPath     string `json:"confPath,omitempty"`
	ErrorLogPath string `json:"errorLogPath,omitempty"`
	HTTPLogPath  string `json:"httpLogPath,omitempty"`
	PIDPath      string `json:"pidPath,omitempty"`
	Systemd      bool   `json:"systemd"`
	Docker       bool   `json:"docker"`
	Kernel       string `json:"kernel,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
	Locale       string `json:"locale,omitempty"`
	DiscoveredAt string `json:"discoveredAt,omitempty"`
}

var versionRegex = regexp.MustCompile(`nginx/(\S+)`)

func (r *Runtime) hostInfo(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
		Refresh      bool   `json:"refresh,omitempty"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	profile, perr := r.profile(input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	if !input.Refresh {
		r.mu.Lock()
		cached := r.info[profile.ID]
		r.mu.Unlock()
		if cached != nil {
			return cached, nil
		}
	}
	info, perr := r.discover(ctx, profile)
	if perr != nil {
		return nil, perr
	}
	info.Locale = r.Locale()
	r.mu.Lock()
	r.info[profile.ID] = info
	r.mu.Unlock()
	return info, nil
}

// discover probes the remote machine for the nginx binary, version, paths
// and service environment. It never fails when nginx is absent: Found=false
// is a valid result so the overview can still show machine facts.
func (r *Runtime) discover(ctx context.Context, profile store.Profile) (*HostInfo, *plugin.Error) {
	info := &HostInfo{DiscoveredAt: time.Now().UTC().Format(time.RFC3339)}
	binary := ""
	if out, perr := r.pool.Output(ctx, profile, "command -v nginx 2>/dev/null", 10*time.Second); perr == nil {
		binary = strings.TrimSpace(out)
	}
	if binary == "" {
		candidates := []string{
			"/usr/sbin/nginx", "/usr/local/nginx/sbin/nginx", "/usr/local/sbin/nginx",
			"/usr/local/bin/nginx", "/opt/nginx/sbin/nginx", "/opt/homebrew/bin/nginx",
		}
		for _, candidate := range candidates {
			out, perr := r.pool.Output(ctx, profile, "test -x "+sshx.ShellQuote(candidate)+" && echo yes", 5*time.Second)
			if perr == nil && strings.TrimSpace(out) == "yes" {
				binary = candidate
				break
			}
		}
	}
	if binary == "" {
		return r.basicInfo(ctx, profile, info)
	}
	info.Found = true
	info.Binary = binary
	quoted := sshx.ShellQuote(binary)

	if out, perr := r.pool.Output(ctx, profile, quoted+" -v 2>&1", 10*time.Second); perr == nil {
		if m := versionRegex.FindStringSubmatch(out); m != nil {
			info.Version = m[1]
		}
	}
	if out, perr := r.pool.Output(ctx, profile, quoted+" -V 2>&1", 10*time.Second); perr == nil {
		for _, arg := range strings.Fields(out) {
			switch {
			case strings.HasPrefix(arg, "--prefix="):
				info.Prefix = strings.TrimPrefix(arg, "--prefix=")
			case strings.HasPrefix(arg, "--conf-path="):
				info.ConfPath = strings.TrimPrefix(arg, "--conf-path=")
			case strings.HasPrefix(arg, "--error-log-path="):
				info.ErrorLogPath = strings.TrimPrefix(arg, "--error-log-path=")
			case strings.HasPrefix(arg, "--http-log-path="):
				info.HTTPLogPath = strings.TrimPrefix(arg, "--http-log-path=")
			case strings.HasPrefix(arg, "--pid-path="):
				info.PIDPath = strings.TrimPrefix(arg, "--pid-path=")
			}
		}
	}
	if info.ConfPath == "" {
		info.ConfPath = "/etc/nginx/nginx.conf"
	}
	if info.Prefix == "" {
		info.Prefix = "/etc/nginx"
	}
	return r.basicInfo(ctx, profile, info)
}

func (r *Runtime) basicInfo(ctx context.Context, profile store.Profile, info *HostInfo) (*HostInfo, *plugin.Error) {
	if out, perr := r.pool.Output(ctx, profile, "test -d /run/systemd/system && echo yes || echo no", 5*time.Second); perr == nil {
		info.Systemd = strings.TrimSpace(out) == "yes"
	}
	if out, perr := r.pool.Output(ctx, profile, "test -f /.dockerenv && echo yes || echo no", 5*time.Second); perr == nil {
		info.Docker = strings.TrimSpace(out) == "yes"
	}
	if out, perr := r.pool.Output(ctx, profile, "uname -sr", 5*time.Second); perr == nil {
		info.Kernel = strings.TrimSpace(out)
	}
	if out, perr := r.pool.Output(ctx, profile, "uname -n", 5*time.Second); perr == nil {
		info.Hostname = strings.TrimSpace(out)
	}
	return info, nil
}

// ensureInfo returns cached or freshly discovered host info and fails when
// nginx is absent, for commands that require the binary.
func (r *Runtime) ensureInfo(ctx context.Context, profileID string) (*HostInfo, *plugin.Error) {
	r.mu.Lock()
	cached := r.info[profileID]
	r.mu.Unlock()
	if cached != nil {
		info, perr := requireFound(cached)
		if perr != nil {
			return nil, perr
		}
		return info, nil
	}
	profile, perr := r.profile(profileID)
	if perr != nil {
		return nil, perr
	}
	info, perr := r.discover(ctx, profile)
	if perr != nil {
		return nil, perr
	}
	info.Locale = r.Locale()
	r.mu.Lock()
	r.info[profile.ID] = info
	r.mu.Unlock()
	return requireFound(info)
}

func requireFound(info *HostInfo) (*HostInfo, *plugin.Error) {
	if !info.Found {
		return nil, rpcerr.E(rpcerr.CodeNginxNotFound, "nginx binary was not found on the remote host")
	}
	return info, nil
}

// quotedBinary returns the shell-quoted nginx binary path.
func (info *HostInfo) quotedBinary() string {
	return sshx.ShellQuote(info.Binary)
}
