package commands

import (
	"strings"
	"testing"
	"time"
)

func TestParseNginxVersion(t *testing.T) {
	cases := map[string]string{
		"nginx version: nginx/1.25.3":                   "1.25.3",
		"nginx version: openresty/1.21.4.1":             "1.21.4.1",
		"Tengine version: Tengine/2.3.3 (nginx/1.18.0)": "2.3.3",
		"nginx version: nginx/1.24.0 (Ubuntu)":          "1.24.0",
		"":                                              "",
	}
	for input, want := range cases {
		if got := parseNginxVersion(input); got != want {
			t.Errorf("parseNginxVersion(%q) = %q want %q", input, got, want)
		}
	}
}

func TestParseConfigureArgs(t *testing.T) {
	out := "configure arguments: --prefix='/usr/local/openresty/nginx' --conf-path=/etc/nginx/nginx.conf --error-log-path=/var/log/nginx/error.log --http-log-path=/var/log/nginx/access.log --pid-path=/run/nginx.pid --with-http_ssl_module"
	values := parseConfigureArgs(out)
	want := map[string]string{
		"prefix":         "/usr/local/openresty/nginx",
		"conf-path":      "/etc/nginx/nginx.conf",
		"error-log-path": "/var/log/nginx/error.log",
		"http-log-path":  "/var/log/nginx/access.log",
		"pid-path":       "/run/nginx.pid",
	}
	for key, expected := range want {
		if values[key] != expected {
			t.Errorf("%s = %q want %q", key, values[key], expected)
		}
	}
	if _, ok := values["with-http_ssl_module"]; ok {
		t.Errorf("flag without value should not be stored")
	}
}

func TestSplitShellTokens(t *testing.T) {
	tokens := splitShellTokens("--prefix='/opt/my nginx' --conf-path=/etc/nginx/nginx.conf --flag")
	want := []string{"--prefix=/opt/my nginx", "--conf-path=/etc/nginx/nginx.conf", "--flag"}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %#v", tokens)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Fatalf("token %d = %q want %q", i, tokens[i], want[i])
		}
	}
}

func TestSplitDiscoveryOutput(t *testing.T) {
	out := strings.Join([]string{
		"@@BIN", "/usr/sbin/nginx",
		"@@V", "nginx version: nginx/1.25.3",
		"@@VV", "configure arguments: --prefix=/etc/nginx",
		"@@SYS", "yes",
		"@@DOCKER", "no",
		"@@CGROUP", "0::/",
		"@@KERNEL", "Linux 6.1.0",
		"@@HOST", "web-1",
	}, "\n")
	sections := splitDiscoveryOutput(out)
	if sections["@@BIN"] != "/usr/sbin/nginx" {
		t.Errorf("BIN = %q", sections["@@BIN"])
	}
	if sections["@@V"] != "nginx version: nginx/1.25.3" {
		t.Errorf("V = %q", sections["@@V"])
	}
	if sections["@@HOST"] != "web-1" {
		t.Errorf("HOST = %q", sections["@@HOST"])
	}
}

func TestInfoFresh(t *testing.T) {
	if infoFresh(nil) {
		t.Fatalf("nil info must not be fresh")
	}
	now := &HostInfo{DiscoveredAt: time.Now().UTC().Format(time.RFC3339)}
	if !infoFresh(now) {
		t.Fatalf("just discovered info must be fresh")
	}
	old := &HostInfo{DiscoveredAt: time.Now().Add(-2 * hostInfoTTL).UTC().Format(time.RFC3339)}
	if infoFresh(old) {
		t.Fatalf("old info must not be fresh")
	}
	bad := &HostInfo{DiscoveredAt: "not-a-time"}
	if infoFresh(bad) {
		t.Fatalf("unparsable timestamp must not be fresh")
	}
}
