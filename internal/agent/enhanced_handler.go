package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/konglong87/wukongbot/internal/bus"
	"github.com/konglong87/wukongbot/internal/feishu/enhanced"
	"github.com/konglong87/wukongbot/internal/feishu/progress"
	"github.com/konglong87/wukongbot/internal/tmux"
)

// ErrNotInteractiveOutput indicates the output is not an interactive question
var ErrNotInteractiveOutput = errors.New("output is not an interactive question")

// claudeCodePTYAdapter adapts enhanced.ClaudeCodePTYManager to work with ClaudeCodeManager
type claudeCodePTYAdapter struct {
	pty *enhanced.ClaudeCodePTYManager
}

// CreateSession implements ClaudeCodePTYManagerInterface
func (a *claudeCodePTYAdapter) CreateSession(sessionID, projectPath string) (*enhanced.ClaudeCodeSession, error) {
	return a.pty.CreateSession(sessionID, projectPath)
}

// SendInput implements ClaudeCodePTYManagerInterface
func (a *claudeCodePTYAdapter) SendInput(sessionID, input string) error {
	return a.pty.SendInput(sessionID, input)
}

// CloseSession implements ClaudeCodePTYManagerInterface
func (a *claudeCodePTYAdapter) CloseSession(sessionID string) error {
	return a.pty.CloseSession(sessionID)
}

// Interrupt implements ClaudeCodePTYManagerInterface
func (a *claudeCodePTYAdapter) Interrupt(sessionID string) error {
	return a.pty.Interrupt(sessionID)
}

// SendEOF implements ClaudeCodePTYManagerInterface
func (a *claudeCodePTYAdapter) SendEOF(sessionID string) error {
	return a.pty.SendEOF(sessionID)
}

// EnhancedHandler provides Feishu enhanced functionality for Agent Loop
type EnhancedHandler struct {
	adapter              *enhanced.FeishuAdapter
	bus                  MessageSender
	enabled              bool
	currentSession       string
	currentSender        string
	mu                   sync.RWMutex
	claudeCodePTYManager *enhanced.ClaudeCodePTYManager
	claudeCodeManager    *ClaudeCodeManager
	claudeConfig         ClaudeCodeConfig
	interactiveParser    *enhanced.ClaudeInteractiveParser
	logger               *log.Logger
	// Tmux session management
	tmuxManager        *tmux.TmuxManager
	tmuxConfig         *tmux.TmuxConfig
	cardSessionManager *CardSessionManager // 🆕 Card session manager for interactive cards
}

// MessageSender interface matches FeishuAdapter's requirement
type MessageSender interface {
	SendOutbound(ctx context.Context, channelID, senderID, content string) error
}

// AgentLoopMessageSender wraps MessageBus to satisfy MessageSender interface
type AgentLoopMessageSender struct {
	bus bus.MessageBus
}

// NewAgentLoopMessageSender creates a new message sender
func NewAgentLoopMessageSender(messageBus bus.MessageBus) MessageSender {
	return &AgentLoopMessageSender{bus: messageBus}
}

// SendOutbound sends a message through the bus
func (s *AgentLoopMessageSender) SendOutbound(ctx context.Context, channelID, senderID, content string) error {
	msg := bus.OutboundMessage{
		ChannelID:   channelID,
		RecipientID: senderID,
		Content:     content,
	}
	return s.bus.PublishOutbound(ctx, msg)
}

