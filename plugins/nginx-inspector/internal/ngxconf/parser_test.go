package ngxconf

import (
	"strings"
	"testing"
)

func TestParseBasic(t *testing.T) {
	content := `# top comment
user www-data;
events {
    worker_connections 768;
}
http {
    include /etc/nginx/mime.types; # inline comment
    server {
        listen 443 ssl;
        server_name "example.com" example.org;
        root /var/www/html;
        location /api {
            proxy_pass http://backend;
        }
    }
}
`
	directives, err := Parse("nginx.conf", content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(directives) != 3 {
		t.Fatalf("top-level directives = %d, want 3", len(directives))
	}
	if directives[0].Name != "user" || directives[0].FirstArg() != "www-data" {
		t.Fatalf("user directive wrong: %+v", directives[0])
	}
	if directives[1].Name != "events" || len(directives[1].Block) != 1 {
		t.Fatalf("events block wrong: %+v", directives[1])
	}
	http := directives[2]
	if http.Name != "http" {
		t.Fatalf("expected http block, got %q", http.Name)
	}
	servers := FindServers(http.Block)
	if len(servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(servers))
	}
	if got := ListenAddresses(servers[0]); len(got) != 1 || got[0] != "443" {
		t.Fatalf("listen = %v", got)
	}
	if got := ServerName(servers[0]); len(got) != 2 || got[0] != "example.com" {
		t.Fatalf("server_name = %v", got)
	}
	locations := FindLocations(servers[0])
	if len(locations) != 1 || LocationPath(locations[0]) != "/api" {
		t.Fatalf("locations wrong: %+v", locations)
	}
	if ProxyPass(locations[0]) != "http://backend" {
		t.Fatalf("proxy_pass = %q", ProxyPass(locations[0]))
	}
	if servers[0].Line == 0 {
		t.Fatalf("server line not recorded")
	}
}

