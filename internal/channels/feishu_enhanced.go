package channels

import (
	"context"
	"fmt"
	"time"

	"github.com/konglong87/wukongbot/internal/feishu/enhanced"
	"github.com/konglong87/wukongbot/internal/feishu/progress"
)

// FeishuChannelEnhancedConfig holds configuration for enhanced Feishu features
type FeishuChannelEnhancedConfig struct {
	Enabled             bool `yaml:"enabled"`
	InteractiveCards    bool `yaml:"interactive_cards"`
	RealtimeProgress    bool `yaml:"realtime_progress"`
	ProgressDelay       int  `yaml:"progress_delay_ms"`     // Delay between progress messages
	ChunkMaxSize        int  `yaml:"chunk_max_size"`        // Max size for content chunking
	ShowProgressDetails bool `yaml:"show_progress_details"` // Show detailed progress
}

// FeishuChannelEnhanced extends FeishuChannel with enhanced capabilities
type FeishuChannelEnhanced struct {
	*FeishuChannel
	adapter *enhanced.FeishuAdapter
	config  FeishuChannelEnhancedConfig
}

// NewFeishuChannelEnhanced creates an enhanced Feishu channel
func NewFeishuChannelEnhanced(base *FeishuChannel, config FeishuChannelEnhancedConfig) (*FeishuChannelEnhanced, error) {
	if !config.Enabled {
		// Return base channel without enhancements
		return &FeishuChannelEnhanced{
			FeishuChannel: base,
			config:        config,
		}, nil
	}

	// Create enhancement adapter with wrapper function
	messageSender := func(channelID, recipientID, content string) error {
		return base.Send(OutboundMessage{
			ChannelID:   channelID,
			RecipientID: recipientID,
			Content:     content,
		})
	}
	adapter := enhanced.NewFeishuAdapter(messageSender)

	// Configure adapter settings
	if pm := adapter.GetProgressManager(); pm != nil {
		if config.ProgressDelay > 0 {
			pm.SetMessageDelay(time.Duration(config.ProgressDelay) * time.Millisecond)
		}
		pm.SetShowDetails(config.ShowProgressDetails)
		pm.SetShowSummary(true)
	}

	return &FeishuChannelEnhanced{
		FeishuChannel: base,
		adapter:       adapter,
		config:        config,
	}, nil
}

// SendEnhancedMessage sends a message with enhanced features (cards, chunking, etc.)
func (f *FeishuChannelEnhanced) SendEnhancedMessage(sessionID string, content string) error {
	if f.adapter == nil {
		// Fallback to regular send
		return f.Send(OutboundMessage{
			ChannelID:   f.Name(),
			RecipientID: sessionID,
			Content:     content,
			Type:        "markdown",
		})
	}

	return f.adapter.SendEnhancedMessage(sessionID, content)
}

// SendInteractiveCard sends an interactive card
func (f *FeishuChannelEnhanced) SendInteractiveCard(sessionID string, card *enhanced.FeishuCard) error {
	if f.adapter == nil {
		return fmt.Errorf("adapter not initialized")
	}

	return f.adapter.SendInteractiveCard(sessionID, card)
}

// SendProgressMessage sends a progress message
func (f *FeishuChannelEnhanced) SendProgressMessage(sessionID string, progress progress.ProgressMessage) error {
	if f.adapter == nil {
		return fmt.Errorf("adapter not initialized")
	}

	return f.adapter.SendProgressMessage(sessionID, progress)
}

// StartTask starts tracking a new task
func (f *FeishuChannelEnhanced) StartTask(sessionID, taskID string, totalSteps int) error {
	if f.adapter == nil {
		return nil // No-op if adapter not initialized
	}

	return f.adapter.StartTask(sessionID, taskID, totalSteps)
}

// UpdateProgress updates task progress
func (f *FeishuChannelEnhanced) UpdateProgress(sessionID, stepID, toolName, status, message string, result interface{}) error {
	if f.adapter == nil {
		return nil // No-op if adapter not initialized
	}

	return f.adapter.UpdateProgress(sessionID, stepID, toolName, status, message, result)
}

// CompleteTask completes a task
func (f *FeishuChannelEnhanced) CompleteTask(sessionID, taskID, summary string) error {
	if f.adapter == nil {
		return nil // No-op if adapter not initialized
	}

	return f.adapter.CompleteTask(sessionID, taskID, summary)
}

// FormatErrorMessage formats an error message
func (f *FeishuChannelEnhanced) FormatErrorMessage(error string) string {
	if f.adapter == nil {
		chunker := enhanced.NewContentChunker()
		return chunker.FormatErrorMessage(error)
	}

	return f.adapter.FormatErrorMessage(error)
}

// FormatSuccessMessage formats a success message
func (f *FeishuChannelEnhanced) FormatSuccessMessage(message string) string {
	if f.adapter == nil {
		chunker := enhanced.NewContentChunker()
		return chunker.FormatSuccessMessage(message)
	}

	return f.adapter.FormatSuccessMessage(message)
}

// FormatFileContent formats file content for display
func (f *FeishuChannelEnhanced) FormatFileContent(filename, content string) string {
	if f.adapter == nil {
		chunker := enhanced.NewContentChunker()
		return chunker.FormatFileContent(filename, content)
	}

	return f.adapter.FormatFileContent(filename, content)
}

// DetectIfCardNeeded checks if a card is needed for the given response
func (f *FeishuChannelEnhanced) DetectIfCardNeeded(response string) bool {
	if f.adapter == nil {
		return false
	}

	return f.adapter.DetectIfCardNeeded(response)
}

// GetAdapter returns the underlying adapter (for advanced use)
func (f *FeishuChannelEnhanced) GetAdapter() *enhanced.FeishuAdapter {
	return f.adapter
}

// IsEnhanced returns whether this channel has enhancements enabled
func (f *FeishuChannelEnhanced) IsEnhanced() bool {
	return f.adapter != nil && f.config.Enabled
}

// StartCleanupRoutine starts the cleanup routine for expired sessions
func (f *FeishuChannelEnhanced) StartCleanupRoutine(ctx context.Context, interval, maxAgeSec int) {
	if f.adapter == nil || f.adapter.GetProgressManager() == nil {
		return
	}

	pm := f.adapter.GetProgressManager()
	pm.StartCleanupRoutine(ctx,
		time.Duration(interval)*time.Second,
		time.Duration(maxAgeSec)*time.Second,
	)
}