// NewEnhancedHandler creates a new enhanced handler
func NewEnhancedHandler(sender MessageSender, workspace string, claudeConfig ClaudeCodeConfig) *EnhancedHandler {
	if sender == nil {
		log.Error("enhanced_handler 错误 ", "Message sender is nil")
		return &EnhancedHandler{enabled: false, logger: log.Default()}
	}

	// Create message sender wrapper
	messageSender := func(channelID, recipientID, content string) error {
		log.Info("enhanced_handler", "Sending message", "channel", channelID, "recipient", recipientID, "content", content)
		return sender.SendOutbound(context.Background(), channelID, recipientID, content)
	}

	// Create Feishu adapter
	adapter := enhanced.NewFeishuAdapter(messageSender)

	// Prepare Claude Code command string (join with space)
	claudeCommandStr := ""
	if len(claudeConfig.ClaudeCommand) > 0 {
		claudeCommandStr = strings.Join(claudeConfig.ClaudeCommand, " ")
	} else {
		claudeCommandStr = "claude code"
	}

	// Log the command being used
	log.Debug("enhanced_handler", "Creating Claude Code PTY manager",
		"claude_command", claudeCommandStr,
		"configured_command", claudeConfig.ClaudeCommand,
		"workspace", workspace)
	// Create Claude Code PTY manager
	ptyManager := enhanced.NewClaudeCodePTYManager(claudeCommandStr, workspace, messageSender)

	// Wrap PTY manager with adapter
	ptyAdapter := &claudeCodePTYAdapter{pty: ptyManager}

	// Create Claude Code session manager
	claudeCodeManager := NewClaudeCodeManager(ptyAdapter, claudeConfig)

	// Create enhanced handler first
	handler := &EnhancedHandler{
		adapter:              adapter,
		bus:                  sender,
		claudeCodePTYManager: ptyManager,
		claudeCodeManager:    claudeCodeManager,
		claudeConfig:         claudeConfig,
		interactiveParser:    enhanced.NewClaudeInteractiveParser(),
		enabled:              true,
		logger:               log.Default(),
		cardSessionManager:   NewCardSessionManager(nil), // 🆕 nil for now (no storage)
		// Tmux will be initialized separately via SetTmuxManager
	}

	// Set output processor callback to handle interactive questions and cards
	// This callback will be called when PTY has output to process
	ptyManager.SetOutputProcessor(func(sessionID, output string) error {
		log.Info("enhanced_handler OutputProcessor Callback",
			"session_id", sessionID,
			"output_length", len(output),
			"output_preview", func() string {
				if len(output) > 100 {
					return output[:100] + "..."
				}
				return output
			}())

		// Call HandleClaudeOutput to detect interactive questions and send cards
		if err := handler.HandleClaudeOutput(sessionID, output); err != nil {
			log.Error("enhanced_handler HandleClaudeOutput failed",
				"session_id", sessionID,
				"error", err)
			return err
		}

		// If HandleClaudeOutput returns nil, it means a card was sent
		// We don't need to send the output again as plain text
		return nil
	})

	return handler
}

// SetTmuxManager sets the tmux manager for session management
func (h *EnhancedHandler) SetTmuxManager(manager *tmux.TmuxManager, config *tmux.TmuxConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tmuxManager = manager
	h.tmuxConfig = config
	log.Info("EnhancedHandler SetTmuxManager", "tmux_enabled", manager != nil)
}

// Enable enables or disables enhanced features
func (h *EnhancedHandler) Enable(enabled bool) {
	h.enabled = enabled
}

// IsEnabled returns whether enhanced features are enabled
func (h *EnhancedHandler) IsEnabled() bool {
	return h.enabled && h.adapter != nil
}

// SetContext sets the current session context
func (h *EnhancedHandler) SetContext(channel, sender string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.currentSession = channel
	h.currentSender = sender
}

// EnhanceResponse processes LLM response and returns enhanced message
func (h *EnhancedHandler) EnhanceResponse(content string) string {
	if !h.IsEnabled() {
		return content
	}

	h.mu.RLock()
	sender := h.currentSender
	h.mu.RUnlock()

	if sender == "" {
		return content
	}

	// Check if content needs interactive card
	cardNeeded := h.adapter.DetectIfCardNeeded(content)
	if cardNeeded {
		// Send as enhanced message (will convert to card if needed)
		if err := h.adapter.SendEnhancedMessage(sender, content); err != nil {
			log.Warn("Failed to send enhanced message", "error", err)
		}
		return "" // Don't duplicate send
	}

	// Return as-is, let normal flow handle it
	return content
}

