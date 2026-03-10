package enhanced

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/charmbracelet/log"
	"github.com/creack/pty"
	"github.com/konglong87/wukongbot/internal/feishu/progress"
	"golang.org/x/term"
)

// ClaudeCodeSession represents a Claude Code CLI session
type ClaudeCodeSession struct {
	ID           string
	UserID       string // userID extracted from sessionID (format: userID)
	ProjectPath  string
	PTY          *os.File
	Cmd          *exec.Cmd
	OutputBuffer *strings.Builder
	IsActive     bool
	IsClosing    bool // Flag to indicate session is being closed normally
	LastOutput   string
	Message      progress.MessageSender
	Ctx          context.Context
	Cancel       context.CancelFunc
	OldState     *term.State // Saved terminal state for restoration
	mu           sync.RWMutex
}

// OutputProcessor processes output from Claude Code before sending to user
// This is a callback to handle interactive questions, card generation, etc.
type OutputProcessor func(sessionID, output string) error

// ClaudeCodePTYManager manages Claude Code CLI PTY sessions
type ClaudeCodePTYManager struct {
	sessions        map[string]*ClaudeCodeSession
	claudeCommand   []string // CLI command and args, e.g., ["claude", "code"] or ["opencode", "code"]
	workspace       string
	Message         func(channelID, sessionID, content string) error
	outputProcessor OutputProcessor // Callback for processing output (e.g., detecting interactive questions)
	mu              sync.RWMutex
	logger          *log.Logger
}

// Name implements progress.MessageSender interface
func (m *ClaudeCodePTYManager) Name() string {
	return "feishu"
}

// SendMessage implements progress.MessageSender interface
func (m *ClaudeCodePTYManager) SendMessage(channelID, sessionID, content string) error {
	return m.Message(channelID, sessionID, content)
}

// NewClaudeCodePTYManager creates a new Claude Code PTY manager
func NewClaudeCodePTYManager(claudeCommandStr, workspace string, messageSender func(channelID, sessionID, content string) error) *ClaudeCodePTYManager {
	// Parse command string into []string
	var commandParts []string
	if claudeCommandStr == "" {
		// Empty string -> empty slice, will use default command
		commandParts = []string{}
	} else if strings.Contains(claudeCommandStr, " ") {
		commandParts = strings.Fields(claudeCommandStr)
	} else {
		commandParts = []string{claudeCommandStr}
	}

	return &ClaudeCodePTYManager{
		sessions:        make(map[string]*ClaudeCodeSession),
		claudeCommand:   commandParts,
		workspace:       workspace,
		Message:         messageSender,
		outputProcessor: nil, // Set via SetOutputProcessor
		logger:          log.Default(),
	}
}

// SetOutputProcessor sets the output processor callback
func (m *ClaudeCodePTYManager) SetOutputProcessor(processor OutputProcessor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputProcessor = processor
	log.Info("ClaudeCodePTYManager SetOutputProcessor", "callback_set", processor != nil)
}

// quoteForShell quotes a string for safe shell execution, handling spaces and special characters
func quoteForShell(s string) string {
	// Escape existing double quotes and wrap in double quotes
	escaped := strings.ReplaceAll(s, "\"", "\\\"")
	return fmt.Sprintf("\"%s\"", escaped)
}

// buildShellCommand builds the shell command string for creating a session
// This is exposed for testing purposes
func (m *ClaudeCodePTYManager) buildShellCommand(projectPath string) string {
	// Build the complete command string (e.g., "claude code" or "opencode")
	claudeCommandStr := "claude code" // default
	if len(m.claudeCommand) > 0 {
		// Join configured command parts with space (e.g., ["cc", "r", "code"] -> "opencode")
		claudeCommandStr = strings.Join(m.claudeCommand, " ")
	}

	log.Debug("claude_code", "Building shell command",
		"project_path", projectPath,
		"claude_command", claudeCommandStr,
		"claude_command_parts", m.claudeCommand)

	// Build shell command: cd to projectPath, then execute claudeCommand
	// Format: cd "/path/to/project" && opencode
	// Note: claudeCommandStr is executed as-is, without additional path argument
	// because the cd command has already set the working directory
	shellCmd := fmt.Sprintf("cd %s && %s", quoteForShell(projectPath), claudeCommandStr)

	log.Info("claude_code", "buildShellCommand result",
		"shell_command", shellCmd)

	return shellCmd
}

