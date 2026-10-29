package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// panelOrigin owns the localStorage tokens the plugin needs.
	panelOrigin  = "https://api.hualong.online"
	cdpTimeout   = 4 * time.Second
	cdpMaxListed = 32
	// tokenExpression reads the four documented keys in one round trip. The
	// expression is a fixed literal: no user input is interpolated into it.
	tokenExpression = `JSON.stringify({href:location.href,` +
		`auth_token:localStorage.getItem('auth_token'),` +
		`refresh_token:localStorage.getItem('refresh_token'),` +
		`token_expires_at:localStorage.getItem('token_expires_at'),` +
		`auth_user:localStorage.getItem('auth_user')})`
)

var errCDPUnavailable = errors.New("devtools endpoint unavailable")

// tokenSet is the panel session as read from localStorage. It never leaves the
// backend: only derived fields reach the window.
type tokenSet struct {
	Access    string
	Refresh   string
	ExpiresAt int64
	UserID    int64
	Email     string
}

type cdpTarget struct {
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type cdpFrame struct {
	ID     int `json:"id"`
	Result struct {
		Result struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// cdpSource reads the tokens from the private browser started by account.login.
// The dial function is a field so tests can supply a socket-level stub.
type cdpSource struct {
	dataDir string
	client  *http.Client
	dial    func(string) (*wsConn, error)
}

func newCDPSource(dataDir string) *cdpSource {
	return &cdpSource{dataDir: dataDir, dial: dialWebSocket, client: devtoolsHTTPClient()}
}

// devtoolsHTTPClient talks only to the loopback DevTools endpoint: no proxy, no
// keep-alives, no redirects.
func devtoolsHTTPClient() *http.Client {
	return &http.Client{
		Timeout: chromeProbeTime,
		Transport: &http.Transport{
			Proxy:               nil,
			DisableKeepAlives:   true,
			DisableCompression:  true,
			MaxConnsPerHost:     2,
			TLSHandshakeTimeout: chromeProbeTime,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// browserSocket reads the browser-level DevTools socket that owns the process,
// which is how logout asks Chrome to close itself.
func browserSocket(port int) (string, error) {
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/json/version", nil)
	if err != nil {
		return "", errCDPUnavailable
	}
	client := devtoolsHTTPClient()
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		return "", errCDPUnavailable
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", errCDPUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return "", errCDPUnavailable
	}
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if json.Unmarshal(raw, &version) != nil || !strings.HasPrefix(version.WebSocketDebuggerURL, "ws://") || len(version.WebSocketDebuggerURL) > 2048 {
		return "", errCDPUnavailable
	}
	return version.WebSocketDebuggerURL, nil
}

// tokens reports the panel session when the browser already holds one. A
// missing browser, a closed port or a still-signed-out SPA is not an error:
// those are the legitimate pending states the window polls through.
func (s *cdpSource) tokens(ctx context.Context) (tokenSet, bool, error) {
	state, ok := readSession(s.dataDir)
	if !ok {
		return tokenSet{}, false, nil
	}
	targets, err := s.targets(ctx, state.Port)
	if err != nil {
		return tokenSet{}, false, nil
	}
	target, ok := pickTarget(targets)
	if !ok {
		return tokenSet{}, false, nil
	}
	conn, err := s.dial(target.WebSocketDebuggerURL)
	if err != nil {
		return tokenSet{}, false, nil
	}
	defer func() { _ = conn.Close() }()
	value, err := evaluate(ctx, conn, tokenExpression)
	if err != nil {
		return tokenSet{}, false, nil
	}
	set, ok := parseTokenSet(value)
	return set, ok, nil
}

func (s *cdpSource) targets(ctx context.Context, port int) ([]cdpTarget, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/json/list", nil)
	if err != nil {
		return nil, errCDPUnavailable
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, errCDPUnavailable
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, errCDPUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || len(raw) == 0 {
		return nil, errCDPUnavailable
	}
	var targets []cdpTarget
	if json.Unmarshal(raw, &targets) != nil || len(targets) > cdpMaxListed {
		return nil, errCDPUnavailable
	}
	return targets, nil
}

// pickTarget prefers the panel SPA, then any page that exposes a debugger URL.
func pickTarget(targets []cdpTarget) (cdpTarget, bool) {
	var fallback cdpTarget
	found := false
	for _, target := range targets {
		if target.Type != "page" || target.WebSocketDebuggerURL == "" || !strings.HasPrefix(target.WebSocketDebuggerURL, "ws://") {
			continue
		}
		if strings.HasPrefix(target.URL, panelOrigin) {
			return target, true
		}
		if !found {
			fallback, found = target, true
		}
	}
	return fallback, found
}

// evaluate runs one Runtime.evaluate call and returns its string value.
func evaluate(ctx context.Context, conn *wsConn, expression string) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"id":     1,
		"method": "Runtime.evaluate",
		"params": map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true},
	})
	if err != nil {
		return "", errCDPUnavailable
	}
	if err := conn.writeFrame(wsOpcodeText, payload); err != nil {
		return "", errCDPUnavailable
	}
	deadline := time.Now().Add(cdpTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.conn.SetDeadline(deadline); err != nil {
		return "", errCDPUnavailable
	}
	for time.Now().Before(deadline) {
		raw, err := conn.readText()
		if err != nil {
			return "", errCDPUnavailable
		}
		var frame cdpFrame
		if json.Unmarshal(raw, &frame) != nil || frame.ID != 1 {
			continue
		}
		if frame.Error != nil {
			return "", errCDPUnavailable
		}
		var value string
		if json.Unmarshal(frame.Result.Result.Value, &value) != nil {
			return "", errCDPUnavailable
		}
		return value, nil
	}
	return "", errCDPUnavailable
}

// parseTokenSet extracts the panel session from the evaluated JSON. Anything
// without an access token means the user has not finished logging in yet.
func parseTokenSet(raw string) (tokenSet, bool) {
	if len(raw) == 0 || len(raw) > 64<<10 {
		return tokenSet{}, false
	}
	var stored struct {
		AuthToken      *string `json:"auth_token"`
		RefreshToken   *string `json:"refresh_token"`
		TokenExpiresAt *string `json:"token_expires_at"`
		AuthUser       *string `json:"auth_user"`
	}
	if json.Unmarshal([]byte(raw), &stored) != nil {
		return tokenSet{}, false
	}
	set := tokenSet{Access: trimmed(stored.AuthToken), Refresh: trimmed(stored.RefreshToken)}
	if set.Access == "" || len(set.Access) > 8192 || len(set.Refresh) > 8192 {
		return tokenSet{}, false
	}
	if expires, err := strconv.ParseInt(strings.TrimSpace(trimmed(stored.TokenExpiresAt)), 10, 64); err == nil && expires > 0 {
		set.ExpiresAt = expires
	}
	if user := trimmed(stored.AuthUser); user != "" && len(user) <= 8192 {
		var profile struct {
			ID    json.Number `json:"id"`
			Email string      `json:"email"`
		}
		if json.Unmarshal([]byte(user), &profile) == nil {
			if profile.Email != "" && len(profile.Email) <= 320 {
				set.Email = profile.Email
			}
			if id, err := profile.ID.Int64(); err == nil && id > 0 {
				set.UserID = id
			}
		}
	}
	return set, true
}

func trimmed(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// userHint is the signed-in identity shown while the window waits for status.
type userHint struct {
	UserID int64  `json:"userId,omitempty"`
	Email  string `json:"email,omitempty"`
}
