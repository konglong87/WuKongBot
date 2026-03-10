package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCardSessionManager_CreateSession(t *testing.T) {
	// Create manager with nil storage (storage is TODO)
	manager := NewCardSessionManager(nil)

	userID := "test_user"
	channelID := "test_channel"
	toolName := "tmux"
	toolCallID := "call_123"
	cardType := CardTypeConfirm
	cardContent := &CardContent{
		Title:       "Test Card",
		Description: "This is a test card",
		Question:    "Are you sure?",
	}
	toolParams := map[string]interface{}{"command": "rm -rf /"}

	session, err := manager.CreateSession(userID, channelID, toolName, toolCallID, cardType, cardContent, toolParams)

	require.NoError(t, err)
	assert.NotNil(t, session)

	// Verify session properties
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, userID, session.UserID)
	assert.Equal(t, channelID, session.ChannelID)
	assert.Equal(t, toolName, session.ToolName)
	assert.Equal(t, toolCallID, session.ToolCallID)
	assert.Equal(t, CardStatePending, session.State)
	assert.Equal(t, cardType, session.CardType)
	assert.Equal(t, cardContent, session.CardContent)
	assert.Equal(t, toolParams, session.ToolParams)

	// Verify timestamps
	assert.WithinDuration(t, time.Now(), session.CreatedAt, time.Second)
	assert.WithinDuration(t, time.Now(), session.UpdatedAt, time.Second)
	assert.WithinDuration(t, time.Now().Add(5*time.Minute), session.ExpiresAt, time.Second)
}

func TestCardSessionManager_GetSession(t *testing.T) {
	manager := NewCardSessionManager(nil)

	// Create a session via the manager
	session, err := manager.CreateSession("user1", "channel1", "tmux", "call_1",
		CardTypeConfirm, &CardContent{
			Title:       "Test Card",
			Description: "This is a test card",
			Question:    "Are you sure?",
		}, map[string]interface{}{"command": "rm -rf /"})

	require.NoError(t, err)
	sessionID := session.ID

	// Test getting existing session
	retrieved, err := manager.GetSession(sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, retrieved.ID)
	assert.Equal(t, "user1", retrieved.UserID)
	assert.Equal(t, "channel1", retrieved.ChannelID)
	assert.Equal(t, "tmux", retrieved.ToolName)
	assert.Equal(t, CardStatePending, retrieved.State)

	// Test getting non-existent session
	_, err = manager.GetSession("non_existent")
	assert.Error(t, err)
	assert.Equal(t, ErrSessionNotFound, err)
}

func TestCardSessionManager_SetAnswer(t *testing.T) {
	manager := NewCardSessionManager(nil)

	// Create a session via the manager
	session, err := manager.CreateSession("user1", "channel1", "tmux", "call_1",
		CardTypeConfirm, &CardContent{
			Title:       "Test Card",
			Description: "This is a test card",
			Question:    "Are you sure?",
		}, map[string]interface{}{"command": "rm -rf /"})

	require.NoError(t, err)
	sessionID := session.ID

	// Test setting answer
	answers := []string{"yes"}
	err = manager.SetAnswer(sessionID, answers)
	require.NoError(t, err)

	// Verify session was updated
	assert.Equal(t, CardStateAnswered, session.State)
	assert.Equal(t, answers, session.UserAnswers)
	assert.WithinDuration(t, time.Now(), session.AnsweredAt, time.Second)
	assert.WithinDuration(t, time.Now(), session.UpdatedAt, time.Second)

	// Test setting answer on already answered session
	err = manager.SetAnswer(sessionID, []string{"no"})
	assert.Error(t, err)
	assert.Equal(t, ErrSessionAlreadyAnswered, err)

	// Test setting answer on expired session
	expiredSession, err := manager.CreateSession("user1", "channel1", "tmux", "call_2",
		CardTypeConfirm, &CardContent{
			Title:       "Expired Card",
			Description: "This card will expire",
			Question:    "Continue?",
		}, map[string]interface{}{})

	require.NoError(t, err)

	// Manually expire the session
	expiredSession.ExpiresAt = time.Now().Add(-1 * time.Minute)

	// Try to set answer on expired session
	err = manager.SetAnswer(expiredSession.ID, []string{"yes"})
	assert.Error(t, err)
	assert.Equal(t, ErrSessionExpired, err)

	// Test setting answer on non-existent session
	err = manager.SetAnswer("non_existent", []string{"yes"})
	assert.Error(t, err)
	assert.Equal(t, ErrSessionNotFound, err)
}

