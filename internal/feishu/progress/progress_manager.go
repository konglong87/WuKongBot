package progress

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// MessageSender interface for sending progress messages (avoiding circular import)
type MessageSender interface {
	SendMessage(channelID, recipientID, content string) error
	Name() string
}

// ProgressLevel represents the level of progress message
type ProgressLevel string

const (
	ProgressStart     ProgressLevel = "start"
	ProgressProgress  ProgressLevel = "progress"
	ProgressCompleted ProgressLevel = "completed"
	ProgressFailed    ProgressLevel = "failed"
)

// ProgressMessage represents a progress message
type ProgressMessage struct {
	Level   ProgressLevel
	Tool    string
	Message string
	Detail  string
	Error   error
}

// ProgressStep represents a single step in a progress session
type ProgressStep struct {
	StepID    string
	ToolName  string
	Status    string // "pending", "running", "completed", "failed"
	Message   string
	Timestamp time.Time
	Result    interface{}
}

// ProgressSession represents a progress tracking session
type ProgressSession struct {
	SessionID    string
	TaskID       string
	StartTime    time.Time
	LastActivity time.Time
	Steps        []ProgressStep
	CurrentStep  int
	TotalSteps   int
	IsCompleted  bool
	mu           sync.RWMutex
}

// ProgressManager manages progress sessions
type ProgressManager struct {
	sessions     map[string]*ProgressSession
	channel      MessageSender
	messageDelay time.Duration
	showDetails  bool
	showSummary  bool
	mu           sync.RWMutex
	logger       *log.Logger
}

// NewProgressManager creates a new progress manager
func NewProgressManager(channel MessageSender) *ProgressManager {
	return &ProgressManager{
		sessions:     make(map[string]*ProgressSession),
		channel:      channel,
		messageDelay: 500 * time.Millisecond,
		showDetails:  true,
		showSummary:  true,
		logger:       log.Default(),
	}
}

// SetMessageDelay sets the delay between progress messages
func (pm *ProgressManager) SetMessageDelay(delay time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.messageDelay = delay
}

// SetShowDetails sets whether to show detailed progress
func (pm *ProgressManager) SetShowDetails(show bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.showDetails = show
}

// SetShowSummary sets whether to show summary at the end
func (pm *ProgressManager) SetShowSummary(show bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.showSummary = show
}

// CreateProgressSession creates a new progress session
func (pm *ProgressManager) CreateProgressSession(sessionID string) *ProgressSession {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	session := &ProgressSession{
		SessionID:    sessionID,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		Steps:        make([]ProgressStep, 0),
		CurrentStep:  0,
		TotalSteps:   0,
		IsCompleted:  false,
	}

	pm.sessions[sessionID] = session
	pm.logger.Debug("progress", "created session", sessionID)

	return session
}

// StartTask starts a new task with expected number of steps
func (pm *ProgressManager) StartTask(sessionID, taskID string, totalSteps int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	session, ok := pm.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.TaskID = taskID
	session.TotalSteps = totalSteps
	session.LastActivity = time.Now()

	pm.logger.Debug("progress", "started task", taskID, "total_steps", totalSteps, "session", sessionID)

	return nil
}

// UpdateProgress updates the progress of a specific step
func (pm *ProgressManager) UpdateProgress(sessionID, stepID, toolName, status, message string, result interface{}) error {
	pm.mu.RLock()
	session, ok := pm.sessions[sessionID]
	pm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// Check if step already exists
	var step *ProgressStep
	for i := range session.Steps {
		if session.Steps[i].StepID == stepID {
			step = &session.Steps[i]
			break
		}
	}

	// Create new step if doesn't exist
	if step == nil {
		if status == "pending" {
			session.Steps = append(session.Steps, ProgressStep{
				StepID:    stepID,
				ToolName:  toolName,
				Status:    "pending",
				Timestamp: time.Now(),
			})
			session.TotalSteps = len(session.Steps)
		}
		return nil
	}

	// Update step
	step.Status = status
	step.Message = message
	step.Result = result
	step.Timestamp = time.Now()
	session.LastActivity = time.Now()

	pm.logger.Debug("progress", "updated step", stepID, "status", status, "tool", toolName, "session", sessionID)

	return nil
}