// SendProgressMessage sends a progress message
func (h *EnhancedHandler) SendProgressMessage(level progress.ProgressLevel, tool, message string) {
	if !h.IsEnabled() {
		return
	}

	h.mu.RLock()
	sender := h.currentSender
	h.mu.RUnlock()

	if sender == "" {
		return
	}

	msg := progress.ProgressMessage{
		Level:   level,
		Tool:    tool,
		Message: message,
	}

	if err := h.adapter.SendProgressMessage(sender, msg); err != nil {
		log.Warn("Failed to send progress message", "error", err, "level", level, "tool", tool)
	}
}

// RouteMessage routes messages to appropriate handlers
func (h *EnhancedHandler) RouteMessage(channel, sender, content string) (bool, error) {
	log.Info("RouteMessage Entry",
		"channel", channel,
		"sender", sender,
		"content", content,
		"enabled", h.IsEnabled())

	if !h.IsEnabled() {
		log.Debug("RouteMessage Enhanced handler not enabled, skipping")
		return false, nil
	}

	// Set context
	h.SetContext(channel, sender)

	// Clean whitespace for command detection (fix: handle leading/trailing spaces)
	cleanContent := strings.TrimSpace(content)

	// Check for tmux command
	if h.tmuxManager != nil && strings.HasPrefix(cleanContent, "/tmux") {
		return h.handleTmuxCommand(sender, cleanContent)
	}

	// Check for Claude Code command
	if h.claudeCodeManager != nil && h.claudeConfig.Enabled {
		log.Debug("claude_route Checking for Claude Code command",
			"content", content,
			"clean_content", cleanContent,
			"enabled", h.claudeConfig.Enabled)
		isCommand, command, _ := h.claudeCodeManager.DetectClaudeCodeCommand(cleanContent)
		log.Debug("claude_route Command detection result",
			"is_command", isCommand,
			"extracted_command", command,
			"session_prefix", h.claudeConfig.SessionPrefix)
		if isCommand {
			return h.handleClaudeCodeCommand(sender, command)
		}

		// Check if user has active Claude Code session
		hasActive := h.claudeCodeManager.HasActiveSession(sender)
		log.Info("[RouteMessage] Checking for active Claude Code session",
			"user_id", sender,
			"has_active_session", hasActive,
			"content", content,
			"clean_content", cleanContent)
		if hasActive {
			log.Info("[RouteMessage] User has active Claude Code session, forwarding to PTY",
				"user_id", sender,
				"content", content)
			return h.forwardToClaudeCode(sender, content)
		} else {
			log.Info("[RouteMessage] No active Claude Code session, continuing to LLM",
				"user_id", sender)
		}
	}

	return false, nil
}

