package providers

import (
	"context"
	"encoding/json"
)

// UsageStats tracks token usage
type UsageStats struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ToolCall represents a tool invocation from the LLM
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// Message represents a chat message
type Message struct {
	Role    string                 `json:"role"` // system, user, assistant, tool
	Content string                 `json:"content"`
	Name    string                 `json:"name,omitempty"`
	ToolID  string                 `json:"tool_call_id,omitempty"`
	ToolResult interface{}         `json:"tool_result,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Media    []Media                `json:"media,omitempty"` // Images, videos, etc.
}

// Media represents media content (images, videos, etc.)
type Media struct {
	Type     string `json:"type"`     // "image", "video", etc.
	URL      string `json:"url,omitempty"`
	Data     string `json:"data,omitempty"` // Base64 encoded data
	MimeType string `json:"mime_type,omitempty"`
}

// ChatRequest represents a request to the LLM
type ChatRequest struct {
	Messages     []Message          `json:"messages"`
	Tools       []ToolDefinition   `json:"tools,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
	Model       string             `json:"model,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ToolDefinition defines a tool that can be called by the LLM
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// LLMResponse represents the response from the LLM
type LLMResponse struct {
	Content      string            `json:"content"`
	ToolCalls    []ToolCall        `json:"tool_calls,omitempty"`
	FinishReason string            `json:"finish_reason"`
	Usage        UsageStats        `json:"usage"`
	Model        string            `json:"model"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// LLMProvider defines the interface for LLM providers
type LLMProvider interface {
	// Chat sends a chat request to the LLM and returns a response
	Chat(ctx context.Context, req ChatRequest) (*LLMResponse, error)

	// GetDefaultModel returns the default model for this provider
	GetDefaultModel() string

	// Name returns the provider name
	Name() string
}