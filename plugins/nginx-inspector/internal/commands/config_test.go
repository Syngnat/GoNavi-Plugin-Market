package commands

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/ngxconf"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// fakeConfigReader stands in for the SSH pool so include expansion and tree
// building can be tested without a remote host.
type fakeConfigReader struct {
	files    map[string]string
	readErr  map[string]*plugin.Error
	globOut  string
	globErr  *plugin.Error
	commands []string
}

func (f *fakeConfigReader) ReadFile(ctx context.Context, prof store.Profile, path string, maxBytes int64) (sshx.ReadResult, *plugin.Error) {
	if perr, ok := f.readErr[path]; ok {
		return sshx.ReadResult{}, perr
	}
	content, ok := f.files[path]
	if !ok {
		return sshx.ReadResult{}, rpcerr.E(rpcerr.CodeFileUnreadable, "cannot read "+path)
	}
	result := sshx.ReadResult{Path: path, Content: content, SizeBytes: int64(len(content))}
	if maxBytes > 0 && int64(len(result.Content)) > maxBytes {
		result.Content = result.Content[:maxBytes]
		result.Truncated = true
	}
	return result, nil
}

func (f *fakeConfigReader) Output(ctx context.Context, prof store.Profile, command string, timeout time.Duration) (string, *plugin.Error) {
	f.commands = append(f.commands, command)
	return f.globOut, f.globErr
}

func testHostInfo() *HostInfo {
	return &HostInfo{ConfPath: "/etc/nginx/nginx.conf", Prefix: "/etc/nginx"}
}

func testLoadedTree() *loadedTree {
	tree := &ConfigTree{Files: []ConfigFileInfo{}, Servers: []ServerInfo{}, Upstreams: []UpstreamInfo{}}
	return &loadedTree{tree: tree, budget: maxTreeNodes, stack: map[string]bool{}}
}

func parseTestConfig(t *testing.T, src string) []ngxconf.Directive {
	t.Helper()
	directives, err := ngxconf.ParsePartial("/etc/nginx/nginx.conf", src)
	if err != nil {
		t.Fatalf("parse test input: %v", err)
	}
	return directives
}

func countDirectives(directives []ngxconf.Directive, name string) int {
	count := 0
	ngxconf.Walk(directives, func(d *ngxconf.Directive) bool {
		if d.Name == name {
			count++
		}
		return true
	})
	return count
}

func expandTestIncludes(t *testing.T, reader *fakeConfigReader, src string) ([]ngxconf.Directive, *loadedTree) {
	t.Helper()
	loaded := testLoadedTree()
	merged, perr := expandIncludes(context.Background(), reader, store.Profile{}, testHostInfo(), parseTestConfig(t, src), 0, loaded)
	if perr != nil {
		t.Fatalf("expandIncludes: code=%s message=%s", string(perr.Data), perr.Message)
	}
	return merged, loaded
}

func TestExpandIncludesRepeatsSharedSnippet(t *testing.T) {
	reader := &fakeConfigReader{files: map[string]string{
		"/etc/nginx/snippets/ssl.conf": "ssl_certificate /etc/ssl/a.pem;\n",
	}}
	src := "http {\n" +
		"    server {\n        listen 80;\n        include snippets/ssl.conf;\n    }\n" +
		"    server {\n        listen 443 ssl;\n        include snippets/ssl.conf;\n    }\n" +
		"}\n"
	merged, loaded := expandTestIncludes(t, reader, src)

	if got := countDirectives(merged, "ssl_certificate"); got != 2 {
		t.Fatalf("shared snippet expanded %d times, want 2 (once per server)", got)
	}
	if loaded.tree.IncludedCount != 2 {
		t.Fatalf("includedCount = %d, want 2", loaded.tree.IncludedCount)
	}
	if len(loaded.tree.Files) != 2 {
		t.Fatalf("files = %d, want 2 snippet entries", len(loaded.tree.Files))
	}
}

func TestExpandIncludesExpandsGlob(t *testing.T) {
	reader := &fakeConfigReader{
		files: map[string]string{
			"/etc/nginx/conf.d/a.conf": "worker_processes 1;\n",
			"/etc/nginx/conf.d/b.conf": "worker_processes 2;\n",
		},
		globOut: "/etc/nginx/conf.d/a.conf\n/etc/nginx/conf.d/b.conf\n",
	}
	merged, loaded := expandTestIncludes(t, reader, "include conf.d/*.conf;\n")

	if len(merged) != 2 {
		t.Fatalf("merged = %d directives, want 2: %+v", len(merged), merged)
	}
	for _, d := range merged {
		if strings.HasPrefix(d.Name, "#") {
			t.Fatalf("glob produced placeholder %s", d.Name)
		}
	}
	if loaded.tree.IncludedCount != 2 {
		t.Fatalf("includedCount = %d, want 2", loaded.tree.IncludedCount)
	}
	if len(reader.commands) != 1 || !strings.Contains(reader.commands[0], "for f in /etc/nginx/conf.d/*.conf") {
		t.Fatalf("glob command = %v", reader.commands)
	}
}

