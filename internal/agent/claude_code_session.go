// Package agent provides Claude Code session management infrastructure for
// integrating with PTY-based command-line interfaces.
package agent

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/konglong87/wukongbot/internal/feishu/enhanced"
)

// SessionState 会话状态
type SessionState string

const (
	SessionStateActive   SessionState = "active"
	SessionStateQuestion SessionState = "question"
	SessionStateClosed   SessionState = "closed"
)

// InteractiveQuestion 交互式问题
type InteractiveQuestion struct {
	Type      string                 // select, input, confirm
	Prompt    string                 // 问题提示
	Options   []string               // 选项（仅选择题）
	SessionID string                 // 关联的会话 ID
	CreatedAt time.Time              // 创建时间
	Metadata  map[string]interface{} // 额外元数据
}

// ClaudeCodeConfig Claude Code 配置
type ClaudeCodeConfig struct {
	Enabled        bool          // 是否启用
	Workspace      string        // 工作目录
	SessionPrefix  string        // 会话前缀，如 "/claude:"
	SessionTimeout time.Duration // 会话超时时间
	AutoCleanup    bool          // 自动清理不活跃会话
	MaxSessions    int           // 最大并发会话数
	ClaudeCommand  []string      // CLI命令，例如 ["claude", "code"] 或 ["opencode"]
}

// ClaudeCodeSession Claude Code 会话
type ClaudeCodeSession struct {
	ID              string                 // 会话 ID，格式: "userID:timestamp"
	UserID          string                 // 用户 ID
	ProjectPath     string                 // 项目路径
	IsActive        bool                   // 是否活跃
	LastActivity    time.Time              // 最后活动时间
	CreatedAt       time.Time              // 创建时间
	State           SessionState           // 会话状态
	CurrentQuestion *InteractiveQuestion   // 当前的交互式问题
	Context         map[string]interface{} // 会话上下文
	mu              sync.RWMutex           // 读写锁
}

// ClaudeCodeManagerInterface Claude Code 管理器接口
type ClaudeCodeManagerInterface interface {
	CreateSession(userID, projectPath string) (*ClaudeCodeSession, error)
	GetSession(sessionID string) (*ClaudeCodeSession, error)
	GetUserSession(userID string) (*ClaudeCodeSession, error)
	HasActiveSession(userID string) bool
	SendInput(sessionID, input string) error
	InterruptSession(sessionID string) error
	SendEOF(sessionID string) error
	CloseSession(sessionID string) error
	CleanInactiveSessions()
	DetectClaudeCodeCommand(input string) (bool, string, string)
	ValidateInput(input string) error
	ListSessions() []*ClaudeCodeSession
	GetUserSessionsInfo(userID string) map[string]interface{}
}

// ClaudeCodePTYManagerInterface PTY 管理器接口
type ClaudeCodePTYManagerInterface interface {
	CreateSession(sessionID, projectPath string) (*enhanced.ClaudeCodeSession, error)
	SendInput(sessionID, input string) error
	CloseSession(sessionID string) error
	Interrupt(sessionID string) error
	SendEOF(sessionID string) error
}

// ClaudeCodeManager Claude Code 会话管理器
type ClaudeCodeManager struct {
	ptyManager   ClaudeCodePTYManagerInterface // PTY 管理器
	config       ClaudeCodeConfig              // 配置
	sessions     map[string]*ClaudeCodeSession // 会话列表，key: sessionID
	userSessions map[string]string             // 用户当前会话，key: userID, value: sessionID
	mu           sync.RWMutex                  // 读写锁
	logger       *log.Logger                   // 日志记录器
}

// NewClaudeCodeManager 创建 Claude Code 管理器
func NewClaudeCodeManager(ptyManager ClaudeCodePTYManagerInterface, config ClaudeCodeConfig) *ClaudeCodeManager {
	return &ClaudeCodeManager{
		ptyManager:   ptyManager,
		config:       config,
		sessions:     make(map[string]*ClaudeCodeSession),
		userSessions: make(map[string]string),
		logger:       log.New(os.Stdout),
	}
}

