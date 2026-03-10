package agent

import (
	"errors"
	"sync"
	"time"

	log "github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/konglong87/wukongbot/internal/session"
)

var (
	// ErrSessionNotFound is returned when a session is not found
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired is returned when a session has expired
	ErrSessionExpired = errors.New("session expired")
	// ErrSessionAlreadyAnswered is returned when a session is already answered
	ErrSessionAlreadyAnswered = errors.New("session already answered")
	// ErrDuplicateSession is returned when a session already exists for the user
	ErrDuplicateSession = errors.New("session already exists for this user, channel, and tool")
)

// CardSessionManager manages the lifecycle of card sessions
type CardSessionManager struct {
	sessions        map[string]*CardSession
	userIndex       map[string]string // userID:channelID:toolName:toolCallID -> sessionID
	mu              sync.RWMutex
	defaultTimeout  time.Duration
	cleanupInterval time.Duration
	storage         session.StorageInterface
	logger          *log.Logger
	stopChan        chan struct{}
}

// NewCardSessionManager creates a new CardSessionManager
func NewCardSessionManager(storage session.StorageInterface) *CardSessionManager {
	mgr := &CardSessionManager{
		sessions:        make(map[string]*CardSession),
		userIndex:       make(map[string]string),
		defaultTimeout:  5 * time.Minute,
		cleanupInterval: 1 * time.Minute,
		storage:         storage,
		logger:          log.Default(),
		stopChan:        make(chan struct{}),
	}

	mgr.logger.Info("[CardSessionManager] NewCardSessionManager Entry")
	defer mgr.logger.Info("[CardSessionManager] NewCardSessionManager Exit")

	// Start cleanup routine
	go mgr.startCleanupRoutine()

	return mgr
}

// CreateSession creates a new card session
func (m *CardSessionManager) CreateSession(
	userID, channelID, toolName, toolCallID string,
	cardType CardType,
	cardContent *CardContent,
	toolParams map[string]interface{},
) (*CardSession, error) {
	m.logger.Info("[CardSessionManager] CreateSession Entry",
		"user_id", userID,
		"channel_id", channelID,
		"tool_name", toolName,
		"tool_call_id", toolCallID)
	defer m.logger.Info("[CardSessionManager] CreateSession Exit",
		"user_id", userID,
		"channel_id", channelID,
		"tool_name", toolName,
		"tool_call_id", toolCallID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if session already exists for this user, channel, and tool
	indexKey := userID + ":" + channelID + ":" + toolName + ":" + toolCallID
	if _, exists := m.userIndex[indexKey]; exists {
		m.logger.Error("[CardSessionManager] CreateSession duplicate session",
			"user_id", userID,
			"channel_id", channelID,
			"tool_name", toolName,
			"tool_call_id", toolCallID)
		return nil, ErrDuplicateSession
	}

	// Generate session ID
	sessionID := uuid.New().String()

	now := time.Now()

	// Create session
	session := &CardSession{
		ID:          sessionID,
		UserID:      userID,
		ChannelID:   channelID,
		ToolName:    toolName,
		ToolCallID:  toolCallID,
		State:       CardStatePending,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(m.defaultTimeout),
		CardType:    cardType,
		CardContent: cardContent,
		ToolParams:  toolParams,
		Metadata:    make(map[string]string),
	}

	// Store session in memory
	m.sessions[sessionID] = session

	// Update user index
	m.userIndex[indexKey] = sessionID

	// TODO: Implement storage for card sessions when session.StorageInterface is extended
	// if err := m.storage.SaveCardSession(session); err != nil {
	// 	// Log error but don't fail the operation
	// 	m.logger.Error("[CardSessionManager] CreateSession failed to save session to storage",
	// 		"session_id", sessionID,
	// 		"error", err)
	// }

	m.logger.Info("[CardSessionManager] CreateSession created session",
		"session_id", sessionID,
		"user_id", userID,
		"channel_id", channelID,
		"tool_name", toolName,
		"tool_call_id", toolCallID)

	return session, nil
}

// GetSession retrieves a session by ID
func (m *CardSessionManager) GetSession(sessionID string) (*CardSession, error) {
	m.logger.Info("[CardSessionManager] GetSession Entry",
		"session_id", sessionID)
	defer m.logger.Info("[CardSessionManager] GetSession Exit",
		"session_id", sessionID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.logger.Warn("[CardSessionManager] GetSession session not found",
			"session_id", sessionID)
		return nil, ErrSessionNotFound
	}

	// Check if session has expired
	if session.IsExpired() {
		m.logger.Warn("[CardSessionManager] GetSession session expired",
			"session_id", sessionID,
			"expires_at", session.ExpiresAt)
		return nil, ErrSessionExpired
	}

	m.logger.Info("[CardSessionManager] GetSession retrieved session successfully",
		"session_id", sessionID)
	return session, nil
}

// SetAnswer sets user answers and transitions session state
func (m *CardSessionManager) SetAnswer(sessionID string, answers []string) error {
	m.logger.Info("[CardSessionManager] SetAnswer Entry",
		"session_id", sessionID,
		"answer_count", len(answers))
	defer m.logger.Info("[CardSessionManager] SetAnswer Exit",
		"session_id", sessionID)

	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.logger.Warn("[CardSessionManager] SetAnswer session not found",
			"session_id", sessionID)
		return ErrSessionNotFound
	}

	// Check if session has expired
	if session.IsExpired() {
		m.logger.Warn("[CardSessionManager] SetAnswer session expired",
			"session_id", sessionID,
			"expires_at", session.ExpiresAt)
		return ErrSessionExpired
	}

	// Check if session is already answered
	if session.State != CardStatePending {
		m.logger.Warn("[CardSessionManager] SetAnswer session already answered",
			"session_id", sessionID,
			"current_state", session.State)
		return ErrSessionAlreadyAnswered
	}

	// Set the answer (keep lock held to prevent race condition)
	session.UserAnswers = answers
	session.State = CardStateAnswered
	session.AnsweredAt = time.Now()
	session.UpdatedAt = time.Now()

	// TODO: Implement storage for card sessions when session.StorageInterface is extended
	// if err := m.storage.UpdateCardSession(session); err != nil {
	// 	m.logger.Error("[CardSessionManager] SetAnswer failed to update session in storage",
	// 		"session_id", sessionID,
	// 		"error", err)
	// }

	m.logger.Info("[CardSessionManager] SetAnswer session answered successfully",
		"session_id", sessionID,
		"answer_count", len(answers))

	return nil
}