func TestExpandIncludesGlobFailureBecomesPlaceholder(t *testing.T) {
	reader := &fakeConfigReader{globErr: rpcerr.E(rpcerr.CodeExecFailed, "shell exploded")}
	merged, _ := expandTestIncludes(t, reader, "include conf.d/*.conf;\n")

	if len(merged) != 1 || merged[0].Name != "#unresolved-include" {
		t.Fatalf("merged = %+v", merged)
	}
	args := merged[0].ArgValues()
	if len(args) != 2 || !strings.Contains(args[1], "shell exploded") {
		t.Fatalf("placeholder args = %v", args)
	}
}

func TestExpandIncludesEmptyGlobBecomesPlaceholder(t *testing.T) {
	reader := &fakeConfigReader{globOut: ""}
	merged, _ := expandTestIncludes(t, reader, "include conf.d/*.conf;\n")

	if len(merged) != 1 || merged[0].Name != "#unresolved-include" {
		t.Fatalf("merged = %+v", merged)
	}
	if args := merged[0].ArgValues(); len(args) != 2 || args[1] != "pattern matched no files" {
		t.Fatalf("placeholder args = %v", args)
	}
}

func TestExpandIncludesBreaksCycles(t *testing.T) {
	reader := &fakeConfigReader{files: map[string]string{
		"/etc/nginx/a.conf": "include a.conf;\n",
	}}
	merged, loaded := expandTestIncludes(t, reader, "include a.conf;\n")

	if got := countDirectives(merged, "#include-cycle"); got != 1 {
		t.Fatalf("cycle markers = %d, want 1 in %+v", got, merged)
	}
	if loaded.tree.IncludedCount != 1 {
		t.Fatalf("includedCount = %d, want 1", loaded.tree.IncludedCount)
	}
}

func TestExpandIncludesUnreadableFileBecomesPlaceholder(t *testing.T) {
	reader := &fakeConfigReader{
		files:   map[string]string{},
		readErr: map[string]*plugin.Error{"/etc/nginx/missing.conf": rpcerr.E(rpcerr.CodeFileUnreadable, "permission denied")},
	}
	merged, _ := expandTestIncludes(t, reader, "include missing.conf;\n")

	if len(merged) != 1 || merged[0].Name != "#unreadable-include" {
		t.Fatalf("merged = %+v", merged)
	}
	if args := merged[0].ArgValues(); len(args) != 2 || !strings.Contains(args[1], "permission denied") {
		t.Fatalf("placeholder args = %v", args)
	}
}

// An include outside the read allowlist must not leak file content into the
// tree: config.get refuses the same path, so the tree has to agree.
func TestExpandIncludesRejectsDisallowedPath(t *testing.T) {
	reader := &fakeConfigReader{files: map[string]string{
		"/etc/shadow":                "root:$6$secret\n",
		"/etc/nginx/snippets/x.conf": "worker_processes 1;\n",
	}}
	src := "include /etc/shadow;\ninclude snippets/x.conf;\n"
	merged, loaded := expandTestIncludes(t, reader, src)

	if len(merged) != 2 {
		t.Fatalf("merged = %+v", merged)
	}
	if merged[0].Name != "#disallowed-include" {
		t.Fatalf("first node = %q, want #disallowed-include", merged[0].Name)
	}
	if strings.Contains(strings.Join(merged[0].ArgValues(), " "), "root:") {
		t.Fatal("content of a disallowed include must never reach the tree")
	}
	if merged[1].Name != "worker_processes" {
		t.Fatalf("allowed include after a rejected one = %+v", merged[1])
	}
	if len(reader.commands) != 0 {
		t.Fatalf("a rejected include must not run a remote command: %v", reader.commands)
	}
	if loaded.tree.IncludedCount != 1 {
		t.Fatalf("includedCount = %d, want 1 (only the allowed snippet)", loaded.tree.IncludedCount)
	}
}

func TestExpandIncludesCapsTotalFiles(t *testing.T) {
	files := map[string]string{}
	names := make([]string, 0, maxIncludeFiles)
	for i := 0; i < maxIncludeFiles; i++ {
		path := "/etc/nginx/conf.d/f" + strconv.Itoa(i) + ".conf"
		files[path] = "worker_processes 1;\n"
		names = append(names, path)
	}
	files["/etc/nginx/extra.conf"] = "worker_processes 2;\n"
	reader := &fakeConfigReader{files: files, globOut: strings.Join(names, "\n") + "\n"}

	merged, loaded := expandTestIncludes(t, reader, "include conf.d/*.conf;\ninclude extra.conf;\n")

	if len(merged) != maxIncludeFiles {
		t.Fatalf("merged = %d, want %d", len(merged), maxIncludeFiles)
	}
	if loaded.dropped != 1 {
		t.Fatalf("dropped = %d, want 1", loaded.dropped)
	}
	if !loaded.tree.Truncated {
		t.Fatal("tree should be marked truncated once the include cap is hit")
	}
}

