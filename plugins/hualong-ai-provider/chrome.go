package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

const (
	// chromeEnvOverride points the launcher at a Chrome the user installed elsewhere.
	chromeEnvOverride = "GONAVI_HUALONG_CHROME"
	chromeProfileName = "chrome-profile"
	sessionFileName   = "session.json"
	// loginURL is the 華龍 panel SPA route that writes the tokens into localStorage.
	loginURL          = "https://api.hualong.online/login"
	chromeStartBudget = 8 * time.Second
	chromeProbeTime   = 2 * time.Second
	chromeProbePause  = 150 * time.Millisecond
)

var (
	errChromeMissing = errors.New("chrome executable not found")
	errChromeLaunch  = errors.New("chrome did not expose a debugging port")
)

// chromeCandidate is either an absolute path or a name resolved through PATH.
type chromeCandidate struct {
	path   string
	lookup bool
}

// chromeLauncher owns the private browser profile and the debugging port record.
// Tests never construct one: they stub the token source instead.
type chromeLauncher struct {
	dataDir string
	mu      sync.Mutex
	process *os.Process
}

func newChromeLauncher(dataDir string) *chromeLauncher {
	return &chromeLauncher{dataDir: dataDir}
}

// sessionState is the on-disk record that lets a later plugin process reuse the
// browser an earlier generation already started.
type sessionState struct {
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Profile string `json:"profile"`
}

// ensure reuses a live debugging endpoint or starts a private browser. It never
// waits for the login itself: the window polls account.poll instead.
func (l *chromeLauncher) ensure(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if state, ok := readSession(l.dataDir); ok && probeCDP(state.Port) {
		return nil
	}
	return l.start(ctx)
}

func (l *chromeLauncher) start(ctx context.Context) error {
	executable, err := resolveChrome(runtime.GOOS, os.Getenv, exec.LookPath, pathExists)
	if err != nil {
		return err
	}
	port, err := freePort()
	if err != nil {
		return errChromeLaunch
	}
	profile := filepath.Join(l.dataDir, chromeProfileName)
	if err := os.MkdirAll(profile, 0o700); err != nil {
		return errChromeLaunch
	}
	command := exec.Command(executable,
		"--remote-debugging-port="+strconv.Itoa(port),
		"--user-data-dir="+profile,
		"--no-first-run",
		"--no-default-browser-check",
		loginURL)
	command.SysProcAttr = chromeSysProcAttr()
	// Chrome outlives this process while the user logs in, so it is not tied to
	// our pipes; session.json is the durable handle instead.
	command.Stdout, command.Stderr = nil, nil
	if err := command.Start(); err != nil {
		return errChromeLaunch
	}
	l.process = command.Process
	state := sessionState{Port: port, PID: command.Process.Pid, Profile: profile}
	if err := writeJSONFile(filepath.Join(l.dataDir, sessionFileName), state); err != nil {
		_ = command.Process.Kill()
		return errChromeLaunch
	}
	// A second launch with the same profile hands the URL to the first instance
	// and exits, so a port that never answers means this browser is unusable.
	if !waitForCDP(ctx, port) {
		_ = command.Process.Kill()
		_ = os.Remove(filepath.Join(l.dataDir, sessionFileName))
		return errChromeLaunch
	}
	return nil
}

// stop closes only the browser this plugin owns, identified by session.json. The
// process handle is used when this very process launched Chrome; otherwise the
// browser is asked to shut itself down over CDP, so a recycled PID can never make
// the plugin kill an unrelated program.
func (l *chromeLauncher) stop() {
	l.mu.Lock()
	state, ok := readSession(l.dataDir)
	process := l.process
	l.process = nil
	l.mu.Unlock()
	_ = os.Remove(filepath.Join(l.dataDir, sessionFileName))
	if !ok {
		return
	}
	if process != nil && process.Pid == state.PID {
		_ = process.Kill()
		_, _ = process.Wait()
		return
	}
	closeChrome(state.Port)
}

// closeChrome sends Browser.close on the browser-level DevTools socket. Chrome
// drops the connection before answering, so a write error after the call is the
// expected outcome and not reported.
func closeChrome(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	endpoint, err := browserSocket(port)
	if err != nil {
		return false
	}
	conn, err := dialWebSocket(endpoint)
	if err != nil {
		return false
	}
	defer func() { _ = conn.Close() }()
	payload := []byte(`{"id":1,"method":"Browser.close"}`)
	return conn.writeFrame(wsOpcodeText, payload) == nil
}

func readSession(dataDir string) (sessionState, bool) {
	raw, err := os.ReadFile(filepath.Join(dataDir, sessionFileName))
	if err != nil {
		return sessionState{}, false
	}
	var state sessionState
	if json.Unmarshal(raw, &state) != nil || state.Port <= 0 || state.Port > 65535 || state.PID < 0 {
		return sessionState{}, false
	}
	return state, true
}

func waitForCDP(ctx context.Context, port int) bool {
	deadline := time.Now().Add(chromeStartBudget)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return false
		}
		if probeCDP(port) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(chromeProbePause):
		}
	}
	return false
}

// probeCDP answers whether a port still serves a DevTools endpoint.
func probeCDP(port int) bool {
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/json/version", nil)
	if err != nil {
		return false
	}
	client := devtoolsHTTPClient()
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	_ = response.Body.Close()
	return response.StatusCode == http.StatusOK
}

// freePort asks the kernel for an unused loopback port and releases it again.
// Chrome binds it moments later; the gap is a local race only.
func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return 0, errChromeLaunch
	}
	port := address.Port
	if err := listener.Close(); err != nil {
		return 0, err
	}
	return port, nil
}

// resolveChrome follows the documented order: explicit override, then the
// platform's well-known install locations.
func resolveChrome(goos string, env func(string) string, lookup func(string) (string, error), exists func(string) bool) (string, error) {
	if override := env(chromeEnvOverride); override != "" {
		if exists(override) {
			return override, nil
		}
		return "", errChromeMissing
	}
	for _, candidate := range chromeCandidates(goos, env) {
		if candidate.lookup {
			if path, err := lookup(candidate.path); err == nil && path != "" {
				return path, nil
			}
			continue
		}
		if exists(candidate.path) {
			return candidate.path, nil
		}
	}
	return "", errChromeMissing
}

func chromeCandidates(goos string, env func(string) string) []chromeCandidate {
	switch goos {
	case "darwin":
		return []chromeCandidate{{path: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}}
	case "windows":
		var candidates []chromeCandidate
		for _, base := range []string{env("ProgramFiles"), env("ProgramFiles(x86)")} {
			if base != "" {
				candidates = append(candidates, chromeCandidate{path: filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe")})
			}
		}
		if local := env("LOCALAPPDATA"); local != "" {
			candidates = append(candidates, chromeCandidate{path: filepath.Join(local, "Google", "Chrome", "Application", "chrome.exe")})
		}
		return candidates
	default:
		return []chromeCandidate{
			{path: "google-chrome", lookup: true},
			{path: "google-chrome-stable", lookup: true},
			{path: "chromium", lookup: true},
			{path: "chromium-browser", lookup: true},
		}
	}
}

func pathExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// writeJSONFile replaces a small state file atomically with owner-only permissions.
func writeJSONFile(path string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, append(encoded, '\n'), 0o600); err != nil {
		return err
	}
	// POSIX rename replaces the target atomically; Windows refuses to replace an
	// existing file, so the target is removed before the second attempt.
	if err := os.Rename(temporary, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		return os.Rename(temporary, path)
	}
	return nil
}