// startCleanupRoutine starts the background cleanup routine
func (m *CardSessionManager) startCleanupRoutine() {
	m.logger.Info("[CardSessionManager] startCleanupRoutine Entry")
	defer m.logger.Info("[CardSessionManager] startCleanupRoutine Exit")

	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	m.logger.Info("[CardSessionManager] startCleanupRoutine started cleanup routine",
		"interval", m.cleanupInterval)

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredSessions()
		case <-m.stopChan:
			m.logger.Info("[CardSessionManager] startCleanupRoutine received stop signal")
			return
		}
	}
}

// Stop stops the CardSessionManager and its background cleanup routine
func (m *CardSessionManager) Stop() {
	m.logger.Info("[CardSessionManager] Stop Entry")
	defer m.logger.Info("[CardSessionManager] Stop Exit")

	close(m.stopChan)
	m.logger.Info("[CardSessionManager] Stop cleanup routine stopped")
}

// GetUserSession retrieves the user's active session for a specific channel, tool, and tool call
func (m *CardSessionManager) GetUserSession(userID, channelID, toolName, toolCallID string) (*CardSession, error) {
	m.logger.Info("[CardSessionManager] GetUserSession Entry",
		"user_id", userID,
		"channel_id", channelID,
		"tool_name", toolName,
		"tool_call_id", toolCallID)
	defer m.logger.Info("[CardSessionManager] GetUserSession Exit",
		"user_id", userID,
		"channel_id", channelID,
		"tool_name", toolName,
		"tool_call_id", toolCallID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build index key for direct lookup
	indexKey := userID + ":" + channelID + ":" + toolName + ":" + toolCallID
	sessionID, exists := m.userIndex[indexKey]
	if !exists {
		m.logger.Warn("[CardSessionManager] GetUserSession no session found for key",
			"user_id", userID,
			"channel_id", channelID,
			"tool_name", toolName,
			"tool_call_id", toolCallID)
		return nil, ErrSessionNotFound
	}

	session, exists := m.sessions[sessionID]
	if !exists {
		m.logger.Warn("[CardSessionManager] GetUserSession session not found in memory",
			"user_id", userID,
			"session_id", sessionID)
		return nil, ErrSessionNotFound
	}

	// Check if session has expired
	if session.IsExpired() {
		m.logger.Warn("[CardSessionManager] GetUserSession session expired",
			"user_id", userID,
			"session_id", sessionID,
			"expires_at", session.ExpiresAt)
		return nil, ErrSessionExpired
	}

	m.logger.Info("[CardSessionManager] GetUserSession retrieved session successfully",
		"user_id", userID,
		"session_id", sessionID)
	return session, nil
}

// CompleteSession marks a session as completed and removes it from both userIndex and sessions
func (m *CardSessionManager) CompleteSession(sessionID string) error {
	m.logger.Info("[CardSessionManager] CompleteSession Entry",
		"session_id", sessionID)
	defer m.logger.Info("[CardSessionManager] CompleteSession Exit",
		"session_id", sessionID)

	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.logger.Warn("[CardSessionManager] CompleteSession session not found",
			"session_id", sessionID)
		return ErrSessionNotFound
	}

	// Update session state
	session.State = CardStateCompleted
	session.UpdatedAt = time.Now()

	// Remove from user index
	indexKey := session.UserID + ":" + session.ChannelID + ":" +
		session.ToolName + ":" + session.ToolCallID
	delete(m.userIndex, indexKey)

	// Remove from sessions map to prevent memory leak
	delete(m.sessions, sessionID)

	// TODO: Implement storage update for card sessions when session.StorageInterface is extended
	// if err := m.storage.UpdateCardSession(session); err != nil {
	// 	m.logger.Error("[CardSessionManager] CompleteSession failed to update session in storage",
	// 		"session_id", sessionID,
	// 		"error", err)
	// }

	m.logger.Info("[CardSessionManager] CompleteSession completed session successfully",
		"session_id", sessionID,
		"user_id", session.UserID)

	return nil
}