// extractUserIDFromSessionID extracts the userID portion from a composite sessionID.
// SessionID format: "userID:timestamp" -> returns "userID"
func extractUserIDFromSessionID(sessionID string) string {
	// Split on first ':' to separate userID from timestamp
	parts := strings.SplitN(sessionID, ":", 2)
	if len(parts) == 0 {
		return sessionID // Fallback: return sessionID if splitting fails
	}
	return parts[0]
}

// expandHome expands the tilde (~) in a path to the user's home directory.
func expandHome(path string) (string, error) {
	if path == "" {
		return path, nil
	}

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return home + path[1:], nil
	}

	// Handle plain ~
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return home, nil
	}

	return path, nil
}

// CreateSession creates a new Claude Code session in the specified project path
func (m *ClaudeCodePTYManager) CreateSession(sessionID, projectPath string) (*ClaudeCodeSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if projectPath == "" {
		projectPath = m.workspace
	}

	// Expand ~ to user's home directory
	expandedPath, err := expandHome(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand project path: %w", err)
	}
	if expandedPath != projectPath {
		log.Debug("claude_code", "Expanded project path",
			"original", projectPath,
			"expanded", expandedPath)
	}
	projectPath = expandedPath

	// Extract userID from sessionID for message routing
	userID := extractUserIDFromSessionID(sessionID)

	shellCmd := m.buildShellCommand(projectPath)

	log.Info("claude_code", "CreateSession called",
		"session_id", sessionID,
		"user_id", userID,
		"project_path", projectPath,
		"shell_command", shellCmd)

	ctx, cancel := context.WithCancel(context.Background())

	// Start an interactive bash shell for TUI support
	// The -i flag is CRITICAL for TUI applications (provides interactive terminal)
	// We'll send the initial command after the shell starts
	cmd := exec.CommandContext(ctx, "/bin/bash", "-i")

	// Set comprehensive terminal environment variables for TUI applications
	// These are critical for TUI apps like Claude Code CLI to function properly
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",          // Standard terminal type for TUI apps
		"TERM_PROGRAM=vscode",          // Some TUI apps check this
		"TERMINFO=/usr/share/terminfo", // Terminal info database location
		"LINES=50",                     // Terminal height for TUI layout
		"COLUMNS=120",                  // Terminal width for TUI layout
		"PS1=$ ",                       // Minimal prompt
		"LANG=en_US.UTF-8",             // UTF-8 support
		"LC_ALL=en_US.UTF-8",           // Force UTF-8 locale
	)

	// Set working directory to project path
	cmd.Dir = projectPath

	log.Info("claude_code", "Starting interactive bash shell for TUI",
		"cmd_path", cmd.Path,
		"cmd_args", cmd.Args,
		"working_dir", projectPath)

	// Start PTY
	ptyOS, err := pty.Start(cmd)
	if err != nil {
		cancel()
		log.Error("claude_code", "Failed to start PTY",
			"error", err,
			"command", cmd,
			"args", cmd.Args)
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	// CRITICAL: Set PTY window size for TUI applications
	// TUI apps require proper window dimensions for rendering
	// Without this, TUI apps may not respond or render incorrectly
	winSize := &pty.Winsize{
		Rows: 50,   // Height in lines (must be >= 24 for most TUI apps)
		Cols: 120,  // Width in columns (must be >= 80 for most TUI apps)
		X:    1200, // Width in pixels (optional, helps some TUI apps)
		Y:    800,  // Height in pixels (optional, helps some TUI apps)
	}

	// Set window size using pty library
	if err := pty.Setsize(ptyOS, winSize); err != nil {
		log.Error("claude_code", "Failed to set PTY window size",
			"error", err,
			"session_id", sessionID)
	} else {
		log.Info("claude_code", "PTY window size set successfully",
			"session_id", sessionID,
			"rows", winSize.Rows,
			"cols", winSize.Cols)
	}

	// Double-check: Force window size via syscall (some systems need this)
	// This is the low-level equivalent of pty.Setsize but more direct
	fd := int(ptyOS.Fd())
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(winSize)),
		0, 0, 0); errno != 0 {
		log.Error("claude_code", "Failed to force PTY window size via syscall",
			"errno", errno,
			"session_id", sessionID)
	}

	// Set PTY to raw mode for proper terminal behavior
	// This ensures TUI key inputs (arrows, Tab, etc.) are passed through correctly
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		log.Error("claude_code", "Failed to set PTY raw mode",
			"error", err,
			"session_id", sessionID)
		// Don't fail the session, just log the error
	} else {
		log.Info("claude_code", "PTY raw mode set successfully",
			"session_id", sessionID,
			"fd", fd)
	}

	session := &ClaudeCodeSession{
		ID:           sessionID,
		UserID:       userID,
		ProjectPath:  projectPath,
		PTY:          ptyOS,
		Cmd:          cmd,
		OutputBuffer: &strings.Builder{},
		IsActive:     true,
		Message:      m,
		Ctx:          ctx,
		Cancel:       cancel,
		OldState:     oldState,
		mu:           sync.RWMutex{},
	}

	m.sessions[sessionID] = session

	// Build command string for logging
	commandStr := strings.Join(m.claudeCommand, " ")
	if commandStr == "" {
		commandStr = "claude code"
	}

	log.Info("claude_code", "Session created successfully",
		"session_id", sessionID,
		"user_id", userID,
		"command", commandStr,
		"project_path", projectPath,
		"shell_command", shellCmd)

	// CRITICAL: Wait for interactive shell to initialize before sending command
	// The shell needs time to set up its environment
	log.Info("claude_code", "Waiting for interactive shell to initialize",
		"session_id", sessionID,
		"delay_seconds", 2)
	time.Sleep(2 * time.Second)

	// Send the initial command to start Claude Code in the interactive shell
	log.Info("claude_code", "Sending initial command to start Claude Code",
		"session_id", sessionID,
		"command", shellCmd)

	// CRITICAL: Use \r\n (not just \n) to properly trigger command execution
	cmdWithEnter := shellCmd + "\r\n"
	_, err = ptyOS.WriteString(cmdWithEnter)
	if err != nil {
		log.Error("claude_code", "Failed to send initial command to PTY",
			"error", err,
			"session_id", sessionID)
		cancel()
		return nil, fmt.Errorf("failed to send initial command: %w", err)
	}

	// CRITICAL: Flush the PTY buffer to ensure command is sent immediately
	// Without this, the command may stay in the buffer and not execute
	if err := ptyOS.Sync(); err != nil {
		log.Warn("claude_code", "Failed to sync PTY buffer (non-fatal)",
			"error", err,
			"session_id", sessionID)
	}

	// Start monitoring output
	go m.monitorOutput(session)

	// Start process monitor
	go m.monitorProcess(session, sessionID)

	return session, nil
}

