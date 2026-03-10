package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/konglong87/wukongbot/internal/toolcontext"
)

// TmuxInteractTool provides a unified interface for tmux interaction
// It automatically handles: send-keys -> wait -> capture-pane
type TmuxInteractTool struct {
	socketPath string
}

// NewTmuxInteractTool creates a new tmux interact tool
func NewTmuxInteractTool() *TmuxInteractTool {
	socketDir := os.Getenv("WUKONGBOT_TMUX_SOCKET_DIR")
	if socketDir == "" {
		socketDir = os.Getenv("WUKONGBOT_TMUX_SOCKET_DIR")
	}
	if socketDir == "" {
		socketDir = "/tmp/wukongbot-tmux-sockets"
	}

	return &TmuxInteractTool{
		socketPath: socketDir + "/wukongbot.sock",
	}
}

// Name returns the tool name
func (t *TmuxInteractTool) Name() string {
	return "tmux_interact"
}

// Description returns the tool description
func (t *TmuxInteractTool) Description() string {
	return `Interact with a tmux session. Automatically captures output after sending keys.

Usage modes:
1. Send text: {"keys": "your text"}
2. Send Enter: {"enter": true}
3. Send command: {"command": "your command"}  (equivalent to keys + Enter)

All modes automatically capture pane output after the operation.`
}

