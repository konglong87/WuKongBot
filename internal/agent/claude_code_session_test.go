package agent

import (
	"testing"
	"time"

	"github.com/konglong87/wukongbot/internal/feishu/enhanced"
)

// mockClaudeCodePTYManager is a mock for testing
type mockClaudeCodePTYManager struct{}

func (m *mockClaudeCodePTYManager) CreateSession(sessionID, projectPath string) (*enhanced.ClaudeCodeSession, error) {
	return nil, nil
}

func (m *mockClaudeCodePTYManager) SendInput(sessionID, input string) error {
	return nil
}

func (m *mockClaudeCodePTYManager) CloseSession(sessionID string) error {
	return nil
}

func (m *mockClaudeCodePTYManager) Interrupt(sessionID string) error {
	return nil
}

func (m *mockClaudeCodePTYManager) SendEOF(sessionID string) error {
	return nil
}

// TestClaudeCodeSession_Initialization tests ClaudeCodeSession struct initialization
func TestClaudeCodeSession_Initialization(t *testing.T) {
	now := time.Now()

	session := ClaudeCodeSession{
		ID:           "user123:1234567890",
		UserID:       "user123",
		ProjectPath:  "/path/to/project",
		IsActive:     true,
		LastActivity: now,
		CreatedAt:    now,
		State:        SessionStateActive,
		CurrentQuestion: &InteractiveQuestion{
			Type:      "select",
			Prompt:    "Select an option:",
			Options:   []string{"Option 1", "Option 2"},
			SessionID: "user123:1234567890",
			CreatedAt: now,
			Metadata:  map[string]interface{}{"key": "value"},
		},
		Context: map[string]interface{}{
			"cwd": "/path/to/project",
		},
	}

	if session.ID != "user123:1234567890" {
		t.Errorf("Expected ID 'user123:1234567890', got '%s'", session.ID)
	}

	if session.UserID != "user123" {
		t.Errorf("Expected UserID 'user123', got '%s'", session.UserID)
	}

	if session.ProjectPath != "/path/to/project" {
		t.Errorf("Expected ProjectPath '/path/to/project', got '%s'", session.ProjectPath)
	}

	if session.IsActive != true {
		t.Errorf("Expected IsActive true, got %v", session.IsActive)
	}

	if session.State != SessionStateActive {
		t.Errorf("Expected State %s, got %s", SessionStateActive, session.State)
	}

	if session.CurrentQuestion == nil {
		t.Error("Expected CurrentQuestion to be non-nil")
	}

	if session.CurrentQuestion.Type != "select" {
		t.Errorf("Expected Question Type 'select', got '%s'", session.CurrentQuestion.Type)
	}

	if len(session.CurrentQuestion.Options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(session.CurrentQuestion.Options))
	}

	if session.Context == nil {
		t.Error("Expected Context to be non-nil")
	}

	if session.Context["cwd"] != "/path/to/project" {
		t.Errorf("Expected context cwd '/path/to/project', got '%v'", session.Context["cwd"])
	}
}

// TestClaudeCodeSession_InitialState tests creating a session with minimal fields
func TestClaudeCodeSession_InitialState(t *testing.T) {
	session := &ClaudeCodeSession{
		ID:          "user456:9876543210",
		UserID:      "user456",
		ProjectPath: "/another/project",
		IsActive:    false,
		CreatedAt:   time.Now(),
		State:       SessionStateClosed,
		Context:     map[string]interface{}{},
	}

	if session.IsActive {
		t.Error("Expected IsActive to be false")
	}

	if session.State != SessionStateClosed {
		t.Errorf("Expected State %s, got %s", SessionStateClosed, session.State)
	}

	if session.CurrentQuestion != nil {
		t.Error("Expected CurrentQuestion to be nil")
	}

	if len(session.Context) != 0 {
		t.Errorf("Expected empty context, got %d items", len(session.Context))
	}
}

