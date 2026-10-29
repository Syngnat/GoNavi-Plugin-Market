// Command hualong-ai-provider forwards OpenAI-compatible chat requests to the 華龍算力 gateway.
// The declared models are configuration targets for the upstream, not locally implemented models.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

const (
	providerID   = "hualong"
	providerName = "華龍算力"

	contextWindow   = 128000
	maxOutputTokens = 16384
	visionTokens    = 8192
)

func main() {
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := serve(root); err != nil {
		// Keep diagnostics constant: upstream errors and configuration may contain secrets.
		if _, writeErr := fmt.Fprintln(os.Stderr, "AI provider stopped with an error"); writeErr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

// providerModels mirrors contributes.aiProviders[0].models in gonavi.plugin.json exactly.
func providerModels() []plugin.AIModel {
	return []plugin.AIModel{
		{ID: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens},
		{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens},
		{ID: "deepseek-v4.1-flash", Name: "DeepSeek V4.1 Flash", ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens},
		{ID: "glm-5.3", Name: "GLM 5.3", ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens},
		{ID: "glm-5.3-flash", Name: "GLM 5.3 Flash", ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens},
		{ID: "kimi-k3", Name: "Kimi K3", ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens},
		{ID: "minimax-m3", Name: "MiniMax M3", ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens},
		// The gateway exposes glm-5v-turbo as a vision model with a smaller output cap.
		{ID: "glm-5v-turbo", Name: "GLM 5V Turbo", ContextWindow: contextWindow, MaxOutputTokens: visionTokens},
		{ID: "kimi-k2.6", Name: "Kimi K2.6", ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens},
		{ID: "kimi-k2.7", Name: "Kimi K2.7", ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens},
		{ID: "kimi-k3.1-pro", Name: "Kimi K3.1 Pro", ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens},
		{ID: "minimax-m3.1-flash", Name: "MiniMax M3.1 Flash", ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens},
	}
}

func serve(ctx context.Context) error {
	upstream := newHTTPProvider()
	defer upstream.client.CloseIdleConnections()
	dispatcher, err := plugin.NewAIProviderDispatcher(ctx, plugin.AIProviderOptions{
		Providers: map[string]plugin.AIProviderRegistration{
			providerID: {Models: providerModels(), SupportsTools: true, Generate: upstream.generate},
		},
	})
	if err != nil {
		return failure("generation_failed")
	}
	server := plugin.NewServer(plugin.Options{
		Handlers: newHandlers(dispatcher, newAccountService()),
		OnClose:  dispatcher.Abort,
	})
	serveErr := server.ServeStdio(ctx)
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := dispatcher.Close(cleanup); err != nil {
		return err
	}
	if serveErr != nil && !errors.Is(serveErr, context.Canceled) {
		return failure("generation_failed")
	}
	return nil
}

// newHandlers binds every command declared in gonavi.plugin.json to its
// implementation, so a missing entry fails this build rather than the window.
func newHandlers(dispatcher *plugin.AIProviderDispatcher, account *accountService) map[string]plugin.Handler {
	return map[string]plugin.Handler{
		"provider":           dispatcher.Handle,
		"account.login":      account.login,
		"account.poll":       account.poll,
		"account.status":     account.status,
		"account.keys":       account.keys,
		"account.groups":     account.groups,
		"account.listModels": account.listModels,
		"account.createKey":  account.createKey,
		"account.updateKey":  account.updateKey,
		"account.deleteKey":  account.deleteKey,
		"account.logout":     account.logout,
	}
}
