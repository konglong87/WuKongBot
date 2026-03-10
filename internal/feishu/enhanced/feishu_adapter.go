package enhanced

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/konglong87/wukongbot/internal/feishu/progress"
)

// FeishuAdapter handles enhanced Feishu interactions
type FeishuAdapter struct {
	// Use a simple message sending interface to avoid circular import
	messageSender    func(channelID, recipientID, content string) error
	cardGenerator    *CardGenerator
	questionDetector *QuestionDetector
	contentChunker   *ContentChunker
	progressManager  *progress.ProgressManager
	logger           *log.Logger
}

// NewFeishuAdapter creates a new Feishu adapter
func NewFeishuAdapter(messageSender func(channelID, recipientID, content string) error) *FeishuAdapter {
	// Wrap the simple message sender to satisfy progress.MessageSender
	sender := &simpleMessageSender{sendFunc: messageSender}

	return &FeishuAdapter{
		messageSender:    messageSender,
		cardGenerator:    NewCardGenerator(),
		questionDetector: NewQuestionDetector(),
		contentChunker:   NewContentChunker(),
		progressManager:  progress.NewProgressManager(sender),
		logger:           log.Default(),
	}
}

type simpleMessageSender struct {
	sendFunc func(channelID, recipientID, content string) error
}

func (s *simpleMessageSender) SendMessage(channelID, recipientID, content string) error {
	return s.sendFunc(channelID, recipientID, content)
}

func (s *simpleMessageSender) Name() string {
	return "feishu"
}

// SendEnhancedMessage sends an enhanced message with card detection
func (f *FeishuAdapter) SendEnhancedMessage(sessionID string, content string) error {
	// 1. Check if content contains a question that needs a card
	if card, err := f.cardGenerator.GenerateFromLLMResponse(content, sessionID); err == nil {
		// Send interactive card
		return f.SendInteractiveCard(sessionID, card)
	}

	// 2. Send as regular message
	return f.messageSender("feishu", sessionID, content)
}

// SendInteractiveCard sends an interactive card to the user
func (f *FeishuAdapter) SendInteractiveCard(sessionID string, card *FeishuCard) error {
	// Convert card to JSON
	cardJSON, err := card.ToJSON()
	if err != nil {
		f.logger.Error("feishu", "failed to marshal card", "error", err)
		return err
	}

	return f.messageSender("feishu", sessionID, cardJSON)
}

// HandleCardCallback handles callbacks from interactive cards
func (f *FeishuAdapter) HandleCardCallback(callback *FeishuCardCallback) (string, error) {
	// Validate callback
	if err := f.ValidateCardCallback(callback); err != nil {
		return "", err
	}

	// Convert callback to user message
	userMessage := f.CallbackToUserMessage(callback)

	f.logger.Debug("feishu", "handled card callback", "session", callback.SessionID, "type", callback.Type)

	return userMessage, nil
}

// ValidateCardCallback validates a card callback
func (f *FeishuAdapter) ValidateCardCallback(callback *FeishuCardCallback) error {
	if callback.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	if callback.UserID == "" {
		return fmt.Errorf("user ID is required")
	}

	if callback.Type == "" {
		return fmt.Errorf("callback type is required")
	}

	return nil
}

// CallbackToUserMessage converts a callback to a user message string
func (f *FeishuAdapter) CallbackToUserMessage(callback *FeishuCardCallback) string {
	switch callback.Type {
	case "single_choice":
		if value, ok := callback.Values["value"].(string); ok {
			return fmt.Sprintf("用户选择: %s", value)
		}
	case "multiple_choice":
		if values, ok := callback.Values["values"].([]interface{}); ok {
			selected := fmt.Sprintf("%v", values)
			return fmt.Sprintf("用户选择: %s", selected)
		}
	case "checklist":
		if values, ok := callback.Values["values"].([]interface{}); ok {
			selected := fmt.Sprintf("%v", values)
			return fmt.Sprintf("用户选择: %s", selected)
		}
	case "confirm":
		if value, ok := callback.Values["value"].(string); ok {
			if value == "yes" {
				return "用户: 确认"
			} else {
				return "用户: 取消"
			}
		}
	case "action":
		if action, ok := callback.Values["value"].(string); ok {
			switch action {
			case "submit":
				return "用户: 提交"
			case "reset":
				return "用户: 重置"
			default:
				return fmt.Sprintf("用户: %s", action)
			}
		}
	}

	return fmt.Sprintf("用户操作: %s", callback.Type)
}

// SendProgressMessage sends a progress message
func (f *FeishuAdapter) SendProgressMessage(sessionID string, progress progress.ProgressMessage) error {
	return f.progressManager.SendProgressMessage(sessionID, progress)
}

// StartTask starts a new task
func (f *FeishuAdapter) StartTask(sessionID, taskID string, totalSteps int) error {
	return f.progressManager.StartTask(sessionID, taskID, totalSteps)
}

// UpdateProgress updates task progress
func (f *FeishuAdapter) UpdateProgress(sessionID, stepID, toolName, status, message string, result interface{}) error {
	return f.progressManager.UpdateProgress(sessionID, stepID, toolName, status, message, result)
}

// CompleteTask completes a task
func (f *FeishuAdapter) CompleteTask(sessionID, taskID, summary string) error {
	return f.progressManager.CompleteTask(sessionID, taskID, summary)
}

// FormatErrorMessage formats an error message for display
func (f *FeishuAdapter) FormatErrorMessage(error string) string {
	return f.contentChunker.FormatErrorMessage(error)
}

// FormatWarningMessage formats a warning message for display
func (f *FeishuAdapter) FormatWarningMessage(warning string) string {
	return f.contentChunker.FormatWarningMessage(warning)
}

// FormatSuccessMessage formats a success message for display
func (f *FeishuAdapter) FormatSuccessMessage(message string) string {
	return f.contentChunker.FormatSuccessMessage(message)
}

// FormatFileContent formats file content for display
func (f *FeishuAdapter) FormatFileContent(filename, content string) string {
	return f.contentChunker.FormatFileContent(filename, content)
}

// FormatCodeOutput formats code output for display
func (f *FeishuAdapter) FormatCodeOutput(output, language string) string {
	return f.contentChunker.FormatCodeOutput(output, language)
}

// ParseCardCallbackFromJSON parses a card callback from JSON
func (f *FeishuAdapter) ParseCardCallbackFromJSON(jsonStr string) (*FeishuCardCallback, error) {
	var callback FeishuCardCallback
	err := json.Unmarshal([]byte(jsonStr), &callback)
	if err != nil {
		return nil, fmt.Errorf("failed to parse card callback: %w", err)
	}
	return &callback, nil
}

// DetectIfCardNeeded checks if a card is needed for the given response
func (f *FeishuAdapter) DetectIfCardNeeded(response string) bool {
	_, card := f.questionDetector.DetectQuestion(response)
	return card != nil
}

// GetProgressManager returns the progress manager
func (f *FeishuAdapter) GetProgressManager() *progress.ProgressManager {
	return f.progressManager
}

// GetContentChunker returns the content chunker
func (f *FeishuAdapter) GetContentChunker() *ContentChunker {
	return f.contentChunker
}

// GetCardGenerator returns the card generator
func (f *FeishuAdapter) GetCardGenerator() *CardGenerator {
	return f.cardGenerator
}

// GetQuestionDetector returns the question detector
func (f *FeishuAdapter) GetQuestionDetector() *QuestionDetector {
	return f.questionDetector
}