// CreateSession 创建新的 Claude Code 会话
func (m *ClaudeCodeManager) CreateSession(userID, projectPath string) (*ClaudeCodeSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Info("CreateSession Start",
		"user_id", userID,
		"project_path", projectPath,
		"current_sessions_count", len(m.sessions),
		"current_user_sessions", m.userSessions[userID])

	// 检查并发限制
	if m.config.MaxSessions > 0 && len(m.sessions) >= m.config.MaxSessions {
		return nil, fmt.Errorf("maximum sessions reached (%d)", m.config.MaxSessions)
	}

	// 检查用户是否有活跃会话 - 清理所有旧的会话（无论是否 active）
	if existingSessionID, ok := m.userSessions[userID]; ok {
		log.Info("CreateSession Found old session, cleaning up",
			"user_id", userID,
			"existing_session", existingSessionID,
			"session_in_map", m.sessions[existingSessionID] != nil)

		// 关闭旧会话的 PTY（内联操作以避免死锁）
		if err := m.ptyManager.CloseSession(existingSessionID); err != nil {
			log.Warn("CreateSession Failed to close existing PTY session",
				"session_id", existingSessionID,
				"error", err)
		}

		// 标记旧会话为关闭状态并删除
		if existingSession, ok := m.sessions[existingSessionID]; ok {
			existingSession.IsActive = false
			existingSession.State = SessionStateClosed
		}

		// 删除映射以确保使用新会话
		delete(m.userSessions, userID)
		delete(m.sessions, existingSessionID)

		log.Info("CreateSession Old session removed",
			"user_id", userID,
			"removed_session", existingSessionID,
			"user_sessions_after", m.userSessions[userID])
	}

	// 生成会话 ID
	sessionID := fmt.Sprintf("%s:%d", userID, time.Now().UnixNano())

	log.Info("CreateSession Creating PTY session",
		"user_id", userID,
		"new_session_id", sessionID)

	// 使用 PTY 管理器创建 PTY 会话
	_, err := m.ptyManager.CreateSession(sessionID, projectPath)
	if err != nil {
		log.Error("CreateSession Failed to create PTY session",
			"user_id", userID,
			"session_id", sessionID,
			"error", err)
		return nil, fmt.Errorf("failed to create PTY session: %w", err)
	}

	// 创建会话对象
	session := &ClaudeCodeSession{
		ID:           sessionID,
		UserID:       userID,
		ProjectPath:  projectPath,
		IsActive:     true,
		LastActivity: time.Now(),
		CreatedAt:    time.Now(),
		State:        SessionStateActive,
		Context:      make(map[string]interface{}),
	}

	// 存储会话
	m.sessions[sessionID] = session
	m.userSessions[userID] = sessionID

	// 验证存储成功
	storedSessionID, ok := m.userSessions[userID]
	if !ok || storedSessionID != sessionID {
		log.Error("CreateSession Failed to store user session mapping",
			"user_id", userID,
			"expected_session_id", sessionID,
			"stored_session", storedSessionID)
		return nil, fmt.Errorf("internal error: failed to store user session mapping")
	}

	log.Info("CreateSession Complete",
		"session_id", sessionID,
		"user_id", userID,
		"project_path", projectPath,
		"stored_session_id", storedSessionID,
		"total_sessions", len(m.sessions))

	return session, nil
}

// GetSession 根据会话 ID 获取会话
func (m *ClaudeCodeManager) GetSession(sessionID string) (*ClaudeCodeSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// 检查会话是否超时
	if time.Since(session.LastActivity) > m.config.SessionTimeout {
		return nil, fmt.Errorf("session expired: %s", sessionID)
	}

	return session, nil
}

