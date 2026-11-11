package commands

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
	plugin "github.com/Syngnat/GoNavi/sdk/go"
	"golang.org/x/crypto/ssh"
)

type fakeSSH struct {
	listener net.Listener
	handler  func(command string) (stdout, stderr string, code int)
}

func startFakeSSH(t *testing.T, handler func(string) (string, string, int)) (string, int) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if meta.User() == "u" && string(password) == "p" {
				return nil, nil
			}
			return nil, fmt.Errorf("denied")
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &fakeSSH{listener: listener, handler: handler}
	t.Cleanup(func() { _ = listener.Close() })
	go server.serve(config)
	tcp := listener.Addr().(*net.TCPAddr)
	return "127.0.0.1", tcp.Port
}

func (s *fakeSSH) serve(config *ssh.ServerConfig) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn, config)
	}
}

func (s *fakeSSH) handleConn(conn net.Conn, config *ssh.ServerConfig) {
	serverConn, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer serverConn.Close()
	go ssh.DiscardRequests(requests)
	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "only sessions")
			continue
		}
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer channel.Close()
			for request := range channelRequests {
				if request.Type != "exec" {
					_ = request.Reply(false, nil)
					continue
				}
				var payload struct{ Command string }
				if err := ssh.Unmarshal(request.Payload, &payload); err != nil {
					_ = request.Reply(false, nil)
					return
				}
				_ = request.Reply(true, nil)
				stdout, stderr, code := s.handler(payload.Command)
				_, _ = channel.Write([]byte(stdout))
				if stderr != "" {
					_, _ = channel.Stderr().Write([]byte(stderr))
				}
				status := struct{ Status uint32 }{uint32(code)}
				_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(&status))
				return
			}
		}()
	}
}

func unquoteShellPath(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == 39 && value[len(value)-1] == 39 {
		return value[1 : len(value)-1]
	}
	return value
}

func fileCommands(files map[string][]byte) func(string) (string, string, int) {
	nl := string(rune(10))
	missing := func(path string) (string, string, int) {
		return "", "cat: " + path + ": No such file or directory", 1
	}
	return func(command string) (string, string, int) {
		command = strings.TrimSpace(command)
		switch {
		case strings.HasPrefix(command, "wc -c < "):
			path := unquoteShellPath(strings.TrimPrefix(command, "wc -c < "))
			data, ok := files[path]
			if !ok {
				return missing(path)
			}
			return strconv.Itoa(len(data)), "", 0
		case strings.HasPrefix(command, "wc -l -c < "):
			path := unquoteShellPath(strings.TrimPrefix(command, "wc -l -c < "))
			data, ok := files[path]
			if !ok {
				return missing(path)
			}
			lines := strings.Count(string(data), nl)
			return strconv.Itoa(lines) + " " + strconv.Itoa(len(data)), "", 0
		case strings.HasPrefix(command, "cat "):
			path := unquoteShellPath(strings.TrimPrefix(command, "cat "))
			data, ok := files[path]
			if !ok {
				return missing(path)
			}
			return string(data), "", 0
		case strings.HasPrefix(command, "head -c 2 "):
			return "31 0a", "", 0
		case strings.HasPrefix(command, "head -c "):
			fields := strings.Fields(command)
			if len(fields) != 4 {
				return "", "unsupported head: " + command, 127
			}
			count, err := strconv.Atoi(fields[2])
			if err != nil || count < 0 {
				return "", "bad head count", 1
			}
			path := unquoteShellPath(fields[3])
			data, ok := files[path]
			if !ok {
				return missing(path)
			}
			if count > len(data) {
				count = len(data)
			}
			return string(data[:count]), "", 0
		case strings.HasPrefix(command, "tail -c "):
			fields := strings.Fields(command)
			if len(fields) < 4 {
				return "", "bad tail", 1
			}
			count, err := strconv.Atoi(fields[2])
			if err != nil {
				return "", "bad tail count", 1
			}
			path := unquoteShellPath(fields[3])
			data, ok := files[path]
			if !ok {
				return missing(path)
			}
			if count > len(data) {
				count = len(data)
			}
			return string(data[len(data)-count:]), "", 0
		case strings.HasPrefix(command, "tail -n "):
			fields := strings.Fields(command)
			if len(fields) < 4 {
				return "", "bad tail", 1
			}
			count, err := strconv.Atoi(fields[2])
			if err != nil {
				return "", "bad tail count", 1
			}
			path := unquoteShellPath(fields[3])
			data, ok := files[path]
			if !ok {
				return missing(path)
			}
			lines := strings.Split(strings.TrimRight(string(data), nl), nl)
			if count < len(lines) {
				lines = lines[len(lines)-count:]
			}
			return strings.Join(lines, nl) + nl, "", 0
		default:
			return "", "unsupported command: " + command, 127
		}
	}
}

