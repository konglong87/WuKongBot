package channels

import (
	"context"
	"sync"

	telebot "github.com/go-telegram-bot-api/telegram-bot-api"
)

// TelegramChannel implements BaseChannel for Telegram
type TelegramChannel struct {
	name     string
	bot      *telebot.BotAPI
	updates  telebot.UpdatesChannel
	cfg      TelegramConfig
	mu       sync.RWMutex
	running  bool
	messages chan Message
}

// NewTelegramChannel creates a new Telegram channel
func NewTelegramChannel(cfg TelegramConfig) *TelegramChannel {
	return &TelegramChannel{
		name:     "telegram",
		cfg:      cfg,
		messages: make(chan Message, 100),
	}
}

// Name returns the channel name
func (c *TelegramChannel) Name() string {
	return c.name
}

// Type returns the channel type
func (c *TelegramChannel) Type() string {
	return "telegram"
}

// Start initializes and starts the Telegram bot
func (c *TelegramChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	bot, err := telebot.NewBotAPI(c.cfg.Token)
	if err != nil {
		return err
	}
	c.bot = bot

	u := telebot.NewUpdate(0)
	u.Timeout = 60

	updates, err := c.bot.GetUpdatesChan(u)
	if err != nil {
		return err
	}
	c.updates = updates
	c.running = true

	go c.handleUpdates(ctx)

	return nil
}

// Stop gracefully stops the Telegram bot
func (c *TelegramChannel) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	return nil
}

// Send sends a message through Telegram
func (c *TelegramChannel) Send(msg OutboundMessage) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.bot == nil {
		return nil
	}

	// For messages with images, send as text with URL for now
	// Note: Sending actual binary images to Telegram requires file upload
	// which is more complex. This implementation sends image URLs as text.
	return c.sendText(msg)
}

// sendText sends a text message
func (c *TelegramChannel) sendText(msg OutboundMessage) error {
	m := telebot.NewMessageToChannel(msg.RecipientID, msg.Content)
	m.ParseMode = telebot.ModeMarkdown
	_, err := c.bot.Send(m)
	return err
}

// looksLikeJSON checks if a string looks like JSON (for tool result messages)
func looksLikeJSON(s string) bool {
	return len(s) > 0 && (s[0] == '{' || s[0] == '[')
}

// Receive returns a channel for incoming messages
func (c *TelegramChannel) Receive() <-chan Message {
	return c.messages
}

// IsRunning returns whether the channel is running
func (c *TelegramChannel) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// IsAllowed checks if a sender is authorized
func (c *TelegramChannel) IsAllowed(senderID string) bool {
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

func (c *TelegramChannel) handleUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-c.updates:
			if !ok {
				return
			}
			if update.Message == nil {
				continue
			}

			senderName := ""
			senderIDStr := ""
			if update.Message.From != nil {
				senderName = update.Message.From.UserName
				senderIDStr = update.Message.From.String()
			}

			msg := Message{
				ID:        "",
				ChannelID: "telegram",
				SenderID:  senderName,
				Sender:    senderIDStr, // Use actual chat/user ID for identification
				Content:   update.Message.Text,
				Timestamp: update.Message.Time(),
			}

			// Handle photo/image messages
			if update.Message.Photo != nil && len(*update.Message.Photo) > 0 {
				// Get the largest (last) photo
				photos := *update.Message.Photo
				largestPhoto := photos[len(photos)-1]
				msg.Media = []Media{
					{
						Type: "image",
						Data: largestPhoto.FileID, // Use FileID, will convert to URL when sending
					},
				}
				if update.Message.Caption != "" {
					msg.Content = update.Message.Caption
				}
			}

			select {
			case c.messages <- msg:
			default:
			}
		}
	}
}

// TelegramConfig holds Telegram configuration
type TelegramConfig struct {
	Enabled   bool
	Token     string
	AllowFrom []string
	Proxy     string
}
