package market

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

const (
	maxArtifactAIProviders    = 8
	maxArtifactAIModels       = 32
	minArtifactAIContext      = 1024
	maxArtifactAIContext      = 262144
	maxArtifactAIOutputTokens = 65536
	// maxArtifactAIBaseURLBytes 与宿主一致：预填地址是展示默认值，长度必须受限。
	maxArtifactAIBaseURLBytes = 2048
)

var (
	artifactAIProviderIDPattern   = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
	artifactAIModelIDPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/@+-]{0,127}$`)
	artifactAIProviderIconPattern = regexp.MustCompile(`(?i)\.(png|jpe?g|webp|gif)$`)
)

// Keep publication metadata independent of the host runtime and provider packages.
// Pointer booleans distinguish explicit false capabilities from omitted claims.
type artifactAIProvider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// DefaultBaseURL 是插件自带的网关地址，宿主在用户还没填地址时预填进表单；
	// 插件自己的兜底仍然生效，所以这里只是可选的展示默认值。
	DefaultBaseURL string            `json:"defaultBaseUrl,omitempty"`
	Command        string            `json:"command"`
	Icon           string            `json:"icon,omitempty"`
	Models         []artifactAIModel `json:"models"`
	SupportsTools  *bool             `json:"supportsTools"`
	SupportsImages *bool             `json:"supportsImages"`
}

type artifactAIModel struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ContextWindow   int    `json:"contextWindow"`
	MaxOutputTokens int    `json:"maxOutputTokens"`
}

func validateArtifactAIProviders(manifest artifactManifest, commands map[string]bool) error {
	providers := manifest.Contributes.AIProviders
	if len(providers) == 0 {
		return nil
	}
	if len(providers) > maxArtifactAIProviders {
		return fmt.Errorf("too many ai provider contributions")
	}
	if !slices.Contains(manifest.Permissions, "ai.providers") {
		return fmt.Errorf("ai provider contributions require ai.providers permission")
	}
	if manifest.Backend == nil {
		return fmt.Errorf("ai provider contributions require a backend")
	}
	seen := make(map[string]bool, len(providers))
	for _, provider := range providers {
		if !artifactAIProviderIDPattern.MatchString(provider.ID) || seen[provider.ID] {
			return fmt.Errorf("invalid or duplicate ai provider id")
		}
		if !artifactText(provider.Name, 256) {
			return fmt.Errorf("invalid ai provider name")
		}
		if !commands[provider.Command] {
			return fmt.Errorf("ai provider command must name a declared command")
		}
		if provider.Icon != "" && (len(provider.Icon) > 256 || !artifactAIProviderIconPattern.MatchString(provider.Icon) || safeArtifactPath(provider.Icon) != nil) {
			return fmt.Errorf("invalid ai provider icon")
		}
		if provider.DefaultBaseURL != "" && !validArtifactAIBaseURL(provider.DefaultBaseURL) {
			return fmt.Errorf("invalid ai provider default base url")
		}
		if provider.SupportsTools == nil || provider.SupportsImages == nil {
			return fmt.Errorf("ai provider capabilities require explicit supportsTools and supportsImages")
		}
		if err := validateArtifactAIModels(provider.Models); err != nil {
			return err
		}
		seen[provider.ID] = true
	}
	return nil
}

func validateArtifactAIModels(models []artifactAIModel) error {
	if len(models) == 0 || len(models) > maxArtifactAIModels {
		return fmt.Errorf("ai provider must declare between 1 and 32 models")
	}
	seen := make(map[string]bool, len(models))
	for _, model := range models {
		if !artifactAIModelIDPattern.MatchString(model.ID) || seen[model.ID] {
			return fmt.Errorf("invalid or duplicate ai provider model id")
		}
		if !artifactText(model.Name, 256) {
			return fmt.Errorf("invalid ai provider model name")
		}
		if model.ContextWindow < minArtifactAIContext || model.ContextWindow > maxArtifactAIContext {
			return fmt.Errorf("ai provider model context window is outside supported bounds")
		}
		if model.MaxOutputTokens < 1 || model.MaxOutputTokens > maxArtifactAIOutputTokens || model.MaxOutputTokens > model.ContextWindow {
			return fmt.Errorf("ai provider model output limit is outside supported bounds")
		}
		seen[model.ID] = true
	}
	return nil
}

// validArtifactAIBaseURL 与宿主同口径：只接受绝对 http(s) 地址，无用户信息、
// 无查询串、无片段，长度受限。预填地址会进设置表单，不能让包内的异常值蒙混过关。
func validArtifactAIBaseURL(raw string) bool {
	if len(raw) > maxArtifactAIBaseURLBytes || strings.TrimSpace(raw) != raw {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}