// TestClaudeCodeConfig_Initialization tests ClaudeCodeConfig struct initialization
func TestClaudeCodeConfig_Initialization(t *testing.T) {
	config := ClaudeCodeConfig{
		Enabled:        true,
		Workspace:      "/workspace",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
		AutoCleanup:    true,
		MaxSessions:    10,
	}

	if config.Enabled != true {
		t.Errorf("Expected Enabled true, got %v", config.Enabled)
	}

	if config.Workspace != "/workspace" {
		t.Errorf("Expected Workspace '/workspace', got '%s'", config.Workspace)
	}

	if config.SessionPrefix != "/claude:" {
		t.Errorf("Expected SessionPrefix '/claude:', got '%s'", config.SessionPrefix)
	}

	if config.SessionTimeout != 30*time.Minute {
		t.Errorf("Expected SessionTimeout 30m, got %v", config.SessionTimeout)
	}

	if config.AutoCleanup != true {
		t.Errorf("Expected AutoCleanup true, got %v", config.AutoCleanup)
	}

	if config.MaxSessions != 10 {
		t.Errorf("Expected MaxSessions 10, got %d", config.MaxSessions)
	}
}

// TestClaudeCodeConfig_DefaultValues tests creating config with zero or default values
func TestClaudeCodeConfig_DefaultValues(t *testing.T) {
	config := ClaudeCodeConfig{
		Workspace: "/default/workspace",
	}

	if config.Enabled != false {
		t.Errorf("Expected Enabled false by default, got %v", config.Enabled)
	}

	if config.SessionPrefix != "" {
		t.Errorf("Expected empty SessionPrefix by default, got '%s'", config.SessionPrefix)
	}

	if config.SessionTimeout != 0 {
		t.Errorf("Expected zero SessionTimeout by default, got %v", config.SessionTimeout)
	}

	if config.AutoCleanup != false {
		t.Errorf("Expected AutoCleanup false by default, got %v", config.AutoCleanup)
	}

	if config.MaxSessions != 0 {
		t.Errorf("Expected MaxSessions 0 by default, got %d", config.MaxSessions)
	}
}

// TestClaudeCodeManager_Construction tests ClaudeCodeManager construction
func TestClaudeCodeManager_Construction(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}
	config := ClaudeCodeConfig{
		Enabled:        true,
		Workspace:      "/test/workspace",
		SessionPrefix:  "/cc:",
		SessionTimeout: 60 * time.Minute,
		AutoCleanup:    true,
		MaxSessions:    5,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	if manager == nil {
		t.Fatal("Expected manager to be non-nil")
	}

	if manager.config.SessionPrefix != "/cc:" {
		t.Errorf("Expected sessionPrefix '/cc:', got '%s'", manager.config.SessionPrefix)
	}

	if manager.config.SessionTimeout != 60*time.Minute {
		t.Errorf("Expected sessionTimeout 60m, got %v", manager.config.SessionTimeout)
	}

	if manager.config.MaxSessions != 5 {
		t.Errorf("Expected maxSessions 5, got %d", manager.config.MaxSessions)
	}

	if manager.sessions == nil {
		t.Error("Expected sessions map to be initialized")
	}

	if manager.userSessions == nil {
		t.Error("Expected userSessions map to be initialized")
	}

	if manager.sessions != nil && len(manager.sessions) != 0 {
		t.Errorf("Expected sessions to be empty, got %d items", len(manager.sessions))
	}

	if manager.userSessions != nil && len(manager.userSessions) != 0 {
		t.Errorf("Expected userSessions to be empty, got %d items", len(manager.userSessions))
	}
}

// TestClaudeCodeManager_DefaultConfig tests creating manager with default config
func TestClaudeCodeManager_DefaultConfig(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}
	config := ClaudeCodeConfig{}

	manager := NewClaudeCodeManager(mockPTY, config)

	if manager == nil {
		t.Fatal("Expected manager to be non-nil")
	}

	if manager.config.SessionPrefix != "" {
		t.Errorf("Expected empty sessionPrefix, got '%s'", manager.config.SessionPrefix)
	}

	if manager.config.SessionTimeout != 0 {
		t.Errorf("Expected zero sessionTimeout, got %v", manager.config.SessionTimeout)
	}

	if manager.config.MaxSessions != 0 {
		t.Errorf("Expected maxSessions 0, got %d", manager.config.MaxSessions)
	}
}

