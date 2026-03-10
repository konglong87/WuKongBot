package providers

import (
	"fmt"
	"strings"
)

// ProviderFactory creates LLM providers based on model name or explicit config
type ProviderFactory struct{}

// NewProviderFactory creates a new provider factory
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{}
}

// CreateProvider creates a provider based on the model name
func (f *ProviderFactory) CreateProvider(apiKey, apiBase, defaultModel string) (LLMProvider, error) {
	if apiKey == "" && defaultModel == "" {
		return nil, fmt.Errorf("either api_key or default_model is required")
	}

	model := strings.ToLower(defaultModel)

	switch {
	case strings.HasPrefix(model, "openrouter/"):
		return NewOpenRouterProvider(apiKey, apiBase, strings.TrimPrefix(model, "openrouter/"))
	case strings.HasPrefix(apiKey, "sk-or-"):
		return NewOpenRouterProvider(apiKey, apiBase, defaultModel)
	case strings.Contains(apiBase, "openrouter"):
		return NewOpenRouterProvider(apiKey, apiBase, defaultModel)
	case strings.Contains(model, "claude") || strings.Contains(model, "anthropic"):
		return NewAnthropicProvider(apiKey, apiBase, defaultModel)
	case strings.Contains(model, "gpt") || strings.Contains(model, "openai"):
		return NewOpenAIProvider(apiKey, apiBase, defaultModel)
	default:
		return NewOpenAIProvider(apiKey, apiBase, defaultModel)
	}
}

// GetProviderName extracts the provider name from a model string
func GetProviderName(model string) string {
	model = strings.ToLower(model)
	switch {
	case strings.HasPrefix(model, "openrouter/"):
		return "openrouter"
	case strings.Contains(model, "claude"):
		return "anthropic"
	case strings.Contains(model, "gpt"):
		return "openai"
	case strings.Contains(model, "gemini"):
		return "gemini"
	case strings.Contains(model, "glm") || strings.Contains(model, "zhipu") || strings.Contains(model, "zai"):
		return "zhipu"
	case strings.Contains(model, "deepseek"):
		return "deepseek"
	case strings.Contains(model, "groq"):
		return "groq"
	case strings.Contains(model, "vllm"):
		return "vllm"
	default:
		return "openai"
	}
}