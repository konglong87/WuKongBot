package agent

import (
	"context"
	"testing"

	"github.com/konglong87/wukongbot/internal/feishu/enhanced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnhancedHandler_HandleToolCardCallback(t *testing.T) {
	// Create a mock message sender
	mockSender := &mockMessageSender{}

	// Create enhanced handler
	handler := NewEnhancedHandler(mockSender, "/tmp/test", ClaudeCodeConfig{})

	// Create a card session manager session
	sessionID := "test_session_123"
	userID := "user_123"
	channelID := "feishu"
	toolName := "tmux"
	toolCallID := "call_456"

	cardContent := &CardContent{
		Title:       "Dangerous Command",
		Description: "This command is dangerous",
		Question:    "Are you sure you want to run this?",
	}

	session, err := handler.cardSessionManager.CreateSession(
		userID, channelID, toolName, toolCallID,
		CardTypeConfirm, cardContent,
		map[string]interface{}{"command": "rm -rf /"},
	)

	require.NoError(t, err)
	require.NotNil(t, session)

	// Test handling confirm callback with yes
	callback := &enhanced.FeishuCardCallback{
		Type:      "confirm",
		SessionID: sessionID,
		UserID:    userID,
		Values:    map[string]interface{}{"value": "yes"},
		Timestamp: 1234567890,
	}

	// Note: This will fail because sessionID doesn't match the actual created session
	// We need to use the actual session ID from the created session
	callback.SessionID = session.ID

	response, err := handler.HandleToolCardCallback(callback)

	require.NoError(t, err)
	assert.Contains(t, response, "确认")

	// Verify session is answered
	answeredSession, err := handler.cardSessionManager.GetSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, CardStateAnswered, answeredSession.State)
	assert.Equal(t, []string{"yes"}, answeredSession.UserAnswers)
}

func TestEnhancedHandler_HandleToolCardCallback_SingleChoice(t *testing.T) {
	// Create a mock message sender
	mockSender := &mockMessageSender{}

	// Create enhanced handler
	handler := NewEnhancedHandler(mockSender, "/tmp/test", ClaudeCodeConfig{})

	// Create a card session
	userID := "user_123"
	channelID := "feishu"
	toolName := "tmux"
	toolCallID := "call_789"

	cardContent := &CardContent{
		Title:       "Select Option",
		Description: "Choose an option",
		Question:    "Which option do you want?",
		Options: []*CardOption{
			{Label: "Option 1", Value: "opt1"},
			{Label: "Option 2", Value: "opt2"},
			{Label: "Option 3", Value: "opt3"},
		},
	}

	session, err := handler.cardSessionManager.CreateSession(
		userID, channelID, toolName, toolCallID,
		CardTypeSingleChoice, cardContent,
		map[string]interface{}{},
	)

	require.NoError(t, err)
	require.NotNil(t, session)

	// Test handling single_choice callback
	callback := &enhanced.FeishuCardCallback{
		Type:      "single_choice",
		SessionID: session.ID,
		UserID:    userID,
		Values:    map[string]interface{}{"value": "opt1"},
		Timestamp: 1234567890,
	}

	response, err := handler.HandleToolCardCallback(callback)

	require.NoError(t, err)
	assert.Contains(t, response, "选择")

	// Verify session is answered
	answeredSession, err := handler.cardSessionManager.GetSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, CardStateAnswered, answeredSession.State)
	assert.Equal(t, []string{"opt1"}, answeredSession.UserAnswers)
}