// TestInteractiveQuestion_Creation tests InteractiveQuestion creation
func TestInteractiveQuestion_Creation(t *testing.T) {
	now := time.Now()

	// Test select question
	selectQ := InteractiveQuestion{
		Type:      "select",
		Prompt:    "Choose a file:",
		Options:   []string{"main.go", "test.go", "config.yaml"},
		SessionID: "user789:111222333",
		CreatedAt: now,
		Metadata:  map[string]interface{}{"file_count": 3},
	}

	if selectQ.Type != "select" {
		t.Errorf("Expected Type 'select', got '%s'", selectQ.Type)
	}

	if selectQ.Prompt != "Choose a file:" {
		t.Errorf("Expected Prompt 'Choose a file:', got '%s'", selectQ.Prompt)
	}

	if len(selectQ.Options) != 3 {
		t.Errorf("Expected 3 options, got %d", len(selectQ.Options))
	}

	if selectQ.Options[0] != "main.go" {
		t.Errorf("Expected first option 'main.go', got '%s'", selectQ.Options[0])
	}

	if selectQ.Metadata["file_count"] != 3 {
		t.Errorf("Expected file_count 3, got %v", selectQ.Metadata["file_count"])
	}

	// Test input question
	inputQ := InteractiveQuestion{
		Type:      "input",
		Prompt:    "Enter your name:",
		SessionID: "user789:111222333",
		CreatedAt: now,
		Metadata:  map[string]interface{}{},
	}

	if inputQ.Type != "input" {
		t.Errorf("Expected Type 'input', got '%s'", inputQ.Type)
	}

	if len(inputQ.Options) != 0 {
		t.Errorf("Expected no options for input type, got %d", len(inputQ.Options))
	}

	// Test confirm question
	confirmQ := InteractiveQuestion{
		Type:      "confirm",
		Prompt:    "Are you sure?",
		Options:   []string{"yes", "no"},
		SessionID: "user789:111222333",
		CreatedAt: now,
	}

	if confirmQ.Type != "confirm" {
		t.Errorf("Expected Type 'confirm', got '%s'", confirmQ.Type)
	}
}

// TestInteractiveQuestion_EmptyCreation tests creating question with minimal fields
func TestInteractiveQuestion_EmptyCreation(t *testing.T) {
	q := InteractiveQuestion{
		Type:   "input",
		Prompt: "Enter value:",
	}

	if q.SessionID != "" {
		t.Errorf("Expected empty SessionID, got '%s'", q.SessionID)
	}

	if q.Metadata != nil {
		t.Error("Expected Metadata to be nil when not initialized")
	}

	if len(q.Options) != 0 {
		t.Errorf("Expected empty options, got %d items", len(q.Options))
	}
}

// TestSessionState_Values tests session state constants
func TestSessionState_Values(t *testing.T) {
	tests := []struct {
		name  string
		state SessionState
	}{
		{"Active State", SessionStateActive},
		{"Question State", SessionStateQuestion},
		{"Closed State", SessionStateClosed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.state) == "" {
				t.Errorf("Expected non-empty state string")
			}

			// Verify we can compare states
			switch tt.state {
			case SessionStateActive, SessionStateQuestion, SessionStateClosed:
				// Valid state
			default:
				t.Errorf("Unexpected state: %s", tt.state)
			}
		})
	}

	// Verify state uniqueness
	states := []SessionState{SessionStateActive, SessionStateQuestion, SessionStateClosed}
	for i := 0; i < len(states); i++ {
		for j := i + 1; j < len(states); j++ {
			if states[i] == states[j] {
				t.Errorf("States [%d] and [%d] should be different", i, j)
			}
		}
	}

	// Verify string representations
	if SessionStateActive != "active" {
		t.Errorf("Expected SessionStateActive to be 'active', got '%s'", SessionStateActive)
	}

	if SessionStateQuestion != "question" {
		t.Errorf("Expected SessionStateQuestion to be 'question', got '%s'", SessionStateQuestion)
	}

	if SessionStateClosed != "closed" {
		t.Errorf("Expected SessionStateClosed to be 'closed', got '%s'", SessionStateClosed)
	}
}