func TestCardSessionManager_DuplicateSession(t *testing.T) {
	manager := NewCardSessionManager(nil)

	userID := "user1"
	channelID := "channel1"
	toolName := "tmux"
	toolCallID := "call_1"

	// Create first session
	session1, err := manager.CreateSession(userID, channelID, toolName, toolCallID,
		CardTypeConfirm, &CardContent{
			Title:       "First Card",
			Description: "This is the first card",
			Question:    "Continue?",
		}, map[string]interface{}{"command": "ls"})

	require.NoError(t, err)
	require.NotNil(t, session1)

	// Try to create duplicate session with same user, channel, tool, and toolCallID
	session2, err := manager.CreateSession(userID, channelID, toolName, toolCallID,
		CardTypeConfirm, &CardContent{
			Title:       "Second Card",
			Description: "This is the second card",
			Question:    "Continue?",
		}, map[string]interface{}{"command": "pwd"})

	assert.Error(t, err)
	assert.Equal(t, ErrDuplicateSession, err)
	assert.Nil(t, session2)

	// Verify that creating a session with different toolCallID works
	session3, err := manager.CreateSession(userID, channelID, toolName, "call_2",
		CardTypeConfirm, &CardContent{
			Title:       "Third Card",
			Description: "This is the third card",
			Question:    "Continue?",
		}, map[string]interface{}{"command": "whoami"})

	require.NoError(t, err)
	require.NotNil(t, session3)
	assert.NotEqual(t, session1.ID, session3.ID)
}

func TestCardSessionManager_Stop(t *testing.T) {
	manager := NewCardSessionManager(nil)

	// Create a session
	session, err := manager.CreateSession("user1", "channel1", "tmux", "call_1",
		CardTypeConfirm, &CardContent{
			Title:       "Test Card",
			Description: "This is a test card",
			Question:    "Are you sure?",
		}, map[string]interface{}{})

	require.NoError(t, err)
	require.NotNil(t, session)

	// Stop the manager
	manager.Stop()

	// Verify that the manager is stopped (no panic should occur)
	// The stopChan should be closed, which will cause the cleanup routine to exit
}

func TestCardSessionManager_GetUserSession(t *testing.T) {
	manager := NewCardSessionManager(nil)

	userID := "user1"
	channelID := "channel1"
	toolName := "tmux"
	toolCallID := "call_1"

	// Create a session
	session, err := manager.CreateSession(userID, channelID, toolName, toolCallID,
		CardTypeConfirm, &CardContent{
			Title:       "Test Card",
			Description: "This is a test card",
			Question:    "Are you sure?",
		}, map[string]interface{}{"command": "ls"})

	require.NoError(t, err)
	require.NotNil(t, session)

	// Test getting user's session with full parameters
	userSession, err := manager.GetUserSession(userID, channelID, toolName, toolCallID)
	require.NoError(t, err)
	assert.NotNil(t, userSession)
	assert.Equal(t, session.ID, userSession.ID)
	assert.Equal(t, userID, userSession.UserID)
	assert.Equal(t, channelID, userSession.ChannelID)
	assert.Equal(t, toolName, userSession.ToolName)
	assert.Equal(t, toolCallID, userSession.ToolCallID)

	// Test getting session for user with no active session
	_, err = manager.GetUserSession("user_without_session", "channel1", "tmux", "call_1")
	assert.Error(t, err)
	assert.Equal(t, ErrSessionNotFound, err)

	// Test getting session with wrong toolCallID
	_, err = manager.GetUserSession(userID, channelID, toolName, "wrong_call_id")
	assert.Error(t, err)
	assert.Equal(t, ErrSessionNotFound, err)

	// Test getting session for user with expired session
	expiredSession, err := manager.CreateSession("user2", "channel1", "tmux", "call_2",
		CardTypeConfirm, &CardContent{
			Title:       "Expired Card",
			Description: "This card will expire",
			Question:    "Continue?",
		}, map[string]interface{}{})

	require.NoError(t, err)

	// Manually expire the session
	expiredSession.ExpiresAt = time.Now().Add(-1 * time.Minute)

	// Try to get user's expired session
	_, err = manager.GetUserSession("user2", "channel1", "tmux", "call_2")
	assert.Error(t, err)
	assert.Equal(t, ErrSessionExpired, err)
}