// monitorOutput monitors and sends output from the PTY
func (m *ClaudeCodePTYManager) monitorOutput(session *ClaudeCodeSession) {
	log.Info("ClaudeCodePTYManager monitorOutput Entry",
		"session_id", session.ID,
		"user_id", session.UserID,
		"project_path", session.ProjectPath)

	// Use larger buffer for TUI applications (8KB instead of 1KB)
	// TUI apps often output large chunks of data for screen rendering
	buf := make([]byte, 8192)
	outputBuilder := &strings.Builder{}
	lastFlush := time.Now()

	defer func() {
		log.Info("ClaudeCodePTYManager monitorOutput Exit",
			"session_id", session.ID,
			"user_id", session.UserID)
	}()

	for {
		select {
		case <-session.Ctx.Done():
			return
		default:
			n, err := session.PTY.Read(buf)
			if err != nil {
				if err.Error() != "EOF" {
					log.Error("ClaudeCodePTYManager monitorOutput Read error",
						"session_id", session.ID,
						"user_id", session.UserID,
						"error", err)
				} else {
					log.Info("ClaudeCodePTYManager monitorOutput EOF received",
						"session_id", session.ID,
						"user_id", session.UserID)
				}
				return
			}

			if n > 0 {
				output := string(buf[:n])
				log.Debug("ClaudeCodePTYManager monitorOutput PTY output read",
					"session_id", session.ID,
					"user_id", session.UserID,
					"bytes_read", n,
					"output_preview", func() string {
						if len(output) > 100 {
							return output[:100] + "..."
						}
						return output
					}())

				outputBuilder.WriteString(output)
				session.mu.Lock()
				session.LastOutput = session.LastOutput + output
				session.mu.Unlock()

				// Flush output periodically or on specific conditions
				shouldFlush := time.Since(lastFlush) > 100*time.Millisecond || // 100ms minimum delay
					strings.Contains(output, "\n") || // Flush on newlines
					strings.Contains(output, "? ") // Flush when Claude asks questions

				if shouldFlush {
					currentOutput := outputBuilder.String()
					if currentOutput != "" {
						log.Debug("ClaudeCodePTYManager monitorOutput Flushing output to user",
							"session_id", session.ID,
							"user_id", session.UserID,
							"output_length", len(currentOutput),
							"buffer_length", outputBuilder.Len())

						// Convert ANSI to Feishu format
						formatted := m.convertANSIToFeishu(currentOutput)

						log.Debug("ClaudeCodePTYManager monitorOutput ANSI conversion complete",
							"session_id", session.ID,
							"user_id", session.UserID,
							"original_length", len(currentOutput),
							"formatted_length", len(formatted))

						// Process output through callback if set (for interactive questions, cards, etc.)
						// Always send output to user - either as card (for interactive) or as plain text
						shouldSendAsText := true // Default: send as plain text

						if m.outputProcessor != nil {
							log.Info("ClaudeCodePTYManager monitorOutput Calling output processor",
								"session_id", session.ID,
								"user_id", session.UserID,
								"output_length", len(formatted))
							if processErr := m.outputProcessor(session.ID, formatted); processErr != nil {
								// Check if this is "not interactive" error (expected case)
								if processErr.Error() == "output is not an interactive question" {
									log.Info("ClaudeCodePTYManager monitorOutput Output is not interactive, will send as plain text",
										"session_id", session.ID,
										"user_id", session.UserID)
									// shouldSendAsText is already true, will send below
								} else {
									// Real error occurred
									log.Error("ClaudeCodePTYManager monitorOutput Output processor failed",
										"session_id", session.ID,
										"user_id", session.UserID,
										"error", processErr)
									// Still send as text even if processor failed
								}
							} else {
								// Processor succeeded - card was sent
								log.Info("ClaudeCodePTYManager monitorOutput Output processor succeeded (card sent)",
									"session_id", session.ID,
									"user_id", session.UserID)
								// Card sent successfully, don't send as plain text
								shouldSendAsText = false
							}
						}

						// Send to Feishu as plain text if needed
						// Use goroutine for async sending to avoid blocking PTY read loop
						if shouldSendAsText {
							log.Info("[PTY DEBUG] Starting async send to Feishu",
								"session_id", session.ID,
								"content_length", len(formatted))

							go func(content string) {
								startTime := time.Now()

								// Use UserID (plain open_id) instead of session.ID (userID:timestamp) for Feishu API
								err := m.Message("feishu", session.UserID, content)

								elapsed := time.Since(startTime)
								log.Info("[PTY DEBUG] Feishu API call completed",
									"session_id", session.ID,
									"duration_ms", elapsed.Milliseconds(),
									"error", err)

								if err != nil {
									log.Error("ClaudeCodePTYManager monitorOutput Send to user failed",
										"session_id", session.ID,
										"user_id", session.UserID,
										"error", err,
										"content_preview", func() string {
											if len(content) > 50 {
												return content[:50] + "..."
											}
											return content
										}())
								} else {
									log.Info("ClaudeCodePTYManager monitorOutput Output sent to user successfully",
										"session_id", session.ID,
										"user_id", session.UserID,
										"content_length", len(content),
										"content_preview", func() string {
											if len(content) > 50 {
												return content[:50] + "..."
											}
											return content
										}())
								}
							}(formatted)
						}

						outputBuilder.Reset()
						lastFlush = time.Now()
					}
				}
			}
		}
	}
}