// GetUserSession 获取用户的当前活跃会话
func (m *ClaudeCodeManager) GetUserSession(userID string) (*ClaudeCodeSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	log.Debug("GetUserSession Start",
		"user_id", userID,
		"all_user_sessions", m.userSessions)

	sessionID, ok := m.userSessions[userID]
	if !ok {
		return nil, fmt.Errorf("no active session for user: %s", userID)
	}

	log.Debug("GetUserSession Found session ID",
		"user_id", userID,
		"session_id", sessionID,
		"session_exists_in_map", m.sessions[sessionID] != nil)

	session, ok := m.sessions[sessionID]
	if !ok {
		log.Error("GetUserSession Session not found in map",
			"user_id", userID,
			"session_id", sessionID)
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	log.Debug("GetUserSession Returning session",
		"user_id", userID,
		"session_id", session.ID,
		"is_active", session.IsActive,
		"state", session.State)

	return session, nil
}

// HasActiveSession 检查用户是否有活跃会话
func (m *ClaudeCodeManager) HasActiveSession(userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionID, ok := m.userSessions[userID]
	if !ok {
		log.Debug("HasActiveSession No session mapping found",
			"user_id", userID,
			"all_user_sessions", m.userSessions)
		return false
	}

	session, ok := m.sessions[sessionID]
	if !ok {
		log.Warn("HasActiveSession Session ID exists in userSessions but not in sessions map",
			"user_id", userID,
			"session_id", sessionID,
			"problem", "inconsistent state")
		return false
	}

	log.Debug("HasActiveSession Session found",
		"user_id", userID,
		"session_id", sessionID,
		"is_active", session.IsActive,
		"state", session.State)

	return ok && session.IsActive
}

// SendInput 发送用户输入到 Claude Code PTY
func (m *ClaudeCodeManager) SendInput(sessionID, input string) error {
	log.Info("SendInput Start",
		"session_id", sessionID,
		"user_input", input,
		"input_length", len(input))

	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		log.Error("SendInput Session not found",
			"session_id", sessionID,
			"user_input", input)
		return fmt.Errorf("session not found: %s", sessionID)
	}

	log.Debug("SendInput Session found",
		"session_id", sessionID,
		"user_id", session.UserID,
		"is_active", session.IsActive,
		"state", session.State,
		"user_input", input)

	if !session.IsActive {
		log.Error("SendInput Session is not active",
			"session_id", sessionID,
			"user_id", session.UserID,
			"is_active", session.IsActive,
			"state", session.State,
			"user_input", input)
		return fmt.Errorf("session is not active: %s", sessionID)
	}

	// 检查会话是否在等待问题回答
	if session.State == SessionStateQuestion && session.CurrentQuestion != nil {
		// 使用交互式解析器格式化答案
		// 这里需要导入 enhanced 包的解析器
		// 为了避免循环依赖，我们暂时直接格式化
		input = m.formatAnswer(session.CurrentQuestion, input)
		session.CurrentQuestion = nil
		session.State = SessionStateActive
	}

	// 通过 PTY 管理器发送输入
	log.Debug("SendInput Sending to PTY manager",
		"session_id", sessionID,
		"user_input", input)
	err := m.ptyManager.SendInput(sessionID, input)
	if err != nil {
		log.Error("SendInput PTY send failed",
			"session_id", sessionID,
			"user_id", session.UserID,
			"user_input", input,
			"error", err)
		return fmt.Errorf("failed to send input to PTY: %w", err)
	}

	log.Info("SendInput Success",
		"session_id", sessionID,
		"user_id", session.UserID,
		"user_input", input)

	// 更新活动时间
	session.LastActivity = time.Now()

	return nil
}

// formatAnswer 格式化答案（简化版）
// 注意：这是简化实现，没有使用 enhanced.ClaudeInteractiveParser.FormatAnswer()
// 实际处理交互式问题时应该使用 enhanced 包的解析器
func (m *ClaudeCodeManager) formatAnswer(question *InteractiveQuestion, answer string) string {
	// TODO: 集成 enhanced.ClaudeInteractiveParser 用于正确格式化答案
	// 当前直接返回原始答案
	return answer
}

// InterruptSession 发送 Ctrl+C 中断信号
func (m *ClaudeCodeManager) InterruptSession(sessionID string) error {
	// 验证会话是否存在
	m.mu.RLock()
	_, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return m.ptyManager.Interrupt(sessionID)
}

// SendEOF 发送 Ctrl+D 结束输入
func (m *ClaudeCodeManager) SendEOF(sessionID string) error {
	// 验证会话是否存在
	m.mu.RLock()
	_, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return m.ptyManager.SendEOF(sessionID)
}