// CancelSession cancels a session and removes it from both userIndex and sessions
func (m *CardSessionManager) CancelSession(sessionID string) error {
	m.logger.Info("[CardSessionManager] CancelSession Entry",
		"session_id", sessionID)
	defer m.logger.Info("[CardSessionManager] CancelSession Exit",
		"session_id", sessionID)

	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.logger.Warn("[CardSessionManager] CancelSession session not found",
			"session_id", sessionID)
		return ErrSessionNotFound
	}

	// Update session state
	session.State = CardStateCancelled
	session.UpdatedAt = time.Now()

	// Remove from user index
	indexKey := session.UserID + ":" + session.ChannelID + ":" +
		session.ToolName + ":" + session.ToolCallID
	delete(m.userIndex, indexKey)

	// Remove from sessions map to prevent memory leak
	delete(m.sessions, sessionID)

	// TODO: Implement storage update for card sessions when session.StorageInterface is extended
	// if err := m.storage.UpdateCardSession(session); err != nil {
	// 	m.logger.Error("[CardSessionManager] CancelSession failed to update session in storage",
	// 		"session_id", sessionID,
	// 		"error", err)
	// }

	m.logger.Info("[CardSessionManager] CancelSession cancelled session successfully",
		"session_id", sessionID,
		"user_id", session.UserID)

	return nil
}

// HasActiveSession checks if the user has an active pending session for a specific channel, tool, and tool call
func (m *CardSessionManager) HasActiveSession(userID, channelID, toolName, toolCallID string) bool {
	m.logger.Info("[CardSessionManager] HasActiveSession Entry",
		"user_id", userID,
		"channel_id", channelID,
		"tool_name", toolName,
		"tool_call_id", toolCallID)
	defer m.logger.Info("[CardSessionManager] HasActiveSession Exit",
		"user_id", userID,
		"channel_id", channelID,
		"tool_name", toolName,
		"tool_call_id", toolCallID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build index key for direct lookup
	indexKey := userID + ":" + channelID + ":" + toolName + ":" + toolCallID
	sessionID, exists := m.userIndex[indexKey]
	if !exists {
		m.logger.Debug("[CardSessionManager] HasActiveSession no session found for key",
			"user_id", userID,
			"channel_id", channelID,
			"tool_name", toolName,
			"tool_call_id", toolCallID)
		return false
	}

	session, exists := m.sessions[sessionID]
	if !exists {
		m.logger.Debug("[CardSessionManager] HasActiveSession session not found in memory",
			"user_id", userID,
			"session_id", sessionID)
		return false
	}

	// Check if session is in pending state and not expired
	isActive := session.State == CardStatePending && !session.IsExpired()
	if isActive {
		m.logger.Info("[CardSessionManager] HasActiveSession found active session",
			"user_id", userID,
			"session_id", sessionID)
	}

	return isActive
}

// cleanupExpiredSessions removes expired sessions from memory
func (m *CardSessionManager) cleanupExpiredSessions() {
	m.logger.Info("[CardSessionManager] cleanupExpiredSessions Entry")
	defer m.logger.Info("[CardSessionManager] cleanupExpiredSessions Exit")

	m.mu.Lock()
	defer m.mu.Unlock()

	expiredCount := 0

	// Find and remove expired sessions
	for sessionID, session := range m.sessions {
		if session.IsExpired() {
			delete(m.sessions, sessionID)

			// Remove from user index
			indexKey := session.UserID + ":" + session.ChannelID + ":" +
				session.ToolName + ":" + session.ToolCallID
			delete(m.userIndex, indexKey)

			expiredCount++
			m.logger.Debug("[CardSessionManager] cleanupExpiredSessions removed expired session",
				"session_id", sessionID,
				"user_id", session.UserID)
		}
	}

	if expiredCount > 0 {
		m.logger.Info("[CardSessionManager] cleanupExpiredSessions cleaned up sessions",
			"expired_count", expiredCount)
	}

	// TODO: Implement storage cleanup for card sessions when session.StorageInterface is extended
	// if err := m.storage.ExpireCardSessions(now); err != nil {
	// 	m.logger.Error("[CardSessionManager] cleanupExpiredSessions failed to expire sessions in storage",
	// 		"error", err)
	// }
}