// monitorProcess monitors the Claude Code process
func (m *ClaudeCodePTYManager) monitorProcess(session *ClaudeCodeSession, sessionID string) {
	log.Info("ClaudeCodePTYManager monitorProcess Entry",
		"session_id", sessionID,
		"user_id", session.UserID)

	err := session.Cmd.Wait()

	session.mu.Lock()
	session.IsActive = false
	isClosing := session.IsClosing
	session.mu.Unlock()

	// Only send error message if this is NOT a normal shutdown
	if err != nil && !isClosing {
		log.Error("ClaudeCodePTYManager claude_code", "process error (unexpected)",
			"error", err,
			"session", sessionID,
			"user_id", session.UserID,
			"is_closing", isClosing)
		// Use UserID (plain open_id) instead of sessionID (userID:timestamp) for Feishu API
		m.Message("feishu", session.UserID, fmt.Sprintf("❌ Claude Code 进程异常结束: %v", err))
	} else if !isClosing {
		// Normal completion (not killed by CloseSession)
		log.Info("ClaudeCodePTYManager claude_code", "process completed normally",
			"session", sessionID,
			"user_id", session.UserID)
		// Use UserID (plain open_id) instead of sessionID (userID:timestamp) for Feishu API
		m.Message("feishu", session.UserID, "✅ Claude Code 会话已结束")
	} else {
		// Normal shutdown via CloseSession - don't send duplicate message
		log.Info("ClaudeCodePTYManager claude_code", "process killed by CloseSession (normal shutdown)",
			"session", sessionID,
			"user_id", session.UserID,
			"error", err)
	}
}