func TestNewFileInfoSharesNodeBudget(t *testing.T) {
	budget := 3
	directives := parseTestConfig(t, "a 1;\nb 2;\nc 3;\nd 4;\n")
	read := sshx.ReadResult{Path: "/etc/nginx/nginx.conf", SizeBytes: 42}

	first, exhausted := newFileInfo("/etc/nginx/nginx.conf", read, directives, &budget)
	if !exhausted {
		t.Fatal("first file should exhaust the budget")
	}
	if len(first.Directives) != 3 || !first.Truncated {
		t.Fatalf("first file nodes = %d truncated=%v", len(first.Directives), first.Truncated)
	}
	if first.Directives[0].File != "/etc/nginx/nginx.conf" {
		t.Fatalf("node file = %q", first.Directives[0].File)
	}
	if budget != 0 {
		t.Fatalf("budget = %d, want 0", budget)
	}

	second, exhausted := newFileInfo("/etc/nginx/nginx.conf", read, directives, &budget)
	if !exhausted || len(second.Directives) != 0 || !second.Truncated {
		t.Fatalf("second file = %d nodes truncated=%v", len(second.Directives), second.Truncated)
	}
}

func TestParseConfigFileTruncatedTailKeepsPrefix(t *testing.T) {
	content := "worker_processes 1;\nhttp {\n    server {\n        listen 80;\n        client_max_body_size 1m;\n        server_nam"
	tree := &ConfigTree{}

	directives, perr := parseConfigFile("/etc/nginx/nginx.conf", sshx.ReadResult{Content: content, SizeBytes: 1 << 30, Truncated: true}, tree)
	if perr != nil {
		t.Fatalf("truncated file must not fail the page: %s", perr.Message)
	}
	if !tree.Truncated {
		t.Fatal("tree should be truncated")
	}
	if len(tree.Warnings) == 0 || !strings.Contains(tree.Warnings[0], "larger than 512 KiB") {
		t.Fatalf("warnings = %v", tree.Warnings)
	}
	if len(directives) != 2 || directives[1].Name != "http" {
		t.Fatalf("prefix = %+v", directives)
	}
	if len(directives[1].Block) != 1 {
		t.Fatalf("http block = %+v", directives[1].Block)
	}
	server := directives[1].Block[0]
	if len(server.Block) != 2 || server.Block[1].Name != "client_max_body_size" {
		t.Fatalf("partial server block = %+v", server.Block)
	}
}

func TestParseConfigFileKeepsPrefixOnHardError(t *testing.T) {
	tree := &ConfigTree{}
	directives, perr := parseConfigFile("/etc/nginx/broken.conf", sshx.ReadResult{Content: "worker_processes 1;\n}\n"}, tree)
	if perr != nil {
		t.Fatalf("prefix should survive a broken tail: %s", perr.Message)
	}
	if len(directives) != 1 || directives[0].Name != "worker_processes" {
		t.Fatalf("prefix = %+v", directives)
	}
	if tree.Error == "" || !strings.Contains(tree.Error, "unbalanced") {
		t.Fatalf("tree error = %q", tree.Error)
	}
	if !tree.Truncated || len(tree.Warnings) == 0 {
		t.Fatalf("truncated=%v warnings=%v", tree.Truncated, tree.Warnings)
	}
}

func TestParseConfigFileAllBrokenStillFails(t *testing.T) {
	tree := &ConfigTree{}
	_, perr := parseConfigFile("/etc/nginx/broken.conf", sshx.ReadResult{Content: "}\n"}, tree)
	if perr == nil {
		t.Fatal("a file with no parsable directive must fail")
	}
	if !strings.Contains(string(perr.Data), rpcerr.CodeParseFailed) {
		t.Fatalf("error data = %s", string(perr.Data))
	}
}

func TestGlobIncludeCommand(t *testing.T) {
	command, ok := globIncludeCommand("/etc/nginx/conf.d/*.conf")
	if !ok || !strings.Contains(command, "for f in /etc/nginx/conf.d/*.conf") {
		t.Fatalf("command = %q ok=%v", command, ok)
	}
	for _, bad := range []string{
		"/etc/nginx/a b/*.conf",
		"/etc/nginx/$(id).conf",
		"/etc/nginx/x\"y/*.conf",
		"/etc/nginx/semi;rm -rf /*",
	} {
		if _, ok := globIncludeCommand(bad); ok {
			t.Fatalf("unsafe pattern accepted: %q", bad)
		}
	}
}

func TestResolveIncludeRelativePath(t *testing.T) {
	reader := &fakeConfigReader{}
	targets, perr := resolveInclude(context.Background(), reader, store.Profile{}, testHostInfo(), "mime.types")
	if perr != nil {
		t.Fatalf("resolveInclude: %s", perr.Message)
	}
	if len(targets) != 1 || targets[0] != "/etc/nginx/mime.types" {
		t.Fatalf("targets = %v", targets)
	}
	if len(reader.commands) != 0 {
		t.Fatalf("a literal include must not run a remote command: %v", reader.commands)
	}
}