func TestEnhancedHandler_HandleToolCardCallback_MultipleChoice(t *testing.T) {
	// Create a mock message sender
	mockSender := &mockMessageSender{}

	// Create enhanced handler
	handler := NewEnhancedHandler(mockSender, "/tmp/test", ClaudeCodeConfig{})

	// Create a card session
	userID := "user_123"
	channelID := "feishu"
	toolName := "tmux"
	toolCallID := "call_multi"

	cardContent := &CardContent{
		Title:       "Select Options",
		Description: "Choose multiple options",
		Question:    "Which options do you want?",
		Options: []*CardOption{
			{Label: "Option A", Value: "optA"},
			{Label: "Option B", Value: "optB"},
			{Label: "Option C", Value: "optC"},
		},
	}

	session, err := handler.cardSessionManager.CreateSession(
		userID, channelID, toolName, toolCallID,
		CardTypeMultipleChoice, cardContent,
		map[string]interface{}{},
	)

	require.NoError(t, err)
	require.NotNil(t, session)

	// Test handling multiple_choice callback
	callback := &enhanced.FeishuCardCallback{
		Type:      "multiple_choice",
		SessionID: session.ID,
		UserID:    userID,
		Values: map[string]interface{}{
			"values": []interface{}{"optA", "optC"},
		},
		Timestamp: 1234567890,
	}

	response, err := handler.HandleToolCardCallback(callback)

	require.NoError(t, err)
	assert.Contains(t, response, "选择")

	// Verify session is answered
	answeredSession, err := handler.cardSessionManager.GetSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, CardStateAnswered, answeredSession.State)
	assert.Equal(t, []string{"optA", "optC"}, answeredSession.UserAnswers)
}

func TestEnhancedHandler_HandleToolCardCallback_InvalidSession(t *testing.T) {
	// Create a mock message sender
	mockSender := &mockMessageSender{}

	// Create enhanced handler
	handler := NewEnhancedHandler(mockSender, "/tmp/test", ClaudeCodeConfig{})

	// Test handling callback with invalid session ID
	callback := &enhanced.FeishuCardCallback{
		Type:      "confirm",
		SessionID: "invalid_session_id",
		UserID:    "user_123",
		Values:    map[string]interface{}{"value": "yes"},
		Timestamp: 1234567890,
	}

	_, err := handler.HandleToolCardCallback(callback)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestEnhancedHandler_parseCardAnswers(t *testing.T) {
	handler := &EnhancedHandler{}

	// Test confirm type
	callback := &enhanced.FeishuCardCallback{
		Type:   "confirm",
		Values: map[string]interface{}{"value": "yes"},
	}

	answers := handler.parseCardAnswers(callback, CardTypeConfirm)
	assert.Equal(t, []string{"yes"}, answers)

	// Test single_choice type
	callback = &enhanced.FeishuCardCallback{
		Type:   "single_choice",
		Values: map[string]interface{}{"value": "opt1"},
	}

	answers = handler.parseCardAnswers(callback, CardTypeSingleChoice)
	assert.Equal(t, []string{"opt1"}, answers)

	// Test multiple_choice type
	callback = &enhanced.FeishuCardCallback{
		Type: "multiple_choice",
		Values: map[string]interface{}{
			"values": []interface{}{"optA", "optB", "optC"},
		},
	}

	answers = handler.parseCardAnswers(callback, CardTypeMultipleChoice)
	assert.Equal(t, []string{"optA", "optB", "optC"}, answers)
}

func TestEnhancedHandler_generateConfirmationMessage(t *testing.T) {
	handler := &EnhancedHandler{}

	// Test confirm type
	msg := handler.generateConfirmationMessage(CardTypeConfirm, []string{"yes"})
	assert.Contains(t, msg, "确认")

	// Test single_choice type
	msg = handler.generateConfirmationMessage(CardTypeSingleChoice, []string{"opt1"})
	assert.Contains(t, msg, "选择")
	assert.Contains(t, msg, "opt1")

	// Test multiple_choice type
	msg = handler.generateConfirmationMessage(CardTypeMultipleChoice, []string{"optA", "optB"})
	assert.Contains(t, msg, "选择")
	assert.Contains(t, msg, "optA")
	assert.Contains(t, msg, "optB")
}

// mockMessageSender is a mock implementation of MessageSender
type mockMessageSender struct{}

func (m *mockMessageSender) SendOutbound(_ context.Context, _, _, _ string) error {
	return nil
}
