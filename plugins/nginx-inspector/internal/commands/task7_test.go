package commands

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
)

func task7Dump() string {
	return strings.Join([]string{
		"# configuration file /etc/nginx/nginx.conf:",
		"http {",
		"    include conf.d/*.conf;",
		"}",
		"# configuration file /etc/nginx/conf.d/b.conf:",
		"server { listen 443; }",
		"# configuration file /etc/nginx/conf.d/a.conf:",
		"server { listen 80; }",
	}, "\n") + "\n"
}

func TestParseConfigDumpSections(t *testing.T) {
	dump := "# configuration file /etc/nginx/nginx.conf:" + "\n" +
		"worker_processes 1;" + "\n" +
		"include conf.d/*.conf;" + "\n" +
		"# configuration file /etc/nginx/conf.d/a.conf:" + "\n" +
		"server { listen 80; }" + "\n" +
		"# configuration file relative.conf:" + "\n" +
		"ignored;" + "\n"
	files, ok := parseConfigDump(dump)
	if !ok {
		t.Fatal("parseConfigDump did not find any section")
	}
	if len(files) != 2 {
		t.Fatalf("sections = %d, want 2: %+v", len(files), files)
	}
	if got := files["/etc/nginx/nginx.conf"]; got != "worker_processes 1;\ninclude conf.d/*.conf;\n" {
		t.Fatalf("main content = %q", got)
	}
	if got := files["/etc/nginx/conf.d/a.conf"]; got != "server { listen 80; }\n" {
		t.Fatalf("include content = %q", got)
	}
	if _, exists := files["relative.conf"]; exists {
		t.Fatal("relative marker must be ignored")
	}
}

func TestDumpReaderResolvesIncludeGlob(t *testing.T) {
	info := &HostInfo{ConfPath: "/etc/nginx/nginx.conf", Prefix: "/etc/nginx"}
	reader := &dumpReader{files: map[string]string{
		"/etc/nginx/nginx.conf":    "http {\n    include conf.d/*.conf;\n}\n",
		"/etc/nginx/conf.d/b.conf": "server { listen 443; }\n",
		"/etc/nginx/conf.d/a.conf": "server { listen 80; }\n",
	}}
	loaded, perr := buildTree(context.Background(), reader, store.Profile{}, info)
	if perr != nil {
		t.Fatalf("buildTree: %s", perr.Message)
	}
	if len(loaded.tree.Files) != 3 {
		t.Fatalf("files = %d, want main plus two includes", len(loaded.tree.Files))
	}
	if loaded.tree.IncludedCount != 2 {
		t.Fatalf("includedCount = %d", loaded.tree.IncludedCount)
	}
	if got := countDirectives(loaded.directives, "server"); got != 2 {
		t.Fatalf("servers = %d, want 2", got)
	}
	if loaded.tree.Files[1].Path != "/etc/nginx/conf.d/a.conf" || loaded.tree.Files[2].Path != "/etc/nginx/conf.d/b.conf" {
		t.Fatalf("matches are not sorted: %+v", []string{loaded.tree.Files[1].Path, loaded.tree.Files[2].Path})
	}
}

