package commands

import (
	"context"
	"encoding/json"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/ngxconf"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
)

// Loader bounds keep every response inside the one MiB protocol frame and
// bound remote work so a hostile or cyclic config cannot run away.
const (
	maxConfigFileBytes = 512 << 10
	maxIncludeFiles    = 64
	maxIncludeDepth    = 8
	maxTreeNodes       = 4000
)

// ConfigNode is one directive in a rendered file tree.
type ConfigNode struct {
	Name     string       `json:"name"`
	Args     []string     `json:"args,omitempty"`
	Line     int          `json:"line"`
	File     string       `json:"file,omitempty"`
	Children []ConfigNode `json:"children,omitempty"`
}

// ConfigFileInfo is one parsed configuration file.
type ConfigFileInfo struct {
	Path       string       `json:"path"`
	SizeBytes  int64        `json:"sizeBytes"`
	Truncated  bool         `json:"truncated"`
	Directives []ConfigNode `json:"directives"`
}

// LocationInfo summarizes one location block.
type LocationInfo struct {
	Path      string `json:"path"`
	File      string `json:"file,omitempty"`
	Line      int    `json:"line"`
	ProxyPass string `json:"proxyPass,omitempty"`
	Root      string `json:"root,omitempty"`
	Alias     string `json:"alias,omitempty"`
}

// ServerInfo summarizes one server block.
type ServerInfo struct {
	File         string         `json:"file,omitempty"`
	Line         int            `json:"line"`
	Listen       []string       `json:"listen,omitempty"`
	Names        []string       `json:"names,omitempty"`
	SSL          bool           `json:"ssl"`
	Certificates []string       `json:"certificates,omitempty"`
	Locations    []LocationInfo `json:"locations,omitempty"`
}

// UpstreamMemberInfo is one upstream member.
type UpstreamMemberInfo struct {
	Address   string   `json:"address"`
	Params    []string `json:"params,omitempty"`
	Line      int      `json:"line"`
	Alive     *bool    `json:"alive,omitempty"`
	LatencyMs *int64   `json:"latencyMs,omitempty"`
}

// UpstreamInfo summarizes one upstream block.
type UpstreamInfo struct {
	Name    string               `json:"name"`
	File    string               `json:"file,omitempty"`
	Line    int                  `json:"line"`
	Servers []UpstreamMemberInfo `json:"servers,omitempty"`
}

// ConfigTree is the config.tree response.
type ConfigTree struct {
	MainPath      string           `json:"mainPath"`
	Prefix        string           `json:"prefix,omitempty"`
	Version       string           `json:"version,omitempty"`
	Files         []ConfigFileInfo `json:"files"`
	Servers       []ServerInfo     `json:"servers"`
	Upstreams     []UpstreamInfo   `json:"upstreams"`
	IncludedCount int              `json:"includedCount"`
	Truncated     bool             `json:"truncated"`
}

// loadedTree is the merged in-memory view shared by config, certs and upstreams.
type loadedTree struct {
	tree       *ConfigTree
	directives []ngxconf.Directive
	seen       map[string]bool
	budget     int
}

func (r *Runtime) loadTree(ctx context.Context, profileID string) (*loadedTree, *plugin.Error) {
	info, perr := r.ensureInfo(ctx, profileID)
	if perr != nil {
		return nil, perr
	}
	profile, perr := r.profile(profileID)
	if perr != nil {
		return nil, perr
	}
	tree := &ConfigTree{
		MainPath: info.ConfPath, Prefix: info.Prefix, Version: info.Version,
		Files: []ConfigFileInfo{}, Servers: []ServerInfo{}, Upstreams: []UpstreamInfo{},
	}
	loaded := &loadedTree{tree: tree, budget: maxTreeNodes, seen: map[string]bool{info.ConfPath: true}}

	main, perr := r.pool.ReadFile(ctx, profile, info.ConfPath, maxConfigFileBytes)
	if perr != nil {
		return nil, perr
	}
	directives, err := ngxconf.Parse(info.ConfPath, main.Content)
	if err != nil {
		return nil, rpcerr.E(rpcerr.CodeParseFailed, "parse "+info.ConfPath+": "+err.Error())
	}
	tree.Files = append(tree.Files, newFileInfo(info.ConfPath, main, directives, &loaded.budget))

	merged, perr := r.expandIncludes(ctx, profile, info, directives, 0, loaded)
	if perr != nil {
		return nil, perr
	}
	loaded.directives = merged
	tree.Servers = extractServers(merged)
	tree.Upstreams = extractUpstreams(merged)
	return loaded, nil
}

