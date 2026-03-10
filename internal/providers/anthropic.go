package providers

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// AnthropicProvider implements LLMProvider for Anthropic Claude
type AnthropicProvider struct {
	client       *openai.Client
	defaultModel string
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey, apiBase, defaultModel string) (*AnthropicProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.anthropic.com/v1"
	if apiBase != "" {
		config.BaseURL = apiBase
	}

	return &AnthropicProvider{
		client:       openai.NewClientWithConfig(config),
		defaultModel: defaultModel,
	}, nil
}

// Name returns the provider name
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// GetDefaultModel returns the default model
func (p *AnthropicProvider) GetDefaultModel() string {
	return p.defaultModel
}

// Chat sends a chat request to Anthropic
func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest) (*LLMResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Convert float64 to float32 for API compatibility
	temp := float32(req.Temperature)
	topP := float32(req.TopP)

	chatReq := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: temp,
		TopP:        topP,
	}

	if len(req.Tools) > 0 {
		tools := make([]openai.Tool, len(req.Tools))
		for i, tool := range req.Tools {
			tools[i] = openai.Tool{
				Type:     openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			}
		}
		chatReq.Tools = tools
		chatReq.ToolChoice = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": "",
			},
		}
	}

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

	toolCalls := make([]ToolCall, 0)
	if choice.Message.ToolCalls != nil {
		for _, tc := range choice.Message.ToolCalls {
			toolCalls = append(toolCalls, ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Arguments: map[string]interface{}{
					"raw": tc.Function.Arguments,
				},
			})
		}
	}

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