// SendInput sends user input to the Claude Code PTY
func (m *ClaudeCodePTYManager) SendInput(sessionID, input string) error {
	log.Info("ClaudeCodePTYManager SendInput Entry",
		"session_id", sessionID,
		"user_input", input,
		"input_length", len(input))

	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		log.Error("ClaudeCodePTYManager SendInput Session not found",
			"session_id", sessionID,
			"user_input", input)
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if !session.IsActive {
		log.Error("ClaudeCodePTYManager SendInput Session is not active",
			"session_id", sessionID,
			"user_id", session.UserID,
			"user_input", input)
		return fmt.Errorf("session is not active: %s", sessionID)
	}

	// Add newline if not present
	// Try \r\n for better shell compatibility
	if !strings.HasSuffix(input, "\n") && !strings.HasSuffix(input, "\r\n") {
		input = input + "\r\n"
	}

	// Write to PTY
	log.Info("[PTY DEBUG] SendInput Writing to PTY",
		"session_id", sessionID,
		"user_id", session.UserID,
		"input", input,
		"input_bytes", []byte(input),
		"input_length", len(input))

	bytesWritten, err := session.PTY.Write([]byte(input))
	if err != nil {
		log.Error("ClaudeCodePTYManager SendInput Write to PTY failed",
			"session_id", sessionID,
			"user_id", session.UserID,
			"input", strings.TrimSpace(input),
			"error", err)
		return fmt.Errorf("failed to write to PTY: %w", err)
	}

	log.Info("[PTY DEBUG] SendInput Write completed",
		"session_id", sessionID,
		"bytes_written", bytesWritten,
		"input_length", len(input))

	// CRITICAL: Flush the PTY buffer to ensure input is sent immediately
	// Without this, the input may stay in the buffer and not execute
	// Note: Sync() may fail on macOS with "inappropriate ioctl for device" - this is safe to ignore
	if err := session.PTY.Sync(); err != nil {
		log.Warn("ClaudeCodePTYManager SendInput Failed to sync PTY buffer (non-fatal)",
			"session_id", sessionID,
			"user_id", session.UserID,
			"error", err)
	}

	log.Info("ClaudeCodePTYManager SendInput Success",
		"session_id", sessionID,
		"user_id", session.UserID,
		"input", strings.TrimSpace(input))

	return nil
}

