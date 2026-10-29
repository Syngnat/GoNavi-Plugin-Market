package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// testCanary stands in for real session material. Every assertion below fails as
// soon as it shows up in a message or a value returned to the window.
const (
	testCanaryAccess  = "CANARY-ACCESS-TOKEN-0123456789"
	testCanaryRefresh = "CANARY-REFRESH-TOKEN-0123456789"
	testNowMillis     = int64(1_700_000_000_000)
)

type stubSource struct {
	set tokenSet
	ok  bool
	err error
}

func (s stubSource) tokens(context.Context) (tokenSet, bool, error) {
	return s.set, s.ok, s.err
}

func fixedNow() time.Time { return time.UnixMilli(testNowMillis) }

func newTestService(t *testing.T, source tokenSource, baseURL string) *accountService {
	t.Helper()
	dir := t.TempDir()
	panel := newPanelClient()
	panel.baseURL = baseURL
	return &accountService{
		dataDir: dir,
		panel:   panel,
		source:  source,
		chrome:  newChromeLauncher(dir),
		now:     fixedNow,
	}
}

// panelStub is a local stand-in for api.hualong.online. Nothing here reaches the
// network: httptest binds 127.0.0.1 on an ephemeral port.
type panelStub struct {
	mu       sync.Mutex
	requests []stubRequest
	handler  func(stubRequest) (int, string)
}

type stubRequest struct {
	Method string
	Path   string
	Query  string
	Auth   string
	Body   string
}

func newPanelStub(t *testing.T, handler func(stubRequest) (int, string)) (*panelStub, string) {
	t.Helper()
	stub := &panelStub{handler: handler}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 0, 256)
		if r.Body != nil {
			buffer := make([]byte, 2<<10)
			n, _ := r.Body.Read(buffer)
			body = buffer[:n]
		}
		request := stubRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Auth: r.Header.Get("Authorization"), Body: string(body)}
		stub.mu.Lock()
		stub.requests = append(stub.requests, request)
		stub.mu.Unlock()
		status, payload := stub.handler(request)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(server.Close)
	return stub, server.URL
}

func (s *panelStub) seen() []stubRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stubRequest(nil), s.requests...)
}

func (s *panelStub) count(path string) int {
	total := 0
	for _, request := range s.seen() {
		if request.Path == path {
			total++
		}
	}
	return total
}

// outcomeOf decodes one handler result and asserts the envelope is well formed.
func outcomeOf(t *testing.T, value any) outcome {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result outcome
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.OK && result.Error != nil {
		t.Fatal("a successful result must not carry an error")
	}
	if !result.OK && result.Error == nil {
		t.Fatal("a failed result must carry an error")
	}
	return result
}

// assertNoCanary fails when session material reaches the window.
func assertNoCanary(t *testing.T, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, canary := range []string{testCanaryAccess, testCanaryRefresh} {
		if strings.Contains(string(raw), canary) {
			t.Fatalf("a returned value leaked session material")
		}
	}
}

