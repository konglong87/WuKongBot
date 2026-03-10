package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// CodeDevTool 代码开发工具 - 调用外部编码工具如 opencode、cursor
type CodeDevTool struct {
	workspace string
	timeout   time.Duration
	executors map[string]ToolExecutor
}

// NewCodeDevTool 创建新的代码开发工具
func NewCodeDevTool(workspace string, timeout int, executors map[string]ToolExecutor) *CodeDevTool {
	pc, _, _, _ := runtime.Caller(0)
	fnName := runtime.FuncForPC(pc).Name()

	// 将秒数转换为 Duration（必须乘 time.Second）
	duration := time.Duration(timeout) * time.Second

	t := &CodeDevTool{
		workspace: workspace,
		timeout:   duration,
		executors: executors,
	}

	// 默认超时1小时（Claude 等工具可能需要很长时间）
	if t.timeout == 0 {
		t.timeout = 1 * time.Hour
	}

	log.Debug(fnName, "workspace", workspace, "timeout", t.timeout, "executors", len(executors))
	return t
}

// Name returns the tool name
func (t *CodeDevTool) Name() string {
	return "external_coding" // Changed to be more obvious for LLMs
}

// Description returns the tool description
func (t *CodeDevTool) Description() string {
	return "当用户说'执行opencode编程'、'用opencode'、'用cursor'、'用claude'或要求使用外部编程工具时，使用这个工具。不要用write_file工具。用法: tool='opencode'/'cursor'/'claude', task=具体的编程需求"
}

// Parameters returns the tool schema
func (t *CodeDevTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"tool": {
				"type": "string",
				"enum": ["opencode", "cursor", "claude"],
				"description": "使用的编程工具 (opencode/cursor/claude)"
			},
			"task": {
				"type": "string",
				"description": "编程任务描述 (例如: 写一个hello world程序)"
			}
		},
		"required": ["tool", "task"]
	}`)
}

// Execute executes the tool
func (t *CodeDevTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pc, _, _, _ := runtime.Caller(0)
	fnName := runtime.FuncForPC(pc).Name()

	// 解析参数
	tool, ok := args["tool"].(string)
	if !ok {
		log.Error(fnName, "error", "tool parameter is required")
		return "", fmt.Errorf("tool parameter is required")
	}

	task, ok := args["task"].(string)
	if !ok {
		log.Error(fnName, "error", "task parameter is required")
		return "", fmt.Errorf("task parameter is required")
	}

	log.Info(fnName, "tool", tool, "task", task)

	// 查找工具执行器
	executor, ok := t.executors[tool]
	if !ok {
		log.Error(fnName, "error", fmt.Sprintf("unknown tool: %s", tool))
		return "", fmt.Errorf("unknown tool: %s", tool)
	}

	// 检查工具是否可用
	if !executor.IsAvailable() {
		log.Error(fnName, "error", fmt.Sprintf("tool %s is not available", tool))
		return "", fmt.Errorf("tool %s is not available", tool)
	}

	log.Debug(fnName, "executor", executor.Name(), "template", executor.Template())

	// 构建命令
	template := executor.Template()
	if template == "" {
		template = fmt.Sprintf("%s code \"{task}\"", tool)
	}
	command := strings.ReplaceAll(template, "{task}", task)
	command = strings.ReplaceAll(command, "{workspace}", t.workspace)

	log.Debug(fnName, "command", command)

	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	// 解析命令
	cmdParts := t.parseCommand(command)
	if len(cmdParts) == 0 {
		log.Error(fnName, "error", "empty command")
		return "", fmt.Errorf("empty command")
	}

	// 创建并执行命令
	cmd := exec.CommandContext(timeoutCtx, cmdParts[0], cmdParts[1:]...)
	cmd.Dir = t.workspace

	output, err := cmd.CombinedOutput()

	if timeoutCtx.Err() == context.DeadlineExceeded {
		log.Error(fnName, "error", "command timed out")
		return string(output), fmt.Errorf("command timed out after %v", t.timeout)
	}

	if err != nil {
		log.Error(fnName, "error", err, "output_length", len(output))
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	log.Info(fnName, "success", true, "output_length", len(output))
	return string(output), nil
}

// parseCommand 解析命令字符串
func (t *CodeDevTool) parseCommand(cmdStr string) []string {
	pc, _, _, _ := runtime.Caller(0)
	fnName := runtime.FuncForPC(pc).Name()

	// 简单实现：按空格分割
	// TODO: 更复杂的命令解析可以考虑使用 shlex
	parts := strings.Fields(cmdStr)
	log.Debug(fnName, "parts", parts)
	return parts
}

// ToolExecutor 外部工具执行器接口
type ToolExecutor interface {
	Name() string
	Template() string
	IsAvailable() bool
}

// opencodeExecutor opencode 工具
type opencodeExecutor struct{}

func (e *opencodeExecutor) Name() string     { return "opencode" }
func (e *opencodeExecutor) Template() string { return "opencode \"{task}\"" }
func (e *opencodeExecutor) IsAvailable() bool {
	cmd := exec.Command("which", "opencode")
	return cmd.Run() == nil
}

// CursorExecutor Cursor 工具
type CursorExecutor struct{}

func (e *CursorExecutor) Name() string     { return "cursor" }
func (e *CursorExecutor) Template() string { return "cursor ai \"{task}\"" }
func (e *CursorExecutor) IsAvailable() bool {
	cmd := exec.Command("which", "cursor")
	return cmd.Run() == nil
}

// ClaudeExecutor Claude 工具
type ClaudeExecutor struct{}

func (e *ClaudeExecutor) Name() string     { return "claude" }
func (e *ClaudeExecutor) Template() string { return "opencode \"{task}\"" }
func (e *ClaudeExecutor) IsAvailable() bool {
	// Claude uses opencode command as well
	cmd := exec.Command("which", "opencode")
	return cmd.Run() == nil
}

// ConfiguredExecutor 可配置的执行器
type ConfiguredExecutor struct {
	name     string
	template string
	command  string
}

func (e *ConfiguredExecutor) Name() string     { return e.name }
func (e *ConfiguredExecutor) Template() string { return e.template }
func (e *ConfiguredExecutor) IsAvailable() bool {
	if e.command == "" {
		// 使用默认命令
		cmd := exec.Command("which", e.name)
		return cmd.Run() == nil
	}
	// 使用配置的命令
	cmd := exec.Command("which", e.command)
	return cmd.Run() == nil
}

func NewopencodeExecutor() *opencodeExecutor {
	return &opencodeExecutor{}
}

func NewCursorExecutor() *CursorExecutor {
	return &CursorExecutor{}
}

func NewClaudeExecutor() *ClaudeExecutor {
	return &ClaudeExecutor{}
}

func NewConfiguredExecutor(name, template, command string) *ConfiguredExecutor {
	return &ConfiguredExecutor{
		name:     name,
		template: template,
		command:  command,
	}
}

// ConcurrentSafe returns false - code dev tools modify files and should be sequential
func (t *CodeDevTool) ConcurrentSafe() bool {
	return false
}

// IsEnabled returns whether the tool should be enabled based on config
func (t *CodeDevTool) IsEnabled() bool {
	return len(t.executors) > 0
}