func TestParseQuotedAndEscapes(t *testing.T) {
	content := `log_format main 'remote "$remote_addr" ok';
return 301 https://$host$request_uri;
charset "utf-8";
denied 'it\'s "quoted"';
`
	directives, err := Parse("t.conf", content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := directives[0].Args[1].Value; got != `remote "$remote_addr" ok` || !directives[0].Args[1].Quoted {
		t.Fatalf("single-quoted arg = %q quoted=%v", got, directives[0].Args[1].Quoted)
	}
	if got := directives[1].ArgValues(); len(got) != 2 || got[1] != "https://$host$request_uri" {
		t.Fatalf("return args = %v", got)
	}
	if got := directives[2].FirstArg(); got != "utf-8" {
		t.Fatalf("double-quoted arg = %q", got)
	}
	if got := directives[3].FirstArg(); got != `it's "quoted"` {
		t.Fatalf("escaped arg = %q", got)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"unterminated quote":  `log_format 'abc;`,
		"unbalanced brace":    "http { server { } } }",
		"block without name":  "{ }",
		"unexpected end":      "http {",
		"nested unterminated": "http { server { listen 80; }",
	}
	for name, content := range cases {
		if _, err := Parse("t.conf", content); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestNewlineDoesNotTerminate(t *testing.T) {
	// nginx lexing: only ';' or '}' terminates a directive. A directive
	// split across lines keeps collecting arguments.
	directives, err := Parse("t.conf", "user www-data\nworker_processes 2;")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(directives) != 1 || directives[0].Name != "user" || len(directives[0].Args) != 3 {
		t.Fatalf("newline handling wrong: %+v", directives)
	}
}

func TestParseEmpty(t *testing.T) {
	directives, err := Parse("t.conf", "")
	if err != nil || directives != nil {
		t.Fatalf("empty content: %v %v", directives, err)
	}
}

func TestParseCRLFAndComments(t *testing.T) {
	content := "# c1\r\nuser  www-data;\r\n\r\n# c2\r\nhttp {\r\n}\r\n"
	directives, err := Parse("t.conf", content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(directives) != 2 || directives[0].FirstArg() != "www-data" {
		t.Fatalf("crlf parse wrong: %+v", directives)
	}
}

func TestFindUpstreamsAndMembers(t *testing.T) {
	content := `upstream backend {
    least_conn;
    server 10.0.0.1:8080 weight=2 max_fails=3;
    server unix:/tmp/sock;
    server backup.example.com:80 backup;
}
server { listen 80; }`
	directives, _ := Parse("t.conf", content)
	upstreams := FindUpstreams(directives)
	if len(upstreams) != 1 || upstreams[0].FirstArg() != "backend" {
		t.Fatalf("upstreams = %+v", upstreams)
	}
	members := UpstreamMembers(upstreams[0])
	if len(members) != 3 || members[0].Address != "10.0.0.1:8080" || members[0].Params[0] != "weight=2" {
		t.Fatalf("members = %+v", members)
	}
	if members[2].Address != "backup.example.com:80" || members[2].Params[0] != "backup" {
		t.Fatalf("members[2] = %+v", members[2])
	}
}

func TestWalkStopsAtLocations(t *testing.T) {
	content := `server {
    location /inner { server { listen 9; } }
}`
	directives, _ := Parse("t.conf", content)
	if got := FindServers(directives); len(got) != 1 {
		t.Fatalf("FindServers should not descend into location blocks: %d", len(got))
	}
}
func TestParseHashInsideWord(t *testing.T) {
	directives, err := Parse("t.conf", "proxy_pass http://backend/#fragment;\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(directives) != 1 || len(directives[0].Args) != 1 {
		t.Fatalf("tree = %+v", directives)
	}
	if got := directives[0].Args[0].Value; got != "http://backend/#fragment" {
		t.Fatalf("hash inside word = %q", got)
	}
}

func TestParseHashAfterSpaceStartsComment(t *testing.T) {
	directives, err := Parse("t.conf", "listen 80; # trailing comment\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(directives) != 1 || len(directives[0].Args) != 1 {
		t.Fatalf("comment leaked into args: %+v", directives)
	}
}

func TestParseRegexKeepsBackslash(t *testing.T) {
	directives, err := Parse("t.conf", "location ~ \\.php$ {\n    fastcgi_pass unix:/run/php.sock;\n}\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(directives) != 1 || len(directives[0].Args) != 2 {
		t.Fatalf("tree = %+v", directives)
	}
	if got := directives[0].Args[1].Value; got != "\\.php$" {
		t.Fatalf("regex escape = %q, want %q", got, "\\.php$")
	}
}

func TestParseQuoteFollowedByText(t *testing.T) {
	directives, err := Parse("t.conf", "if ($http_user_agent ~* \"bot\") {\n    return 403;\n}\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(directives) != 1 || len(directives[0].Args) != 3 {
		t.Fatalf("tree = %+v", directives)
	}
	want := []string{"($http_user_agent", "~*", "bot)"}
	for i, expected := range want {
		if got := directives[0].Args[i].Value; got != expected {
			t.Fatalf("arg %d = %q, want %q", i, got, expected)
		}
	}
	if !directives[0].Args[2].Quoted {
		t.Fatalf("quoted flag lost on %+v", directives[0].Args[2])
	}
	if len(directives[0].Block) != 1 || directives[0].Block[0].Name != "return" {
		t.Fatalf("block = %+v", directives[0].Block)
	}
}

func TestParseEscapesInsideQuotes(t *testing.T) {
	directives, err := Parse("t.conf", "add_header X \"a\\tb\";\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := directives[0].Args[1].Value; got != "a\tb" {
		t.Fatalf("quoted escape = %q, want a tab", got)
	}
}

func TestSimpleDirectiveCarriesFile(t *testing.T) {
	directives, err := Parse("nginx.conf", "worker_processes 1;\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if directives[0].File != "nginx.conf" {
		t.Fatalf("simple directive file = %q", directives[0].File)
	}
}

func TestCompleteBoundaryStopsAfterLastDirective(t *testing.T) {
	content := "user nginx;\nhttp {\n    server {\n        listen 80;\n        proxy_pass http://a/#frag;\n        client_max"
	want := strings.Index(content, "#frag;") + len("#frag;")
	if got := CompleteBoundary(content); got != want {
		t.Fatalf("boundary = %d, want %d", got, want)
	}
}

func TestCompleteBoundaryIgnoresQuotedSemicolon(t *testing.T) {
	content := "add_header X \"a;b\";\nserver"
	want := len("add_header X \"a;b\";")
	if got := CompleteBoundary(content); got != want {
		t.Fatalf("boundary = %d, want %d", got, want)
	}
}

func TestParsePartialKeepsParsedPrefix(t *testing.T) {
	directives, err := ParsePartial("t.conf", "worker_processes 1;\n}\n")
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(directives) != 1 || directives[0].Name != "worker_processes" {
		t.Fatalf("prefix = %+v", directives)
	}
	if !strings.Contains(err.Error(), "unbalanced") {
		t.Fatalf("error = %v", err)
	}
}

func TestParsePartialRetainsOpenBlock(t *testing.T) {
	directives, err := ParsePartial("t.conf", "http {\n    server {\n        listen 80;\n")
	if err == nil {
		t.Fatal("expected an end-of-file error")
	}
	if !strings.Contains(err.Error(), "unexpected end of file") {
		t.Fatalf("error = %v", err)
	}
	if len(directives) != 1 || directives[0].Name != "http" || len(directives[0].Block) != 1 {
		t.Fatalf("partial tree = %+v", directives)
	}
	inner := directives[0].Block[0]
	if inner.Name != "server" || len(inner.Block) != 1 || inner.Block[0].Name != "listen" {
		t.Fatalf("partial block = %+v", inner)
	}
}

func TestFindServersSkipsUpstreamAndMail(t *testing.T) {
	content := "http {\n    upstream pool {\n        server 10.0.0.1:8080;\n    }\n    server {\n        listen 80;\n        location / {\n            proxy_pass http://pool;\n        }\n    }\n}\nmail {\n    server {\n        listen 25;\n    }\n}\n"
	directives, err := Parse("t.conf", content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	servers := FindServers(directives)
	if len(servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(servers))
	}
	if got := ListenAddresses(servers[0]); len(got) != 1 || got[0] != "80" {
		t.Fatalf("server listen = %v", got)
	}
}

func TestServerEnabledReadsListenParameters(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"plain listener with certificate", "server {\n    listen 80;\n    ssl_certificate /etc/ssl/a.pem;\n}\n", false},
		{"ssl parameter", "server {\n    listen 443 ssl;\n}\n", true},
		{"quic parameter", "server {\n    listen 443 quic reuseport;\n}\n", true},
		{"http2 only", "server {\n    listen 8443 http2;\n}\n", false},
		{"explicit address with ssl", "server {\n    listen 192.0.2.1:443 ssl default_server;\n}\n", true},
	}
	for _, c := range cases {
		directives, err := Parse("t.conf", c.src)
		if err != nil {
			t.Fatalf("%s: parse: %v", c.name, err)
		}
		servers := FindServers(directives)
		if len(servers) != 1 {
			t.Fatalf("%s: servers = %d", c.name, len(servers))
		}
		if got := ServerEnabled(servers[0]); got != c.want {
			t.Fatalf("%s: ServerEnabled = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLocationPathDropsModifiers(t *testing.T) {
	content := "server {\n" +
		"    location = /exact { root /a; }\n" +
		"    location ^~ /static/ { root /b; }\n" +
		"    location ~ \\.php$ { fastcgi_pass unix:/run/php.sock; }\n" +
		"    location ~* \\.(jpg|png)$ { expires 1d; }\n" +
		"    location /plain { proxy_pass http://pool; }\n" +
		"}\n"
	directives, err := Parse("t.conf", content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	servers := FindServers(directives)
	if len(servers) != 1 {
		t.Fatalf("servers = %d", len(servers))
	}
	locations := FindLocations(servers[0])
	want := []string{"/exact", "/static/", "\\.php$", "\\.(jpg|png)$", "/plain"}
	if len(locations) != len(want) {
		t.Fatalf("locations = %d, want %d", len(locations), len(want))
	}
	for i, expected := range want {
		if got := LocationPath(locations[i]); got != expected {
			t.Fatalf("location %d path = %q, want %q", i, got, expected)
		}
	}
}