func TestCardSessionManager_CompleteSession(t *testing.T) {
	manager := NewCardSessionManager(nil)

	userID := "user1"
	channelID := "channel1"
	toolName := "tmux"
	toolCallID := "call_1"

	// Create a session
	session, err := manager.CreateSession(userID, channelID, toolName, toolCallID,
		CardTypeConfirm, &CardContent{
			Title:       "Test Card",
			Description: "This is a test card",
			Question:    "Are you sure?",
		}, map[string]interface{}{"command": "ls"})

	require.NoError(t, err)
	require.NotNil(t, session)

	// Test completing session
	err = manager.CompleteSession(session.ID)
	require.NoError(t, err)

	// Verify session was marked as completed
	assert.Equal(t, CardStateCompleted, session.State)
	assert.WithinDuration(t, time.Now(), session.UpdatedAt, time.Second)

	// Verify session is removed from user index
	_, err = manager.GetUserSession(userID, channelID, toolName, toolCallID)
	assert.Error(t, err)
	assert.Equal(t, ErrSessionNotFound, err)

	// Test completing non-existent session
	err = manager.CompleteSession("non_existent")
	assert.Error(t, err)
	assert.Equal(t, ErrSessionNotFound, err)
}

func TestCardSessionManager_CancelSession(t *testing.T) {
	manager := NewCardSessionManager(nil)

	userID := "user1"
	channelID := "channel1"
	toolName := "tmux"
	toolCallID := "call_1"

	// Create a session
	session, err := manager.CreateSession(userID, channelID, toolName, toolCallID,
		CardTypeConfirm, &CardContent{
			Title:       "Test Card",
			Description: "This is a test card",
			Question:    "Are you sure?",
		}, map[string]interface{}{"command": "ls"})

	require.NoError(t, err)
	require.NotNil(t, session)

	// Test cancelling session
	err = manager.CancelSession(session.ID)
	require.NoError(t, err)

	// Verify session was marked as cancelled
	assert.Equal(t, CardStateCancelled, session.State)
	assert.WithinDuration(t, time.Now(), session.UpdatedAt, time.Second)

	// Verify session is removed from user index
	_, err = manager.GetUserSession(userID, channelID, toolName, toolCallID)
	assert.Error(t, err)
	assert.Equal(t, ErrSessionNotFound, err)

	// Test cancelling non-existent session
	err = manager.CancelSession("non_existent")
	assert.Error(t, err)
	assert.Equal(t, ErrSessionNotFound, err)
}

func TestCardSessionManager_HasActiveSession(t *testing.T) {
	manager := NewCardSessionManager(nil)

	userID := "user1"
	channelID := "channel1"
	toolName := "tmux"
	toolCallID := "call_1"

	// Test with no sessions
	assert.False(t, manager.HasActiveSession(userID, channelID, toolName, toolCallID))

	// Create a session
	_, err := manager.CreateSession(userID, channelID, toolName, toolCallID,
		CardTypeConfirm, &CardContent{
			Title:       "Test Card",
			Description: "This is a test card",
			Question:    "Are you sure?",
		}, map[string]interface{}{"command": "ls"})

	require.NoError(t, err)

	// Test with active session
	assert.True(t, manager.HasActiveSession(userID, channelID, toolName, toolCallID))

	// Test with answered session
	session, err := manager.GetUserSession(userID, channelID, toolName, toolCallID)
	require.NoError(t, err)
	err = manager.SetAnswer(session.ID, []string{"yes"})
	require.NoError(t, err)

	// Answered session should not be considered active
	assert.False(t, manager.HasActiveSession(userID, channelID, toolName, toolCallID))

	// Create a new session for testing expired session
	expiredSession, err := manager.CreateSession(userID, channelID, toolName, "call_2",
		CardTypeConfirm, &CardContent{
			Title:       "Expired Card",
			Description: "This card will expire",
			Question:    "Continue?",
		}, map[string]interface{}{})

	require.NoError(t, err)

	// Manually expire the session
	expiredSession.ExpiresAt = time.Now().Add(-1 * time.Minute)

	// Expired session should not be considered active
	assert.False(t, manager.HasActiveSession(userID, channelID, toolName, "call_2"))

	// Test with user without session
	assert.False(t, manager.HasActiveSession("user_without_session", "channel1", "tmux", "call_1"))

	// Test with different channelID
	assert.False(t, manager.HasActiveSession(userID, "different_channel", toolName, toolCallID))
}
