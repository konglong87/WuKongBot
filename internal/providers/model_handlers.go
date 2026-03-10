package providers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/sashabaranov/go-openai"
)

// ModelHandler defines the interface for model-specific handling
type ModelHandler interface {
	// ConvertMessages converts our message format to OpenAI format
	ConvertMessages(messages []Message) []openai.ChatCompletionMessage

	// ParseToolCalls parses the tool calls from LLM response
	ParseToolCalls(choice openai.ChatCompletionChoice) []ToolCall

	// BuildChatRequest builds the OpenAI chat request with model-specific settings
	BuildChatRequest(model string, messages []openai.ChatCompletionMessage, tools []ToolDefinition, maxTokens int, temperature, topP float64) openai.ChatCompletionRequest
}

// DefaultHandler is the default handler for standard OpenAI-compatible models
type DefaultHandler struct{}

func NewDefaultHandler() *DefaultHandler {
	return &DefaultHandler{}
}

func (h *DefaultHandler) ConvertMessages(messages []Message) []openai.ChatCompletionMessage {
	openaiMessages := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "tool" {
			openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    msg.Content,
				ToolCallID: msg.ToolID,
			})
		} else if msg.Role == "assistant" && len(msg.Metadata) > 0 {
			if toolCalls, ok := msg.Metadata["tool_calls"].([]map[string]interface{}); ok && len(toolCalls) > 0 {
				aiToolCalls := make([]openai.ToolCall, len(toolCalls))
				for i, tc := range toolCalls {
					aiToolCalls[i] = openai.ToolCall{
						ID:   tc["id"].(string),
						Type: openai.ToolTypeFunction,
						Function: openai.FunctionCall{
							Name:      tc["function"].(map[string]interface{})["name"].(string),
							Arguments: tc["function"].(map[string]interface{})["arguments"].(string),
						},
					}
				}
				openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
					Role:      openai.ChatMessageRoleAssistant,
					Content:   msg.Content,
					ToolCalls: aiToolCalls,
				})
			} else {
				openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: msg.Content,
				})
			}
		} else {
			// Handle messages with media (images, videos, etc.)
			if len(msg.Media) > 0 {
				// Convert media content to OpenAI format using MultiContent
				multiContent := []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: msg.Content,
					},
				}

				for _, media := range msg.Media {
					if media.Type == "image" {
						imageURL := media.URL
						if media.Data != "" {
							// Use base64 data
							imageURL = fmt.Sprintf("data:%s;base64,%s", media.MimeType, media.Data)
						}
						multiContent = append(multiContent, openai.ChatMessagePart{
							Type: openai.ChatMessagePartTypeImageURL,
							ImageURL: &openai.ChatMessageImageURL{
								URL: imageURL,
							},
						})
					}
				}

				openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
					Role:         msg.Role,
					MultiContent: multiContent,
				})
			} else {
				openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		}
	}
	return openaiMessages
}

func (h *DefaultHandler) ParseToolCalls(choice openai.ChatCompletionChoice) []ToolCall {
	toolCalls := make([]ToolCall, 0)
	if choice.Message.ToolCalls != nil {
		for _, tc := range choice.Message.ToolCalls {
			toolCalls = append(toolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: parseToolArguments(tc.Function.Arguments),
			})
		}
	}
	return toolCalls
}

// parseToolArguments parses the JSON arguments string into a map
// If parsing fails, returns a map with the raw string under the "raw" key
func parseToolArguments(argsStr string) map[string]interface{} {
	if argsStr == "" {
		return map[string]interface{}{}
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsStr), &args); err == nil {
		// Successfully parsed JSON
		return args
	}

	// Parse failed, keep as raw string
	log.Debug("[PROVIDER] Failed to parse tool arguments as JSON, keeping as raw string",
		"args_length", len(argsStr), "args_preview", argsStr[:min(100, len(argsStr))])
	return map[string]interface{}{
		"raw": argsStr,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *DefaultHandler) BuildChatRequest(model string, messages []openai.ChatCompletionMessage, tools []ToolDefinition, maxTokens int, temperature, topP float64) openai.ChatCompletionRequest {
	req := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: float32(temperature),
		TopP:        float32(topP),
	}

	if len(tools) > 0 {
		aiTools := make([]openai.Tool, len(tools))
		for i, tool := range tools {
			aiTools[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			}
		}
		req.Tools = aiTools
		req.ToolChoice = "auto"
	}

	return req
}

// KimiHandler is the handler for Kimi models (Moonshot AI)
type KimiHandler struct{}

func NewKimiHandler() *KimiHandler {
	return &KimiHandler{}
}

