package commands

import (
	"context"
	"path"
	"strings"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/ngxconf"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// codePathNotAllowed is the application code the window UI localizes for
// paths outside the read allowlist. It mirrors the codes declared in
// internal/rpcerr without extending that package from the command layer.
const codePathNotAllowed = "PATH_NOT_ALLOWED"

// allowedPathRoots lists the read roots this plugin accepts for nginx
// configuration, log and certificate files. They cover the common distro,
// OpenResty, Homebrew (arm and Intel), custom /opt builds and TLS stores.
// The discovered nginx prefix and the directory of the main config file are
// appended at runtime, so a custom build is never locked out of its own tree.
//
// Note: the check is lexical. A symlink placed inside an allowed root can
// still point elsewhere; following it requires an extra remote round trip and
// is intentionally left to the SSH user's own filesystem permissions.
var allowedPathRoots = []string{
	"/etc/nginx",
	"/etc/ssl",
	"/etc/pki",
	"/etc/letsencrypt",
	"/var/log/nginx",
	"/usr/local/nginx/conf",
	"/usr/local/openresty/nginx/conf",
	"/usr/local/etc/nginx",
	"/opt/homebrew/etc/nginx",
	"/opt/nginx/conf",
	"/usr/share/nginx",
}

// pathRoots returns the allowed read roots for one discovered host.
func pathRoots(info *HostInfo) []string {
	roots := make([]string, 0, len(allowedPathRoots)+3)
	roots = append(roots, allowedPathRoots...)
	for _, candidate := range []string{info.Prefix, path.Dir(info.ConfPath)} {
		candidate = path.Clean(strings.TrimSpace(candidate))
		// Never accept the filesystem root: a broken --prefix=/ would
		// otherwise turn the whole host into an allowed read root.
		if candidate == "" || candidate == "/" || candidate == "." || !strings.HasPrefix(candidate, "/") {
			continue
		}
		roots = append(roots, candidate)
	}
	return dedupe(roots)
}

// resolveAllowedPath maps a caller-supplied path onto an absolute file inside
// the allowlist. Relative paths are rejected here: callers that accept nginx
// directive arguments use allowedRelativePath instead.
func (r *Runtime) resolveAllowedPath(ctx context.Context, connectionID, raw string) (string, *plugin.Error) {
	info, perr := r.ensureInfo(ctx, connectionID)
	if perr != nil {
		return "", perr
	}
	return allowPath(info, raw, false)
}

// allowedRelativePath is resolveAllowedPath for values that come from the
// merged nginx configuration (for example "ssl_certificate certs/a.pem"):
// relative paths are joined to the nginx prefix exactly like nginx does.
func allowedRelativePath(info *HostInfo, raw string) (string, *plugin.Error) {
	return allowPath(info, raw, true)
}

// allowPath validates and cleans raw. It rejects any ".." component before
// cleaning so a traversal attempt is reported as such instead of silently
// normalizing into an allowed directory.
func allowPath(info *HostInfo, raw string, allowRelative bool) (string, *plugin.Error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", rpcerr.E(rpcerr.CodeInvalidArgs, "path is required")
	}
	if len(raw) > 4096 || strings.ContainsAny(raw, "\x00\r\n") {
		return "", rpcerr.E(rpcerr.CodeInvalidArgs, "path contains unsupported characters")
	}
	for _, segment := range strings.Split(strings.ReplaceAll(raw, "\\", "/"), "/") {
		if segment == ".." {
			return "", pathNotAllowed(raw, info, "path traversal is not allowed")
		}
	}
	resolved := raw
	if !strings.HasPrefix(resolved, "/") {
		if !allowRelative {
			return "", rpcerr.E(rpcerr.CodeInvalidArgs, "path must be absolute")
		}
		prefix := strings.TrimSuffix(strings.TrimSpace(info.Prefix), "/")
		if prefix == "" || !strings.HasPrefix(prefix, "/") {
			return "", rpcerr.E(rpcerr.CodeInvalidArgs, "cannot resolve a relative path without a known nginx prefix")
		}
		resolved = prefix + "/" + resolved
	}
	clean := path.Clean(resolved)
	if clean == "/" {
		return "", pathNotAllowed(raw, info, "the filesystem root is not readable")
	}
	for _, root := range pathRoots(info) {
		if clean == root || strings.HasPrefix(clean, strings.TrimSuffix(root, "/")+"/") {
			return clean, nil
		}
	}
	return "", pathNotAllowed(clean, info, "path is outside the allowed nginx, log and certificate directories")
}

// walkDirectives visits every directive depth-first, including block bodies.
// certs and logs use it instead of the non-recursive ngxconf helpers so
// ssl_certificate and log directives inside server/location/if blocks are
// seen.
func walkDirectives(directives []ngxconf.Directive, visit func(*ngxconf.Directive)) {
	for i := range directives {
		directive := &directives[i]
		visit(directive)
		if len(directive.Block) > 0 {
			walkDirectives(directive.Block, visit)
		}
	}
}

func pathNotAllowed(raw string, info *HostInfo, reason string) *plugin.Error {
	return rpcerr.E(codePathNotAllowed, reason, map[string]any{
		"path":         raw,
		"allowedRoots": pathRoots(info),
	})
}