// SendProgressMessage sends a progress message to the user
func (pm *ProgressManager) SendProgressMessage(sessionID string, msg ProgressMessage) error {
	pm.mu.RLock()
	session, ok := pm.sessions[sessionID]
	pm.mu.RUnlock()

	if !ok {
		// Create session if it doesn't exist
		session = pm.CreateProgressSession(sessionID)
	}

	// Format message based on level
	content := pm.formatProgressMessage(msg, session)

	// Send to channel
	err := pm.channel.SendMessage(pm.channel.Name(), sessionID, content)
	if err != nil {
		pm.logger.Error("progress", "failed to send message", "error", err, "session", sessionID)
		return err
	}

	// Add delay if configured
	if pm.messageDelay > 0 {
		time.Sleep(pm.messageDelay)
	}

	return nil
}

// formatProgressMessage formats a progress message
func (pm *ProgressManager) formatProgressMessage(msg ProgressMessage, session *ProgressSession) string {
	switch msg.Level {
	case ProgressStart:
		return fmt.Sprintf("🔄 开始执行: %s，详情：%v，msg=%v", msg.Tool, msg.Detail, msg.Message)
	case ProgressProgress:
		if pm.showDetails && msg.Detail != "" {
			return fmt.Sprintf("   %s: %s", msg.Tool, msg.Detail)
		}
		return fmt.Sprintf("⏳ 执行中: %s", msg.Tool)
	case ProgressCompleted:
		if pm.showDetails && msg.Detail != "" {
			return fmt.Sprintf("✅ %s 完成\n   结果: %s", msg.Tool, msg.Detail)
		}
		return fmt.Sprintf("✅ 完成: %s", msg.Tool)
	case ProgressFailed:
		errorMsg := ""
		if msg.Error != nil {
			errorMsg = fmt.Sprintf("\n   错误: %v", msg.Error)
		}
		return fmt.Sprintf("❌ 失败: %s%s", msg.Tool, errorMsg)
	default:
		return msg.Message
	}
}

// CompleteTask marks a task as completed and optionally sends summary
func (pm *ProgressManager) CompleteTask(sessionID, taskID, summary string) error {
	pm.mu.RLock()
	session, ok := pm.sessions[sessionID]
	pm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	session.IsCompleted = true
	session.LastActivity = time.Now()

	// Generate summary if enabled
	if pm.showSummary {
		session.mu.Unlock()
		pm.sendTaskSummary(session, summary)
		session.mu.Lock()
	}

	session.mu.Unlock()

	pm.logger.Debug("progress", "completed task", taskID, "session", sessionID)

	return nil
}

// sendTaskSummary sends a summary of the completed task
func (pm *ProgressManager) sendTaskSummary(session *ProgressSession, additionalSummary string) {
	var completedCount, failedCount int

	for _, step := range session.Steps {
		if step.Status == "completed" {
			completedCount++
		} else if step.Status == "failed" {
			failedCount++
		}
	}

	summary := "📊 **任务完成**\n\n"
	summary += fmt.Sprintf("• 总步骤: %d\n", len(session.Steps))
	summary += fmt.Sprintf("• 成功: %d ✅\n", completedCount)
	summary += fmt.Sprintf("• 失败: %d ❌\n", failedCount)
	summary += fmt.Sprintf("• 耗时: %s\n", time.Since(session.StartTime).Round(time.Millisecond))

	if additionalSummary != "" {
		summary += fmt.Sprintf("\n%s", additionalSummary)
	}

	pm.channel.SendMessage(pm.channel.Name(), session.SessionID, summary)
}

// GetProgressSession retrieves a progress session
func (pm *ProgressManager) GetProgressSession(sessionID string) (*ProgressSession, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	session, ok := pm.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// DeleteSession deletes a progress session
func (pm *ProgressManager) DeleteSession(sessionID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.sessions, sessionID)
	pm.logger.Debug("progress", "deleted session", sessionID)
}

// StartCleanupRoutine starts a background routine to clean up expired sessions
func (pm *ProgressManager) StartCleanupRoutine(ctx context.Context, interval time.Duration, maxAge time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.cleanupExpiredSessions(maxAge)
		}
	}
}

// cleanupExpiredSessions removes sessions that haven't been active for a while
func (pm *ProgressManager) cleanupExpiredSessions(maxAge time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	for sessionID, session := range pm.sessions {
		session.mu.RLock()
		age := now.Sub(session.LastActivity)
		session.mu.RUnlock()

		if age > maxAge {
			delete(pm.sessions, sessionID)
			pm.logger.Debug("progress", "cleaned up expired session", sessionID, "age", age)
		}
	}
}
