package providers

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements LLMProvider for OpenAI-compatible APIs
type OpenAIProvider struct {
	client       *openai.Client
	defaultModel string
	providerName string
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey, apiBase, defaultModel string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}

	config := openai.DefaultConfig(apiKey)
	if apiBase != "" {
		config.BaseURL = apiBase
	}

	return &OpenAIProvider{
		client:       openai.NewClientWithConfig(config),
		defaultModel: defaultModel,
		providerName: "openai",
	}, nil
}

// NewOpenRouterProvider creates a new OpenRouter provider
func NewOpenRouterProvider(apiKey, apiBase, defaultModel string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}

	config := openai.DefaultConfig(apiKey)
	if apiBase == "" {
		apiBase = "https://openrouter.ai/api/v1"
	}
	config.BaseURL = apiBase

	return &OpenAIProvider{
		client:       openai.NewClientWithConfig(config),
		defaultModel: defaultModel,
		providerName: "openrouter",
	}, nil
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return p.providerName
}

// GetDefaultModel returns the default model
func (p *OpenAIProvider) GetDefaultModel() string {
	return p.defaultModel
}

// Chat sends a chat request to the OpenAI API
func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*LLMResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	// Get model-specific handler
	handler := GetHandler(model)

	// Convert messages to OpenAI format
	messages := handler.ConvertMessages(req.Messages)

	// Build the chat request with model-specific settings
	chatReq := handler.BuildChatRequest(model, messages, req.Tools, req.MaxTokens, req.Temperature, req.TopP)

	resp, err := p.client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return &LLMResponse{
			Content:      "",
			FinishReason: "stop",
			Usage: UsageStats{
				TotalTokens: resp.Usage.TotalTokens,
			},
		}, nil
	}

	choice := resp.Choices[0]

	// Parse tool calls using model-specific handler
	toolCalls := handler.ParseToolCalls(choice)

	return &LLMResponse{
		Content:      choice.Message.Content,
		ToolCalls:    toolCalls,
		FinishReason: string(choice.FinishReason),
		Usage: UsageStats{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		Model:   model,
	}, nil
}
