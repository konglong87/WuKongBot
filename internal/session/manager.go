package session

import (
	"context"
	"fmt"

	"github.com/konglong87/wukongbot/internal/config"
	"github.com/konglong87/wukongbot/internal/providers"
)

// Manager manages chat sessions
type Manager struct {
	storage    StorageInterface
	providers  map[string]providers.LLMProvider
	defaultLLM providers.LLMProvider
}

// ManagerConfig configures the session manager
type ManagerConfig struct {
	Config     interface{} // Can be *config.Config or string for backward compatibility
	DefaultLLM providers.LLMProvider
}

// NewManager creates a new session manager
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	var storage StorageInterface
	var err error

	// Check if Config is a string (old DBPath) or a config.Config object
	switch v := cfg.Config.(type) {
	case string:
		// Backward compatibility: treat string as DBPath for SQLite
		storage, err = NewSQLiteStorage(v)
	case *config.Config:
		// New approach: use full config object
		storage, err = NewStorage(v)
	default:
		return nil, fmt.Errorf("invalid config type: expected string or *config.Config")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	return &Manager{
		storage:    storage,
		providers:  make(map[string]providers.LLMProvider),
		defaultLLM: cfg.DefaultLLM,
	}, nil
}

// RegisterProvider registers an LLM provider for a specific session key
func (m *Manager) RegisterProvider(sessionKey string, provider providers.LLMProvider) {
	m.providers[sessionKey] = provider
}

// GetProvider gets the LLM provider for a session
func (m *Manager) GetProvider(sessionKey string) providers.LLMProvider {
	if provider, exists := m.providers[sessionKey]; exists {
		return provider
	}
	return m.defaultLLM
}

// GetOrCreateSession gets or creates a session by key
func (m *Manager) GetOrCreateSession(ctx context.Context, sessionKey string) (*Session, error) {
	return m.storage.GetOrCreateSession(sessionKey)
}

// AddMessage adds a message to a session
func (m *Manager) AddMessage(ctx context.Context, sessionID int64, role, content string) error {
	return m.storage.AddMessage(sessionID, role, content)
}

// AddMessageByKey adds a message to a session by session key
func (m *Manager) AddMessageByKey(sessionKey, role, content string) error {
	session, err := m.storage.GetOrCreateSession(sessionKey)
	if err != nil {
		return err
	}
	return m.storage.AddMessage(session.ID, role, content)
}

// GetMessages retrieves messages for a session
func (m *Manager) GetMessages(ctx context.Context, sessionID int64, limit int) ([]Message, error) {
	return m.storage.GetMessages(sessionID, limit)
}

// GetLastNMessages retrieves the last N messages from a session
func (m *Manager) GetLastNMessages(ctx context.Context, sessionID int64, n int) ([]Message, error) {
	return m.storage.GetLastNMessages(sessionID, n)
}

// GetSessionMessagesAsLLMMessages converts session messages to LLM message format
func (m *Manager) GetSessionMessagesAsLLMMessages(ctx context.Context, sessionKey string, limit int) ([]providers.Message, error) {
	session, err := m.storage.GetOrCreateSession(sessionKey)
	if err != nil {
		return nil, err
	}

	messages, err := m.storage.GetMessages(session.ID, limit)
	if err != nil {
		return nil, err
	}

	llmMessages := make([]providers.Message, len(messages))
	for i, msg := range messages {
		llmMessages[i] = providers.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	return llmMessages, nil
}

// GetSessionMessagesWithTimestamp returns session messages with timestamps
func (m *Manager) GetSessionMessagesWithTimestamp(ctx context.Context, sessionKey string, limit int) ([]Message, error) {
	session, err := m.storage.GetOrCreateSession(sessionKey)
	if err != nil {
		return nil, err
	}

	return m.storage.GetMessages(session.ID, limit)
}

// DeleteSession deletes a session and all its messages
func (m *Manager) DeleteSession(ctx context.Context, sessionID int64) error {
	return m.storage.DeleteSession(sessionID)
}

// DeleteSessionByKey deletes a session by key
func (m *Manager) DeleteSessionByKey(ctx context.Context, sessionKey string) error {
	session, err := m.storage.GetOrCreateSession(sessionKey)
	if err != nil {
		return err
	}
	return m.storage.DeleteSession(session.ID)
}

// GetConfig retrieves a config value
func (m *Manager) GetConfig(key string) (string, error) {
	return m.storage.GetConfig(key)
}

// SetConfig stores a config value
func (m *Manager) SetConfig(key, value string) error {
	return m.storage.SetConfig(key, value)
}

// SaveCronJob saves or updates a cron job
func (m *Manager) SaveCronJob(job *CronJob) error {
	return m.storage.SaveCronJob(job)
}

// GetCronJob retrieves a cron job by ID
func (m *Manager) GetCronJob(jobID string) (*CronJob, error) {
	return m.storage.GetCronJob(jobID)
}

// GetAllCronJobs retrieves all cron jobs
func (m *Manager) GetAllCronJobs() ([]*CronJob, error) {
	return m.storage.GetAllCronJobs()
}

// DeleteCronJob deletes a cron job
func (m *Manager) DeleteCronJob(jobID string) error {
	return m.storage.DeleteCronJob(jobID)
}

// Close closes the session manager and releases resources
func (m *Manager) Close() error {
	return m.storage.Close()
}

// GetStorage returns the underlying storage for direct access
func (m *Manager) GetStorage() StorageInterface {
	return m.storage
}
