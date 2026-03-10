package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToolContext_BasicFields(t *testing.T) {
	bus := &MockMessageSender{}
	adapter := &MockCardAdapter{}
	storage := &MockCardStorage{}

	ctx := ToolContext{
		UserID:     "user123",
		ChannelID:  "channel456",
		SessionID:  "session789",
		ToolName:   "tmux",
		ToolCallID: "call-001",
		Params:     map[string]interface{}{"command": "rm -rf"},
		CardSession: &CardSession{
			ID: "card-001",
		},
		Bus:       bus,
		Adapter:   adapter,
		Storage:   storage,
		StartTime: time.Now(),
		Metadata:  map[string]string{"source": "feishu"},
	}

	assert.Equal(t, "user123", ctx.UserID)
	assert.Equal(t, "channel456", ctx.ChannelID)
	assert.Equal(t, "session789", ctx.SessionID)
	assert.Equal(t, "tmux", ctx.ToolName)
	assert.Equal(t, "call-001", ctx.ToolCallID)
	assert.Equal(t, "rm -rf", ctx.Params["command"])
	assert.Equal(t, "card-001", ctx.CardSession.ID)
	assert.NotNil(t, ctx.Bus)
	assert.NotNil(t, ctx.Adapter)
	assert.NotNil(t, ctx.Storage)
	assert.NotNil(t, ctx.StartTime)
	assert.Equal(t, "feishu", ctx.Metadata["source"])
}

func TestCardSession_StateTransitions(t *testing.T) {
	session := &CardSession{
		ID:        "card-001",
		State:     CardStatePending,
		CreatedAt: time.Now(),
	}

	// Initial state should be Pending
	assert.Equal(t, CardStatePending, session.State)

	// Transition to Answered
	session.SetAnswer([]string{"yes"})
	assert.Equal(t, CardStateAnswered, session.State)
	assert.NotNil(t, session.AnsweredAt)
	assert.Equal(t, []string{"yes"}, session.UserAnswers)
}

func TestCardSession_IsExpired(t *testing.T) {
	tests := []struct {
		name        string
		expiresAt   time.Time
		expected    bool
		description string
	}{
		{
			name:        "not expired",
			expiresAt:   time.Now().Add(1 * time.Hour),
			expected:    false,
			description: "expires in the future",
		},
		{
			name:        "expired",
			expiresAt:   time.Now().Add(-1 * time.Hour),
			expected:    true,
			description: "expired in the past",
		},
		{
			name:        "zero time - not expired",
			expiresAt:   time.Time{},
			expected:    false,
			description: "zero time means no expiration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &CardSession{
				ID:        "card-001",
				ExpiresAt: tt.expiresAt,
			}
			assert.Equal(t, tt.expected, session.IsExpired(), tt.description)
		})
	}
}

func TestToolDecision_ActionTypes(t *testing.T) {
	tests := []struct {
		name        string
		action      ToolAction
		description string
	}{
		{
			name:        "ActionContinue",
			action:      ActionContinue,
			description: "continue with tool execution",
		},
		{
			name:        "ActionWaitCard",
			action:      ActionWaitCard,
			description: "wait for card response",
		},
		{
			name:        "ActionSkip",
			action:      ActionSkip,
			description: "skip tool execution",
		},
		{
			name:        "ActionCancel",
			action:      ActionCancel,
			description: "cancel tool execution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := ToolDecision{
				Action:     tt.action,
				CardNeeded: tt.action == ActionWaitCard,
				CardType:   CardTypeConfirm,
			}

			assert.Equal(t, tt.action, decision.Action)
			assert.Equal(t, tt.action == ActionWaitCard, decision.CardNeeded)
			assert.Equal(t, CardTypeConfirm, decision.CardType)
		})
	}
}

// Mock types for testing
type MockCardAdapter struct{}

func (m *MockCardAdapter) SendCard(userID, channelID string, card *CardContent) error {
	return nil
}

func (m *MockCardAdapter) UpdateCard(userID, channelID, sessionID string, card *CardContent) error {
	return nil
}

func (m *MockCardAdapter) DeleteCard(userID, channelID, sessionID string) error {
	return nil
}

type MockCardStorage struct{}

func (m *MockCardStorage) SaveCardSession(session *CardSession) error {
	return nil
}

func (m *MockCardStorage) GetCardSession(id string) (*CardSession, error) {
	return nil, nil
}

func (m *MockCardStorage) GetActiveCardSession(userID, channelID, toolName, toolCallID string) (*CardSession, error) {
	return nil, nil
}

func (m *MockCardStorage) UpdateCardSession(session *CardSession) error {
	return nil
}

func (m *MockCardStorage) DeleteCardSession(id string) error {
	return nil
}

func (m *MockCardStorage) ExpireCardSessions(expiresBefore time.Time) error {
	return nil
}

type MockMessageSender struct{}

func (m *MockMessageSender) SendOutbound(ctx context.Context, channelID, senderID, content string) error {
	return nil
}
