package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// dataEnvVariable is injected by the host for every plugin process.
	dataEnvVariable = "GONAVI_PLUGIN_DATA"
	accountFileName = "account.json"
	// refreshSkew renews an access token shortly before it would start failing.
	refreshSkew     = 90 * time.Second
	maxAccountBytes = 64 << 10

	stateReady     = "ready"
	statePending   = "pending"
	stateSignedOut = "signed_out"
)

// Fixed user-facing messages. Nothing derived from a response body is ever
// interpolated here, so no message can carry a token.
const (
	messageInternal          = "插件内部错误，请重试。"
	messageInvalidResponse   = "華龍算力接口返回了无法解析的数据。"
	messagePanelUnavailable  = "连接華龍算力接口失败，请检查网络后重试。"
	messagePanelUnauthorized = "登录状态已失效，请重新登录。"
	messagePanelRejected     = "華龍算力接口拒绝了这次请求。"
	messageTokenExpired      = "登录已过期，请重新登录。"
	messageNotSignedIn       = "尚未登录，请先打开登录浏览器。"
	messageCDPUnavailable    = "浏览器调试端口不可用，请重新打开登录浏览器。"
	messageChromeMissing     = "未找到 Chrome，请安装 Chrome 或设置 GONAVI_HUALONG_CHROME。"
	messageChromeFailed      = "Chrome 启动失败，请关闭本插件打开的浏览器后重试。"
	messageStoreFailed       = "本地账号存储读写失败。"
	messageInvalidName       = "密钥名称无效。"
	messageInvalidID         = "密钥标识无效。"
)

// codedError pairs a stable wire code with a fixed message.
type codedError struct {
	code    string
	message string
}

func (e *codedError) Error() string { return e.message }

func coded(code, message string) error { return &codedError{code: code, message: message} }

func codeOf(err error) string {
	var failure *codedError
	if errors.As(err, &failure) {
		return failure.code
	}
	return "internal"
}

type accountFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// outcome is the single result shape every account command returns.
type outcome struct {
	OK    bool            `json:"ok"`
	Data  any             `json:"data,omitempty"`
	Error *accountFailure `json:"error,omitempty"`
}

func success(data any) outcome { return outcome{OK: true, Data: data} }

// rejected keeps the failure inside the result: the host turns a handler error
// into a bare 502 without the plugin's text.
func rejected(err error) outcome {
	var failure *codedError
	if errors.As(err, &failure) {
		return outcome{Error: &accountFailure{Code: failure.code, Message: failure.message}}
	}
	return outcome{Error: &accountFailure{Code: "internal", Message: messageInternal}}
}

// accountRecord is the on-disk session. Tokens stay in this file only; they are
// never logged, echoed in errors or returned to the window.
type accountRecord struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    int64  `json:"expiresAt"`
	UserID       int64  `json:"userId"`
	SavedAt      int64  `json:"savedAt"`
}

// tokenSource reads the panel session from the private browser.
type tokenSource interface {
	tokens(ctx context.Context) (tokenSet, bool, error)
}

type accountService struct {
	dataDir string
	panel   *panelClient
	source  tokenSource
	chrome  *chromeLauncher
	now     func() time.Time
	mu      sync.Mutex
}

func newAccountService() *accountService {
	dataDir := os.Getenv(dataEnvVariable)
	if dataDir == "" {
		// The host always injects the variable; the fallback only keeps a
		// developer build usable outside the host.
		base, err := os.UserConfigDir()
		if err != nil {
			base = os.TempDir()
		}
		dataDir = filepath.Join(base, "GoNavi", "hualong-ai-provider")
	}
	return &accountService{
		dataDir: dataDir,
		panel:   newPanelClient(),
		source:  newCDPSource(dataDir),
		chrome:  newChromeLauncher(dataDir),
		now:     time.Now,
	}
}

type loginResult struct {
	Started bool `json:"started"`
}

type pollResult struct {
	State string    `json:"state"`
	User  *userHint `json:"user,omitempty"`
}

type statusResult struct {
	State          string  `json:"state"`
	Reason         string  `json:"reason,omitempty"`
	UserID         int64   `json:"userId,omitempty"`
	Email          string  `json:"email,omitempty"`
	Role           string  `json:"role,omitempty"`
	Balance        float64 `json:"balance"`
	FrozenBalance  float64 `json:"frozenBalance"`
	TotalRecharged float64 `json:"totalRecharged"`
	Concurrency    int64   `json:"concurrency"`
	Status         string  `json:"status,omitempty"`
	LastActiveAt   string  `json:"lastActiveAt,omitempty"`
	ExpiresAt      int64   `json:"expiresAt,omitempty"`
	Refreshed      bool    `json:"refreshed,omitempty"`
}

type keyItem struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Key            string  `json:"key"`
	Status         string  `json:"status"`
	GroupID        int64   `json:"groupId,omitempty"`
	GroupName      string  `json:"groupName,omitempty"`
	RateMultiplier float64 `json:"rateMultiplier"`
	Platform       string  `json:"platform,omitempty"`
	Quota          float64 `json:"quota"`
	QuotaUsed      float64 `json:"quotaUsed"`
	LastUsedAt     string  `json:"lastUsedAt,omitempty"`
}

type keyListResult struct {
	State string    `json:"state"`
	Items []keyItem `json:"items"`
}

type logoutResult struct {
	State string `json:"state"`
}

// login starts (or reuses) the private browser and returns immediately. The
// user finishes the sign-in inside Chrome; the window polls account.poll.
func (s *accountService) login(ctx context.Context, _ json.RawMessage) (any, error) {
	if err := s.chrome.ensure(ctx); err != nil {
		if errors.Is(err, errChromeMissing) {
			return rejected(coded("chrome_missing", messageChromeMissing)), nil
		}
		return rejected(coded("chrome_failed", messageChromeFailed)), nil
	}
	return success(loginResult{Started: true}), nil
}