func dataOf(t *testing.T, result outcome) map[string]any {
	t.Helper()
	raw, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPollReportsPendingWithoutTokens(t *testing.T) {
	service := newTestService(t, stubSource{}, panelBaseURL)
	value, err := service.poll(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK || dataOf(t, result)["state"] != statePending {
		t.Fatalf("expected pending, got %#v", result)
	}
	if _, statErr := os.Stat(filepath.Join(service.dataDir, accountFileName)); !os.IsNotExist(statErr) {
		t.Fatal("a pending poll must not write a session file")
	}
}

func TestPollPersistsTokensAndReturnsReady(t *testing.T) {
	source := stubSource{ok: true, set: tokenSet{
		Access: testCanaryAccess, Refresh: testCanaryRefresh,
		ExpiresAt: testNowMillis + 3_600_000, UserID: 43, Email: "user@example.test",
	}}
	service := newTestService(t, source, panelBaseURL)
	value, err := service.poll(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	data := dataOf(t, result)
	if !result.OK || data["state"] != stateReady {
		t.Fatalf("expected ready, got %#v", result)
	}
	assertNoCanary(t, value)

	raw, err := os.ReadFile(filepath.Join(service.dataDir, accountFileName))
	if err != nil {
		t.Fatal(err)
	}
	var record accountRecord
	if err = json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	if record.AccessToken != testCanaryAccess || record.RefreshToken != testCanaryRefresh || record.ExpiresAt == 0 || record.UserID != 43 {
		t.Fatal("the stored session does not match what CDP returned")
	}
	info, err := os.Stat(filepath.Join(service.dataDir, accountFileName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("the session file must be owner-only, got %v", info.Mode().Perm())
	}
}

const mePayload = `{"code":0,"data":{"id":43,"email":"user@example.test","role":"user","balance":63.69358181,` +
	`"frozen_balance":0,"concurrency":200,"status":"active","total_recharged":12.88,` +
	`"last_active_at":"2026-10-28T12:00:00Z","created_at":"2026-10-01T00:00:00Z","run_mode":"standard"}}`

func TestStatusUsesTheStoredTokenWhenItIsStillValid(t *testing.T) {
	stub, baseURL := newPanelStub(t, func(stubRequest) (int, string) { return http.StatusOK, mePayload })
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, RefreshToken: testCanaryRefresh, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.status(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	data := dataOf(t, result)
	if !result.OK || data["state"] != stateReady {
		t.Fatalf("expected a ready snapshot, got %#v", result)
	}
	if data["balance"].(float64) != 63.69358181 || data["frozenBalance"].(float64) != 0 ||
		data["totalRecharged"].(float64) != 12.88 || data["concurrency"].(float64) != 200 ||
		data["status"] != "active" || data["email"] != "user@example.test" {
		t.Fatalf("unexpected snapshot: %#v", data)
	}
	assertNoCanary(t, value)
	if stub.count("/api/v1/auth/refresh") != 0 {
		t.Fatal("a valid token must not be refreshed")
	}
	requests := stub.seen()
	if len(requests) != 1 || requests[0].Auth != "Bearer "+testCanaryAccess {
		t.Fatalf("unexpected panel traffic: %#v", requests)
	}
	if requests[0].Query != "timezone=Asia%2FShanghai" {
		t.Fatalf("unexpected query: %q", requests[0].Query)
	}
}

func TestStatusRefreshesAnExpiredToken(t *testing.T) {
	const rotated = "rotated-access-token"
	stub, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		if request.Path == "/api/v1/auth/refresh" {
			if !strings.Contains(request.Body, `"refresh_token":"`+testCanaryRefresh+`"`) {
				return http.StatusBadRequest, `{"code":400,"message":"bad body"}`
			}
			return http.StatusOK, `{"code":0,"data":{"access_token":"` + rotated + `","refresh_token":"rotated-refresh","expires_in":3600,"token_type":"Bearer"}}`
		}
		return http.StatusOK, mePayload
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, RefreshToken: testCanaryRefresh, ExpiresAt: testNowMillis - 1000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.status(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	data := dataOf(t, result)
	if !result.OK || data["state"] != stateReady || data["refreshed"] != true {
		t.Fatalf("expected a refreshed snapshot, got %#v", result)
	}
	if stub.count("/api/v1/auth/refresh") != 1 {
		t.Fatal("an expired token must be refreshed exactly once")
	}
	requests := stub.seen()
	if len(requests) != 2 || requests[1].Auth != "Bearer "+rotated {
		t.Fatalf("the renewed token was not used: %#v", requests)
	}
	record, ok := service.loadRecord()
	if !ok || record.AccessToken != rotated || record.RefreshToken != "rotated-refresh" || record.ExpiresAt != testNowMillis+3_600_000 {
		t.Fatalf("the rotated session was not persisted: %#v", record)
	}
	assertNoCanary(t, value)
}

func TestStatusReportsSignedOutWithoutASession(t *testing.T) {
	service := newTestService(t, stubSource{}, panelBaseURL)
	value, err := service.status(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK || dataOf(t, result)["state"] != stateSignedOut {
		t.Fatalf("expected signed_out, got %#v", result)
	}
}

func TestStatusSignsOutWhenThePanelRejectsTheToken(t *testing.T) {
	stub, baseURL := newPanelStub(t, func(stubRequest) (int, string) {
		return http.StatusUnauthorized, `{"code":401,"message":"` + testCanaryAccess + `"}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, RefreshToken: testCanaryRefresh}); err != nil {
		t.Fatal(err)
	}
	value, err := service.status(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK || dataOf(t, result)["state"] != stateSignedOut {
		t.Fatalf("expected signed_out, got %#v", result)
	}
	assertNoCanary(t, value)
	if stub.count("/api/v1/auth/refresh") != 0 {
		t.Fatal("a token with no recorded expiry must not be refreshed blindly")
	}
	if _, ok := service.loadRecord(); ok {
		t.Fatal("a rejected session must be cleared")
	}
}

func TestStatusRejectsNonZeroCodeWithoutLeakingTheToken(t *testing.T) {
	stub, baseURL := newPanelStub(t, func(stubRequest) (int, string) {
		return http.StatusOK, `{"code":1001,"message":"` + testCanaryAccess + `"}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, RefreshToken: testCanaryRefresh}); err != nil {
		t.Fatal(err)
	}
	value, err := service.status(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if result.OK || result.Error.Code != "panel_rejected" {
		t.Fatalf("expected panel_rejected, got %#v", result)
	}
	if result.Error.Message != messagePanelRejected {
		t.Fatalf("server text must not reach the window: %q", result.Error.Message)
	}
	assertNoCanary(t, value)
	if stub.count("/api/v1/keys") != 0 {
		t.Fatal("unexpected traffic")
	}
}

func TestStatusRejectsANonOKStatus(t *testing.T) {
	_, baseURL := newPanelStub(t, func(stubRequest) (int, string) {
		return http.StatusInternalServerError, `{"code":500,"message":"` + testCanaryAccess + `"}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess}); err != nil {
		t.Fatal(err)
	}
	value, err := service.status(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if result.OK || result.Error.Code != "panel_unavailable" {
		t.Fatalf("expected panel_unavailable, got %#v", result)
	}
	assertNoCanary(t, value)
}

const keysPayload = `{"code":0,"data":{"items":[` +
	`{"id":5921,"name":"默认 Key","key":"sk-abcdefghijklmnop","status":"active","group_id":34,"quota":50,` +
	`"quota_used":0.76,"last_used_at":"2026-10-28T09:00:00Z","created_at":"2026-10-01T00:00:00Z","expires_at":null,` +
	`"group":{"name":"Grok-heavy分组","rate_multiplier":0.18,"platform":"grok"}},` +
	`{"id":5922,"name":"备用 Key","key":"sk-zyxwvutsrqponmlk","status":"disabled","quota":10,"quota_used":10,` +
	`"last_used_at":null,"created_at":"2026-10-02T00:00:00Z","expires_at":null,"group":null}` +
	`],"total":2,"page":1,"page_size":50,"pages":1}}`

func TestKeysMapsEveryField(t *testing.T) {
	stub, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		if request.Path != "/api/v1/keys" {
			return http.StatusNotFound, `{"code":404}`
		}
		return http.StatusOK, keysPayload
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.keys(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK {
		t.Fatalf("expected a key list, got %#v", result)
	}
	raw, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		State string    `json:"state"`
		Items []keyItem `json:"items"`
	}
	if err = json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.State != stateReady || len(payload.Items) != 2 {
		t.Fatalf("unexpected list: %#v", payload)
	}
	first := payload.Items[0]
	if first.ID != 5921 || first.Name != "默认 Key" || first.Key != "sk-abcdefghijklmnop" || first.Status != "active" ||
		first.GroupName != "Grok-heavy分组" || first.RateMultiplier != 0.18 || first.Platform != "grok" ||
		first.Quota != 50 || first.QuotaUsed != 0.76 || first.LastUsedAt != "2026-10-28T09:00:00Z" {
		t.Fatalf("unexpected first key: %#v", first)
	}
	second := payload.Items[1]
	if second.GroupName != "" || second.Platform != "" || second.LastUsedAt != "" {
		t.Fatalf("a key without a group or usage must stay empty: %#v", second)
	}
	requests := stub.seen()
	if len(requests) != 1 || requests[0].Query != "page=1&page_size=50" || requests[0].Auth != "Bearer "+testCanaryAccess {
		t.Fatalf("unexpected request: %#v", requests)
	}
}

func TestKeysRequiresASession(t *testing.T) {
	service := newTestService(t, stubSource{}, panelBaseURL)
	value, err := service.keys(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if result.OK || result.Error.Code != "not_signed_in" {
		t.Fatalf("expected not_signed_in, got %#v", result)
	}
}

func TestKeysDropsTheSessionWhenRefreshIsRejected(t *testing.T) {
	_, baseURL := newPanelStub(t, func(stubRequest) (int, string) {
		return http.StatusUnauthorized, `{"code":401}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, RefreshToken: testCanaryRefresh, ExpiresAt: testNowMillis - 1}); err != nil {
		t.Fatal(err)
	}
	value, err := service.keys(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if result.OK || result.Error.Code != "panel_unauthorized" {
		t.Fatalf("expected panel_unauthorized, got %#v", result)
	}
	assertNoCanary(t, value)
	if _, ok := service.loadRecord(); ok {
		t.Fatal("a rejected session must be cleared")
	}
}

func TestStatusReportsTokenExpiryWithoutARefreshToken(t *testing.T) {
	service := newTestService(t, stubSource{}, panelBaseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis - 1}); err != nil {
		t.Fatal(err)
	}
	value, err := service.status(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK || dataOf(t, result)["state"] != stateSignedOut {
		t.Fatalf("expected signed_out, got %#v", result)
	}
	assertNoCanary(t, value)
}

func TestLogoutClearsTheSession(t *testing.T) {
	service := newTestService(t, stubSource{}, panelBaseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess}); err != nil {
		t.Fatal(err)
	}
	value, err := service.logout(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK || dataOf(t, result)["state"] != stateSignedOut {
		t.Fatalf("expected signed_out, got %#v", result)
	}
	if _, ok := service.loadRecord(); ok {
		t.Fatal("logout must forget the stored session")
	}
	if _, err = os.Stat(filepath.Join(service.dataDir, sessionFileName)); !os.IsNotExist(err) {
		t.Fatal("logout must drop the browser session record")
	}
}

// TestPollSurvivesAStubError keeps a transport failure inside the envelope
// instead of escalating to a host-level error.
func TestPollSurvivesAStubError(t *testing.T) {
	service := newTestService(t, stubSource{err: errCDPUnavailable}, panelBaseURL)
	value, err := service.poll(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if result.OK || result.Error.Code != "cdp_unavailable" {
		t.Fatalf("expected cdp_unavailable, got %#v", result)
	}
}

// TestOutcomeNeverCarriesSessionMaterial feeds session-looking errors through the
// envelope: only fixed messages may survive.
func TestOutcomeNeverCarriesSessionMaterial(t *testing.T) {
	result := rejected(coded("panel_unavailable", messagePanelUnavailable))
	if strings.Contains(result.Error.Message, testCanaryAccess) {
		t.Fatal("the fixed message was replaced")
	}
	if fallback := rejected(errCDPUnavailable); fallback.Error.Code != "internal" || fallback.Error.Message != messageInternal {
		t.Fatalf("an uncoded error must map to the fixed internal failure, got %#v", fallback)
	}
}

// TestKeysRejectsANonOKStatusWithoutLeakingTheToken covers the list endpoint's
// failure branch: a body that echoes the token must not reach the window.
func TestKeysRejectsANonOKStatusWithoutLeakingTheToken(t *testing.T) {
	_, baseURL := newPanelStub(t, func(stubRequest) (int, string) {
		return http.StatusBadGateway, `{"code":502,"message":"` + testCanaryAccess + `"}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.keys(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if result.OK || result.Error.Code != "panel_unavailable" {
		t.Fatalf("expected panel_unavailable, got %#v", result)
	}
	assertNoCanary(t, value)
}

// TestKeysRejectsANonZeroCode covers the envelope branch of the list endpoint.
func TestKeysRejectsANonZeroCode(t *testing.T) {
	_, baseURL := newPanelStub(t, func(stubRequest) (int, string) {
		return http.StatusOK, `{"code":7,"message":"` + testCanaryAccess + `","data":null}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.keys(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if result.OK || result.Error.Code != "panel_rejected" {
		t.Fatalf("expected panel_rejected, got %#v", result)
	}
	assertNoCanary(t, value)
}

// TestKeysClearsARevokedSession keeps a directly rejected access token from
// lingering in the store after the list call.
func TestKeysClearsARevokedSession(t *testing.T) {
	_, baseURL := newPanelStub(t, func(stubRequest) (int, string) {
		return http.StatusUnauthorized, `{"code":401,"message":"` + testCanaryAccess + `"}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.keys(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if result.OK || result.Error.Code != "panel_unauthorized" {
		t.Fatalf("expected panel_unauthorized, got %#v", result)
	}
	assertNoCanary(t, value)
	if _, ok := service.loadRecord(); ok {
		t.Fatal("a revoked session must be cleared")
	}
}