// handleClaudeCodeCommand handles Claude Code specific commands
func (h *EnhancedHandler) handleClaudeCodeCommand(userID, command string) (bool, error) {
	log.Info("[handleClaudeCodeCommand] _(:大活动」) Handling Claude Code command", "user_id", userID, "command", command)
	switch {
	case command == "启动" || containsKeyWords(command, []string{"启动", "开始", "运行"}):
		projectPath := h.extractProjectPath(command)
		if projectPath == "" {
			projectPath = h.claudeConfig.Workspace
		}

		log.Info("[handleClaudeCodeCommand] Creating Claude Code session",
			"user_id", userID,
			"project_path", projectPath)

		session, err := h.claudeCodeManager.CreateSession(userID, projectPath)
		if err != nil {
			log.Error("[handleClaudeCodeCommand] Failed to create session",
				"user_id", userID,
				"error", err)
			if sendErr := h.sendMessage(userID, fmt.Sprintf("❌ 启动 Claude Code 失败: %v", err)); sendErr != nil {
				log.Warn("Failed to send error message", "error", sendErr)
			}
			return true, err
		}

		// Verify session was stored correctly
		verifySession, verifyErr := h.claudeCodeManager.GetUserSession(userID)
		if verifyErr != nil {
			log.Error("[handleClaudeCodeCommand] Failed to verify new session",
				"user_id", userID,
				"created_session_id", session.ID,
				"verify_error", verifyErr)
		} else {
			log.Info("[handleClaudeCodeCommand] Session verified",
				"user_id", userID,
				"session_id", verifySession.ID,
				"is_active", verifySession.IsActive)
		}

		if sendErr := h.sendMessage(userID, fmt.Sprintf("✅ Claude Code 已启动\n📁 项目路径: %s\n会话 ID: %s\n\n您现在可以直接给我编程任务！\n\n提示: 使用 /claude: 前缀来区分普通对话和 Claude Code 命令", session.ProjectPath, session.ID)); sendErr != nil {
			log.Warn("Failed to send startup message", "error", sendErr)
		}
		return true, nil

	case command == "退出" || command == "exit" || containsKeyWords(command, []string{"退出", "关闭", "exit"}):
		session, err := h.claudeCodeManager.GetUserSession(userID)
		if err != nil {
			if sendErr := h.sendMessage(userID, "❌ 没有活跃的 Claude Code 会话"); sendErr != nil {
				log.Warn("Failed to send error message", "error", sendErr)
			}
			return true, err
		}

		err = h.claudeCodeManager.CloseSession(session.ID)
		if err != nil {
			if sendErr := h.sendMessage(userID, fmt.Sprintf("❌ 关闭会话失败: %v", err)); sendErr != nil {
				log.Warn("Failed to send error message", "error", sendErr)
			}
			return true, err
		}

		if sendErr := h.sendMessage(userID, "✅ Claude Code 会话已关闭"); sendErr != nil {
			log.Warn("Failed to send close message", "error", sendErr)
		}
		return true, nil

	default:
		return h.forwardToClaudeCode(userID, command)
	}
}

// forwardToClaudeCode forwards user input to Claude Code session
func (h *EnhancedHandler) forwardToClaudeCode(userID, input string) (bool, error) {
	log.Info("[PTY MODE] forwardToClaudeCode Start",
		"user_id", userID,
		"user_input", input,
		"input_length", len(input))

	session, err := h.claudeCodeManager.GetUserSession(userID)
	if err != nil {
		log.Error("[PTY MODE] forwardToClaudeCode Failed to get user session",
			"user_id", userID,
			"user_input", input,
			"error", err)
		return false, fmt.Errorf("no active session: %w", err)
	}

	log.Info("[PTY MODE] forwardToClaudeCode Session found",
		"user_id", userID,
		"session_id", session.ID,
		"is_active", session.IsActive,
		"state", session.State,
		"user_input", input)

	if err := h.claudeCodeManager.ValidateInput(input); err != nil {
		if sendErr := h.sendMessage(userID, fmt.Sprintf("⚠️ 输入无效: %v", err)); sendErr != nil {
			log.Warn("Failed to send validation error message", "error", sendErr)
		}
		return true, err
	}

	err = h.claudeCodeManager.SendInput(session.ID, input)
	if err != nil {
		log.Error("[PTY MODE] forwardToClaudeCode SendInput failed",
			"user_id", userID,
			"input", input,
			"session_id", session.ID,
			"session_is_active", session.IsActive,
			"error", err)
		if sendErr := h.sendMessage(userID, fmt.Sprintf("❌ 发送到 Claude Code 失败: %v", err)); sendErr != nil {
			log.Warn("Failed to send error message", "error", sendErr)
		}
		return true, err
	}

	log.Info("[PTY MODE] forwardToClaudeCode Success",
		"user_id", userID,
		"session_id", session.ID,
		"user_input", input)

	return true, nil
}