// Interrupt sends interrupt signal (Ctrl+C) to the session
func (m *ClaudeCodePTYManager) Interrupt(sessionID string) error {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok || !session.IsActive {
		return fmt.Errorf("session not active: %s", sessionID)
	}

	// Send Ctrl+C (ASCII 3)
	_, err := session.PTY.Write([]byte{3})
	if err != nil {
		return fmt.Errorf("failed to interrupt: %w", err)
	}

	log.Debug("claude_code", "interrupt sent", "session", sessionID)
	return nil
}

// SendEOF sends Ctrl+D to the session
func (m *ClaudeCodePTYManager) SendEOF(sessionID string) error {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok || !session.IsActive {
		return fmt.Errorf("session not active: %s", sessionID)
	}

	// Send Ctrl+D (ASCII 4)
	_, err := session.PTY.Write([]byte{4})
	if err != nil {
		return fmt.Errorf("failed to send EOF: %w", err)
	}

	log.Debug("claude_code", "EOF sent", "session", sessionID)
	return nil
}

// GetSession retrieves a session by ID
func (m *ClaudeCodePTYManager) GetSession(sessionID string) (*ClaudeCodeSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// IsSessionActive checks if a session is currently active
func (m *ClaudeCodePTYManager) IsSessionActive(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return false
	}

	return session.IsActive
}

// CloseSession closes and terminates a Claude Code session
func (m *ClaudeCodePTYManager) CloseSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	log.Info("ClaudeCodePTYManager claude_code session closing !!! ", "session_id", sessionID, "user_id", session.UserID)

	// Set closing flag before killing process
	// This tells monitorProcess that this is a normal shutdown, not a crash
	session.mu.Lock()
	session.IsClosing = true
	session.mu.Unlock()

	session.Cancel()

	// Restore terminal state if we saved it
	if session.OldState != nil && session.PTY != nil {
		fd := int(session.PTY.Fd())
		if err := term.Restore(fd, session.OldState); err != nil {
			log.Error("claude_code", "Failed to restore terminal state",
				"error", err,
				"session_id", sessionID)
		} else {
			log.Info("claude_code", "Terminal state restored",
				"session_id", sessionID)
		}
	}

	// Close PTY
	if session.PTY != nil {
		session.PTY.Close()
	}

	// Kill process if still running
	if session.Cmd.Process != nil {
		session.Cmd.Process.Kill()
	} else if session.IsActive {
		session.IsActive = false
	}

	delete(m.sessions, sessionID)

	log.Info("claude_code", "session closed", "session_id", sessionID)

	return nil
}

