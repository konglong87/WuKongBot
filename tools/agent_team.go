package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/konglong87/wukongbot/internal/agentteam"
	"github.com/konglong87/wukongbot/internal/providers"
)

// AgentTeamTool enables triggering agent team execution
type AgentTeamTool struct {
	provider    providers.LLMProvider
	coordinator *agentteam.TaskCoordinator
	taskQueue   *agentteam.TaskQueue
	timeout     time.Duration
}

// NewAgentTeamTool creates a new agent team tool
func NewAgentTeamTool(provider providers.LLMProvider, coordinator *agentteam.TaskCoordinator, queue *agentteam.TaskQueue, timeout time.Duration) *AgentTeamTool {
	return &AgentTeamTool{
		provider:    provider,
		coordinator: coordinator,
		taskQueue:   queue,
		timeout:     timeout,
	}
}

// Name returns the tool name
func (t *AgentTeamTool) Name() string {
	return "team_execute"
}

// Description returns the tool description
func (t *AgentTeamTool) Description() string {
	return "启动 agent team 协作完成复杂任务。当用户请求复杂任务（如开发完整项目）时，此工具会自动将任务分解为多个子任务，智能分配给不同的专业 agent（前端、后端、测试等）并行或串行执行。参数: task='任务描述', agents=['agent-frontend', 'agent-backend']（可选指定使用的 agents）"
}

// ConcurrentSafe returns whether the tool can be executed concurrently
func (t *AgentTeamTool) ConcurrentSafe() bool {
	return false // Agent team execution should not run concurrently
}

// Parameters returns the tool schema
func (t *AgentTeamTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"task": {
				"type": "string",
				"description": "任务描述，例如：'开发一个带登录功能的电商网站'"
			},
			"agents": {
				"type": "array",
				"items": {
					"type": "string",
					"enum": ["agent-frontend", "agent-backend", "agent-database", "agent-testing"],
					"description": "可选：指定要使用的 agent（可多个，逗号分隔）。不指定则由系统自动选择"
				}
			},
			"max_concurrent": {
				"type": "integer",
				"default": 3,
				"description": "最大并发执行数，默认3个 agent"
			}
		},
		"required": ["task"]
	}`)
}

// Execute executes the agent team task
func (t *AgentTeamTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	fnName := "AgentTeamTool Execute"
	log.Info(fnName, "executing_agent_team", "")

	// Parse parameters
	task, _ := args["task"].(string)
	agentsArg, _ := args["agents"]

	// Prepare task request
	request := agentteam.TaskRequest{
		Title:       task,
		Description: task,
		Type:        agentteam.TaskTypeDevelopment,
		Preferences: agentteam.TaskPreferences{
			MaxConcurrentTasks: t.getMaxConcurrent(args),
			Timeout:            int(t.timeout.Seconds()),
			AutoRetry:          false,
		},
	}

	// Parse agents preference if provided
	if agentsArg != nil {
		if agentsStr, ok := agentsArg.(string); ok && agentsStr != "" {
			agents := strings.Split(agentsStr, ",")
			for _, agentID := range agents {
				agentsStripped := strings.TrimSpace(agentID)
				request.Agents += agentsStripped
			}
		}
		log.Info(fnName, "agents_specified", "agents", request.Agents)
	}

	// Execute task through coordinator
	log.Info(fnName, "calling_coordinator", "", "max_concurrent", request.Preferences.MaxConcurrentTasks)

	result, err := t.coordinator.ExecuteTask(ctx, &request)
	if err != nil {
		return "", fmt.Errorf("agent team execution failed: %w", err)
	}

	// Format result with clear completion status
	var output strings.Builder

	if result.State == "completed" {
		// Task completed successfully - VERY CLEAR SUCCESS MESSAGE
		output.WriteString(fmt.Sprintf("🎉 **任务已完成！所有文件已创建到工作区**\n\n"))
		output.WriteString(fmt.Sprintf("**任务内容**：%s\n\n", result.Title))
		output.WriteString(fmt.Sprintf("**完成状态**：✅ 成功完成\n"))
		output.WriteString(fmt.Sprintf("**耗时**：%d 秒\n\n", result.Duration))
		output.WriteString(fmt.Sprintf("**执行说明**：%s\n\n", result.Summary))

		if len(result.Subtasks) > 0 {
			output.WriteString("**完成的子任务**：\n\n")
			for _, subtask := range result.Subtasks {
				if subtask.State == "completed" {
					output.WriteString(fmt.Sprintf("✅ %s\n", subtask.Title))
					if subtask.Result != "" {
						// Extract key info from result
						resultPreview := subtask.Result
						if len(resultPreview) > 200 {
							resultPreview = resultPreview[:200] + "..."
						}
						output.WriteString(fmt.Sprintf("   结果：%s\n\n", resultPreview))
					}
				}
			}
		}

		// MULTIPLE CLEAR SUCCESS INDICATORS FOR LLM
		output.WriteString(fmt.Sprintf("\n📁 **文件已创建**\n\n"))
		output.WriteString(fmt.Sprintf("[SUCCESS_COMPLETED] 任务100%%完成，所有文件已成功创建到工作区目录。不需要再创建任何文件，直接通知用户项目已完成即可。"))
	} else if result.State == "failed" {
		// Task failed
		output.WriteString(fmt.Sprintf("❌ **任务失败**\n\n"))
		output.WriteString(fmt.Sprintf("任务：%s\n\n", result.Title))
		output.WriteString(fmt.Sprintf("错误：%s\n\n", result.Error))
		output.WriteString(fmt.Sprintf("概要：%s\n\n", result.Summary))

		// Add failure indicator for LLM to recognize
		output.WriteString(fmt.Sprintf("[TASK_FAILED] 任务执行失败，需要通知用户错误信息。"))
	} else {
		// Partial completion
		output.WriteString(fmt.Sprintf("⚠️ **任务部分完成**\n\n"))
		output.WriteString(fmt.Sprintf("任务：%s\n\n", result.Title))
		output.WriteString(fmt.Sprintf("状态：%s\n\n", result.State))
		output.WriteString(fmt.Sprintf("概要：%s\n\n", result.Summary))

		if len(result.Subtasks) > 0 {
			output.WriteString("**子任务状态：**\n\n")
			for _, subtask := range result.Subtasks {
				icon := "⏳"
				if subtask.State == "completed" {
					icon = "✅"
				} else if subtask.State == "failed" {
					icon = "❌"
				}
				output.WriteString(fmt.Sprintf("%s %s（%s）\n", icon, subtask.Title, subtask.AssignedTo))
				if subtask.Error != "" {
					output.WriteString(fmt.Sprintf("   错误：%s\n", subtask.Error))
				}
			}
		}

		// Add partial completion indicator
		output.WriteString(fmt.Sprintf("\n[PARTIAL] 任务部分完成，某些子任务可能失败。"))
	}

	return output.String(), nil
}

// getMaxConcurrent gets max concurrent tasks from args
func (t *AgentTeamTool) getMaxConcurrent(args map[string]interface{}) int {
	if max, ok := args["max_concurrent"].(int); ok && max > 0 {
		return max
	}
	return 3 // Default
}
