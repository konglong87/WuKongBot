package bus

import (
	"context"
	"time"
)

// InboundMessage represents a message from a channel to the agent
type InboundMessage struct {
	MessageID  string                 `json:"message_id"`
	ChannelID  string                 `json:"channel_id"`
	SenderID   string                 `json:"sender_id"`
	SenderName string                 `json:"sender_name,omitempty"`
	Content    string                 `json:"content"`
	Type       string                 `json:"type"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Media      []Media                `json:"media,omitempty"`    // Images, videos, etc.
	TraceId    string                 `json:"trace_id,omitempty"` // Trace ID for full-chain tracing
}

// Media represents media content (images, videos, etc.)
type Media struct {
	Type     string `json:"type"` // "image", "video", etc.
	URL      string `json:"url,omitempty"`
	Data     string `json:"data,omitempty"` // Base64 encoded data
	MimeType string `json:"mime_type,omitempty"`
}

// OutboundMessage represents a message from the agent to a channel
type OutboundMessage struct {
	MessageID   string                 `json:"message_id"`
	ChannelID   string                 `json:"channel_id"`
	RecipientID string                 `json:"recipient_id"`
	Content     string                 `json:"content"`
	Type        string                 `json:"type"` // text, markdown, html
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Media       []Media                `json:"media,omitempty"`    // Images, videos, etc.
	TraceId     string                 `json:"trace_id,omitempty"` // Trace ID for full-chain tracing
}

// MessageBus defines the interface for message passing between channels and agent
type MessageBus interface {
	// PublishInbound publishes an inbound message to the bus
	PublishInbound(ctx context.Context, msg InboundMessage) error

	// ConsumeInbound retrieves an inbound message from the bus
	ConsumeInbound(ctx context.Context) (InboundMessage, error)

	// PublishOutbound publishes an outbound message to the bus
	PublishOutbound(ctx context.Context, msg OutboundMessage) error

	// ConsumeOutbound retrieves an outbound message from the bus
	ConsumeOutbound(ctx context.Context) (OutboundMessage, error)

	// Close closes the message bus and releases resources
	Close() error
}
