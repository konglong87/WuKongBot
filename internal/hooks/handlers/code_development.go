package handlers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	h "github.com/konglong87/wukongbot/internal/hooks"
)

// CodeDevHandler provides code development-specific hook handlers
type CodeDevHandler struct {
	workspace string
	timeout   int // timeout in seconds
	command   string
}

// NewCodeDevHandler creates a new code development handler
func NewCodeDevHandler(workspace string, command string, timeout int) *CodeDevHandler {
	if timeout == 0 {
		timeout = 300 // 5 minutes default
	}
	if command == "" {
		command = "opencode {task}" // default command
	}
	return &CodeDevHandler{
		workspace: workspace,
		timeout:   timeout,
		command:   command,
	}
}

// BackupBeforeWrite creates a backup before writing to a file
func (cdh *CodeDevHandler) BackupBeforeWrite() h.Hook {
	return h.NewInlineHook(
		"backup-before-write",
		h.PreToolUse,
		"write_file",
		30, // 30 second timeout
		func(ctx context.Context, input h.HookInput) (h.HookOutput, error) {
			path, ok := input.ToolInput["path"].(string)
			if !ok {
				return h.HookOutput{Decision: h.HookAllow}, nil
			}

			// Expand and absolutize the path
			if strings.HasPrefix(path, "~") {
				home, _ := os.UserHomeDir()
				path = filepath.Join(home, path[1:])
			}
			absPath, err := filepath.Abs(path)
			if err != nil {
				return h.HookOutput{Decision: h.HookAllow}, nil
			}

			// Check if file exists
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				return h.HookOutput{Decision: h.HookAllow}, nil
			}

			// Create backup
			backupPath := absPath + ".backup"
			inputData, err := os.ReadFile(absPath)
			if err != nil {
				return h.HookOutput{Decision: h.HookAllow}, nil
			}

			if err := os.WriteFile(backupPath, inputData, 0644); err == nil {
				return h.HookOutput{
					Decision: h.HookAllow,
					Message:  fmt.Sprintf("Created backup: %s", backupPath),
				}, nil
			}

			return h.HookOutput{Decision: h.HookAllow}, nil
		},
	)
}

// DangerousCommandCheck checks for dangerous commands before execution
func (cdh *CodeDevHandler) DangerousCommandCheck() h.Hook {
	return h.NewInlineHook(
		"dangerous-command-check",
		h.PreToolUse,
		"exec",
		10,
		func(ctx context.Context, input h.HookInput) (h.HookOutput, error) {
			command, ok := input.ToolInput["command"].(string)
			if !ok {
				return h.HookOutput{Decision: h.HookAllow}, nil
			}

			cmdLower := strings.ToLower(command)

			// Check for dangerous patterns
			dangerousPatterns := []string{
				`rm\s+-rf?`,     // rm -rf or rm -r
				`del\s+/[fq]`,   // Windows del /f or del /q
				`dd\s+if=`,      // dd command
				`mkfs\.\w+`,     // mkfs (format filesystem)
				`format\s+\w+:`, // Windows format
				`: \(\)`,        // Fork bomb
				`mkdir\b`,       // mkdir (directory creation blocked)
			}

			for _, pattern := range dangerousPatterns {
				if matched, _ := regexp.MatchString(pattern, cmdLower); matched {
					return h.HookOutput{
						Decision: h.HookDeny,
						Reason:   fmt.Sprintf("Dangerous command blocked: %s", pattern),
					}, nil
				}
			}

			return h.HookOutput{Decision: h.HookAllow}, nil
		},
	)
}