// Parameters returns the JSON schema for parameters
func (t *TmuxInteractTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"session": {
				"type": "string",
				"description": "Target session/window/pane (e.g., 's1', 'session:0.0'). Default: 's1'",
				"default": "s1"
			},
			"keys": {
				"type": "string",
				"description": "Text to send to the terminal (without pressing Enter)"
			},
			"command": {
				"type": "string",
				"description": "Command to execute (text + Enter)"
			},
			"enter": {
				"type": "boolean",
				"description": "Send Enter key only (no text)"
			},
			"wait_ms": {
				"type": "integer",
				"description": "Milliseconds to wait before capturing output. Default: 500",
				"default": 500
			},
			"lines": {
				"type": "integer",
				"description": "Number of lines to capture from history. Default: 200",
				"default": 200
			}
		}
	}`)
}

// Execute executes the tmux interaction
func (t *TmuxInteractTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get session target
	session := "s1"
	if s, ok := args["session"].(string); ok && s != "" {
		session = s
	}

	// Get wait time
	waitMs := 500
	if w, ok := args["wait_ms"].(float64); ok {
		waitMs = int(w)
	}

	// Get capture lines
	lines := 200
	if l, ok := args["lines"].(float64); ok {
		lines = int(l)
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("=== tmux session: %s ===\n\n", session))

	// Determine what to send
	var keysToSend []string
	shouldPressEnter := false

	if cmd, ok := args["command"].(string); ok && cmd != "" {
		// Command mode: send text + Enter
		keysToSend = []string{cmd}
		shouldPressEnter = true
		output.WriteString(fmt.Sprintf("Command: %s\n", cmd))
	} else if keys, ok := args["keys"].(string); ok && keys != "" {
		// Keys mode: send text without Enter
		keysToSend = []string{keys}
		output.WriteString(fmt.Sprintf("Keys: %s\n", keys))
	} else if enter, ok := args["enter"].(bool); ok && enter {
		// Enter mode: just press Enter
		shouldPressEnter = true
		output.WriteString("Action: Press Enter\n")
	} else {
		return "Error: Must specify 'command', 'keys', or 'enter'", nil
	}

	// Step 1: Send keys
	if len(keysToSend) > 0 {
		args := []string{"-S", t.socketPath, "send-keys", "-t", session, "-l", "--"}
		args = append(args, keysToSend...)
		cmd := exec.CommandContext(ctx, "tmux", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Sprintf("Error sending keys: %v", err), nil
		}
		output.WriteString("✓ Keys sent\n")
	}

	// Step 2: Press Enter if needed
	if shouldPressEnter {
		cmd := exec.CommandContext(ctx, "tmux", "-S", t.socketPath, "send-keys", "-t", session, "Enter")
		if err := cmd.Run(); err != nil {
			return fmt.Sprintf("Error sending Enter: %v", err), nil
		}
		output.WriteString("✓ Enter pressed\n")
	}

	// Step 3: Wait for TUI to update
	time.Sleep(time.Duration(waitMs) * time.Millisecond)
	output.WriteString(fmt.Sprintf("✓ Waited %dms\n\n", waitMs))

	// Step 4: Capture pane output
	captureArgs := []string{
		"-S", t.socketPath,
		"capture-pane",
		"-t", session,
		"-p", "-J",
		"-S", fmt.Sprintf("-%d", lines),
	}

	captureCmd := exec.CommandContext(ctx, "tmux", captureArgs...)
	captureOutput, err := captureCmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Error capturing pane: %v\nOutput: %s", err, string(captureOutput)), nil
	}

	output.WriteString("=== Terminal Output ===\n")
	output.WriteString(string(captureOutput))

	// Step 5: Show monitoring commands
	output.WriteString(fmt.Sprintf("\n=== Monitoring Commands ===\n"))
	output.WriteString(fmt.Sprintf("Attach: tmux -S %s attach -t %s\n", t.socketPath, session))
	output.WriteString(fmt.Sprintf("Capture: tmux -S %s capture-pane -p -J -t %s -S -%d\n",
		t.socketPath, session, lines))

	return output.String(), nil
}

// ConcurrentSafe returns false - tmux interactions are sequential
func (t *TmuxInteractTool) ConcurrentSafe() bool {
	return false
}

// BeforeExecute is called before tool execution to check for dangerous commands
func (t *TmuxInteractTool) BeforeExecute(ctx *toolcontext.ToolContext) (*toolcontext.ToolDecision, error) {
	log.Info("[TmuxInteractTool] BeforeExecute Entry",
		"user_id", ctx.UserID,
		"channel_id", ctx.ChannelID,
		"tool_name", ctx.ToolName,
		"tool_call_id", ctx.ToolCallID)
	defer log.Info("[TmuxInteractTool] BeforeExecute Exit",
		"user_id", ctx.UserID,
		"tool_name", ctx.ToolName)

	// Only check command mode
	command, hasCommand := ctx.Params["command"].(string)
	if !hasCommand || command == "" {
		// Keys mode or enter mode are safe
		log.Debug("[TmuxInteractTool] BeforeExecute skipping check - not command mode")
		return nil, nil
	}

	log.Debug("[TmuxInteractTool] BeforeExecute checking command",
		"command", command)

	// Check if command is dangerous
	if t.isDangerousCommand(command) {
		log.Warn("[TmuxInteractTool] BeforeExecute detected dangerous command",
			"command", command)

		// Create confirmation card
		cardContent := &toolcontext.CardContent{
			Title:       "⚠️ 危险操作确认",
			Description: fmt.Sprintf("即将在 tmux 会话中执行以下危险命令：\n\n```\n%s\n```", command),
			Question:    "是否继续执行此操作？",
			Options: []*toolcontext.CardOption{
				{Label: "取消", Value: "cancel", Description: "不执行此命令"},
				{Label: "继续执行", Value: "continue", Description: "确认执行此危险命令"},
			},
			WarnLevel: "high",
		}

		decision := &toolcontext.ToolDecision{
			Action:      toolcontext.ActionWaitCard,
			CardNeeded:  true,
			CardType:    toolcontext.CardTypeConfirm,
			CardContent: cardContent,
			Timeout:     5 * time.Minute,
		}

		log.Info("[TmuxInteractTool] BeforeExecute returning wait card decision",
			"action", decision.Action)
		return decision, nil
	}

	log.Debug("[TmuxInteractTool] BeforeExecute command is safe")
	return nil, nil
}

// AfterExecute is called after tool execution to detect interactive prompts
func (t *TmuxInteractTool) AfterExecute(ctx *toolcontext.ToolContext, result string, err error) error {
	log.Info("[TmuxInteractTool] AfterExecute Entry",
		"user_id", ctx.UserID,
		"channel_id", ctx.ChannelID,
		"tool_name", ctx.ToolName,
		"tool_call_id", ctx.ToolCallID,
		"result_length", len(result),
		"error", err)
	defer log.Info("[TmuxInteractTool] AfterExecute Exit",
		"user_id", ctx.UserID,
		"tool_name", ctx.ToolName)

	if err != nil {
		log.Error("[TmuxInteractTool] AfterExecute tool execution error",
			"error", err)
		return nil // Don't return error to avoid masking original error
	}

	// Check if we can send a card (Adapter is available)
	if ctx.Adapter == nil {
		log.Debug("[TmuxInteractTool] AfterExecute Adapter not available, skipping card")
		return nil
	}

	// Priority 1: Detect numbered options (e.g., "1. Option A", "2. Option B")
	if options := t.detectNumberedOptions(result); len(options) > 0 {
		log.Info("[TmuxInteractTool] AfterExecute detected numbered options",
			"count", len(options),
			"options", options)

		// Build card options from detected options
		cardOptions := make([]*toolcontext.CardOption, len(options))
		for i, opt := range options {
			cardOptions[i] = &toolcontext.CardOption{
				Label:       fmt.Sprintf("%d. %s", i+1, opt),
				Value:       fmt.Sprintf("%d", i+1),
				Description: opt,
			}
		}

		cardContent := &toolcontext.CardContent{
			Title:       "🔢 检测到选项列表",
			Description: fmt.Sprintf("发现 %d 个选项，请选择：\n\n%s", len(options), formatOptionsList(options)),
			Question:    "请选择一个选项：",
			Options:     cardOptions,
			WarnLevel:   "low",
		}

		log.Info("[TmuxInteractTool] AfterExecute sending numbered options card",
			"user_id", ctx.UserID,
			"channel_id", ctx.ChannelID,
			"options_count", len(options))

		err := ctx.Adapter.SendCard(ctx.UserID, ctx.ChannelID, cardContent)
		if err != nil {
			log.Error("[TmuxInteractTool] AfterExecute failed to send numbered options card",
				"error", err)
		}

		return nil
	}

	// Priority 2: Detect general interactive questions
	if t.hasInteractiveQuestion(result) {
		log.Info("[TmuxInteractTool] AfterExecute detected interactive question in output",
			"output_preview", result[:min(200, len(result))])

		cardContent := &toolcontext.CardContent{
			Title:       "🔔 检测到交互式提示",
			Description: "命令执行后输出包含交互式提示，需要您的输入：",
			Question:    "请选择下一步操作：",
			Options: []*toolcontext.CardOption{
				{Label: "继续等待", Value: "wait", Description: "等待更多输出"},
				{Label: "发送输入", Value: "input", Description: "手动发送输入到终端"},
				{Label: "取消", Value: "cancel", Description: "取消此操作"},
			},
			WarnLevel: "medium",
		}

		log.Info("[TmuxInteractTool] AfterExecute sending interactive card",
			"user_id", ctx.UserID,
			"channel_id", ctx.ChannelID)

		err := ctx.Adapter.SendCard(ctx.UserID, ctx.ChannelID, cardContent)
		if err != nil {
			log.Error("[TmuxInteractTool] AfterExecute failed to send card",
				"error", err)
		}
	}

	return nil
}

// formatOptionsList formats options for display
func formatOptionsList(options []string) string {
	var sb strings.Builder
	for i, opt := range options {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, opt))
	}
	return sb.String()
}

// isDangerousCommand checks if a command contains dangerous patterns
func (t *TmuxInteractTool) isDangerousCommand(command string) bool {
	dangerousPatterns := []struct {
		pattern string
		desc    string
	}{
		{`rm\s+-rf\s+[/~]`, "Recursive delete"},
		{`rm\s+-r\s+[/~]`, "Recursive delete"},
		{`dd\s+if=`, "Disk write with dd"},
		{`mkfs\.`, "Filesystem creation"},
		{`shutdown\s+`, "System shutdown"},
		{`reboot\s+`, "System reboot"},
		{`kill\s+-9\s+-1`, "Kill all processes"},
		{`killall\s+-9\s+\*`, "Kill all processes"},
		{`:(){:|:&};:`, "Fork bomb"},
		{`>\s*/dev/`, "Direct device write"},
		{`chmod\s+-R\s+777\s+[/~]`, "Recursive 777 permissions"},
		{`chown\s+-R\s+root\s+[/~]`, "Recursive ownership change"},
	}

	lowerCmd := strings.ToLower(command)
	for _, dp := range dangerousPatterns {
		matched, _ := regexp.MatchString(dp.pattern, lowerCmd)
		if matched {
			log.Debug("[TmuxInteractTool] isDangerousCommand matched pattern",
				"pattern", dp.pattern,
				"description", dp.desc)
			return true
		}
	}

	return false
}

// hasInteractiveQuestion checks if output contains interactive prompts
func (t *TmuxInteractTool) hasInteractiveQuestion(output string) bool {
	interactivePatterns := []string{
		// 原有的交互模式
		`\(y/n\)`,
		`\[y/N\]`,
		`\[Y/n\]`,
		`\(yes/no\)`,
		`Are you sure`,
		`Do you want to`,
		"Do you want to proceed?",
		`Continue\?`,
		`Press any key`,
		`Please select`,
		`Enter your choice`,
		`Choose an option`,
		`Input required`,
		`Password:`,

		// 新增：常见问答模式
		`Please choose`,
		`Select an option`,
		`Which .+ would you like`,
		`What would you like to do`,
		`How would you like to proceed`,
		`Would you like to`,
		`Should I`,
		`Can I`,
		`May I`,

		// 新增：Superpower/技能相关
		`请选择`,
		`可能的选项`,
		`以下选项`,
		`选择以下`,
		`select from the following`,
		`choose from the following`,
		`possible options`,
		`available options`,

		// 新增：命令行工具交互
		`Enter selection`,
		`Your choice`,
		`Your selection`,
		`Please enter`,
		`Please input`,
		`Type your`,
		`Provide the`,

		// 新增：确认和取消
		`confirm|cancel`,
		`proceed|abort`,
		`accept|decline`,
		`enable|disable`,

		// 新增：配置向导
		`Configuration:`,
		`Setup:`,
		`Settings:`,
		`Customize:`,
	}

	for _, pattern := range interactivePatterns {
		matched, _ := regexp.MatchString(`(?i)`+pattern, output)
		if matched {
			log.Debug("[TmuxInteractTool] hasInteractiveQuestion matched pattern",
				"pattern", pattern)
			return true
		}
	}

	return false
}

// detectNumberedOptions detects numbered option lists in output
// Returns the options if found, nil otherwise
func (t *TmuxInteractTool) detectNumberedOptions(output string) []string {
	// Pattern: matches lines like "1. Option text" or "1) Option text"
	// Supports multi-line numbered lists
	numberedPattern := regexp.MustCompile(`(?m)^\s*(\d+)[.)]\s+(.+)$`)

	matches := numberedPattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}

	options := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 3 {
			// match[1] is the number, match[2] is the text
			optionText := strings.TrimSpace(match[2])
			if optionText != "" {
				options = append(options, optionText)
			}
		}
	}

	// Only consider it a valid option list if we have at least 2 options
	if len(options) >= 2 {
		log.Debug("[TmuxInteractTool] detectNumberedOptions found options",
			"count", len(options),
			"options", options)
		return options
	}

	return nil
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