// extractProjectPath extracts project path from command
func (h *EnhancedHandler) extractProjectPath(command string) string {
	// Simple path extraction
	if strings.Contains(command, "~/") {
		start := strings.Index(command, "~/")
		end := strings.Index(command[start:], " ")
		if end == -1 {
			return strings.TrimSpace(command[start:])
		}
		return strings.TrimSpace(command[start : start+end])
	}

	if strings.Contains(command, "/") {
		start := strings.Index(command, "/")
		end := strings.Index(command[start:], " ")
		if end == -1 {
			return strings.TrimSpace(command[start:])
		}
		return strings.TrimSpace(command[start : start+end])
	}

	return ""
}

// HandleClaudeOutput handles output from Claude Code
func (h *EnhancedHandler) HandleClaudeOutput(sessionID, output string) error {
	log.Info("[发送飞书卡片] HandleClaudeOutput", "session_id", sessionID, "output", output)
	// Parse interactive question
	// Note: enhanced.ClaudeInteractiveParser.ParseInteractiveQuestion returns *enhanced.InteractiveQuestion
	// We need to convert it to our internal feishuInteractiveQuestion type
	question, ok := h.interactiveParser.ParseInteractiveQuestion(output, sessionID)
	if ok {
		// Convert enhanced.InteractiveQuestion to feishuInteractiveQuestion
		feishuQ := convertEnhancedQuestionToFeishu(question)
		return h.handleInteractiveQuestion(feishuQ)
	}
	log.Info("[发送飞书卡片] HandleClaudeOutput", "question", question, "output", output, "ok", ok)
	// Return special error to indicate this is not an interactive question
	// The caller should send this as plain text instead
	return ErrNotInteractiveOutput
}

