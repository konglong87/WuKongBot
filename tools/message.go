package tools

import (
	"context"
	"encoding/json"
	"sync"
)

// MessageTool sends messages to users on chat channels
type MessageTool struct {
	sendCallback   func(channel, chatID, content string) error
	mu             sync.RWMutex
	defaultChannel string
	defaultChatID  string
}

// NewMessageTool creates a new message tool
func NewMessageTool() *MessageTool {
	return &MessageTool{}
}

// Name returns the tool name
func (t *MessageTool) Name() string {
	return "message"
}

// Description returns the tool description
func (t *MessageTool) Description() string {
	return "Send an IMMEDIATE message to the user. Use this ONLY for immediate responses to the current conversation. For timed reminders, scheduled messages, or delayed messages, you MUST use the 'cron' tool instead."
}

// Parameters returns the JSON schema for parameters
func (t *MessageTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"content": {
				"type": "string",
				"description": "The message content to send"
			},
			"channel": {
				"type": "string",
				"description": "Optional: target channel (telegram, whatsapp, etc.)"
			},
			"chat_id": {
				"type": "string",
				"description": "Optional: target chat/user ID"
			}
		},
		"required": ["content"]
	}`)
}

// SetSendCallback sets the callback for sending messages
func (t *MessageTool) SetSendCallback(callback func(channel, chatID, content string) error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendCallback = callback
}

// SetContext sets the current message context
func (t *MessageTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.defaultChannel = channel
	t.defaultChatID = chatID
}

// Execute sends the message
func (t *MessageTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	content, ok := args["content"].(string)
	if !ok {
		return "Error: content is required", nil
	}

	t.mu.RLock()
	channel := t.defaultChannel
	chatID := t.defaultChatID
	sendCallback := t.sendCallback
	t.mu.RUnlock()

	// Override with provided values
	if c, ok := args["channel"].(string); ok && c != "" {
		channel = c
	}
	if c, ok := args["chat_id"].(string); ok && c != "" {
		chatID = c
	}

	if channel == "" || chatID == "" {
		return "Error: no target channel/chat specified", nil
	}

	if sendCallback == nil {
		return "Error: message sending not configured", nil
	}

	if err := sendCallback(channel, chatID, content); err != nil {
		return "Error sending message: " + err.Error(), nil
	}

	return "Message sent to " + channel + ":" + chatID, nil
}

// ConcurrentSafe returns false - message sending should be sequential to maintain order
func (t *MessageTool) ConcurrentSafe() bool {
	return false
}