// CloseSession 关闭 Claude Code 会话
func (m *ClaudeCodeManager) CloseSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// 关闭 PTY 会话
	err := m.ptyManager.CloseSession(sessionID)
	if err != nil {
		log.Warn("Failed to close PTY session", "session_id", sessionID, "error", err)
	}

	// 标记会话为非活跃
	session.IsActive = false
	session.State = SessionStateClosed

	// 从用户当前会话映射中移除
	if currentSessionID, ok := m.userSessions[session.UserID]; ok && currentSessionID == sessionID {
		delete(m.userSessions, session.UserID)
	}

	// Remove from sessions map to prevent memory leaks
	delete(m.sessions, sessionID)

	log.Info("Claude Code session closed",
		"session_id", sessionID,
		"user_id", session.UserID)

	return nil
}

// CleanInactiveSessions 清理不活跃的会话
func (m *ClaudeCodeManager) CleanInactiveSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	timeoutCount := 0
	for sessionID, session := range m.sessions {
		if !session.IsActive {
			continue
		}

		if time.Since(session.LastActivity) > m.config.SessionTimeout {
			log.Info("Cleaning up inactive session",
				"session_id", sessionID,
				"user_id", session.UserID)

			_ = m.ptyManager.CloseSession(sessionID)
			session.IsActive = false
			session.State = SessionStateClosed

			if currentSessionID, ok := m.userSessions[session.UserID]; ok && currentSessionID == sessionID {
				delete(m.userSessions, session.UserID)
			}

			// Remove from sessions map to prevent memory leaks
			delete(m.sessions, sessionID)

			timeoutCount++
		}
	}

	if timeoutCount > 0 {
		log.Info("Cleaned inactive sessions", "count", timeoutCount)
	}
}

// DetectClaudeCodeCommand 检测输入是否为 Claude Code 命令
func (m *ClaudeCodeManager) DetectClaudeCodeCommand(input string) (bool, string, string) {
	// 检查会话前缀，如 "/claude:" 或 "⚙️"
	if m.config.SessionPrefix != "" && strings.HasPrefix(input, m.config.SessionPrefix) {
		// 提取实际命令（去除前缀和多余空格）
		command := strings.TrimSpace(strings.TrimPrefix(input, m.config.SessionPrefix))

		log.Debug("claude_code", "Claude Code command detected",
			"input", input,
			"prefix", m.config.SessionPrefix,
			"extracted_command", command)

		return true, command, ""
	}

	log.Debug("claude_code", "Not a Claude Code command",
		"input", input,
		"prefix", m.config.SessionPrefix)

	return false, "", ""
}

// ValidateInput 验证用户输入是否有效
func (m *ClaudeCodeManager) ValidateInput(input string) error {
	if strings.TrimSpace(input) == "" {
		return fmt.Errorf("input cannot be empty")
	}
	return nil
}

// ListSessions 列出所有会话
func (m *ClaudeCodeManager) ListSessions() []*ClaudeCodeSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*ClaudeCodeSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// GetUserSessionsInfo 获取用户的会话信息
func (m *ClaudeCodeManager) GetUserSessionsInfo(userID string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionID, ok := m.userSessions[userID]
	if !ok {
		return map[string]interface{}{
			"has_session": false,
		}
	}

	session, ok := m.sessions[sessionID]
	if !ok {
		return map[string]interface{}{
			"has_session": false,
		}
	}

	return map[string]interface{}{
		"has_session":   true,
		"session_id":    session.ID,
		"project_path":  session.ProjectPath,
		"is_active":     session.IsActive,
		"state":         session.State,
		"created_at":    session.CreatedAt,
		"last_activity": session.LastActivity,
	}
}

// SetSessionQuestion sets the session state to question state
func (m *ClaudeCodeManager) SetSessionQuestion(sessionID string, question *InteractiveQuestion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.State = SessionStateQuestion
	session.CurrentQuestion = question
	return nil
}

// ClearSessionQuestion clears the current question and returns to active state
func (m *ClaudeCodeManager) ClearSessionQuestion(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.CurrentQuestion = nil
	session.State = SessionStateActive
	return nil
}