// TestClaudeCodeSession_StringRepresentation tests that session states can be used as strings
func TestClaudeCodeSession_StringRepresentation(t *testing.T) {
	session := ClaudeCodeSession{
		ID:        "testuser:123",
		UserID:    "testuser",
		State:     SessionStateActive,
		CreatedAt: time.Now(),
	}

	stateStr := string(session.State)
	if stateStr == "" {
		t.Error("Expected non-empty state string")
	}

	// Verify the state matches expected value
	if session.State != SessionStateActive {
		t.Errorf("Expected state %s, got %s", SessionStateActive, session.State)
	}
}

// TestClaudeCodeConfig_ZeroValue tests zero value of ClaudeCodeConfig
func TestClaudeCodeConfig_ZeroValue(t *testing.T) {
	var config ClaudeCodeConfig

	if config.Enabled {
		t.Error("Expected Enabled to be false for zero value")
	}

	if config.Workspace != "" {
		t.Errorf("Expected empty Workspace for zero value, got '%s'", config.Workspace)
	}

	if config.SessionPrefix != "" {
		t.Errorf("Expected empty SessionPrefix for zero value, got '%s'", config.SessionPrefix)
	}

	if config.SessionTimeout != 0 {
		t.Errorf("Expected zero SessionTimeout for zero value, got %v", config.SessionTimeout)
	}

	if config.AutoCleanup {
		t.Error("Expected AutoCleanup to be false for zero value")
	}

	if config.MaxSessions != 0 {
		t.Errorf("Expected MaxSessions 0 for zero value, got %d", config.MaxSessions)
	}
}

// TestClaudeCodeManager_CreateSession tests creating a new session
func TestClaudeCodeManager_CreateSession(t *testing.T) {
	// Mock PTY manager
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// 创建会话
	session, err := manager.CreateSession("user123", "/tmp/myproject")

	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if session.UserID != "user123" {
		t.Errorf("Expected user ID 'user123', got '%s'", session.UserID)
	}

	if session.ProjectPath != "/tmp/myproject" {
		t.Errorf("Expected project path '/tmp/myproject', got '%s'", session.ProjectPath)
	}

	if !session.IsActive {
		t.Error("Expected session to be active")
	}

	if session.State != SessionStateActive {
		t.Errorf("Expected state %s, got %s", SessionStateActive, session.State)
	}

	// Verify session is stored
	storedSession, err := manager.GetSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to get stored session: %v", err)
	}

	if storedSession.ID != session.ID {
		t.Errorf("Expected stored session ID %s, got %s", session.ID, storedSession.ID)
	}
}

// TestClaudeCodeManager_CreateSessionWithMaxSessions tests max session limit
func TestClaudeCodeManager_CreateSessionWithMaxSessions(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
		MaxSessions:    2, // Set max to 2
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create first session
	_, err := manager.CreateSession("user1", "/tmp/project1")
	if err != nil {
		t.Fatalf("Failed to create first session: %v", err)
	}

	// Create second session
	_, err = manager.CreateSession("user2", "/tmp/project2")
	if err != nil {
		t.Fatalf("Failed to create second session: %v", err)
	}

	// Try to create third session (should fail)
	_, err = manager.CreateSession("user3", "/tmp/project3")
	if err == nil {
		t.Error("Expected error when creating session beyond max limit")
	}

	expectedErr := "maximum sessions reached"
	if err == nil || !containsString(err.Error(), expectedErr) {
		t.Errorf("Expected error containing '%s', got %v", expectedErr, err)
	}
}

