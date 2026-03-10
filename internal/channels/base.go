package channels

import (
	"context"
	"time"
)

// Media represents media content (images, videos, etc.)
type Media struct {
	Type     string `json:"type"`     // "image", "video", etc.
	URL      string `json:"url,omitempty"`
	Data     string `json:"data,omitempty"` // Base64 encoded data
	MimeType string `json:"mime_type,omitempty"`
}

// Message represents a chat message from a channel
type Message struct {
	ID        string                 `json:"id"`
	ChannelID string                 `json:"channel_id"`
	SenderID  string                 `json:"sender_id"`
	Sender    string                 `json:"sender,omitempty"`
	Content   string                 `json:"content"`
	Type      string                 `json:"type"` // text, image, file, etc.
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Media     []Media                `json:"media,omitempty"` // Images, videos, etc.
	TraceId   string                 `json:"trace_id,omitempty"` // Trace ID for full-chain tracing
}

// OutboundMessage represents a message to be sent to a channel
type OutboundMessage struct {
	ChannelID   string                 `json:"channel_id"`
	RecipientID string                 `json:"recipient_id"`
	Content     string                 `json:"content"`
	Type        string                 `json:"type"` // text, markdown, html, etc.
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Media       []Media                `json:"media,omitempty"` // Images, videos, etc.
	TraceId     string                 `json:"trace_id,omitempty"` // Trace ID for full-chain tracing
}

// BaseChannel defines the interface for communication channels
type BaseChannel interface {
	// Start initializes and starts the channel
	Start(ctx context.Context) error

	// Stop gracefully stops the channel
	Stop(ctx context.Context) error

	// Send sends an outbound message through the channel
	Send(msg OutboundMessage) error

	// Receive returns a channel for incoming messages
	Receive() <-chan Message

	// Name returns the channel name
	Name() string

	// IsRunning returns whether the channel is currently running
	IsRunning() bool

	// IsAllowed checks if a sender is authorized to use the channel
	IsAllowed(senderID string) bool

	// Type returns the channel type (telegram, whatsapp, feishu, etc.)
	Type() string
}