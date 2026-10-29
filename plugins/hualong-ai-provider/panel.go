package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// panelBaseURL is the 華龍 panel API. Only the account module talks to it;
	// chat traffic keeps using the gateway base URL from the host config.
	panelBaseURL     = "https://api.hualong.online"
	panelTimeout     = 8 * time.Second
	panelMaxBytes    = 1 << 20
	panelKeyPageSize = 50
	// maxListedModels, maxModelIDBytes, maxModelNameBytes and maxModelContextWindow
	// bound one gateway model scan; anything outside the bounds is treated as absent
	// rather than passed on to the host.
	maxListedModels       = 512
	maxModelIDBytes       = 128
	maxModelNameBytes     = 120
	maxModelContextWindow = 20_000_000
)

// panelModel is one entry of GET /v1/models. Gateways that only publish ids leave
// Name and ContextWindow empty; the host fills them from the models.dev catalog.
type panelModel struct {
	ID            string
	Name          string
	ContextWindow int
}

// panelAccount is GET /api/v1/auth/me -> data.
type panelAccount struct {
	ID             int64   `json:"id"`
	Email          string  `json:"email"`
	Role           string  `json:"role"`
	Balance        float64 `json:"balance"`
	FrozenBalance  float64 `json:"frozen_balance"`
	Concurrency    int64   `json:"concurrency"`
	Status         string  `json:"status"`
	TotalRecharged float64 `json:"total_recharged"`
	LastActiveAt   string  `json:"last_active_at"`
	RunMode        string  `json:"run_mode"`
}

type panelKeyGroup struct {
	Name           string  `json:"name"`
	RateMultiplier float64 `json:"rate_multiplier"`
	Platform       string  `json:"platform"`
}

// panelKey is one item of GET /api/v1/keys -> data.items.
type panelKey struct {
	ID         int64          `json:"id"`
	Name       string         `json:"name"`
	Key        string         `json:"key"`
	Status     string         `json:"status"`
	GroupID    int64          `json:"group_id"`
	Quota      float64        `json:"quota"`
	QuotaUsed  float64        `json:"quota_used"`
	LastUsedAt *string        `json:"last_used_at"`
	CreatedAt  string         `json:"created_at"`
	ExpiresAt  *string        `json:"expires_at"`
	Group      *panelKeyGroup `json:"group"`
}

type panelKeyPage struct {
	Items []panelKey `json:"items"`
	Total int64      `json:"total"`
	Page  int64      `json:"page"`
	Pages int64      `json:"pages"`
}

// panelGroup is one item of GET /api/v1/groups/available.
type panelGroup struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	RateMultiplier float64 `json:"rate_multiplier"`
	Platform       string  `json:"platform"`
}

// panelToken is POST /api/v1/auth/refresh -> data. The panel bundle posts
// {"refresh_token":"…"} and reads back access_token, refresh_token, expires_in.
type panelToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type panelClient struct {
	client  *http.Client
	baseURL string
}

