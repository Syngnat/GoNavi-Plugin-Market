package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"testing"
)

func TestParseTokenSetExtractsEveryPanelKey(t *testing.T) {
	raw := `{"href":"https://api.hualong.online/login",` +
		`"auth_token":"` + testCanaryAccess + `",` +
		`"refresh_token":"` + testCanaryRefresh + `",` +
		`"token_expires_at":"1760003600000",` +
		`"auth_user":"{\"id\":43,\"email\":\"user@example.test\",\"role\":\"user\"}"}`
	set, ok := parseTokenSet(raw)
	if !ok {
		t.Fatal("expected a usable token set")
	}
	if set.Access != testCanaryAccess || set.Refresh != testCanaryRefresh {
		t.Fatal("the access or refresh token was not extracted")
	}
	if set.ExpiresAt != 1760003600000 {
		t.Fatalf("unexpected expiry: %d", set.ExpiresAt)
	}
	if set.UserID != 43 || set.Email != "user@example.test" {
		t.Fatalf("unexpected user hint: %d %q", set.UserID, set.Email)
	}
}

func TestParseTokenSetRequiresAnAccessToken(t *testing.T) {
	for name, raw := range map[string]string{
		"all null":      `{"auth_token":null,"refresh_token":null,"token_expires_at":null,"auth_user":null}`,
		"blank token":   `{"auth_token":"   ","refresh_token":"x"}`,
		"empty object":  `{}`,
		"invalid json":  `{"auth_token":`,
		"empty payload": ``,
	} {
		if _, ok := parseTokenSet(raw); ok {
			t.Fatalf("%s: a payload without an access token must stay pending", name)
		}
	}
}

func TestParseTokenSetToleratesUnusableHints(t *testing.T) {
	raw := `{"auth_token":"token","refresh_token":"refresh","token_expires_at":"not-a-number","auth_user":"not-json"}`
	set, ok := parseTokenSet(raw)
	if !ok {
		t.Fatal("an access token alone is enough to sign in")
	}
	if set.ExpiresAt != 0 || set.UserID != 0 || set.Email != "" {
		t.Fatalf("unusable hints must stay empty: %#v", set)
	}
}

func TestPickTargetPrefersThePanelPage(t *testing.T) {
	targets := []cdpTarget{
		{Type: "background_page", URL: "chrome://newtab", WebSocketDebuggerURL: "ws://127.0.0.1:1/devtools/page/BG"},
		{Type: "page", URL: "https://example.test/", WebSocketDebuggerURL: "ws://127.0.0.1:1/devtools/page/OTHER"},
		{Type: "page", URL: panelOrigin + "/login", WebSocketDebuggerURL: "ws://127.0.0.1:1/devtools/page/PANEL"},
	}
	target, ok := pickTarget(targets)
	if !ok || target.WebSocketDebuggerURL != "ws://127.0.0.1:1/devtools/page/PANEL" {
		t.Fatalf("expected the panel page, got %#v", target)
	}
	if _, ok = pickTarget(nil); ok {
		t.Fatal("an empty target list has no page")
	}
	if _, ok = pickTarget([]cdpTarget{{Type: "page", URL: panelOrigin, WebSocketDebuggerURL: "http://127.0.0.1/x"}}); ok {
		t.Fatal("a non-websocket debugger URL must be skipped")
	}
}

// TestEvaluateReadsTokensFromCDP drives the whole CDP path against a local
// socket stub: one masked request out, one CDP result frame back.
func TestEvaluateReadsTokensFromCDP(t *testing.T) {
	payload := `{"href":"https://api.hualong.online/login",` +
		`"auth_token":"` + testCanaryAccess + `",` +
		`"refresh_token":"` + testCanaryRefresh + `",` +
		`"token_expires_at":"1760003600000","auth_user":"{\"id\":43,\"email\":\"user@example.test\"}"}`
	url, done := startWebSocketServer(t, wsServerOptions{frames: func(conn net.Conn, reader *bufio.Reader) {
		opcode, request := readMaskedFrame(t, reader)
		if opcode != wsOpcodeText {
			t.Errorf("expected a text frame, got %d", opcode)
		}
		var call struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
			Params struct {
				Expression string `json:"expression"`
			} `json:"params"`
		}
		if err := json.Unmarshal(request, &call); err != nil {
			t.Error(err)
			return
		}
		if call.Method != "Runtime.evaluate" || call.Params.Expression != tokenExpression {
			t.Errorf("unexpected CDP call: %s", call.Method)
		}
		response, err := json.Marshal(map[string]any{
			"id": call.ID,
			"result": map[string]any{
				"result": map[string]any{"type": "string", "value": payload},
			},
		})
		if err != nil {
			t.Error(err)
			return
		}
		if err = writeServerFrame(conn, wsOpcodeText, response); err != nil {
			t.Error(err)
		}
	}})
	conn, err := dialWebSocket(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	value, err := evaluate(context.Background(), conn, tokenExpression)
	if err != nil {
		t.Fatal(err)
	}
	set, ok := parseTokenSet(value)
	if !ok || set.Access != testCanaryAccess || set.Refresh != testCanaryRefresh || set.ExpiresAt != 1760003600000 {
		t.Fatalf("unexpected token set: %#v", set)
	}
	<-done
}

// TestEvaluateSkipsUnrelatedFrames keeps event frames from satisfying the call.
func TestEvaluateSkipsUnrelatedFrames(t *testing.T) {
	url, done := startWebSocketServer(t, wsServerOptions{frames: func(conn net.Conn, reader *bufio.Reader) {
		readMaskedFrame(t, reader)
		_ = writeServerFrame(conn, wsOpcodeText, []byte(`{"method":"Page.loadEventFired","params":{}}`))
		_ = writeServerFrame(conn, wsOpcodeText, []byte(`{"id":1,"result":{"result":{"type":"string","value":"{\"auth_token\":\"t\"}"}}}`))
	}})
	conn, err := dialWebSocket(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	value, err := evaluate(context.Background(), conn, tokenExpression)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := parseTokenSet(value); !ok {
		t.Fatalf("unexpected value: %q", value)
	}
	<-done
}

func TestEvaluateReportsACDPError(t *testing.T) {
	url, done := startWebSocketServer(t, wsServerOptions{frames: func(conn net.Conn, reader *bufio.Reader) {
		readMaskedFrame(t, reader)
		_ = writeServerFrame(conn, wsOpcodeText, []byte(`{"id":1,"error":{"code":-32000,"message":"boom"}}`))
	}})
	conn, err := dialWebSocket(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err = evaluate(context.Background(), conn, tokenExpression); err == nil {
		t.Fatal("a CDP error must fail the evaluation")
	}
	<-done
}