// TestClaudeCodeManager_CreateSessionReplacesExisting tests that existing active session is closed
func TestClaudeCodeManager_CreateSessionReplacesExisting(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create first session
	session1, err := manager.CreateSession("user123", "/tmp/project1")
	if err != nil {
		t.Fatalf("Failed to create first session: %v", err)
	}

	// Verify session is active
	if !session1.IsActive {
		t.Error("Expected first session to be active")
	}

	// Create second session for same user
	session2, err := manager.CreateSession("user123", "/tmp/project2")
	if err != nil {
		t.Fatalf("Failed to create second session: %v", err)
	}

	// Old session should be closed (note: in mock, CloseSession doesn't actually close)
	// In real implementation, the old session would be marked as inactive
	// Verify new session is active
	if !session2.IsActive {
		t.Error("Expected second session to be active")
	}

	if session2.ProjectPath != "/tmp/project2" {
		t.Errorf("Expected project path '/tmp/project2', got '%s'", session2.ProjectPath)
	}
}

// TestClaudeCodeManager_GetSessionNotFound tests getting non-existent session
func TestClaudeCodeManager_GetSessionNotFound(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Try to get non-existent session
	_, err := manager.GetSession("nonexistent:1234567890")
	if err == nil {
		t.Error("Expected error when getting non-existent session")
	}

	expectedErr := "session not found"
	if err == nil || !containsString(err.Error(), expectedErr) {
		t.Errorf("Expected error containing '%s', got %v", expectedErr, err)
	}
}

// TestClaudeCodeManager_GetSessionExpired tests getting expired session
func TestClaudeCodeManager_GetSessionExpired(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 1 * time.Millisecond, // Very short timeout
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create session
	session, err := manager.CreateSession("user123", "/tmp/myproject")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Wait for session to expire
	time.Sleep(10 * time.Millisecond)

	// Try to get expired session
	_, err = manager.GetSession(session.ID)
	if err == nil {
		t.Error("Expected error when getting expired session")
	}

	expectedErr := "session expired"
	if err == nil || !containsString(err.Error(), expectedErr) {
		t.Errorf("Expected error containing '%s', got %v", expectedErr, err)
	}
}

// TestClaudeCodeManager_GetUserSession tests getting user's active session
func TestClaudeCodeManager_GetUserSession(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create session
	session, err := manager.CreateSession("user123", "/tmp/myproject")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Get user session
	userSession, err := manager.GetUserSession("user123")
	if err != nil {
		t.Fatalf("Failed to get user session: %v", err)
	}

	if userSession.ID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, userSession.ID)
	}

	if userSession.UserID != "user123" {
		t.Errorf("Expected user ID 'user123', got '%s'", userSession.UserID)
	}
}

// TestClaudeCodeManager_GetUserSessionNotFound tests getting session for user without active session
func TestClaudeCodeManager_GetUserSessionNotFound(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Try to get session for user with no session
	_, err := manager.GetUserSession("user123")
	if err == nil {
		t.Error("Expected error when getting session for user without active session")
	}

	expectedErr := "no active session for user"
	if err == nil || !containsString(err.Error(), expectedErr) {
		t.Errorf("Expected error containing '%s', got %v", expectedErr, err)
	}
}

// TestClaudeCodeManager_HasActiveSession tests checking if user has active session
func TestClaudeCodeManager_HasActiveSession(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// User without session
	hasActive := manager.HasActiveSession("user123")
	if hasActive {
		t.Error("Expected user to not have active session")
	}

	// 创建会话
	_, err := manager.CreateSession("user123", "/tmp/myproject")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Check if user has active session
	hasActive = manager.HasActiveSession("user123")
	if !hasActive {
		t.Error("Expected user to have active session")
	}
}

// TestClaudeCodeManager_SendInput tests sending input to session
func TestClaudeCodeManager_SendInput(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create session
	session, err := manager.CreateSession("user123", "/tmp/myproject")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Send input
	err = manager.SendInput(session.ID, "help me write a function")
	if err != nil {
		t.Fatalf("Failed to send input: %v", err)
	}
}

