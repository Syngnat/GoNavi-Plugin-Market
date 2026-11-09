package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/ngxconf"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// Loader bounds keep every response inside the one MiB protocol frame and
// bound remote work so a hostile or cyclic config cannot run away.
const (
	maxConfigFileBytes = 512 << 10
	maxIncludeFiles    = 64
	maxIncludeDepth    = 8
	maxTreeNodes       = 4000
	maxWarnings        = 32
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
	// Error explains why the tree is partial when a hard parse error still
	// left a usable prefix; Warnings lists non-fatal degradations (truncated
	// files, unresolved globs, skipped includes) so the UI can surface them.
	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// loadedTree is the merged in-memory view shared by config, certs and upstreams.
type loadedTree struct {
	tree       *ConfigTree
	directives []ngxconf.Directive
	// stack holds the files being expanded right now, so include cycles are
	// broken without suppressing legitimate repeats: nginx parses an included
	// file again every time another branch includes it.
	stack   map[string]bool
	budget  int
	dropped int
}

// treeCacheTTL bounds how stale a config.tree response may be when a page does
// not ask for refresh. Two minutes keeps navigation snappy while still picking
// up edits made outside the plugin.
const treeCacheTTL = 2 * time.Minute

type cachedTree struct {
	loaded *loadedTree
	at     time.Time
}

// invalidateTree drops the cached merged config for one profile. It is called
// whenever the profile, host discovery or effective configuration may change.
func (r *Runtime) invalidateTree(profileID string) {
	r.treeMu.Lock()
	delete(r.trees, profileID)
	r.treeMu.Unlock()
}

// loadTreeCached serves config.tree, certs.list, logs and upstreams from one
// shared parse per profile. refresh bypasses and replaces the cached copy, so
// the UI can force a re-read without waiting for the TTL.
func (r *Runtime) loadTreeCached(ctx context.Context, profileID string, refresh bool) (*loadedTree, *plugin.Error) {
	if refresh {
		r.invalidateTree(profileID)
	} else {
		r.treeMu.Lock()
		entry, ok := r.trees[profileID]
		r.treeMu.Unlock()
		if ok && time.Since(entry.at) < treeCacheTTL {
			return entry.loaded, nil
		}
	}
	loaded, perr := r.loadTree(ctx, profileID)
	if perr != nil {
		return nil, perr
	}
	r.treeMu.Lock()
	if r.trees == nil {
		r.trees = map[string]cachedTree{}
	}
	r.trees[profileID] = cachedTree{loaded: loaded, at: time.Now()}
	r.treeMu.Unlock()
	return loaded, nil
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
	// nginx -T prints every file it parsed in one round trip. That turns a cold
	// 28-file tree from dozens of SSH commands into one; any failure (old nginx,
	// permission error, truncated dump) falls through to the per-file reader.
	if loaded, ok := r.loadTreeFromDump(ctx, profile, info); ok {
		return loaded, nil
	}
	return buildTree(ctx, r.pool, profile, info)
}

// loadTreeFromDump parses nginx -T output into the same loadedTree the per-file
// path builds. ok is false when the dump is unusable for any reason.
func (r *Runtime) loadTreeFromDump(ctx context.Context, profile store.Profile, info *HostInfo) (*loadedTree, bool) {
	result, perr := r.pool.Run(ctx, profile, info.quotedBinary()+" -T", 20*time.Second, sshx.MaxOutputBytes)
	if perr != nil || result.ExitCode != 0 || result.Truncated {
		return nil, false
	}
	files, ok := parseConfigDump(result.Stdout)
	if !ok {
		return nil, false
	}
	loaded, perr := buildTree(ctx, &dumpReader{files: files}, profile, info)
	if perr != nil {
		return nil, false
	}
	return loaded, true
}

// buildTree reads the main file through reader, expands includes and fills in
// the parsed summary. reader is the SSH pool in production and a local dump in
// the nginx -T fast path.
func buildTree(ctx context.Context, reader configReader, profile store.Profile, info *HostInfo) (*loadedTree, *plugin.Error) {
	tree := &ConfigTree{
		MainPath: info.ConfPath, Prefix: info.Prefix, Version: info.Version,
		Files: []ConfigFileInfo{}, Servers: []ServerInfo{}, Upstreams: []UpstreamInfo{},
	}
	loaded := &loadedTree{tree: tree, budget: maxTreeNodes, stack: map[string]bool{info.ConfPath: true}}

	main, perr := reader.ReadFile(ctx, profile, info.ConfPath, maxConfigFileBytes)
	if perr != nil {
		return nil, perr
	}
	directives, perr := parseConfigFile(info.ConfPath, main, tree)
	if perr != nil {
		return nil, perr
	}
	mainInfo, exhausted := newFileInfo(info.ConfPath, main, directives, &loaded.budget)
	if exhausted {
		tree.Truncated = true
	}
	tree.Files = append(tree.Files, mainInfo)

	merged, perr := expandIncludes(ctx, reader, profile, info, directives, 0, loaded)
	if perr != nil {
		return nil, perr
	}
	if loaded.dropped > 0 {
		tree.Truncated = true
		addWarning(tree, fmt.Sprintf("include limit of %d files reached: %d include(s) skipped", maxIncludeFiles, loaded.dropped))
	}
	loaded.directives = merged
	tree.Servers = extractServers(merged)
	tree.Upstreams = extractUpstreams(merged)
	return loaded, nil
}

// addWarning records one de-duplicated note, bounded so a pathological config
// cannot grow the response without limit.
func addWarning(tree *ConfigTree, message string) {
	for _, existing := range tree.Warnings {
		if existing == message {
			return
		}
	}
	if len(tree.Warnings) >= maxWarnings {
		return
	}
	tree.Warnings = append(tree.Warnings, message)
}

// newFileInfo converts one parsed file into the response shape, charging every
// node against the shared budget. It reports whether the budget ran out, which
// marks the whole tree truncated instead of silently shrinking later files.
func newFileInfo(filePath string, read sshx.ReadResult, directives []ngxconf.Directive, budget *int) (ConfigFileInfo, bool) {
	info := ConfigFileInfo{Path: filePath, SizeBytes: read.SizeBytes, Truncated: read.Truncated}
	exhausted := false
	info.Directives = toNodes(directives, budget, &exhausted)
	if exhausted {
		info.Truncated = true
	}
	return info, exhausted
}

func toNodes(directives []ngxconf.Directive, budget *int, truncated *bool) []ConfigNode {
	out := make([]ConfigNode, 0, len(directives))
	for i := range directives {
		if *budget <= 0 {
			*truncated = true
			return out
		}
		*budget--
		d := &directives[i]
		node := ConfigNode{Name: d.Name, Args: d.ArgValues(), Line: d.Line, File: d.File}
		if len(d.Block) > 0 {
			node.Children = toNodes(d.Block, budget, truncated)
		}
		out = append(out, node)
	}
	return out
}

// parseConfigFile parses one file. A byte-truncated tail is cut back to the
// last complete directive before parsing, and a hard parse error degrades to
// the parsed prefix plus a warning instead of failing the whole page.
func parseConfigFile(filePath string, read sshx.ReadResult, tree *ConfigTree) ([]ngxconf.Directive, *plugin.Error) {
	content := read.Content
	truncatedTail := false
	if read.Truncated {
		kept := ngxconf.CompleteBoundary(content)
		content = content[:kept]
		truncatedTail = true
		tree.Truncated = true
		addWarning(tree, fmt.Sprintf("%s is larger than %d KiB: parsed the first %d bytes", filePath, maxConfigFileBytes>>10, kept))
	}
	directives, err := ngxconf.ParsePartial(filePath, content)
	if err == nil {
		return directives, nil
	}
	message := "parse " + filePath + ": " + err.Error()
	// A cut inside an open block ends with an EOF error: expected for a
	// truncated file, not evidence of a broken configuration.
	if truncatedTail && isTruncationEOF(err) {
		addWarning(tree, message)
		return directives, nil
	}
	if len(directives) > 0 {
		tree.Truncated = true
		tree.Error = message
		addWarning(tree, "showing the parsed prefix: "+message)
		return directives, nil
	}
	return nil, rpcerr.E(rpcerr.CodeParseFailed, message)
}

// isTruncationEOF reports whether err belongs to the "input ended early" class
// of parse failure produced by cutting a file at a byte boundary.
func isTruncationEOF(err error) bool {
	message := err.Error()
	return strings.Contains(message, "unexpected end of file") || strings.Contains(message, "directive missing terminator")
}

// configReader is the remote file/command surface include expansion needs;
// *sshx.Pool is the production implementation and tests inject a fake.
type configReader interface {
	ReadFile(ctx context.Context, prof store.Profile, path string, maxBytes int64) (sshx.ReadResult, *plugin.Error)
	Output(ctx context.Context, prof store.Profile, command string, timeout time.Duration) (string, *plugin.Error)
}

// includeMatcher is implemented by readers that already know every file nginx
// parsed (the -T dump), so include globs resolve locally instead of spawning a
// shell for each pattern. handled=false falls back to Output and the shell.
type includeMatcher interface {
	MatchInclude(pattern string) ([]string, bool)
}

// expandIncludes returns a merged copy where every include directive is
// replaced by the included file's directives; block children are expanded in
// place. Only the active recursion stack is de-duplicated, matching
// ngx_conf_include: a snippet included by several servers is parsed again for
// each of them, so no server silently loses ssl or proxy directives.
func expandIncludes(ctx context.Context, reader configReader, profile store.Profile, info *HostInfo, directives []ngxconf.Directive, depth int, loaded *loadedTree) ([]ngxconf.Directive, *plugin.Error) {
	if depth > maxIncludeDepth {
		loaded.tree.Truncated = true
		addWarning(loaded.tree, fmt.Sprintf("include nesting deeper than %d levels: expansion stopped", maxIncludeDepth))
		return directives, nil
	}
	out := make([]ngxconf.Directive, 0, len(directives))
	for i := range directives {
		d := directives[i]
		patterns, isInclude := ngxconf.IsInclude(&d)
		if !isInclude {
			if len(d.Block) > 0 {
				block, perr := expandIncludes(ctx, reader, profile, info, d.Block, depth, loaded)
				if perr != nil {
					return nil, perr
				}
				d.Block = block
			}
			out = append(out, d)
			continue
		}
		for _, pattern := range patterns {
			targets, perr := resolveInclude(ctx, reader, profile, info, pattern)
			if perr != nil {
				out = append(out, includePlaceholder("#unresolved-include", d, pattern, perr.Message))
				continue
			}
			if len(targets) == 0 {
				out = append(out, includePlaceholder("#unresolved-include", d, pattern, "pattern matched no files"))
				continue
			}
			for _, target := range targets {
				if loaded.tree.IncludedCount >= maxIncludeFiles {
					loaded.tree.Truncated = true
					loaded.dropped++
					continue
				}
				if loaded.stack[target] {
					out = append(out, includePlaceholder("#include-cycle", d, target, "include cycle broken"))
					continue
				}
				read, perr := reader.ReadFile(ctx, profile, target, maxConfigFileBytes)
				if perr != nil {
					out = append(out, includePlaceholder("#unreadable-include", d, target, perr.Message))
					continue
				}
				child, perr := parseConfigFile(target, read, loaded.tree)
				if perr != nil {
					return nil, perr
				}
				fileInfo, exhausted := newFileInfo(target, read, child, &loaded.budget)
				if exhausted {
					loaded.tree.Truncated = true
				}
				loaded.tree.Files = append(loaded.tree.Files, fileInfo)
				loaded.stack[target] = true
				mergedChild, perr := expandIncludes(ctx, reader, profile, info, child, depth+1, loaded)
				delete(loaded.stack, target)
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

// includePlaceholder builds the visible marker node that keeps an include
// problem in the tree instead of dropping it silently.
func includePlaceholder(name string, at ngxconf.Directive, target, reason string) ngxconf.Directive {
	args := []ngxconf.Arg{{Value: target}}
	if reason != "" {
		args = append(args, ngxconf.Arg{Value: reason})
	}
	return ngxconf.Directive{Name: name, Args: args, Line: at.Line, File: at.File}
}

// resolveInclude turns one include argument into concrete file paths. A glob
// must reach the remote shell unquoted to expand at all, so the far side runs
// a for loop and prints one hit per line; without this every standard
// "include /etc/nginx/conf.d/*.conf" silently resolved to nothing.
func resolveInclude(ctx context.Context, reader configReader, profile store.Profile, info *HostInfo, pattern string) ([]string, *plugin.Error) {
	resolved := pattern
	if !strings.HasPrefix(resolved, "/") {
		resolved = info.Prefix + "/" + resolved
	}
	resolved = path.Clean(resolved)
	if len(resolved) > 4096 || strings.ContainsAny(resolved, "\x00\n\r") {
		return nil, rpcerr.E(rpcerr.CodeInvalidArgs, "include path is empty or too long")
	}
	if matcher, ok := reader.(includeMatcher); ok {
		if targets, handled := matcher.MatchInclude(resolved); handled {
			return targets, nil
		}
	}
	if !strings.ContainsAny(resolved, "*?[") {
		return []string{resolved}, nil
	}
	command, ok := globIncludeCommand(resolved)
	if !ok {
		return nil, rpcerr.E(rpcerr.CodeInvalidArgs, "include pattern contains characters that cannot be expanded safely",
			map[string]string{"pattern": resolved})
	}
	out, perr := reader.Output(ctx, profile, command, 10*time.Second)
	if perr != nil {
		return nil, perr
	}
	targets := make([]string, 0, 8)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "/") {
			continue
		}
		// Keep one entry past the cap so the caller can tell "exactly the cap"
		// from "more files than the cap" and mark the tree truncated.
		targets = append(targets, line)
		if len(targets) > maxIncludeFiles {
			break
		}
	}
	return targets, nil
}

const configDumpMarker = "# configuration file "

// parseConfigDump splits nginx -T output into one entry per parsed file. The
// marker is emitted by nginx as "# configuration file <absolute path>:" and the
// file body follows until the next marker. ok is false when no section was
// found, which sends the caller back to the per-file reader.
func parseConfigDump(dump string) (map[string]string, bool) {
	files := map[string]string{}
	current := ""
	var body strings.Builder
	flush := func() {
		if current != "" {
			files[current] = body.String()
		}
		body.Reset()
	}
	for _, line := range strings.SplitAfter(dump, "\n") {
		trimmed := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if strings.HasPrefix(trimmed, configDumpMarker) && strings.HasSuffix(trimmed, ":") {
			flush()
			current = ""
			filePath := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, configDumpMarker), ":"))
			if strings.HasPrefix(filePath, "/") && !strings.ContainsAny(filePath, "\x00\n\r") {
				current = filePath
			}
			continue
		}
		if current != "" {
			body.WriteString(line)
		}
	}
	flush()
	return files, len(files) > 0
}

