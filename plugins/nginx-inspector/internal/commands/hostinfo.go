package commands

import (
	"context"
	"encoding/json"
	"path"
	"strings"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// hostInfoTTL bounds how long a discovery result is reused. nginx can be
// upgraded or moved while the plugin is open, so the cache must expire.
const hostInfoTTL = 10 * time.Minute

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
	Container    bool   `json:"container"`
	Kernel       string `json:"kernel,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
	Locale       string `json:"locale,omitempty"`
	DiscoveredAt string `json:"discoveredAt,omitempty"`
}

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
		if infoFresh(cached) {
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

func infoFresh(info *HostInfo) bool {
	if info == nil {
		return false
	}
	discovered, err := time.Parse(time.RFC3339, info.DiscoveredAt)
	if err != nil {
		return false
	}
	return time.Since(discovered) < hostInfoTTL
}

// discoveryScript collects the binary, the two version outputs, the service
// environment and the machine facts in one SSH round trip. Values are emitted
// under @@ markers because -V output contains spaces and can span lines.
const discoveryScript = `BIN=$(command -v nginx 2>/dev/null)
if [ -z "$BIN" ]; then
  for p in /usr/local/openresty/nginx/sbin/nginx /usr/local/openresty/bin/openresty /opt/homebrew/opt/openresty/nginx/sbin/nginx /usr/local/nginx/sbin/nginx /usr/sbin/nginx /usr/local/sbin/nginx /usr/local/bin/nginx /opt/homebrew/bin/nginx /opt/nginx/sbin/nginx; do
    if [ -x "$p" ]; then BIN="$p"; break; fi
  done
fi
echo "@@BIN"
echo "$BIN"
if [ -n "$BIN" ]; then
  echo "@@V"
  "$BIN" -v 2>&1
  echo "@@VV"
  "$BIN" -V 2>&1
fi
echo "@@SYS"
if [ -d /run/systemd/system ]; then echo yes; else echo no; fi
echo "@@DOCKER"
if [ -f /.dockerenv ] || [ -f /run/.containerenv ]; then echo yes; else echo no; fi
echo "@@CGROUP"
cat /proc/1/cgroup 2>/dev/null
echo "@@KERNEL"
uname -sr
echo "@@HOST"
uname -n`

var discoveryMarkers = []string{"@@BIN", "@@V", "@@VV", "@@SYS", "@@DOCKER", "@@CGROUP", "@@KERNEL", "@@HOST"}

// discover probes the remote machine for the nginx binary, version, paths
// and service environment. It never fails when nginx is absent: Found=false
// is a valid result so the overview can still show machine facts.
func (r *Runtime) discover(ctx context.Context, profile store.Profile) (*HostInfo, *plugin.Error) {
	info := &HostInfo{DiscoveredAt: time.Now().UTC().Format(time.RFC3339)}
	out, perr := r.pool.Output(ctx, profile, discoveryScript, 20*time.Second)
	if perr != nil {
		return nil, perr
	}
	sections := splitDiscoveryOutput(out)
	if binary := strings.TrimSpace(sections["@@BIN"]); binary != "" {
		info.Found = true
		info.Binary = binary
	}
	if info.Found {
		info.Version = parseNginxVersion(sections["@@V"])
		if info.Version == "" {
			info.Version = parseNginxVersion(sections["@@VV"])
		}
		configured := parseConfigureArgs(sections["@@VV"])
		info.Prefix = configured["prefix"]
		info.ConfPath = configured["conf-path"]
		info.ErrorLogPath = configured["error-log-path"]
		info.HTTPLogPath = configured["http-log-path"]
		info.PIDPath = configured["pid-path"]
	}
	if info.ConfPath == "" {
		if info.Prefix != "" {
			info.ConfPath = path.Join(info.Prefix, "conf", "nginx.conf")
		} else {
			info.ConfPath = "/etc/nginx/nginx.conf"
		}
	}
	if info.Prefix == "" {
		info.Prefix = "/etc/nginx"
	}
	if info.HTTPLogPath == "" && info.Found {
		info.HTTPLogPath = path.Join(info.Prefix, "logs", "access.log")
	}
	if info.ErrorLogPath == "" && info.Found {
		info.ErrorLogPath = path.Join(info.Prefix, "logs", "error.log")
	}
	if info.PIDPath == "" && info.Found {
		info.PIDPath = path.Join(info.Prefix, "logs", "nginx.pid")
	}
	info.Systemd = strings.TrimSpace(sections["@@SYS"]) == "yes"
	info.Docker = strings.TrimSpace(sections["@@DOCKER"]) == "yes"
	cgroup := strings.ToLower(sections["@@CGROUP"])
	for _, marker := range []string{"docker", "kubepods", "containerd", "lxc", "podman"} {
		if strings.Contains(cgroup, marker) {
			info.Docker = true
			break
		}
	}
	info.Container = info.Docker
	info.Kernel = strings.TrimSpace(sections["@@KERNEL"])
	info.Hostname = strings.TrimSpace(sections["@@HOST"])
	return info, nil
}

// splitDiscoveryOutput slices the @@-delimited output into sections.
func splitDiscoveryOutput(out string) map[string]string {
	sections := map[string]string{}
	current := ""
	var lines []string
	flush := func() {
		if current != "" {
			sections[current] = strings.Join(lines, "\n")
		}
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSuffix(line, "\r")
		marker := ""
		for _, candidate := range discoveryMarkers {
			if line == candidate {
				marker = candidate
				break
			}
		}
		if marker != "" {
			flush()
			current = marker
			lines = nil
			continue
		}
		if current != "" {
			lines = append(lines, line)
		}
	}
	flush()
	return sections
}

// parseNginxVersion handles nginx, OpenResty and Tengine version banners.
func parseNginxVersion(out string) string {
	for _, field := range strings.Fields(out) {
		lower := strings.ToLower(field)
		for _, prefix := range []string{"nginx/", "openresty/", "tengine/"} {
			if strings.HasPrefix(lower, prefix) {
				return strings.TrimSuffix(field[len(prefix):], ",")
			}
		}
	}
	return ""
}

// parseConfigureArgs extracts --key=value tokens from "nginx -V", honouring
// single/double quotes so --prefix='/a b' stays one value.
func parseConfigureArgs(out string) map[string]string {
	values := map[string]string{}
	for _, token := range splitShellTokens(out) {
		if !strings.HasPrefix(token, "--") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(token, "--"), "=")
		if !ok || value == "" {
			continue
		}
		switch key {
		case "prefix", "conf-path", "error-log-path", "http-log-path", "pid-path", "lock-path", "sbin-path":
			values[key] = value
		}
	}
	return values
}

// splitShellTokens splits a command line into tokens using shell quoting
// rules. It deliberately does not expand variables.
func splitShellTokens(s string) []string {
	var tokens []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == rune(92) && quote != rune(39): // backslash outside single quotes
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == rune(39) || r == rune(34):
			quote = r
		case r == rune(32) || r == rune(9) || r == rune(13) || r == rune(10):
			flush()
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return tokens
}

// ensureInfo returns cached or freshly discovered host info and fails when
// nginx is absent, for commands that require the binary.
func (r *Runtime) ensureInfo(ctx context.Context, profileID string) (*HostInfo, *plugin.Error) {
	r.mu.Lock()
	cached := r.info[profileID]
	r.mu.Unlock()
	if infoFresh(cached) {
		return requireFound(cached)
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