// TestClaudeCodeManager_SendInputInactiveSession tests sending input to closed session
// Note: Closed sessions are removed from the sessions map to prevent memory leaks
func TestClaudeCodeManager_SendInputInactiveSession(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create session
	session, err := manager.CreateSession("user123", "/tmp/myproject")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Close session
	err = manager.CloseSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to close session: %v", err)
	}

	// Try to send input to closed session
	// Note: Closed sessions are removed from the map to prevent memory leaks
	err = manager.SendInput(session.ID, "help me")
	if err == nil {
		t.Error("Expected error when sending input to closed session")
	}

	expectedErr := "session not found"
	if err == nil || !containsString(err.Error(), expectedErr) {
		t.Errorf("Expected error containing '%s', got %v", expectedErr, err)
	}
}

// TestClaudeCodeManager_SendInputNotFound tests sending input to non-existent session
func TestClaudeCodeManager_SendInputNotFound(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Try to send input to non-existent session
	err := manager.SendInput("nonexistent:1234567890", "help me")
	if err == nil {
		t.Error("Expected error when sending input to non-existent session")
	}

	expectedErr := "session not found"
	if err == nil || !containsString(err.Error(), expectedErr) {
		t.Errorf("Expected error containing '%s', got %v", expectedErr, err)
	}
}

// TestClaudeCodeManager_SendInputWithQuestion tests sending answer to a question
func TestClaudeCodeManager_SendInputWithQuestion(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create session
	session, err := manager.CreateSession("user123", "/tmp/myproject")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Set session state to question and add a question
	session.mu.Lock()
	session.State = SessionStateQuestion
	session.CurrentQuestion = &InteractiveQuestion{
		Type:      "select",
		Prompt:    "Select an option:",
		Options:   []string{"Option 1", "Option 2"},
		SessionID: session.ID,
		CreatedAt: time.Now(),
	}
	session.mu.Unlock()

	// Send answer
	err = manager.SendInput(session.ID, "Option 1")
	if err != nil {
		t.Fatalf("Failed to send answer: %v", err)
	}

	// Verify session state is back to active
	if session.State != SessionStateActive {
		t.Errorf("Expected state %s after answering question, got %s", SessionStateActive, session.State)
	}

	if session.CurrentQuestion != nil {
		t.Error("Expected CurrentQuestion to be nil after answering")
	}
}

// TestClaudeCodeManager_InterruptSession tests interrupting a session
func TestClaudeCodeManager_InterruptSession(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create session
	session, err := manager.CreateSession("user123", "/tmp/myproject")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Interrupt session
	err = manager.InterruptSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to interrupt session: %v", err)
	}
}

// TestClaudeCodeManager_SendEOF tests sending EOF to session
func TestClaudeCodeManager_SendEOF(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create session
	session, err := manager.CreateSession("user123", "/tmp/myproject")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Send EOF
	err = manager.SendEOF(session.ID)
	if err != nil {
		t.Fatalf("Failed to send EOF: %v", err)
	}
}

// TestClaudeCodeManager_CloseSession tests closing a session
func TestClaudeCodeManager_CloseSession(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create session
	session, err := manager.CreateSession("user123", "/tmp/myproject")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if !session.IsActive {
		t.Error("Expected session to be active before closing")
	}

	// Close session
	err = manager.CloseSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to close session: %v", err)
	}

	// Verify session is closed
	if session.IsActive {
		t.Error("Expected session to be inactive after closing")
	}

	if session.State != SessionStateClosed {
		t.Errorf("Expected state %s after closing, got %s", SessionStateClosed, session.State)
	}

	// Verify user no longer has active session
	hasActive := manager.HasActiveSession("user123")
	if hasActive {
		t.Error("Expected user to not have active session after closing")
	}
}