func TestLoadTreeUsesNginxDumpFastPath(t *testing.T) {
	var mu sync.Mutex
	var commands []string
	handler := func(command string) (string, string, int) {
		mu.Lock()
		commands = append(commands, command)
		mu.Unlock()
		if strings.Contains(command, " -T") {
			return task7Dump(), "", 0
		}
		return "", "unexpected command: " + command, 127
	}
	host, port := startFakeSSH(t, handler)
	runtime, connectionID := newFakeRuntime(t, host, port)
	loaded, perr := runtime.loadTree(context.Background(), connectionID)
	if perr != nil {
		t.Fatalf("loadTree: %s", perr.Message)
	}
	if len(loaded.tree.Files) != 3 {
		t.Fatalf("files = %d, want main plus two includes", len(loaded.tree.Files))
	}
	if got := countDirectives(loaded.directives, "server"); got != 2 {
		t.Fatalf("servers = %d", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(commands) != 1 || !strings.Contains(commands[0], " -T") {
		t.Fatalf("commands = %v, want exactly one nginx -T", commands)
	}
}

func TestLoadTreeFallsBackWhenDumpFails(t *testing.T) {
	files := map[string][]byte{
		"/etc/nginx/nginx.conf":    []byte("http {\n    include conf.d/a.conf;\n}\n"),
		"/etc/nginx/conf.d/a.conf": []byte("server { listen 8080; }\n"),
	}
	base := fileCommands(files)
	var mu sync.Mutex
	var commands []string
	handler := func(command string) (string, string, int) {
		mu.Lock()
		commands = append(commands, command)
		mu.Unlock()
		if strings.Contains(command, " -T") {
			return "", "nginx: [emerg] invalid configuration", 1
		}
		return base(command)
	}
	host, port := startFakeSSH(t, handler)
	runtime, connectionID := newFakeRuntime(t, host, port)
	loaded, perr := runtime.loadTree(context.Background(), connectionID)
	if perr != nil {
		t.Fatalf("loadTree fallback: %s", perr.Message)
	}
	if len(loaded.tree.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(loaded.tree.Files))
	}
	if loaded.tree.IncludedCount != 1 {
		t.Fatalf("includedCount = %d", loaded.tree.IncludedCount)
	}
	mu.Lock()
	defer mu.Unlock()
	var sawDump, sawRead bool
	for _, command := range commands {
		sawDump = sawDump || strings.Contains(command, " -T")
		sawRead = sawRead || strings.HasPrefix(command, "head -c ")
	}
	if !sawDump || !sawRead {
		t.Fatalf("fallback commands = %v", commands)
	}
}

func TestConfigTreeCacheHitRefreshAndInvalidate(t *testing.T) {
	var mu sync.Mutex
	dumps := 0
	handler := func(command string) (string, string, int) {
		if strings.Contains(command, " -T") {
			mu.Lock()
			dumps++
			mu.Unlock()
			return task7Dump(), "", 0
		}
		return "", "unexpected command: " + command, 127
	}
	host, port := startFakeSSH(t, handler)
	runtime, connectionID := newFakeRuntime(t, host, port)
	treeHandler := runtime.Handlers()["config.tree"]
	args := connectionArgs(t, connectionID)

	first, err := treeHandler(context.Background(), args)
	if err != nil {
		t.Fatalf("first tree: %v", err)
	}
	second, err := treeHandler(context.Background(), args)
	if err != nil {
		t.Fatalf("second tree: %v", err)
	}
	if first.(*ConfigTree) != second.(*ConfigTree) {
		t.Fatal("cache hit should return the same parsed tree")
	}
	if countDumps(&mu, &dumps) != 1 {
		t.Fatalf("cache hit performed %d dumps", dumps)
	}

	refreshArgs := connectionArgs(t, connectionID, "refresh", true)
	if _, err := treeHandler(context.Background(), refreshArgs); err != nil {
		t.Fatalf("refresh tree: %v", err)
	}
	if countDumps(&mu, &dumps) != 2 {
		t.Fatalf("refresh did not bypass the cache: %d dumps", dumps)
	}

	runtime.invalidateTree(connectionID)
	if _, err := treeHandler(context.Background(), args); err != nil {
		t.Fatalf("tree after invalidate: %v", err)
	}
	if countDumps(&mu, &dumps) != 3 {
		t.Fatalf("invalidate did not clear the cache: %d dumps", dumps)
	}
}

func countDumps(mu *sync.Mutex, value *int) int {
	mu.Lock()
	defer mu.Unlock()
	return *value
}

func TestConfigValidateHandlerHasNilError(t *testing.T) {
	handler := func(command string) (string, string, int) {
		if strings.Contains(command, " -t 2>&1") {
			return "nginx: configuration file /etc/nginx/nginx.conf test is successful\n", "", 0
		}
		return "", "unexpected command: " + command, 127
	}
	host, port := startFakeSSH(t, handler)
	runtime, connectionID := newFakeRuntime(t, host, port)
	result, err := runtime.Handlers()["config.validate"](context.Background(), connectionArgs(t, connectionID))
	if err != nil {
		t.Fatalf("config.validate must not return a typed-nil error: %v", err)
	}
	validation, ok := result.(*ValidationResult)
	if !ok || !validation.Ok {
		t.Fatalf("validation = %#v", result)
	}
}

func TestReadFileSingleRoundTripAndTruncation(t *testing.T) {
	files := map[string][]byte{
		"/etc/nginx/small.conf": []byte(strings.Repeat("a", 10)),
		"/etc/nginx/big.conf":   []byte(strings.Repeat("b", 100)),
	}
	base := fileCommands(files)
	var mu sync.Mutex
	var commands []string
	handler := func(command string) (string, string, int) {
		mu.Lock()
		commands = append(commands, command)
		mu.Unlock()
		return base(command)
	}
	host, port := startFakeSSH(t, handler)
	runtime, connectionID := newFakeRuntime(t, host, port)
	profile, ok, err := runtime.store.Get(connectionID)
	if err != nil || !ok {
		t.Fatalf("get profile: %v %v", ok, err)
	}

	small, perr := runtime.pool.ReadFile(context.Background(), profile, "/etc/nginx/small.conf", 10)
	if perr != nil {
		t.Fatalf("small read: %s", perr.Message)
	}
	if small.Content != strings.Repeat("a", 10) || small.Truncated || small.SizeBytes != 10 {
		t.Fatalf("small result = %+v", small)
	}
	mu.Lock()
	smallCommands := append([]string(nil), commands...)
	commands = nil
	mu.Unlock()
	if len(smallCommands) != 1 || !strings.HasPrefix(smallCommands[0], "head -c 11 ") {
		t.Fatalf("untruncated read commands = %v", smallCommands)
	}

	big, perr := runtime.pool.ReadFile(context.Background(), profile, "/etc/nginx/big.conf", 10)
	if perr != nil {
		t.Fatalf("big read: %s", perr.Message)
	}
	if !big.Truncated || len(big.Content) != 10 || big.SizeBytes != 100 {
		t.Fatalf("big result = %+v", big)
	}
	mu.Lock()
	bigCommands := append([]string(nil), commands...)
	mu.Unlock()
	if len(bigCommands) != 2 || !strings.HasPrefix(bigCommands[1], "wc -c < ") {
		t.Fatalf("truncated read commands = %v", bigCommands)
	}

	_, perr = runtime.pool.ReadFile(context.Background(), profile, "/etc/nginx/missing.conf", 10)
	if perr == nil || !strings.Contains(string(perr.Data), rpcerr.CodeFileUnreadable) {
		t.Fatalf("missing read error = %v", perr)
	}
}

func TestSaveConnectionInvalidatesTreeCache(t *testing.T) {
	var mu sync.Mutex
	dumps := 0
	handler := func(command string) (string, string, int) {
		if strings.Contains(command, " -T") {
			mu.Lock()
			dumps++
			mu.Unlock()
			return task7Dump(), "", 0
		}
		return "", "unexpected command: " + command, 127
	}
	host, port := startFakeSSH(t, handler)
	runtime, connectionID := newFakeRuntime(t, host, port)
	treeHandler := runtime.Handlers()["config.tree"]
	if _, err := treeHandler(context.Background(), connectionArgs(t, connectionID)); err != nil {
		t.Fatalf("initial tree: %v", err)
	}
	saveArgs := marshalArgs(t, map[string]any{
		"id": connectionID, "name": "fake-renamed", "host": host, "port": port,
		"username": "u", "authType": store.AuthPassword,
	})
	if _, err := runtime.saveConnection(context.Background(), saveArgs); err != nil {
		t.Fatalf("save connection: %v", err)
	}
	// Saving drops host info too; restore it so this test isolates the tree cache.
	runtime.mu.Lock()
	runtime.info[connectionID] = &HostInfo{
		Found: true, Binary: "/usr/sbin/nginx", Prefix: "/etc/nginx",
		ConfPath: "/etc/nginx/nginx.conf", DiscoveredAt: "2099-01-01T00:00:00Z",
	}
	runtime.mu.Unlock()
	if _, err := treeHandler(context.Background(), connectionArgs(t, connectionID)); err != nil {
		t.Fatalf("tree after save: %v", err)
	}
	if countDumps(&mu, &dumps) != 2 {
		t.Fatalf("save did not invalidate the tree cache: %d dumps", dumps)
	}
}