// dumpReader serves the files nginx -T reported. Include globs are matched
// locally against those exact paths, so the fast path needs no remote shell.
type dumpReader struct {
	files map[string]string
}

func (d *dumpReader) ReadFile(ctx context.Context, prof store.Profile, filePath string, maxBytes int64) (sshx.ReadResult, *plugin.Error) {
	content, ok := d.files[filePath]
	if !ok {
		return sshx.ReadResult{}, rpcerr.E(rpcerr.CodeFileUnreadable, "config dump does not contain "+filePath)
	}
	result := sshx.ReadResult{Path: filePath, Content: content, SizeBytes: int64(len(content))}
	if maxBytes > 0 && int64(len(result.Content)) > maxBytes {
		result.Content = result.Content[:maxBytes]
		result.Truncated = true
	}
	return result, nil
}

func (d *dumpReader) Output(ctx context.Context, prof store.Profile, command string, timeout time.Duration) (string, *plugin.Error) {
	return "", rpcerr.E(rpcerr.CodeExecFailed, "config dump reader cannot run remote shell commands")
}

// MatchInclude resolves one cleaned absolute include pattern against the dump
// paths. A literal pattern is always returned so a missing file still surfaces
// as an unreadable include instead of disappearing.
func (d *dumpReader) MatchInclude(pattern string) ([]string, bool) {
	if !strings.ContainsAny(pattern, "*?[") {
		return []string{pattern}, true
	}
	matches := make([]string, 0, 8)
	for filePath := range d.files {
		if ok, err := path.Match(pattern, filePath); err == nil && ok {
			matches = append(matches, filePath)
		}
	}
	sort.Strings(matches)
	if len(matches) > maxIncludeFiles+1 {
		matches = matches[:maxIncludeFiles+1]
	}
	return matches, true
}

// globIncludeCommand builds the remote POSIX sh command that expands one
// include glob. Only characters that cannot break out of a shell word are
// accepted, so a hostile config cannot inject a command through the pattern.
func globIncludeCommand(pattern string) (string, bool) {
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '/', c == '.', c == '_', c == '-', c == '*', c == '?', c == '[', c == ']', c == '^', c == '!':
		default:
			return "", false
		}
	}
	return "set +f; for f in " + pattern + "; do [ -f \"$f\" ] && printf '%s\\n' \"$f\"; done | LC_ALL=C sort | head -n " + strconv.Itoa(maxIncludeFiles+1), true
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
		Refresh      bool   `json:"refresh,omitempty"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	loaded, perr := r.loadTreeCached(ctx, input.ConnectionID, input.Refresh)
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
	resolved, perr := r.resolveAllowedPath(ctx, input.ConnectionID, input.Path)
	if perr != nil {
		return nil, perr
	}
	read, perr := r.pool.ReadFile(ctx, profile, resolved, maxConfigFileBytes)
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
	result, perr := r.validateConfig(ctx, input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	return result, nil
}
