package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

const contentStream = `data: {"choices":[{"index":0,"delta":{"content":"你好"}}]}

data: {"choices":[{"index":0,"delta":{"content":"，世界"}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"choices":[],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18,"prompt_tokens_details":{"cached_tokens":3}}}

data: [DONE]

`

const toolStream = `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"weather","arguments":"{\"city\":"}}]},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"SF\"}"}}]},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]

`

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func sseResponse(request *http.Request, body string) *http.Response {
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{"Content-Type": []string{"text/event-stream; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

func sampleStart(base string) plugin.AIProviderStart {
	return plugin.AIProviderStart{
		ProviderID: providerID,
		RequestID:  strings.Repeat("a", 43),
		Config: plugin.AIProviderConfig{
			BaseURL: base,
			APIKey:  "sk-test-canary",
			Model:   "deepseek-v4-pro",
		},
		Request: plugin.AIProviderRequest{
			Messages: []plugin.AIProviderMessage{{Role: "user", Content: "hi"}},
		},
	}
}

func collectDeltas() (plugin.AIProviderEmit, *[]plugin.AIProviderDelta) {
	deltas := &[]plugin.AIProviderDelta{}
	return func(delta plugin.AIProviderDelta) error {
		*deltas = append(*deltas, delta)
		return nil
	}, deltas
}

// TestGenerateUsesDefaultGatewayWhenBaseURLEmpty intercepts the transport, so no
// request ever reaches the real 華龍算力 gateway.
func TestGenerateUsesDefaultGatewayWhenBaseURLEmpty(t *testing.T) {
	upstream := newHTTPProvider()
	var target string
	upstream.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		target = request.URL.String()
		return sseResponse(request, contentStream), nil
	})
	emit, deltas := collectDeltas()
	completion, err := upstream.generate(context.Background(), sampleStart(""), emit)
	if err != nil {
		t.Fatal(err)
	}
	if target != "https://api.hualong.online/v1/chat/completions" {
		t.Fatalf("unexpected endpoint: %q", target)
	}
	if completion.FinishReason != "stop" || len(*deltas) != 2 {
		t.Fatal("unexpected completion")
	}
}

func TestGenerateHonoursConfiguredBaseURL(t *testing.T) {
	var target string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		target = request.URL.Path
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(writer, contentStream)
	}))
	defer server.Close()
	upstream := newHTTPProvider()
	emit, _ := collectDeltas()
	if _, err := upstream.generate(context.Background(), sampleStart(server.URL+"/custom/v1/"), emit); err != nil {
		t.Fatal(err)
	}
	if target != "/custom/v1/chat/completions" {
		t.Fatalf("unexpected path: %q", target)
	}
}

func TestGenerateForwardsRequestShape(t *testing.T) {
	upstream := newHTTPProvider()
	var captured *http.Request
	var body map[string]json.RawMessage
	upstream.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		captured = request
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
		return sseResponse(request, contentStream), nil
	})
	start := sampleStart("")
	start.Config.Headers = map[string]string{"X-Tenant": "gonavi"}
	emit, _ := collectDeltas()
	if _, err := upstream.generate(context.Background(), start, emit); err != nil {
		t.Fatal(err)
	}
	if captured.Header.Get("Authorization") != "Bearer sk-test-canary" {
		t.Fatal("the api key was not forwarded")
	}
	if captured.Header.Get("Accept") != "text/event-stream" || captured.Header.Get("Content-Type") != "application/json" {
		t.Fatal("unexpected request headers")
	}
	if captured.Header.Get("X-Tenant") != "gonavi" {
		t.Fatal("configured headers were not applied")
	}
	for field, want := range map[string]string{"stream": "true", "n": "1", "model": `"deepseek-v4-pro"`} {
		if string(body[field]) != want {
			t.Fatalf("unexpected %s: %s", field, body[field])
		}
	}
	if string(body["stream_options"]) != `{"include_usage":true}` {
		t.Fatalf("unexpected stream_options: %s", body["stream_options"])
	}
}