// poll reads the panel localStorage through CDP and persists a session once the
// sign-in has completed.
func (s *accountService) poll(ctx context.Context, _ json.RawMessage) (any, error) {
	tokens, ok, err := s.source.tokens(ctx)
	if err != nil {
		return rejected(coded("cdp_unavailable", messageCDPUnavailable)), nil
	}
	if !ok {
		return success(pollResult{State: statePending}), nil
	}
	if err = s.saveRecord(accountRecord{
		AccessToken:  tokens.Access,
		RefreshToken: tokens.Refresh,
		ExpiresAt:    tokens.ExpiresAt,
		UserID:       tokens.UserID,
	}); err != nil {
		return rejected(coded("store_failed", messageStoreFailed)), nil
	}
	return success(pollResult{State: stateReady, User: &userHint{UserID: tokens.UserID, Email: tokens.Email}}), nil
}

// status returns the balance snapshot, renewing the access token first when it
// is already past its expiry.
func (s *accountService) status(ctx context.Context, _ json.RawMessage) (any, error) {
	record, ok := s.loadRecord()
	if !ok {
		return success(statusResult{State: stateSignedOut}), nil
	}
	record, refreshed, err := s.ensureFresh(ctx, record)
	if err != nil {
		if codeOf(err) == "panel_unauthorized" || codeOf(err) == "token_expired" {
			s.clearRecord()
			return success(statusResult{State: stateSignedOut, Reason: "expired"}), nil
		}
		return rejected(err), nil
	}
	account, err := s.panel.me(ctx, record.AccessToken)
	if err != nil {
		if codeOf(err) == "panel_unauthorized" {
			s.clearRecord()
			return success(statusResult{State: stateSignedOut, Reason: "expired"}), nil
		}
		return rejected(err), nil
	}
	return success(statusResult{
		State:          stateReady,
		UserID:         account.ID,
		Email:          account.Email,
		Role:           account.Role,
		Balance:        account.Balance,
		FrozenBalance:  account.FrozenBalance,
		TotalRecharged: account.TotalRecharged,
		Concurrency:    account.Concurrency,
		Status:         account.Status,
		LastActiveAt:   account.LastActiveAt,
		ExpiresAt:      record.ExpiresAt,
		Refreshed:      refreshed,
	}), nil
}

// session returns a usable access token, renewing it when the stored expiry is close.
func (s *accountService) session(ctx context.Context) (accountRecord, error) {
	record, ok := s.loadRecord()
	if !ok {
		return accountRecord{}, coded("not_signed_in", messageNotSignedIn)
	}
	record, _, err := s.ensureFresh(ctx, record)
	if err != nil {
		if codeOf(err) == "panel_unauthorized" || codeOf(err) == "token_expired" {
			s.clearRecord()
			return accountRecord{}, coded("panel_unauthorized", messagePanelUnauthorized)
		}
		return accountRecord{}, err
	}
	return record, nil
}

// keys lists the API keys that belong to the signed-in account.
func (s *accountService) keys(ctx context.Context, _ json.RawMessage) (any, error) {
	record, err := s.session(ctx)
	if err != nil {
		return rejected(err), nil
	}
	keys, err := s.panel.keys(ctx, record.AccessToken)
	if err != nil {
		if codeOf(err) == "panel_unauthorized" {
			s.clearRecord()
		}
		return rejected(err), nil
	}
	items := make([]keyItem, 0, len(keys))
	for _, key := range keys {
		items = append(items, mapKey(key))
	}
	return success(keyListResult{State: stateReady, Items: items}), nil
}

// logout forgets the stored session and closes the browser this plugin opened.
func (s *accountService) logout(_ context.Context, _ json.RawMessage) (any, error) {
	s.clearRecord()
	s.chrome.stop()
	return success(logoutResult{State: stateSignedOut}), nil
}

// ensureFresh returns a usable record, rotating the access token when the
// stored expiry has passed or is about to.
func (s *accountService) ensureFresh(ctx context.Context, record accountRecord) (accountRecord, bool, error) {
	if record.ExpiresAt <= 0 || s.now().UnixMilli()+refreshSkew.Milliseconds() < record.ExpiresAt {
		return record, false, nil
	}
	if record.RefreshToken == "" {
		return record, false, coded("token_expired", messageTokenExpired)
	}
	token, err := s.panel.refresh(ctx, record.RefreshToken)
	if err != nil {
		return record, false, err
	}
	updated := accountRecord{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		UserID:       record.UserID,
	}
	if token.ExpiresIn > 0 {
		updated.ExpiresAt = s.now().UnixMilli() + token.ExpiresIn*1000
	}
	if err := s.saveRecord(updated); err != nil {
		return record, false, coded("store_failed", messageStoreFailed)
	}
	return updated, true, nil
}

func (s *accountService) recordPath() string { return filepath.Join(s.dataDir, accountFileName) }

// loadRecord reads the stored session. A corrupt or empty file counts as
// signed out: it carries nothing the plugin could still use.
func (s *accountService) loadRecord() (accountRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.recordPath())
	if err != nil || len(raw) > maxAccountBytes {
		return accountRecord{}, false
	}
	var record accountRecord
	if json.Unmarshal(raw, &record) != nil || record.AccessToken == "" {
		return accountRecord{}, false
	}
	return record, true
}

func (s *accountService) saveRecord(record accountRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dataDir, 0o700); err != nil {
		return err
	}
	record.SavedAt = s.now().UnixMilli()
	return writeJSONFile(s.recordPath(), record)
}

func (s *accountService) clearRecord() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = os.Remove(s.recordPath())
}
