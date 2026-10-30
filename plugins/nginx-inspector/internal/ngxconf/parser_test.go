package ngxconf

import "testing"

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