func newPanelClient() *panelClient {
	transport := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second, KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:    8 * time.Second,
		ResponseHeaderTimeout:  panelTimeout,
		MaxResponseHeaderBytes: 16 << 10,
		MaxConnsPerHost:        4,
		DisableCompression:     true,
		DisableKeepAlives:      true,
	}
	return &panelClient{
		baseURL: panelBaseURL,
		client: &http.Client{
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *panelClient) me(ctx context.Context, accessToken string) (panelAccount, error) {
	raw, err := c.get(ctx, "/api/v1/auth/me?timezone=Asia%2FShanghai", accessToken)
	if err != nil {
		return panelAccount{}, err
	}
	var account panelAccount
	if err := decodePanelData(raw, &account); err != nil {
		return panelAccount{}, err
	}
	return account, nil
}

func (c *panelClient) keys(ctx context.Context, accessToken string) ([]panelKey, error) {
	path := "/api/v1/keys?page=1&page_size=" + strconv.Itoa(panelKeyPageSize)
	raw, err := c.get(ctx, path, accessToken)
	if err != nil {
		return nil, err
	}
	var page panelKeyPage
	if err := decodePanelData(raw, &page); err != nil {
		return nil, err
	}
	if page.Items == nil && page.Total > 0 {
		return nil, coded("invalid_response", messageInvalidResponse)
	}
	if len(page.Items) > panelKeyPageSize {
		return nil, coded("invalid_response", messageInvalidResponse)
	}
	return page.Items, nil
}

func (c *panelClient) groups(ctx context.Context, accessToken string) ([]panelGroup, error) {
	raw, err := c.get(ctx, "/api/v1/groups/available", accessToken)
	if err != nil {
		return nil, err
	}
	var items []panelGroup
	if decodePanelData(raw, &items) == nil {
		if len(items) > 256 {
			return nil, coded("invalid_response", messageInvalidResponse)
		}
		return items, nil
	}
	var page struct {
		Items []panelGroup `json:"items"`
	}
	if err := decodePanelData(raw, &page); err != nil {
		return nil, err
	}
	if len(page.Items) > 256 {
		return nil, coded("invalid_response", messageInvalidResponse)
	}
	return page.Items, nil
}

func (c *panelClient) createKey(ctx context.Context, accessToken, name string, groupID int64) (panelKey, error) {
	body := map[string]any{"name": name}
	if groupID > 0 {
		body["group_id"] = groupID
	}
	raw, err := c.post(ctx, "/api/v1/keys", body, accessToken)
	if err != nil {
		return panelKey{}, err
	}
	var key panelKey
	if err := decodePanelData(raw, &key); err != nil {
		return panelKey{}, err
	}
	if strings.TrimSpace(key.Name) == "" && strings.TrimSpace(key.Key) == "" {
		return panelKey{}, coded("invalid_response", messageInvalidResponse)
	}
	return key, nil
}

// models lists the gateway models a single key can reach. Group membership
// decides the set, so the caller scans one key at a time. Name and context window
// are passed through when the gateway publishes them, because the gateway knows
// its own routing better than any public catalog.
func (c *panelClient) models(ctx context.Context, accessToken string) ([]panelModel, error) {
	raw, err := c.get(ctx, "/v1/models", accessToken)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data []struct {
			ID               string `json:"id"`
			Name             string `json:"name"`
			DisplayName      string `json:"display_name"`
			ContextLength    int    `json:"context_length"`
			ContextWindow    int    `json:"context_window"`
			MaxContextLength int    `json:"max_context_length"`
		} `json:"data"`
	}
	if !utf8.Valid(raw) || json.Unmarshal(raw, &payload) != nil {
		return nil, coded("invalid_response", messageInvalidResponse)
	}
	if len(payload.Data) > maxListedModels {
		return nil, coded("invalid_response", messageInvalidResponse)
	}
	models := make([]panelModel, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" || len(id) > maxModelIDBytes {
			continue
		}
		models = append(models, panelModel{
			ID:            id,
			Name:          firstModelName(item.Name, item.DisplayName),
			ContextWindow: firstModelContext(item.ContextLength, item.ContextWindow, item.MaxContextLength),
		})
	}
	return models, nil
}

// firstModelName 取网关给的展示名；超长或空白视为没给。
func firstModelName(candidates ...string) string {
	for _, candidate := range candidates {
		if name := strings.TrimSpace(candidate); name != "" && len(name) <= maxModelNameBytes {
			return name
		}
	}
	return ""
}

// firstModelContext 取网关声明的上下文窗口；非正数或超出上限视为没给。
func firstModelContext(candidates ...int) int {
	for _, candidate := range candidates {
		if candidate > 0 && candidate <= maxModelContextWindow {
			return candidate
		}
	}
	return 0
}