func newFakeRuntime(t *testing.T, host string, port int) (*Runtime, string) {
	t.Helper()
	st := store.Open(filepath.Join(t.TempDir(), "connections.json"))
	profile, err := st.Save(store.Profile{
		Name: "fake", Host: host, Port: port, Username: "u",
		AuthType: store.AuthPassword, Password: "p",
	})
	if err != nil {
		t.Fatalf("save profile: %v", err)
	}
	runtime := &Runtime{store: st, pool: sshx.NewPool(st), info: map[string]*HostInfo{}, trees: map[string]cachedTree{}}
	runtime.info[profile.ID] = &HostInfo{
		Found: true, Binary: "/usr/sbin/nginx", Prefix: "/etc/nginx",
		ConfPath:     "/etc/nginx/nginx.conf",
		HTTPLogPath:  "/var/log/nginx/access.log",
		ErrorLogPath: "/var/log/nginx/error.log",
		DiscoveredAt: time.Now().UTC().Format(time.RFC3339),
	}
	t.Cleanup(func() { runtime.Close() })
	return runtime, profile.ID
}

func connectionArgs(t *testing.T, connectionID string, extra ...any) json.RawMessage {
	t.Helper()
	payload := map[string]any{"connectionId": connectionID}
	for i := 0; i+1 < len(extra); i += 2 {
		key, ok := extra[i].(string)
		if !ok {
			t.Fatalf("connectionArgs key %d is not a string", i)
		}
		payload[key] = extra[i+1]
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return data
}

func TestCertsListIntegration(t *testing.T) {
	certA := generateCertificatePEM(t, "example.com", []string{"example.com", "www.example.com"}, time.Now().Add(90*24*time.Hour))
	certB := generateCertificatePEM(t, "b.example.com", []string{"b.example.com"}, time.Now().Add(120*24*time.Hour))
	config := `http {
    server {
        listen 443 ssl;
        ssl_certificate /etc/nginx/certs/a.pem;
        ssl_certificate_key /etc/nginx/certs/a.key;
    }
    server {
        server_name www.example.com;
        ssl_certificate /etc/nginx/certs/a.pem;
        ssl_certificate certs/b.pem;
    }
    server {
        ssl_certificate /root/secret.pem;
    }
    server {
        ssl_certificate /etc/nginx/certs/missing.pem;
    }
    server {
        ssl_certificate /etc/nginx/certs/bad.pem;
    }
}`
	files := map[string][]byte{
		"/etc/nginx/nginx.conf":    []byte(config),
		"/etc/nginx/certs/a.pem":   certA,
		"/etc/nginx/certs/b.pem":   certB,
		"/etc/nginx/certs/bad.pem": []byte("this is not a certificate"),
	}
	host, port := startFakeSSH(t, fileCommands(files))
	runtime, connectionID := newFakeRuntime(t, host, port)

	result, err := runtime.certsList(context.Background(), connectionArgs(t, connectionID))
	if err != nil {
		t.Fatalf("certs.list: %v", err)
	}
	report, ok := result.(*CertificateReport)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if len(report.Certificates) != 5 {
		t.Fatalf("certificates = %d, want 5: %+v", len(report.Certificates), report.Certificates)
	}
	if report.Skipped != 1 || report.Unreadable != 1 || report.ParseErrors != 1 {
		t.Fatalf("report counts = skipped:%d unreadable:%d parseErrors:%d", report.Skipped, report.Unreadable, report.ParseErrors)
	}
	byPath := map[string]CertificateInfo{}
	for _, entry := range report.Certificates {
		byPath[entry.Path] = entry
	}
	a := byPath["/etc/nginx/certs/a.pem"]
	if !a.Readable || a.CommonName != "example.com" || len(a.Domains) != 2 {
		t.Fatalf("cert a = %+v", a)
	}
	if a.DaysLeft < 89 || a.DaysLeft > 90 {
		t.Fatalf("cert a daysLeft = %d", a.DaysLeft)
	}
	relative := byPath["certs/b.pem"]
	if relative.File != "/etc/nginx/certs/b.pem" || relative.CommonName != "b.example.com" {
		t.Fatalf("relative cert = %+v", relative)
	}
	if !byPath["/etc/nginx/certs/bad.pem"].ParseError {
		t.Fatalf("bad cert should report parseError")
	}
	if byPath["/root/secret.pem"].Reason == "" {
		t.Fatalf("outside-allowlist path should carry a reason")
	}
}

func TestCertsPathAllowlistIntegration(t *testing.T) {
	host, port := startFakeSSH(t, fileCommands(map[string][]byte{}))
	runtime, connectionID := newFakeRuntime(t, host, port)
	_, err := runtime.logsTail(context.Background(), connectionArgs(t, connectionID, "path", "/etc/passwd"))
	if err == nil {
		t.Fatalf("explicit path outside the allowlist must fail")
	}
	perr, ok := err.(*plugin.Error)
	if !ok {
		t.Fatalf("expected *plugin.Error, got %T", err)
	}
	if !strings.Contains(string(perr.Data), codePathNotAllowed) {
		t.Fatalf("error data = %s", string(perr.Data))
	}
	_, err = runtime.logsTail(context.Background(), connectionArgs(t, connectionID, "path", "/etc/nginx/../passwd"))
	if err == nil {
		t.Fatalf("traversal path must fail")
	}
	if perr, ok := err.(*plugin.Error); !ok || !strings.Contains(string(perr.Data), codePathNotAllowed) {
		t.Fatalf("traversal error = %v", err)
	}
}

// A window larger than the file must analyze the whole file: the streaming
// path is not limited by the per-command output cap any more.
func TestAccessStatsAnalyzesWholeWindowBeyondFrameCap(t *testing.T) {
	nl := string(rune(10))
	line := `203.0.113.7 - - [24/Oct/2026:10:00:01 +0800] "GET /api/users HTTP/1.1" 200 1534 "-" "Mozilla/5.0" 0.045 0.044`
	content := strings.Repeat(line+nl, (3<<20)/len(line))
	host, port := startFakeSSH(t, fileCommands(map[string][]byte{
		"/var/log/nginx/access.log": []byte(content),
	}))
	runtime, connectionID := newFakeRuntime(t, host, port)
	const requested = int64(5 << 20)
	result, err := runtime.accessStats(context.Background(), connectionArgs(t, connectionID, "bytes", requested))
	if err != nil {
		t.Fatalf("access.stats: %v", err)
	}
	stats, ok := result.(*AccessStats)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if stats.RequestedBytes != requested {
		t.Fatalf("requestedBytes = %d", stats.RequestedBytes)
	}
	if stats.AnalyzedBytes != int64(len(content)) || stats.WindowBytes != stats.AnalyzedBytes {
		t.Fatalf("analyzed = %d window = %d, want the whole %d-byte file", stats.AnalyzedBytes, stats.WindowBytes, len(content))
	}
	if stats.Truncated {
		t.Fatal("a window larger than the file must not be reported as truncated")
	}
	if stats.Total == 0 || stats.Parsed == 0 {
		t.Fatalf("expected parsed lines, got total=%d parsed=%d", stats.Total, stats.Parsed)
	}
	if stats.UpstreamTime.Samples == 0 || stats.RequestTime.Samples == 0 {
		t.Fatalf("timing samples missing: %+v %+v", stats.RequestTime, stats.UpstreamTime)
	}
	if stats.GzipUnsupported {
		t.Fatalf("text log must not be reported as gzip")
	}
}

// A window smaller than the file analyzes exactly that many newest bytes.
func TestAccessStatsHonorsSmallWindow(t *testing.T) {
	nl := string(rune(10))
	line := `203.0.113.7 - - [24/Oct/2026:10:00:01 +0800] "GET /api/users HTTP/1.1" 200 1534 "-" "Mozilla/5.0" 0.045 0.044`
	content := strings.Repeat(line+nl, 5000)
	host, port := startFakeSSH(t, fileCommands(map[string][]byte{
		"/var/log/nginx/access.log": []byte(content),
	}))
	runtime, connectionID := newFakeRuntime(t, host, port)
	const requested = int64(64 << 10)
	result, err := runtime.accessStats(context.Background(), connectionArgs(t, connectionID, "bytes", requested))
	if err != nil {
		t.Fatalf("access.stats: %v", err)
	}
	stats := result.(*AccessStats)
	if stats.AnalyzedBytes != requested {
		t.Fatalf("analyzed = %d, want the requested %d", stats.AnalyzedBytes, requested)
	}
	if stats.Total == 0 || stats.Total >= 5000 {
		t.Fatalf("total = %d, want a bounded subset of 5000 lines", stats.Total)
	}
}

// Statistics must follow the access_log directive of the merged config, not the
// compile-time default from nginx -V: sites that write their own log file would
// otherwise get statistics from an unrelated (usually empty) default path.
func TestAccessStatsUsesConfiguredAccessLog(t *testing.T) {
	nl := string(rune(10))
	line := `203.0.113.7 - - [24/Oct/2026:10:00:01 +0800] "GET /api/users HTTP/1.1" 200 1534 "-" "Mozilla/5.0" 0.045 0.044`
	config := "http {\n    access_log /var/log/nginx/site.access.log;\n}\n"
	host, port := startFakeSSH(t, fileCommands(map[string][]byte{
		"/etc/nginx/nginx.conf":          []byte(config),
		"/var/log/nginx/site.access.log": []byte(line + nl),
		"/var/log/nginx/access.log":      []byte("this default file must not be analyzed" + nl),
	}))
	runtime, connectionID := newFakeRuntime(t, host, port)
	result, err := runtime.accessStats(context.Background(), connectionArgs(t, connectionID))
	if err != nil {
		t.Fatalf("access.stats: %v", err)
	}
	stats := result.(*AccessStats)
	if stats.Path != "/var/log/nginx/site.access.log" {
		t.Fatalf("path = %q, want the configured access_log", stats.Path)
	}
	if stats.Source != "config" {
		t.Fatalf("source = %q, want config", stats.Source)
	}
	if stats.Total != 1 || stats.Parsed != 1 {
		t.Fatalf("total/parsed = %d/%d, want the configured file's single line", stats.Total, stats.Parsed)
	}
}

// A log target that cannot be read (off/syslog) must report its note instead of
// silently analyzing the binary default.
func TestAccessStatsReportsUnreadableTarget(t *testing.T) {
	config := "http {\n    access_log off;\n}\n"
	host, port := startFakeSSH(t, fileCommands(map[string][]byte{
		"/etc/nginx/nginx.conf": []byte(config),
	}))
	runtime, connectionID := newFakeRuntime(t, host, port)
	result, err := runtime.accessStats(context.Background(), connectionArgs(t, connectionID))
	if err != nil {
		t.Fatalf("access.stats: %v", err)
	}
	stats := result.(*AccessStats)
	if stats.Path != "" {
		t.Fatalf("path = %q, want empty for an unreadable target", stats.Path)
	}
	if !strings.Contains(stats.Note, "off") {
		t.Fatalf("note = %q, want the access_log off explanation", stats.Note)
	}
	if stats.Total != 0 {
		t.Fatalf("total = %d, want 0 analyzed lines", stats.Total)
	}
}

// logs.search must honor the level filter locally: the remote grep matches
// text only, so without this the match budget is spent on excluded lines.
func TestLogsSearchLevelFilter(t *testing.T) {
	nl := string(rune(10))
	lines := []string{
		"2026/10/24 10:00:00 [notice] 1#0: signal 17",
		"2026/10/24 10:00:01 [warn] 1#0: upstream slow",
		"2026/10/24 10:00:02 [error] 1#0: connect() failed",
		"2026/10/24 10:00:03 [emerg] 1#0: no memory",
	}
	content := strings.Join(lines, nl) + nl
	base := fileCommands(map[string][]byte{
		"/var/log/nginx/error.log": []byte(content),
	})
	// Emulate "tail ... | grep -n": every line matches, numbered from 1.
	handler := func(command string) (string, string, int) {
		if strings.Contains(command, "| grep ") {
			var out strings.Builder
			for i, line := range lines {
				out.WriteString(strconv.Itoa(i+1) + ":" + line + nl)
			}
			return out.String(), "", 0
		}
		return base(command)
	}
	host, port := startFakeSSH(t, handler)
	runtime, connectionID := newFakeRuntime(t, host, port)

	result, err := runtime.logsSearch(context.Background(),
		connectionArgs(t, connectionID, "kind", "error", "pattern", "1#0", "level", "error"))
	if err != nil {
		t.Fatalf("logs.search: %v", err)
	}
	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	raw, err := json.Marshal(payload["matches"])
	if err != nil {
		t.Fatalf("marshal matches: %v", err)
	}
	var matches []struct {
		WindowLine int    `json:"windowLine"`
		Text       string `json:"text"`
	}
	if err := json.Unmarshal(raw, &matches); err != nil {
		t.Fatalf("unmarshal matches: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("matches = %d, want the error and emerg lines only: %s", len(matches), raw)
	}
	for _, m := range matches {
		if strings.Contains(m.Text, "[notice]") || strings.Contains(m.Text, "[warn]") {
			t.Fatalf("level filter let a lower level through: %q", m.Text)
		}
	}
	if payload["level"] != "error" {
		t.Fatalf("level echo = %v", payload["level"])
	}

	if _, err := runtime.logsSearch(context.Background(),
		connectionArgs(t, connectionID, "kind", "error", "pattern", "x", "level", "bogus")); err == nil {
		t.Fatal("an unknown level must be rejected")
	}
}

func generateCertificatePEM(t *testing.T, commonName string, dnsNames []string, notAfter time.Time) []byte {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