// ListSessions lists all active Claude Code sessions
func (m *ClaudeCodePTYManager) ListSessions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessionIDs []string
	for id, session := range m.sessions {
		if session.IsActive {
			sessionIDs = append(sessionIDs, fmt.Sprintf("%s (active, project: %s)", id, session.ProjectPath))
		}
	}

	return sessionIDs
}

// convertANSIToFeishu converts ANSI escape sequences to Feishu-friendly format
func (m *ClaudeCodePTYManager) convertANSIToFeishu(ansi string) string {
	// Remove ANSI escape sequences
	noANSI := m.removeANSICodes(ansi)

	// Handle common Claude Code patterns
	formatted := strings.ReplaceAll(noANSI, "Claude Code:", "")
	formatted = strings.TrimSpace(formatted)

	// Handle empty output
	if formatted == "" {
		return ""
	}

	// Add markdown code block markers if it looks like code output
	if strings.Contains(formatted, "```") {
		return formatted
	}

	return formatted
}

// removeANSICodes removes ANSI escape sequences from text
func (m *ClaudeCodePTYManager) removeANSICodes(text string) string {
	// Common ANSI escape sequence patterns
	patters := []string{
		"\x1b[0m", "\x1b[1m", "\x1b[30m", "\x1b[31m",
		"\x1b[32m", "\x1b[33m", "\x1b[34m", "\x1b[35m",
		"\x1b[36m", "\x1b[37m", "\x1b[90m", "\x1b[91m",
		"\x1b[92m", "\x1b[93m", "\x1b[94m", "\x1b[95m",
		"\x1b[96m", "\x1b[97m", "\x1b[2m", "\x1b[4m",
		"\x1b[5m", "\x1b[7m", "\x1b[22m", "\x1b[23m",
		"\x1b[24m", "\x1b[25m", "\x1b[27m", "\x1b[28m",
		"\x1b[0K", "\x1b[2K", "\x1b[H", "\x1b[2J",
		"\x1b[A", "\x1b[B", "\x1b[C", "\x1b[D",
		"\x1b[J", "\x1b[K", "\x1b[L", "\x1b[M",
	}

	result := text
	for _, pattern := range patters {
		result = strings.ReplaceAll(result, pattern, "")
	}

	// Remove all generic ANSI escape sequences (CSI sequences)
	// CSI sequences start with ESC [ and end with a character in range 0x40-0x7E
	// Common terminators include: m, l, h, K, J, H, A, B, C, D, etc.
	start := strings.Index(result, "\x1b[")
	for start != -1 {
		// Find the ending terminator character
		// CSI sequences end with a character from '@' (0x40) to '~' (0x7E)
		sequenceEnd := -1
		remaining := result[start+2:] // Skip "\x1b["

		for i := 0; i < len(remaining); i++ {
			c := remaining[i]
			// Check if this is a Terminator character (0x40-0x7E)
			if c >= 0x40 && c <= 0x7E {
				sequenceEnd = i
				break
			}
		}

		if sequenceEnd == -1 {
			// No valid terminator found, remove just this ESC to prevent infinite loop
			result = result[:start] + result[start+1:]
		} else {
			// Remove the entire CSI sequence
			endPos := start + 2 + sequenceEnd + 1
			if endPos > len(result) {
				result = result[:start]
			} else {
				result = result[:start] + result[endPos:]
			}
		}
		start = strings.Index(result, "\x1b[")
	}

	// Remove non-CSI escape sequences (like single ESC chars)
	// Remove remaining isolated ESC characters
	result = strings.ReplaceAll(result, "\x1b", "")

	return result
}

// StartCleanupRoutine starts a background routine to clean up inactive sessions
func (m *ClaudeCodePTYManager) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.cleanupInactiveSessions()
		}
	}
}

// cleanupInactiveSessions removes sessions that have been inactive
func (m *ClaudeCodePTYManager) cleanupInactiveSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for sessionID, session := range m.sessions {
		if !session.IsActive {
			m.CloseSession(sessionID)
		}
	}
}
