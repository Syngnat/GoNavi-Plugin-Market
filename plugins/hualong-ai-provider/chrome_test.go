package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fakeEnv(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func noLookup(string) (string, error) { return "", errors.New("not installed") }

func TestResolveChromePrefersTheOverride(t *testing.T) {
	env := fakeEnv(map[string]string{chromeEnvOverride: "/opt/chrome"})
	exists := func(path string) bool { return path == "/opt/chrome" }
	path, err := resolveChrome("darwin", env, noLookup, exists)
	if err != nil || path != "/opt/chrome" {
		t.Fatalf("expected the override to win, got %q %v", path, err)
	}
	// A configured override that does not exist is an error: silently falling
	// back would launch a browser the user did not ask for.
	if _, err = resolveChrome("darwin", env, noLookup, func(string) bool { return false }); !errors.Is(err, errChromeMissing) {
		t.Fatalf("expected a missing-executable error, got %v", err)
	}
}

func TestResolveChromeUsesPlatformLocations(t *testing.T) {
	darwinPath := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	path, err := resolveChrome("darwin", fakeEnv(nil), noLookup, func(value string) bool { return value == darwinPath })
	if err != nil || path != darwinPath {
		t.Fatalf("unexpected darwin resolution: %q %v", path, err)
	}

	windows := fakeEnv(map[string]string{
		"ProgramFiles":      `C:\Program Files`,
		"ProgramFiles(x86)": `C:\Program Files (x86)`,
		"LOCALAPPDATA":      `C:\Users\tester\AppData\Local`,
	})
	var seen []string
	path, err = resolveChrome("windows", windows, noLookup, func(value string) bool {
		seen = append(seen, value)
		return strings.HasPrefix(value, `C:\Users\tester\AppData\Local`)
	})
	if err != nil || !strings.Contains(path, "chrome.exe") {
		t.Fatalf("unexpected windows resolution: %q %v", path, err)
	}
	if len(seen) != 3 {
		t.Fatalf("expected three windows candidates, got %d: %#v", len(seen), seen)
	}

	var looked []string
	path, err = resolveChrome("linux", fakeEnv(nil), func(name string) (string, error) {
		looked = append(looked, name)
		if name == "chromium" {
			return "/usr/bin/chromium", nil
		}
		return "", errors.New("missing")
	}, func(string) bool { return false })
	if err != nil || path != "/usr/bin/chromium" {
		t.Fatalf("unexpected linux resolution: %q %v", path, err)
	}
	if len(looked) != 3 || looked[0] != "google-chrome" || looked[2] != "chromium" {
		t.Fatalf("unexpected lookup order: %#v", looked)
	}
}

func TestResolveChromeReportsNothingFound(t *testing.T) {
	if _, err := resolveChrome("linux", fakeEnv(nil), noLookup, func(string) bool { return false }); !errors.Is(err, errChromeMissing) {
		t.Fatalf("expected a missing-executable error, got %v", err)
	}
}

func TestReadSessionIgnoresUnusableRecords(t *testing.T) {
	dir := t.TempDir()
	if _, ok := readSession(dir); ok {
		t.Fatal("a missing record is not a session")
	}
	for name, body := range map[string]string{
		"invalid json": `{"port":`,
		"zero port":    `{"port":0,"pid":10}`,
		"out of range": `{"port":70000,"pid":10}`,
		"negative pid": `{"port":9222,"pid":-1}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, sessionFileName), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, ok := readSession(dir); ok {
			t.Fatalf("%s: expected the record to be rejected", name)
		}
	}
	valid := `{"port":9222,"pid":4242,"profile":"/tmp/profile"}`
	if err := os.WriteFile(filepath.Join(dir, sessionFileName), []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	state, ok := readSession(dir)
	if !ok || state.Port != 9222 || state.PID != 4242 {
		t.Fatalf("unexpected record: %#v", state)
	}
}

func TestProbeCDPDetectsALiveEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Browser":"Chrome/141"}`))
	}))
	defer server.Close()
	address, ok := server.Listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("the test server is not listening on TCP")
	}
	if !probeCDP(address.Port) {
		t.Fatal("a live endpoint must be detected")
	}
	server.Close()
	if probeCDP(address.Port) {
		t.Fatal("a closed endpoint must not be detected")
	}
}

// TestEnsureReusesALiveSession proves the reuse path never launches a browser:
// the recorded port answers /json/version, so ensure returns immediately.
func TestEnsureReusesALiveSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Browser":"Chrome/141"}`))
	}))
	defer server.Close()
	address, ok := server.Listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("the test server is not listening on TCP")
	}
	dir := t.TempDir()
	record := `{"port":` + strconv.Itoa(address.Port) + `,"pid":4242,"profile":"` + filepath.ToSlash(dir) + `/chrome-profile"}`
	if err := os.WriteFile(filepath.Join(dir, sessionFileName), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
	launcher := newChromeLauncher(dir)
	if err := launcher.ensure(context.Background()); err != nil {
		t.Fatalf("a live session must be reused, got %v", err)
	}
	state, ok := readSession(dir)
	if !ok || state.Port != address.Port || state.PID != 4242 {
		t.Fatalf("the session record was overwritten: %#v", state)
	}
}

// startDevToolsServer fakes a Chrome DevTools endpoint: /json/version names the
// browser socket, and that socket accepts the Browser.close call.
func startDevToolsServer(t *testing.T, frames func(net.Conn, *bufio.Reader)) (int, chan struct{}) {
	t.Helper()
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/devtools/browser/T1" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://` + r.Host + `/devtools/browser/T1"}`))
			return
		}
		defer close(done)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer cannot be hijacked")
			return
		}
		conn, buffer, err := hijacker.Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = conn.Close() }()
		response := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + wsAcceptValue(r.Header.Get("Sec-WebSocket-Key")) + "\r\n\r\n"
		if _, err = io.WriteString(conn, response); err != nil {
			return
		}
		frames(conn, buffer.Reader)
	}))
	t.Cleanup(server.Close)
	address, ok := server.Listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("the test server is not listening on TCP")
	}
	return address.Port, done
}

// TestStopAsksChromeToCloseItself covers the cross-process path: no process
// handle is held, so logout shuts the browser down over CDP instead of killing
// a recycled PID.
func TestStopAsksChromeToCloseItself(t *testing.T) {
	port, done := startDevToolsServer(t, func(conn net.Conn, reader *bufio.Reader) {
		_, payload := readMaskedFrame(t, reader)
		var call struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &call); err != nil {
			t.Error(err)
			return
		}
		if call.Method != "Browser.close" {
			t.Errorf("unexpected CDP method: %q", call.Method)
		}
	})
	dir := t.TempDir()
	record := `{"port":` + strconv.Itoa(port) + `,"pid":999999,"profile":"` + filepath.ToSlash(dir) + `/chrome-profile"}`
	if err := os.WriteFile(filepath.Join(dir, sessionFileName), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
	newChromeLauncher(dir).stop()
	if _, err := os.Stat(filepath.Join(dir, sessionFileName)); !os.IsNotExist(err) {
		t.Fatal("logout must drop the session record")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the browser was never asked to close")
	}
}

func TestBrowserSocketRejectsAnEndpointWithoutASocket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"http://127.0.0.1:1/devtools/browser/T1"}`))
	}))
	defer server.Close()
	address, ok := server.Listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("the test server is not listening on TCP")
	}
	if _, err := browserSocket(address.Port); !errors.Is(err, errCDPUnavailable) {
		t.Fatalf("expected an unusable endpoint, got %v", err)
	}
}
