package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

func readStream(t *testing.T, body string) (plugin.AIProviderCompletion, error) {
	t.Helper()
	emit, _ := collectDeltas()
	return readCompletion(context.Background(), strings.NewReader(body), emit)
}

func TestStreamRejectsErrorFrame(t *testing.T) {
	_, err := readStream(t, "data: {\"error\":{\"message\":\"CANARY\"}}\n\n")
	assertGenerationFailure(t, err, "CANARY")
}

func TestStreamRejectsDoneWithoutFinishReason(t *testing.T) {
	_, err := readStream(t, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n")
	assertGenerationFailure(t, err, "")
}

func TestStreamRejectsFramesAfterFinish(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"late\"}}]}\n\ndata: [DONE]\n\n"
	_, err := readStream(t, body)
	assertGenerationFailure(t, err, "")
}

func TestStreamRejectsUnterminatedEvent(t *testing.T) {
	_, err := readStream(t, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}")
	assertGenerationFailure(t, err, "")
}

func TestStreamRejectsUnsupportedFinishReason(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"content_filter\"}]}\n\ndata: [DONE]\n\n"
	_, err := readStream(t, body)
	assertGenerationFailure(t, err, "")
}

func TestStreamRejectsToolCallsWithoutFinish(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"type\":\"function\",\"function\":{\"name\":\"f\",\"arguments\":\"{}\"}}]},\"finish_reason\":null}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	_, err := readStream(t, body)
	assertGenerationFailure(t, err, "")
}

func TestStreamRejectsOversizedLine(t *testing.T) {
	_, err := readStream(t, "data: "+strings.Repeat("a", maxLineBytes+1)+"\n\n")
	if !errors.As(err, new(*plugin.AIProviderFailure)) || err.(*plugin.AIProviderFailure).Code != "limit_exceeded" {
		t.Fatalf("expected limit_exceeded, got %v", err)
	}
}

func TestStreamRejectsSecondChoice(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":1,\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n"
	_, err := readStream(t, body)
	assertGenerationFailure(t, err, "")
}

func TestStreamPropagatesEmitCancellation(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n"
	want := context.Canceled
	_, err := readCompletion(context.Background(), strings.NewReader(body), func(plugin.AIProviderDelta) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected the emit error to propagate, got %v", err)
	}
}

func TestStreamAcceptsCommentsAndCRLF(t *testing.T) {
	body := ": keep-alive\r\n\r\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}\r\n\r\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"length\"}]}\r\n\r\ndata: [DONE]\r\n\r\n"
	completion, err := readStream(t, body)
	if err != nil {
		t.Fatal(err)
	}
	if completion.FinishReason != "length" {
		t.Fatalf("unexpected finish reason: %q", completion.FinishReason)
	}
}
