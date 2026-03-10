package channels

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// WhatsAppChannel implements BaseChannel for WhatsApp
// Uses WebSocket connection to a bridge service
type WhatsAppChannel struct {
	name       string
	cfg        WhatsAppConfig
	mu         sync.RWMutex
	running    bool
	messages   chan Message
	conn       interface{} // WebSocket connection (interface for extensibility)
}

// NewWhatsAppChannel creates a new WhatsApp channel
func NewWhatsAppChannel(cfg WhatsAppConfig) *WhatsAppChannel {
	return &WhatsAppChannel{
		name:     "whatsapp",
		cfg:      cfg,
		messages: make(chan Message, 100),
	}
}

// Name returns the channel name
func (c *WhatsAppChannel) Name() string {
	return c.name
}

// Type returns the channel type
func (c *WhatsAppChannel) Type() string {
	return "whatsapp"
}

// Start initializes and starts the WhatsApp channel
func (c *WhatsAppChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = true
	// Note: Actual WebSocket connection would be implemented here
	// connecting to bridge_url (default: ws://localhost:3001)
	return nil
}

// Stop gracefully stops the WhatsApp channel
func (c *WhatsAppChannel) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	return nil
}

// Send sends a message through WhatsApp
func (c *WhatsAppChannel) Send(msg OutboundMessage) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Construct WhatsApp message payload
	payload := map[string]interface{}{
		"to":      msg.RecipientID,
		"type":    "text",
		"content": msg.Content,
	}
	_ = payload

	// Would send via WebSocket here
	return nil
}

// Receive returns a channel for incoming messages
func (c *WhatsAppChannel) Receive() <-chan Message {
	return c.messages
}

// IsRunning returns whether the channel is running
func (c *WhatsAppChannel) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// IsAllowed checks if a sender is authorized
func (c *WhatsAppChannel) IsAllowed(senderID string) bool {
	if len(c.cfg.AllowFrom) == 0 {
		return true
	}
	for _, id := range c.cfg.AllowFrom {
		if id == senderID {
			return true
		}
	}
	return false
}

// WhatsAppConfig holds WhatsApp configuration
type WhatsAppConfig struct {
	Enabled   bool
	BridgeURL string
	AllowFrom []string
}

// WhatsAppMessage represents an incoming WhatsApp message
type WhatsAppMessage struct {
	ID        string                 `json:"id"`
	From      string                 `json:"from"`
	Content   string                 `json:"content"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ParseWhatsAppMessage parses incoming WhatsApp message
func ParseWhatsAppMessage(data []byte) (*WhatsAppMessage, error) {
	var msg WhatsAppMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}