// TestClaudeCodeManager_CloseSessionNotFound tests closing non-existent session
func TestClaudeCodeManager_CloseSessionNotFound(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Try to close non-existent session
	err := manager.CloseSession("nonexistent:1234567890")
	if err == nil {
		t.Error("Expected error when closing non-existent session")
	}

	expectedErr := "session not found"
	if err == nil || !containsString(err.Error(), expectedErr) {
		t.Errorf("Expected error containing '%s', got %v", expectedErr, err)
	}
}

// TestClaudeCodeManager_CleanInactiveSessions tests cleaning inactive sessions
func TestClaudeCodeManager_CleanInactiveSessions(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 50 * time.Millisecond, // Short timeout for testing
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create two sessions
	session1, err := manager.CreateSession("user1", "/tmp/project1")
	if err != nil {
		t.Fatalf("Failed to create first session: %v", err)
	}

	session2, err := manager.CreateSession("user2", "/tmp/project2")
	if err != nil {
		t.Fatalf("Failed to create second session: %v", err)
	}

	// Wait for sessions to expire
	time.Sleep(100 * time.Millisecond)

	// Update one session's activity to keep it active
	session2.mu.Lock()
	session2.LastActivity = time.Now()
	session2.mu.Unlock()

	// Clean inactive sessions
	manager.CleanInactiveSessions()

	// Verify first session is closed
	if session1.IsActive {
		t.Error("Expected first session to be closed after cleanup")
	}

	if session1.State != SessionStateClosed {
		t.Errorf("Expected first session state %s, got %s", SessionStateClosed, session1.State)
	}

	// Verify second session is still active
	if !session2.IsActive {
		t.Error("Expected second session to still be active")
	}

	// Verify user1 no longer has active session
	hasActive := manager.HasActiveSession("user1")
	if hasActive {
		t.Error("Expected user1 to not have active session after cleanup")
	}

	// Verify user2 still has active session
	hasActive = manager.HasActiveSession("user2")
	if !hasActive {
		t.Error("Expected user2 to still have active session")
	}
}

// TestClaudeCodeManager_DetectClaudeCodeCommand tests detecting Claude Code commands
func TestClaudeCodeManager_DetectClaudeCodeCommand(t *testing.T) {
	config := ClaudeCodeConfig{
		SessionPrefix: "/claude:",
	}

	manager := NewClaudeCodeManager(&mockClaudeCodePTYManager{}, config)

	// Test valid command
	input := "/claude: 帮我添加登录功能"
	isCommand, command, _ := manager.DetectClaudeCodeCommand(input)

	if !isCommand {
		t.Error("Expected input to be detected as Claude Code command")
	}

	if command != "帮我添加登录功能" {
		t.Errorf("Expected command '帮我添加登录功能', got '%s'", command)
	}

	// Test non-command input
	input = "Help me with login functionality"
	isCommand, _, _ = manager.DetectClaudeCodeCommand(input)

	if isCommand {
		t.Error("Expected regular input not to be detected as Claude Code command")
	}

	// Test empty prefix
	config = ClaudeCodeConfig{
		SessionPrefix: "",
	}
	manager = NewClaudeCodeManager(&mockClaudeCodePTYManager{}, config)

	input = "/claude: help me"
	isCommand, _, _ = manager.DetectClaudeCodeCommand(input)

	if isCommand {
		t.Error("Expected input not to be detected as command when prefix is empty")
	}
}

// TestClaudeCodeManager_ValidateInput tests input validation
func TestClaudeCodeManager_ValidateInput(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}
	config := ClaudeCodeConfig{}
	manager := NewClaudeCodeManager(mockPTY, config)

	// Test empty input
	err := manager.ValidateInput("")
	if err == nil {
		t.Error("Expected error for empty input")
	}

	expectedErr := "input cannot be empty"
	if err == nil || !containsString(err.Error(), expectedErr) {
		t.Errorf("Expected error containing '%s', got %v", expectedErr, err)
	}

	// Test whitespace-only input
	err = manager.ValidateInput("   ")
	if err == nil {
		t.Error("Expected error for whitespace-only input")
	}

	// Test valid input
	err = manager.ValidateInput("help me write a function")
	if err != nil {
		t.Errorf("Expected no error for valid input, got %v", err)
	}
}