func newFileInfo(filePath string, read sshx.ReadResult, directives []ngxconf.Directive, budget *int) ConfigFileInfo {
	nodes := *budget
	return ConfigFileInfo{
		Path: filePath, SizeBytes: read.SizeBytes, Truncated: read.Truncated,
		Directives: toNodes(directives, &nodes),
	}
}

func toNodes(directives []ngxconf.Directive, budget *int) []ConfigNode {
	out := make([]ConfigNode, 0, len(directives))
	for i := range directives {
		if *budget <= 0 {
			return out
		}
		*budget--
		d := &directives[i]
		node := ConfigNode{Name: d.Name, Args: d.ArgValues(), Line: d.Line, File: d.File}
		if len(d.Block) > 0 {
			node.Children = toNodes(d.Block, budget)
		}
		out = append(out, node)
	}
	return out
}

// expandIncludes returns a merged copy where every include directive is
// replaced by the included file's directives; block children are expanded
// in place. Cycles are broken by path bookkeeping.
func (r *Runtime) expandIncludes(ctx context.Context, profile store.Profile, info *HostInfo, directives []ngxconf.Directive, depth int, loaded *loadedTree) ([]ngxconf.Directive, *plugin.Error) {
	if depth > maxIncludeDepth {
		loaded.tree.Truncated = true
		return directives, nil
	}
	out := make([]ngxconf.Directive, 0, len(directives))
	for i := range directives {
		d := directives[i]
		patterns, isInclude := ngxconf.IsInclude(&d)
		if !isInclude {
			if len(d.Block) > 0 {
				block, perr := r.expandIncludes(ctx, profile, info, d.Block, depth, loaded)
				if perr != nil {
					return nil, perr
				}
				d.Block = block
			}
			out = append(out, d)
			continue
		}
		for _, pattern := range patterns {
			targets, perr := r.resolveInclude(ctx, profile, info, pattern)
			if perr != nil {
				return nil, perr
			}
			for _, target := range targets {
				if loaded.tree.IncludedCount >= maxIncludeFiles {
					loaded.tree.Truncated = true
					break
				}
				if loaded.seen[target] {
					continue
				}
				loaded.seen[target] = true
				read, perr := r.pool.ReadFile(ctx, profile, target, maxConfigFileBytes)
				if perr != nil {
					out = append(out, ngxconf.Directive{
						Name: "#unreadable-include", Args: []ngxconf.Arg{{Value: target}}, Line: d.Line, File: d.File,
					})
					continue
				}
				child, err := ngxconf.Parse(target, read.Content)
				if err != nil {
					return nil, rpcerr.E(rpcerr.CodeParseFailed, "parse "+target+": "+err.Error())
				}
				loaded.tree.Files = append(loaded.tree.Files, newFileInfo(target, read, child, &loaded.budget))
				mergedChild, perr := r.expandIncludes(ctx, profile, info, child, depth+1, loaded)
				if perr != nil {
					return nil, perr
				}
				loaded.tree.IncludedCount++
				out = append(out, mergedChild...)
			}
		}
	}
	return out, nil
}

func (r *Runtime) resolveInclude(ctx context.Context, profile store.Profile, info *HostInfo, pattern string) ([]string, *plugin.Error) {
	resolved := pattern
	if !strings.HasPrefix(resolved, "/") {
		resolved = info.Prefix + "/" + resolved
	}
	resolved = path.Clean(resolved)
	if len(resolved) > 4096 || strings.ContainsAny(resolved, "\x00\n\r") {
		return nil, nil
	}
	if !strings.ContainsAny(resolved, "*?[") {
		return []string{resolved}, nil
	}
	out, perr := r.pool.Output(ctx, profile, "ls -1d "+sshx.ShellQuote(resolved)+" 2>/dev/null | head -n 65", 10*time.Second)
	if perr != nil {
		return nil, nil
	}
	var targets []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/") {
			targets = append(targets, line)
		}
		if len(targets) >= maxIncludeFiles {
			break
		}
	}
	return targets, nil
}