// CodeDevHook creates a specialized hook for code development tasks
// Executes user-defined command (e.g., `opencode`) for coding tasks
func (cdh *CodeDevHandler) CodeDevHook() h.Hook {
	return h.NewInlineHook(
		"code-development-task",
		h.UserPromptSubmit,
		"", // Match all prompts
		time.Duration(cdh.timeout)*time.Second,
		func(ctx context.Context, input h.HookInput) (h.HookOutput, error) {
			prompt, ok := input.Data["prompt"].(string)
			if !ok {
				return h.HookOutput{Decision: h.HookAllow}, nil
			}

			// Check if this is a coding task
			if !cdh.isCodingTask(prompt) {
				return h.HookOutput{Decision: h.HookAllow}, nil
			}

			// Prepare environment variables for the command
			env := []string{
				fmt.Sprintf("TASK=%s", prompt),
				fmt.Sprintf("WORKSPACE=%s", cdh.workspace),
				fmt.Sprintf("PROJECT_DIR=%s", cdh.workspace),
			}

			// Expand template variables in command
			cmd := cdh.expandCommandTemplate(cdh.command, prompt, cdh.workspace)

			// Execute the user-defined command
			result, err := cdh.executeCommand(ctx, cmd, env)

			if err != nil {
				// Command failed, let main agent handle it
				log.Error("Code dev command failed", "command", cmd, "error", err)
				return h.HookOutput{
					Decision: h.HookAllow,
					Message:  fmt.Sprintf("Coding task failed: %v\n%s", err, result),
				}, nil
			}

			// Command succeeded, return result directly to user (bypass main agent)
			return h.HookOutput{
				Decision: h.HookDeny, // Deny means we're handling it ourselves
				Message:  result,
			}, nil
		},
	)
}

// isCodingTask checks if a prompt is a coding task
func (cdh *CodeDevHandler) isCodingTask(prompt string) bool {
	codingKeywords := []string{
		"写代码", "写一个", "添加功能", "实现", "修复bug", "refactor",
		"写", "加", "实现", "创建", "添加接口", "添加函数",
	}

	promptLower := strings.ToLower(prompt)
	for _, keyword := range codingKeywords {
		if strings.Contains(promptLower, keyword) {
			return true
		}
	}
	return false
}

// expandCommandTemplate expands template variables in command
func (cdh *CodeDevHandler) expandCommandTemplate(commandTemplate, task, workspace string) string {
	cmd := commandTemplate

	// Simple template substitution (order matters for nested expansion)
	// First expand workspace (in case it's used in nested templates)
	cmd = strings.ReplaceAll(cmd, "{workspace}", workspace)

	// Then expand task (quotes and special chars already handled by caller)
	cmd = strings.ReplaceAll(cmd, "{task}", task)

	return cmd
}

// executeCommand runs the user-defined command with timeout
func (cdh *CodeDevHandler) executeCommand(ctx context.Context, cmd string, env []string) (string, error) {
	// Create command context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(cdh.timeout)*time.Second)
	defer cancel()

	// Parse command - handle quoted arguments
	cmdParts, err := parseCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("invalid command syntax: %w", err)
	}

	if len(cmdParts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmdName := cmdParts[0]
	cmdArgs := cmdParts[1:]

	// Create command
	cmdInstance := exec.CommandContext(timeoutCtx, cmdName, cmdArgs...)
	cmdInstance.Dir = cdh.workspace

	// Set environment variables
	cmdInstance.Env = append(os.Environ(), env...)

	// Execute and capture output
	output, err := cmdInstance.CombinedOutput()

	if timeoutCtx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("command timed out after %d seconds", cdh.timeout)
	}

	if err != nil {
		return string(output), err
	}

	return string(output), nil
}

// parseCommand parses a command string into name and args
// Handles quoted arguments with spaces (e.g., "echo \"hello world\"")
func parseCommand(cmd string) ([]string, error) {
	var parts []string
	var current strings.Builder
	inQuotes := false
	escape := false

	for i := 0; i < len(cmd); i++ {
		r := rune(cmd[i])

		switch {
		case r == '\\':
			if escape {
				current.WriteRune(r)
				escape = false
			} else if !inQuotes {
				// Backslash escape next character
				escape = true
			}

		case r == '"', r == '\'':
			if !escape {
				inQuotes = !inQuotes
			} else {
				current.WriteRune(r) // Unescape
			}
			fallthrough

		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	// Simple word splitting for now (shell parsing is complex)
	// For production, consider using shlex package
	words := strings.Fields(current.String())
	parts = append(parts, words...)

	return parts, nil
}