func (h *KimiHandler) ConvertMessages(messages []Message) []openai.ChatCompletionMessage {
	// Kimi uses the same message format as default
	openaiMessages := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "tool" {
			openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    msg.Content,
				ToolCallID: msg.ToolID,
			})
		} else if msg.Role == "assistant" && len(msg.Metadata) > 0 {
			if toolCalls, ok := msg.Metadata["tool_calls"].([]map[string]interface{}); ok && len(toolCalls) > 0 {
				aiToolCalls := make([]openai.ToolCall, len(toolCalls))
				for i, tc := range toolCalls {
					aiToolCalls[i] = openai.ToolCall{
						ID:   tc["id"].(string),
						Type: openai.ToolTypeFunction,
						Function: openai.FunctionCall{
							Name:      tc["function"].(map[string]interface{})["name"].(string),
							Arguments: tc["function"].(map[string]interface{})["arguments"].(string),
						},
					}
				}
				openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
					Role:      openai.ChatMessageRoleAssistant,
					Content:   msg.Content,
					ToolCalls: aiToolCalls,
				})
			} else {
				openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: msg.Content,
				})
			}
		} else {
			// Handle messages with media (images, videos, etc.)
			if len(msg.Media) > 0 {
				// Convert media content to OpenAI format using MultiContent
				multiContent := []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: msg.Content,
					},
				}

				for _, media := range msg.Media {
					if media.Type == "image" {
						imageURL := media.URL
						if media.Data != "" {
							// Use base64 data
							imageURL = fmt.Sprintf("data:%s;base64,%s", media.MimeType, media.Data)
						}
						multiContent = append(multiContent, openai.ChatMessagePart{
							Type: openai.ChatMessagePartTypeImageURL,
							ImageURL: &openai.ChatMessageImageURL{
								URL: imageURL,
							},
						})
					}
				}

				openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
					Role:         msg.Role,
					MultiContent: multiContent,
				})
			} else {
				openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		}
	}
	return openaiMessages
}

func (h *KimiHandler) ParseToolCalls(choice openai.ChatCompletionChoice) []ToolCall {
	toolCalls := make([]ToolCall, 0)
	if choice.Message.ToolCalls != nil {
		// Kimi sometimes splits tool calls into two parts:
		// Part 1: name="tool_name", arguments=""
		// Part 2: name="", arguments="{...}"
		// Or it may concatenate multiple tool calls' arguments in one arguments string

		// First, collect tools with name but no arguments, and tools with arguments but no name
		type partialToolCall struct {
			id   string
			name string
			args string
			tc   openai.ToolCall
		}

		partialWithNames := make([]*partialToolCall, 0)
		partialWithArgs := make([]*partialToolCall, 0)

		for i := 0; i < len(choice.Message.ToolCalls); i++ {
			tc := choice.Message.ToolCalls[i]
			log.Debug("[KIMI-HANDLER] Collecting partial tool call",
				"id", tc.ID,
				"name", tc.Function.Name,
				"arguments", tc.Function.Arguments)

			partial := &partialToolCall{
				id:   tc.ID,
				name: tc.Function.Name,
				args: tc.Function.Arguments,
				tc:   tc,
			}

			if tc.Function.Name != "" && tc.Function.Arguments == "" {
				partialWithNames = append(partialWithNames, partial)
			} else if tc.Function.Name == "" && tc.Function.Arguments != "" {
				partialWithArgs = append(partialWithArgs, partial)
			} else if tc.Function.Name != "" && tc.Function.Arguments != "" {
				// Complete tool call with both name and arguments
				toolCalls = append(toolCalls, ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: parseToolArguments(tc.Function.Arguments),
				})
			}
		}

		log.Info("[KIMI-HANDLER] Collected partials", "with_names", len(partialWithNames), "with_args", len(partialWithArgs))

		// Merge partial calls: assign arguments to corresponding tools
		argQueue := make([]string, 0)

		// Collect all arguments from partialWithArgs, splitting concatenated JSON
		for _, partial := range partialWithArgs {
			splitArgs := h.splitConcatenatedJSON(partial.args)
			switch len(splitArgs) {
			case 0:
				// No valid JSON, use as-is
				log.Warn("[KIMI-HANDLER] No valid JSON in arguments", "id", partial.id, "args", partial.args)
				argQueue = append(argQueue, partial.args)
			case 1:
				argQueue = append(argQueue, splitArgs[0])
			default:
				// Multiple JSON objects - add all to queue
				log.Info("[KIMI-HANDLER] Split arguments into multiple JSONs", "original_id", partial.id, "count", len(splitArgs))
				argQueue = append(argQueue, splitArgs...)
			}
		}

		// Assign arguments to tools by order
		for _, partial := range partialWithNames {
			if len(argQueue) > 0 {
				args := argQueue[0]
				argQueue = argQueue[1:]

				toolCalls = append(toolCalls, ToolCall{
					ID:        partial.id,
					Name:      partial.name,
					Arguments: parseToolArguments(args),
				})
				log.Info("[KIMI-HANDLER] Merged tool call", "id", partial.id, "name", partial.name, "arguments", args)
			} else {
				// No arguments available for this tool
				log.Warn("[KIMI-HANDLER] Tool call without arguments", "id", partial.id, "name", partial.name)
				toolCalls = append(toolCalls, ToolCall{
					ID:        partial.id,
					Name:      partial.name,
					Arguments: map[string]interface{}{},
				})
			}
		}

		// Log any leftover arguments
		if len(argQueue) > 0 {
			log.Warn("[KIMI-HANDLER] Leftover arguments after merging", "count", len(argQueue))
			for i, args := range argQueue {
				log.Warn("[KIMI-HANDLER] Unused argument", "index", i, "args", args)
			}
		}
	}

	// Log final result
	log.Info("[KIMI-HANDLER] Final tool calls", "count", len(toolCalls))
	for i, tc := range toolCalls {
		log.Debug("[KIMI-HANDLER] Tool call", "index", i, "id", tc.ID, "name", tc.Name)
	}

	return toolCalls
}