// handleInteractiveQuestion handles interactive questions from Claude Code
func (h *EnhancedHandler) handleInteractiveQuestion(question *feishuInteractiveQuestion) error {
	log.Info("[发送飞书卡片] handleInteractiveQuestion question type", "type", question.Type, "prompt", question.Prompt)
	session, err := h.claudeCodeManager.GetSession(question.SessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Update session state using the proper method
	err = h.claudeCodeManager.SetSessionQuestion(question.SessionID, convertToAgentInteractiveQuestion(question))
	if err != nil {
		return fmt.Errorf("failed to set session question: %w", err)
	}

	switch question.Type {
	case "select":
		return h.sendSelectionCard(question, session.UserID)
	case "input":
		return h.sendInputCard(question, session.UserID)
	case "confirm":
		return h.sendConfirmCard(question, session.UserID)
	default:
		log.Info("[发送飞书卡片] handleInteractiveQuestion", "unsupported_question_type", question.Type)
		return fmt.Errorf("unsupported question type: %s", question.Type)
	}
}

// sendSelectionCard sends a selection card for user to choose from options
func (h *EnhancedHandler) sendSelectionCard(question *feishuInteractiveQuestion, userID string) error {
	log.Info("[发送飞书卡片] sendSelectionCard", "question", question)
	message := fmt.Sprintf("📝 **Claude Code 需要选择**\n\n%s\n\n选项:\n", question.Prompt)
	for i, opt := range question.Options {
		message += fmt.Sprintf("%d. %s\n", i+1, opt)
	}
	message += "\n请回复选项编号来选择"

	err := h.adapter.SendEnhancedMessage(userID, message)
	if err != nil {
		return fmt.Errorf("failed to send selection card: %w", err)
	}

	return nil
}

// sendInputCard sends an input card for user to provide text input
func (h *EnhancedHandler) sendInputCard(question *feishuInteractiveQuestion, userID string) error {
	log.Info("[发送飞书卡片] sendInputCard", "question", question)
	message := fmt.Sprintf("📝 **Claude Code 需要输入**\n\n%s\n\n请直接回复您的输入", question.Prompt)

	err := h.adapter.SendEnhancedMessage(userID, message)
	if err != nil {
		return fmt.Errorf("failed to send input card: %w", err)
	}

	return nil
}

// sendConfirmCard sends a confirmation card for user to confirm an action
func (h *EnhancedHandler) sendConfirmCard(question *feishuInteractiveQuestion, userID string) error {
	log.Info("[发送飞书卡片] sendConfirmCard", "question", question)
	message := fmt.Sprintf("📝 **Claude Code 需要确认**\n\n%s\n\n请回复 y/yes 确认，或 n/no 取消", question.Prompt)

	err := h.adapter.SendEnhancedMessage(userID, message)
	if err != nil {
		return fmt.Errorf("failed to send confirm card: %w", err)
	}

	return nil
}

// GetClaudeCodeConfig returns the Claude Code configuration
func (h *EnhancedHandler) GetClaudeCodeConfig() ClaudeCodeConfig {
	return h.claudeConfig
}

// IsClaudeCodeEnabled returns whether Claude Code is enabled
func (h *EnhancedHandler) IsClaudeCodeEnabled() bool {
	return h.claudeConfig.Enabled && h.claudeCodeManager != nil
}

// sendMessage sends a message to the user
func (h *EnhancedHandler) sendMessage(userID, content string) error {
	log.Info("EnhancedHandler sendMessage", "Sending message to user",
		"user_id", userID,
		"content_length", len(content),
		"content_preview", func() string {
			if len(content) > 50 {
				return content[:50] + "..."
			}
			return content
		}())
	return h.adapter.SendEnhancedMessage(userID, content)
}

// containsKeyWords checks if string contains any of the keywords
func containsKeyWords(s string, keywords []string) bool {
	s = strings.ToLower(s)
	for _, kw := range keywords {
		if strings.Contains(s, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// convertToAgentInteractiveQuestion converts Feishu interactive question to agent package type
func convertToAgentInteractiveQuestion(q *feishuInteractiveQuestion) *InteractiveQuestion {
	return &InteractiveQuestion{
		Type:      q.Type,
		Prompt:    q.Prompt,
		Options:   q.Options,
		SessionID: q.SessionID,
		CreatedAt: q.CreatedAt,
		Metadata:  q.Metadata,
	}
}

// convertEnhancedQuestionToFeishu converts enhanced.InteractiveQuestion to feishuInteractiveQuestion
func convertEnhancedQuestionToFeishu(q *enhanced.InteractiveQuestion) *feishuInteractiveQuestion {
	metadata := make(map[string]interface{})
	for k, v := range q.Metadata {
		metadata[k] = v
	}

	return &feishuInteractiveQuestion{
		Type:      q.Type,
		Prompt:    q.Prompt,
		Options:   q.Options,
		SessionID: q.SessionID,
		CreatedAt: q.CreatedAt,
		Metadata:  metadata,
	}
}

// feishuInteractiveQuestion represents an interactive question from Feishu parser
type feishuInteractiveQuestion struct {
	Type      string
	Prompt    string
	Options   []string
	SessionID string
	CreatedAt time.Time
	Metadata  map[string]interface{}
}

// HandleToolCardCallback handles card callback from Feishu
func (h *EnhancedHandler) HandleToolCardCallback(callback *enhanced.FeishuCardCallback) (string, error) {
	log.Info("[EnhancedHandler] HandleToolCardCallback Entry",
		"callback_type", callback.Type,
		"session_id", callback.SessionID,
		"user_id", callback.UserID)
	defer log.Info("[EnhancedHandler] HandleToolCardCallback Exit",
		"callback_type", callback.Type,
		"session_id", callback.SessionID,
		"user_id", callback.UserID)

	// Validate callback
	if err := h.validateCardCallback(callback); err != nil {
		log.Error("[EnhancedHandler] HandleToolCardCallback invalid callback",
			"error", err,
			"session_id", callback.SessionID)
		return "", fmt.Errorf("invalid callback: %w", err)
	}

	// Get card session
	session, err := h.cardSessionManager.GetSession(callback.SessionID)
	if err != nil {
		log.Error("[EnhancedHandler] HandleToolCardCallback failed to get session",
			"error", err,
			"session_id", callback.SessionID)
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	// Parse answers from callback
	answers := h.parseCardAnswers(callback, session.CardType)
	if len(answers) == 0 {
		log.Warn("[EnhancedHandler] HandleToolCardCallback no answers parsed",
			"callback_type", callback.Type)
		return "", fmt.Errorf("no answers parsed from callback")
	}

	// Set answer to session
	if err := h.cardSessionManager.SetAnswer(callback.SessionID, answers); err != nil {
		log.Error("[EnhancedHandler] HandleToolCardCallback failed to set answer",
			"error", err,
			"session_id", callback.SessionID)
		return "", fmt.Errorf("failed to set answer: %w", err)
	}

	// Generate confirmation message
	message := h.generateConfirmationMessage(session.CardType, answers)

	log.Info("[EnhancedHandler] HandleToolCardCallback completed successfully",
		"session_id", callback.SessionID,
		"answer_count", len(answers))

	return message, nil
}

// validateCardCallback validates a card callback
func (h *EnhancedHandler) validateCardCallback(callback *enhanced.FeishuCardCallback) error {
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

// parseCardAnswers parses user answers from callback
func (h *EnhancedHandler) parseCardAnswers(callback *enhanced.FeishuCardCallback, cardType CardType) []string {
	log.Debug("[EnhancedHandler] parseCardAnswers Entry",
		"callback_type", callback.Type,
		"card_type", cardType)

	var answers []string

	// Convert CardType to string for comparison
	cardTypeStr := cardTypeToString(cardType)

	switch cardTypeStr {
	case "confirm":
		if value, ok := callback.Values["value"].(string); ok {
			answers = []string{value}
		}

	case "single_choice":
		if value, ok := callback.Values["value"].(string); ok {
			answers = []string{value}
		}

	case "multiple_choice", "checklist":
		if values, ok := callback.Values["values"].([]interface{}); ok {
			answers = make([]string, 0, len(values))
			for _, v := range values {
				if str, ok := v.(string); ok {
					answers = append(answers, str)
				}
			}
		}

	default:
		log.Warn("[EnhancedHandler] parseCardAnswers unknown card type",
			"card_type", cardTypeStr)
	}

	log.Debug("[EnhancedHandler] parseCardAnswers Exit",
		"answer_count", len(answers),
		"answers", answers)

	return answers
}

// generateConfirmationMessage generates confirmation message based on card type and answers
func (h *EnhancedHandler) generateConfirmationMessage(cardType CardType, answers []string) string {
	log.Debug("[EnhancedHandler] generateConfirmationMessage Entry",
		"card_type", cardType,
		"answer_count", len(answers))

	var message string

	// Convert CardType to string for comparison
	cardTypeStr := cardTypeToString(cardType)

	switch cardTypeStr {
	case "confirm":
		if len(answers) > 0 && answers[0] == "yes" {
			message = "✅ 已确认操作"
		} else {
			message = "❌ 已取消操作"
		}

	case "single_choice":
		message = fmt.Sprintf("✅ 已选择: %s", strings.Join(answers, ", "))

	case "multiple_choice":
		message = fmt.Sprintf("✅ 已选择: %s", strings.Join(answers, ", "))

	case "checklist":
		message = fmt.Sprintf("✅ 已勾选: %s", strings.Join(answers, ", "))

	default:
		message = fmt.Sprintf("✅ 操作已确认")
	}

	log.Debug("[EnhancedHandler] generateConfirmationMessage Exit",
		"message", message)

	return message
}

// cardTypeToString converts CardType to string
func cardTypeToString(cardType CardType) string {
	switch cardType {
	case CardTypeConfirm:
		return "confirm"
	case CardTypeSingleChoice:
		return "single_choice"
	case CardTypeMultipleChoice:
		return "multiple_choice"
	case CardTypeInput:
		return "input"
	case CardTypeProgress:
		return "progress"
	case CardTypeResult:
		return "result"
	default:
		return "unknown"
	}
}