func (c *panelClient) updateKey(ctx context.Context, accessToken string, id int64, name string, groupID *int64) (panelKey, error) {
	if id <= 0 {
		return panelKey{}, coded("invalid_id", messageInvalidID)
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if groupID != nil {
		if *groupID > 0 {
			body["group_id"] = *groupID
		} else {
			body["group_id"] = nil
		}
	}
	raw, err := c.put(ctx, "/api/v1/keys/"+strconv.FormatInt(id, 10), body, accessToken)
	if err != nil {
		return panelKey{}, err
	}
	var key panelKey
	if err := decodePanelData(raw, &key); err != nil {
		if decodePanelOK(raw) != nil {
			return panelKey{}, err
		}
		key = panelKey{ID: id, Name: name}
		if groupID != nil {
			key.GroupID = *groupID
		}
	}
	return key, nil
}

func (c *panelClient) deleteKey(ctx context.Context, accessToken string, id int64) error {
	if id <= 0 {
		return coded("invalid_id", messageInvalidID)
	}
	raw, err := c.delete(ctx, "/api/v1/keys/"+strconv.FormatInt(id, 10), accessToken)
	if err != nil {
		return err
	}
	return decodePanelOK(raw)
}

func (c *panelClient) refresh(ctx context.Context, refreshToken string) (panelToken, error) {
	body := map[string]string{"refresh_token": refreshToken}
	raw, err := c.post(ctx, "/api/v1/auth/refresh", body, "")
	if err != nil {
		return panelToken{}, err
	}
	var token panelToken
	if err := decodePanelData(raw, &token); err != nil {
		return panelToken{}, err
	}
	if strings.TrimSpace(token.AccessToken) == "" || len(token.AccessToken) > 8192 || len(token.RefreshToken) > 8192 {
		return panelToken{}, coded("invalid_response", messageInvalidResponse)
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return token, nil
}

func (c *panelClient) get(ctx context.Context, path, accessToken string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, nil, accessToken)
}

func (c *panelClient) post(ctx context.Context, path string, body any, accessToken string) ([]byte, error) {
	return c.write(ctx, http.MethodPost, path, body, accessToken)
}

func (c *panelClient) put(ctx context.Context, path string, body any, accessToken string) ([]byte, error) {
	return c.write(ctx, http.MethodPut, path, body, accessToken)
}

func (c *panelClient) delete(ctx context.Context, path, accessToken string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, path, nil, accessToken)
}

func (c *panelClient) write(ctx context.Context, method, path string, body any, accessToken string) ([]byte, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, coded("internal", messageInternal)
	}
	return c.do(ctx, method, path, encoded, accessToken)
}

func (c *panelClient) do(ctx context.Context, method, path string, body []byte, accessToken string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, panelTimeout)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, coded("panel_unavailable", messagePanelUnavailable)
	}
	// These calls have no side effect beyond a token rotation, but a replayed
	// body is still never re-sent: the transport runs without keep-alives.
	request.GetBody = nil
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, coded("panel_unavailable", messagePanelUnavailable)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, panelMaxBytes+1))
	if err != nil {
		return nil, coded("panel_unavailable", messagePanelUnavailable)
	}
	if len(raw) > panelMaxBytes {
		return nil, coded("invalid_response", messageInvalidResponse)
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, coded("panel_unauthorized", messagePanelUnauthorized)
	}
	if response.StatusCode != http.StatusOK {
		// The response body is never echoed: it can carry session material.
		return nil, coded("panel_unavailable", messagePanelUnavailable)
	}
	return raw, nil
}

// decodePanelData unwraps {code,message,data} and rejects any non-zero code.
// Server-supplied text is dropped on purpose; only fixed messages reach the window.
func decodePanelData(raw []byte, target any) error {
	if !utf8.Valid(raw) {
		return coded("invalid_response", messageInvalidResponse)
	}
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return coded("invalid_response", messageInvalidResponse)
	}
	if envelope.Code != 0 {
		return coded("panel_rejected", messagePanelRejected)
	}
	if len(envelope.Data) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
		return coded("invalid_response", messageInvalidResponse)
	}
	if json.Unmarshal(envelope.Data, target) != nil {
		return coded("invalid_response", messageInvalidResponse)
	}
	return nil
}

// decodePanelOK accepts a zero code even when data is empty, which DELETE uses.
func decodePanelOK(raw []byte) error {
	if !utf8.Valid(raw) {
		return coded("invalid_response", messageInvalidResponse)
	}
	var envelope struct {
		Code int `json:"code"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return coded("invalid_response", messageInvalidResponse)
	}
	if envelope.Code != 0 {
		return coded("panel_rejected", messagePanelRejected)
	}
	return nil
}