func extractServers(directives []ngxconf.Directive) []ServerInfo {
	out := []ServerInfo{}
	for _, server := range ngxconf.FindServers(directives) {
		info := ServerInfo{
			File: server.File, Line: server.Line,
			Listen: ngxconf.ListenAddresses(server), Names: ngxconf.ServerName(server),
			SSL: ngxconf.ServerEnabled(server),
		}
		info.Certificates = dedupe(ngxconf.AllDirectiveArgs(server.Block, "ssl_certificate"))
		for _, loc := range ngxconf.FindLocations(server) {
			info.Locations = append(info.Locations, LocationInfo{
				Path: ngxconf.LocationPath(loc), File: loc.File, Line: loc.Line,
				ProxyPass: ngxconf.ProxyPass(loc),
				Root:      ngxconf.FirstDirective(loc.Block, "root"),
				Alias:     ngxconf.FirstDirective(loc.Block, "alias"),
			})
		}
		out = append(out, info)
	}
	return out
}

func extractUpstreams(directives []ngxconf.Directive) []UpstreamInfo {
	out := []UpstreamInfo{}
	for _, upstream := range ngxconf.FindUpstreams(directives) {
		info := UpstreamInfo{Name: upstream.FirstArg(), File: upstream.File, Line: upstream.Line}
		for _, member := range ngxconf.UpstreamMembers(upstream) {
			info.Servers = append(info.Servers, UpstreamMemberInfo{
				Address: member.Address, Params: member.Params, Line: member.Line,
			})
		}
		out = append(out, info)
	}
	return out
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func (r *Runtime) configTree(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	loaded, perr := r.loadTree(ctx, input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	return loaded.tree, nil
}

func (r *Runtime) configGet(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
		Path         string `json:"path"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	profile, perr := r.profile(input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	read, perr := r.pool.ReadFile(ctx, profile, input.Path, maxConfigFileBytes)
	if perr != nil {
		return nil, perr
	}
	return map[string]any{
		"path": read.Path, "content": read.Content, "sizeBytes": read.SizeBytes,
		"truncated": read.Truncated, "lineCount": strings.Count(read.Content, "\n") + 1,
	}, nil
}

var nginxTestMessage = regexp.MustCompile(`nginx: \[(\w+)\] (.*)`)
var nginxTestLocation = regexp.MustCompile(`^(.*) in (.+):(\d+)$`)

// ValidationMessage is one parsed line of nginx -t output.
type ValidationMessage struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// ValidationResult is the config.validate response.
type ValidationResult struct {
	Ok       bool                `json:"ok"`
	Binary   string              `json:"binary"`
	ExitCode int                 `json:"exitCode"`
	Messages []ValidationMessage `json:"messages"`
}

func (r *Runtime) validateConfig(ctx context.Context, connectionID string) (*ValidationResult, *plugin.Error) {
	info, perr := r.ensureInfo(ctx, connectionID)
	if perr != nil {
		return nil, perr
	}
	profile, perr := r.profile(connectionID)
	if perr != nil {
		return nil, perr
	}
	result, perr := r.pool.Run(ctx, profile, info.quotedBinary()+" -t 2>&1", 20*time.Second, 0)
	if perr != nil {
		return nil, perr
	}
	messages := []ValidationMessage{}
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := nginxTestMessage.FindStringSubmatch(line)
		if m == nil {
			messages = append(messages, ValidationMessage{Level: "info", Message: line})
			continue
		}
		entry := ValidationMessage{Level: strings.ToLower(m[1]), Message: strings.TrimSpace(m[2])}
		if loc := nginxTestLocation.FindStringSubmatch(entry.Message); loc != nil {
			entry.Message = strings.TrimSpace(loc[1])
			entry.File = loc[2]
			entry.Line, _ = strconv.Atoi(loc[3])
		}
		messages = append(messages, entry)
	}
	return &ValidationResult{Ok: result.ExitCode == 0, Binary: info.Binary, ExitCode: result.ExitCode, Messages: messages}, nil
}

func (r *Runtime) configValidate(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	return r.validateConfig(ctx, input.ConnectionID)
}
