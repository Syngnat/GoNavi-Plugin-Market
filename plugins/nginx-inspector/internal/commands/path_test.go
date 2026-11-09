package commands

import (
	"strings"
	"testing"
)

func TestAllowPathCoversRealNginxLayouts(t *testing.T) {
	info := &HostInfo{Prefix: "/etc/nginx", ConfPath: "/etc/nginx/nginx.conf"}
	cases := []struct {
		raw      string
		relative bool
		want     string
		code     string
	}{
		{"/etc/nginx/nginx.conf", false, "/etc/nginx/nginx.conf", ""},
		{"/usr/local/openresty/nginx/conf/nginx.conf", false, "/usr/local/openresty/nginx/conf/nginx.conf", ""},
		{"/opt/homebrew/etc/nginx/nginx.conf", false, "/opt/homebrew/etc/nginx/nginx.conf", ""},
		{"/var/log/nginx/access.log", false, "/var/log/nginx/access.log", ""},
		{"/etc/ssl/private/site.key", false, "/etc/ssl/private/site.key", ""},
		{"/etc/pki/tls/certs/site.pem", false, "/etc/pki/tls/certs/site.pem", ""},
		{"/etc/letsencrypt/live/site/fullchain.pem", false, "/etc/letsencrypt/live/site/fullchain.pem", ""},
		{"certs/a.pem", true, "/etc/nginx/certs/a.pem", ""},
		{"/etc/passwd", false, "", codePathNotAllowed},
		{"/etc/nginx/../passwd", false, "", codePathNotAllowed},
		{"/root/.ssh/id_rsa", false, "", codePathNotAllowed},
		{"/", false, "", codePathNotAllowed},
		{"", false, "", "INVALID_ARGS"},
		{"certs/a.pem", false, "", "INVALID_ARGS"},
	}
	for _, c := range cases {
		got, perr := allowPath(info, c.raw, c.relative)
		if c.code == "" {
			if perr != nil {
				t.Fatalf("allowPath(%q, relative=%v) failed: %v", c.raw, c.relative, perr.Message)
			}
			if got != c.want {
				t.Fatalf("allowPath(%q) = %q want %q", c.raw, got, c.want)
			}
			continue
		}
		if perr == nil {
			t.Fatalf("allowPath(%q) must fail", c.raw)
		}
		if c.code == codePathNotAllowed {
			if !strings.Contains(string(perr.Data), codePathNotAllowed) {
				t.Fatalf("allowPath(%q) data = %s", c.raw, string(perr.Data))
			}
			continue
		}
		if strings.Contains(string(perr.Data), codePathNotAllowed) {
			t.Fatalf("allowPath(%q) should be invalid args, got %s", c.raw, string(perr.Data))
		}
	}
}

func TestPathRootsRejectFilesystemRoot(t *testing.T) {
	roots := pathRoots(&HostInfo{Prefix: "/", ConfPath: "/nginx.conf"})
	for _, root := range roots {
		if root == "/" || !strings.HasPrefix(root, "/") {
			t.Fatalf("pathRoots must not contain %q", root)
		}
	}
	info := &HostInfo{Prefix: "/", ConfPath: "/nginx.conf"}
	if _, perr := allowPath(info, "/etc/passwd", false); perr == nil {
		t.Fatalf("a root prefix must not allow /etc/passwd")
	}
}

func TestAllowPathCustomPrefix(t *testing.T) {
	info := &HostInfo{Prefix: "/opt/app/nginx", ConfPath: "/opt/app/nginx/conf/nginx.conf"}
	got, perr := allowPath(info, "conf/site.conf", true)
	if perr != nil || got != "/opt/app/nginx/conf/site.conf" {
		t.Fatalf("custom prefix = %q, err = %v", got, perr)
	}
}