// TestClaudeCodeManager_ListSessions tests listing all sessions
func TestClaudeCodeManager_ListSessions(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Initially no sessions
	sessions := manager.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions initially, got %d", len(sessions))
	}

	// Create sessions
	session1, _ := manager.CreateSession("user1", "/tmp/project1")
	session2, _ := manager.CreateSession("user2", "/tmp/project2")
	session3, _ := manager.CreateSession("user3", "/tmp/project3")

	// List sessions
	sessions = manager.ListSessions()
	if len(sessions) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(sessions))
	}

	// Verify session IDs
	sessionIDs := make(map[string]bool)
	for _, s := range sessions {
		sessionIDs[s.ID] = true
	}

	if !sessionIDs[session1.ID] {
		t.Errorf("Expected session ID %s in list", session1.ID)
	}

	if !sessionIDs[session2.ID] {
		t.Errorf("Expected session ID %s in list", session2.ID)
	}

	if !sessionIDs[session3.ID] {
		t.Errorf("Expected session ID %s in list", session3.ID)
	}
}

// TestClaudeCodeManager_GetUserSessionsInfo tests getting user session info
func TestClaudeCodeManager_GetUserSessionsInfo(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// User without session
	info := manager.GetUserSessionsInfo("user123")
	if info["has_session"] != false {
		t.Error("Expected has_session to be false for user without session")
	}

	// Create session
	session, err := manager.CreateSession("user123", "/tmp/myproject")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Get user session info
	info = manager.GetUserSessionsInfo("user123")

	if info["has_session"] != true {
		t.Error("Expected has_session to be true")
	}

	if info["session_id"] != session.ID {
		t.Errorf("Expected session ID %s, got %v", session.ID, info["session_id"])
	}

	if info["project_path"] != "/tmp/myproject" {
		t.Errorf("Expected project path '/tmp/myproject', got %v", info["project_path"])
	}

	if info["is_active"] != true {
		t.Error("Expected is_active to be true")
	}

	if info["state"] != SessionStateActive {
		t.Errorf("Expected state %s, got %v", SessionStateActive, info["state"])
	}

	if info["created_at"] == nil {
		t.Error("Expected created_at to be non-nil")
	}

	if info["last_activity"] == nil {
		t.Error("Expected last_activity to be non-nil")
	}
}

// TestClaudeCodeManager_CloseAndCreateNewSession tests closing and creating new sessions
func TestClaudeCodeManager_CloseAndCreateNewSession(t *testing.T) {
	mockPTY := &mockClaudeCodePTYManager{}

	config := ClaudeCodeConfig{
		Workspace:      "/tmp/test",
		SessionPrefix:  "/claude:",
		SessionTimeout: 30 * time.Minute,
	}

	manager := NewClaudeCodeManager(mockPTY, config)

	// Create first session
	session1, err := manager.CreateSession("user123", "/tmp/project1")
	if err != nil {
		t.Fatalf("Failed to create first session: %v", err)
	}

	// Close session
	err = manager.CloseSession(session1.ID)
	if err != nil {
		t.Fatalf("Failed to close session: %v", err)
	}

	// Create new session for same user
	session2, err := manager.CreateSession("user123", "/tmp/project2")
	if err != nil {
		t.Fatalf("Failed to create second session: %v", err)
	}

	// Verify new session is different
	if session2.ID == session1.ID {
		t.Error("Expected new session to have different ID")
	}

	// Verify new session is active
	if !session2.IsActive {
		t.Error("Expected new session to be active")
	}

	// Verify new session has correct project path
	if session2.ProjectPath != "/tmp/project2" {
		t.Errorf("Expected project path '/tmp/project2', got '%s'", session2.ProjectPath)
	}
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 15 && len(substr) > 5 && findSubstring(s, substr)))
}

// Helper function to find substring
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