func TestGenerateAggregatesSSEStream(t *testing.T) {
	upstream := newHTTPProvider()
	upstream.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return sseResponse(request, contentStream), nil
	})
	emit, deltas := collectDeltas()
	completion, err := upstream.generate(context.Background(), sampleStart(""), emit)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, delta := range *deltas {
		joined += delta.Content
	}
	if joined != "你好，世界" || completion.FinishReason != "stop" {
		t.Fatalf("unexpected aggregation: %q %q", joined, completion.FinishReason)
	}
	if completion.Usage == nil {
		t.Fatal("usage was not parsed")
	}
	if completion.Usage.PromptTokens != 11 || completion.Usage.CompletionTokens != 7 || completion.Usage.TotalTokens != 18 {
		t.Fatal("unexpected usage totals")
	}
	if completion.Usage.CachedTokens == nil || *completion.Usage.CachedTokens != 3 {
		t.Fatal("unexpected cached tokens")
	}
}

func TestGenerateAssemblesToolCalls(t *testing.T) {
	upstream := newHTTPProvider()
	upstream.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return sseResponse(request, toolStream), nil
	})
	emit, _ := collectDeltas()
	completion, err := upstream.generate(context.Background(), sampleStart(""), emit)
	if err != nil {
		t.Fatal(err)
	}
	if completion.FinishReason != "tool_calls" || len(completion.ToolCalls) != 1 {
		t.Fatal("unexpected tool completion")
	}
	call := completion.ToolCalls[0]
	if call.ID != "call_1" || call.Type != "function" || call.Function.Name != "weather" {
		t.Fatal("unexpected tool call identity")
	}
	if call.Function.Arguments != `{"city":"SF"}` {
		t.Fatalf("unexpected arguments: %s", call.Function.Arguments)
	}
}

func assertGenerationFailure(t *testing.T, err error, canary string) {
	t.Helper()
	var typed *plugin.AIProviderFailure
	if !errors.As(err, &typed) || typed.Code != "generation_failed" {
		t.Fatalf("expected generation_failed, got %v", err)
	}
	message := err.Error()
	if canary != "" && strings.Contains(message, canary) {
		t.Fatalf("the failure leaked upstream detail: %s", message)
	}
	if strings.Contains(message, "sk-test-canary") {
		t.Fatalf("the failure leaked the api key: %s", message)
	}
}

func TestGenerateRejectsNonOKUpstream(t *testing.T) {
	upstream := newHTTPProvider()
	upstream.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"code":"INVALID_API_KEY-CANARY"}`)),
			Request:    request,
		}, nil
	})
	emit, _ := collectDeltas()
	_, err := upstream.generate(context.Background(), sampleStart(""), emit)
	assertGenerationFailure(t, err, "INVALID_API_KEY-CANARY")
}

func TestGenerateRejectsNonSSEContentType(t *testing.T) {
	upstream := newHTTPProvider()
	upstream.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":"CANARY-BODY"}`)),
			Request:    request,
		}, nil
	})
	emit, _ := collectDeltas()
	_, err := upstream.generate(context.Background(), sampleStart(""), emit)
	assertGenerationFailure(t, err, "CANARY-BODY")
}

func TestCompletionURLValidation(t *testing.T) {
	cases := []struct {
		name   string
		base   string
		want   string
		reject bool
	}{
		{"default gateway", "", "https://api.hualong.online/v1/chat/completions", false},
		{"https base", "https://proxy.example/v1/", "https://proxy.example/v1/chat/completions", false},
		{"http base", "http://127.0.0.1:8080/v1", "http://127.0.0.1:8080/v1/chat/completions", false},
		{"userinfo", "https://user:pass@proxy.example/v1", "", true},
		{"opaque", "https:proxy.example", "", true},
		{"fragment", "https://proxy.example/v1#frag", "", true},
		{"wrong scheme", "ftp://proxy.example/v1", "", true},
		{"missing host", "https:///v1", "", true},
		{"oversized", "https://proxy.example/" + strings.Repeat("a", 4096), "", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := completionURL(testCase.base)
			if testCase.reject {
				if err == nil {
					t.Fatalf("expected rejection, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != testCase.want {
				t.Fatalf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}