// splitConcatenatedJSON splits a string containing multiple concatenated JSON objects
// Example: "{"a":1}{"b":2}" → ["{"a":1}", "{"b":2}"]
func (h *KimiHandler) splitConcatenatedJSON(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	// Try to validate as single JSON first
	var test map[string]interface{}
	if json.Unmarshal([]byte(input), &test) == nil {
		return []string{input}
	}

	result := make([]string, 0)
	var depth int
	var start int
	var inString bool
	var escapeNext bool

	for i, r := range input {
		if escapeNext {
			escapeNext = false
			continue
		}

		switch r {
		case '\\':
			escapeNext = true
		case '"':
			inString = !inString
		case '{':
			if !inString {
				if depth == 0 {
					start = i
				}
				depth++
			}
		case '}':
			if !inString {
				depth--
				if depth == 0 {
					// Found a complete JSON object
					jsonStr := input[start : i+1]
					// Validate and add
					var test map[string]interface{}
					if json.Unmarshal([]byte(jsonStr), &test) == nil {
						result = append(result, jsonStr)
					}
				}
			}
		}
	}

	return result
}

func (h *KimiHandler) BuildChatRequest(model string, messages []openai.ChatCompletionMessage, tools []ToolDefinition, maxTokens int, temperature, topP float64) openai.ChatCompletionRequest {
	req := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: float32(temperature),
		TopP:        float32(topP),
	}

	if len(tools) > 0 {
		aiTools := make([]openai.Tool, len(tools))
		for i, tool := range tools {
			aiTools[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			}
		}
		req.Tools = aiTools
		// Kimi requires "auto" for tool choice
		req.ToolChoice = "auto"
	}

	return req
}

// HandlerMap maps model prefixes to their handlers
var HandlerMap = map[string]ModelHandler{
	"kimi":     NewKimiHandler(),
	"moonshot": NewKimiHandler(),
	// Add more models here
}

// GetHandler returns the appropriate handler for the given model
func GetHandler(model string) ModelHandler {
	for prefix, handler := range HandlerMap {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			log.Info("[PROVIDER] Using model-specific handler", "model", model, "handler_type", prefix)
			return handler
		}
	}
	log.Info("[PROVIDER] Using default handler", "model", model)
	return NewDefaultHandler()
}

// NewCustomHandler creates a new custom handler for a specific model configuration
type CustomHandler struct {
	convertFunc func([]Message) []openai.ChatCompletionMessage
	parseFunc   func(openai.ChatCompletionChoice) []ToolCall
	buildFunc   func(string, []openai.ChatCompletionMessage, []ToolDefinition, int, float64, float64) openai.ChatCompletionRequest
}

func NewCustomHandler(
	convertFunc func([]Message) []openai.ChatCompletionMessage,
	parseFunc func(openai.ChatCompletionChoice) []ToolCall,
	buildFunc func(string, []openai.ChatCompletionMessage, []ToolDefinition, int, float64, float64) openai.ChatCompletionRequest,
) *CustomHandler {
	return &CustomHandler{
		convertFunc: convertFunc,
		parseFunc:   parseFunc,
		buildFunc:   buildFunc,
	}
}

func (h *CustomHandler) ConvertMessages(messages []Message) []openai.ChatCompletionMessage {
	if h.convertFunc != nil {
		return h.convertFunc(messages)
	}
	return NewDefaultHandler().ConvertMessages(messages)
}

func (h *CustomHandler) ParseToolCalls(choice openai.ChatCompletionChoice) []ToolCall {
	if h.parseFunc != nil {
		return h.parseFunc(choice)
	}
	return NewDefaultHandler().ParseToolCalls(choice)
}

func (h *CustomHandler) BuildChatRequest(model string, messages []openai.ChatCompletionMessage, tools []ToolDefinition, maxTokens int, temperature, topP float64) openai.ChatCompletionRequest {
	if h.buildFunc != nil {
		return h.buildFunc(model, messages, tools, maxTokens, temperature, topP)
	}
	return NewDefaultHandler().BuildChatRequest(model, messages, tools, maxTokens, temperature, topP)
}

// RegisterHandler registers a custom handler for a model prefix
func RegisterHandler(prefix string, handler ModelHandler) {
	HandlerMap[prefix] = handler
	log.Info("[PROVIDER] Registered custom handler", "prefix", prefix)
}
