package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/konglong87/wukongbot/internal/bus"
	"github.com/konglong87/wukongbot/internal/config"
	"github.com/konglong87/wukongbot/internal/providers"
	"github.com/konglong87/wukongbot/tools"
)

// SubagentConfig holds subagent configuration
type SubagentConfig struct {
	Workspace        string
	Model            string
	MaxTokens        int
	Temperature      float64
	MaxIterations    int
	SearchProvider   string // "brave" or "duckduckgo"
	BraveAPIKey      string
	ExecTimeout      int
	ExecRestrictToWs bool
	Identity         *config.IdentityConfig
	ImageTools       config.ImageToolsConfig // Image tools configuration
}

// subagentMgr manages background subagent execution
type subagentMgr struct {
	cfg      SubagentConfig
	bus      bus.MessageBus
	provider providers.LLMProvider
	mu       sync.RWMutex
	running  map[string]*subagentTask
}

// subagentTask represents a running subagent task
type subagentTask struct {
	ID       string
	Label    string
	Task     string
	OriginCh string
	OriginID string
	Cancel   context.CancelFunc
	Done     chan struct{}
}

// SubagentTask is the exported version of subagentTask for public use
type SubagentTask struct {
	ID       string
	Label    string
	Task     string
	OriginCh string
	OriginID string
	Done     chan struct{}
}

// NewSubagentMgr creates a new subagent manager
func NewSubagentMgr(bus bus.MessageBus, provider providers.LLMProvider, cfg SubagentConfig) *subagentMgr {
	return &subagentMgr{
		cfg:      cfg,
		bus:      bus,
		provider: provider,
		running:  make(map[string]*subagentTask),
	}
}

// Spawn implements SubagentManager interface
func (m *subagentMgr) Spawn(ctx context.Context, task, label, originChannel, originChatID string) (string, error) {
	taskID := uuid.New().String()[:8]
	if len(task) > 30 {
		label = task[:30] + "..."
	} else {
		label = task
	}

	subagent := &subagentTask{
		ID:       taskID,
		Label:    label,
		Task:     task,
		OriginCh: originChannel,
		OriginID: originChatID,
		Done:     make(chan struct{}),
	}

	subCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	subagent.Cancel = cancel

	m.mu.Lock()
	m.running[taskID] = subagent
	m.mu.Unlock()

	go m.runSubagent(subCtx, subagent)

	go func() {
		<-subagent.Done
		m.mu.Lock()
		delete(m.running, taskID)
		m.mu.Unlock()
	}()

	log.Info("Spawned subagent", "id", taskID, "label", label)
	return taskID, nil
}

// runSubagent executes the subagent task
func (m *subagentMgr) runSubagent(ctx context.Context, task *subagentTask) {
	defer close(task.Done)

	log.Info("Subagent starting", "id", task.ID, "task", task.Label)

	toolRegistry := tools.NewToolRegistry()
	toolRegistry.Register(tools.NewReadFileTool())
	toolRegistry.Register(tools.NewWriteFileTool(m.cfg.Workspace))
	toolRegistry.Register(tools.NewListDirTool())
	toolRegistry.Register(tools.NewExecTool(m.cfg.ExecTimeout, m.cfg.Workspace, m.cfg.ExecRestrictToWs))
	toolRegistry.Register(tools.NewWebSearchTool(m.cfg.SearchProvider, m.cfg.BraveAPIKey, 5))
	toolRegistry.Register(tools.NewWebFetchTool(50000))

	// Register image tools if configured
	if strings.Contains(m.cfg.Identity.Tools, "generate_image") {
		toolRegistry.Register(tools.NewImageGenerationTool(
			m.cfg.ImageTools.Generation.APIKey,
			m.cfg.ImageTools.Generation.APIBase,
			m.cfg.ImageTools.Generation.Model,
			m.cfg.ImageTools.Generation.Models,
		))
	}
	if strings.Contains(m.cfg.Identity.Tools, "analyze_image") {
		toolRegistry.Register(tools.NewImageAnalysisTool(
			m.cfg.ImageTools.Analysis.APIKey,
			m.cfg.ImageTools.Analysis.APIBase,
			m.cfg.ImageTools.Analysis.Model,
		))
	}

	systemPrompt := m.buildSubagentPrompt(task.Task)
	messages := []providers.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: task.Task},
	}

	iteration := 0
	var finalResult string

	for iteration < m.cfg.MaxIterations {
		iteration++

		toolDefs := getToolDefs(toolRegistry)

		req := providers.ChatRequest{
			Messages:    messages,
			Tools:       toolDefs,
			Model:       m.cfg.Model,
			MaxTokens:   m.cfg.MaxTokens,
			Temperature: m.cfg.Temperature,
		}

		response, err := m.provider.Chat(ctx, req)
		if err != nil {
			finalResult = fmt.Sprintf("Error: %v", err)
			m.announceResult(task, finalResult, "error")
			return
		}

		if len(response.ToolCalls) > 0 {
			messages = m.addToolCalls(messages, response.Content, response.ToolCalls)

			for _, tc := range response.ToolCalls {
				result, err := executeToolFromRegistry(toolRegistry, ctx, tc.Name, tc.Arguments)
				if err != nil {
					result = fmt.Sprintf("Error executing tool: %v", err)
				}
				messages = m.addToolResult(messages, tc.ID, tc.Name, result)
			}
		} else {
			finalResult = response.Content
			break
		}
	}

	if finalResult == "" {
		finalResult = "Task completed but no final response was generated."
	}

	log.Info("Subagent completed", "id", task.ID)
	m.announceResult(task, finalResult, "ok")
}

// announceResult announces the subagent result to the main agent
func (m *subagentMgr) announceResult(task *subagentTask, result, status string) {
	statusText := "completed successfully"
	if status == "error" {
		statusText = "failed"
	}

	announceContent := fmt.Sprintf("[Subagent '%s' %s]\n\nTask: %s\n\nResult:\n%s\n\nSummarize this naturally for the user. Keep it brief (1-2 sentences). Do not mention technical details like 'subagent' or task IDs.",
		task.Label, statusText, task.Task, result)

	msg := bus.InboundMessage{
		ChannelID: "system",
		SenderID:  "subagent",
		Content:   announceContent,
		Timestamp: time.Now(),
	}

	if err := m.bus.PublishInbound(context.Background(), msg); err != nil {
		log.Error("Failed to announce subagent result", "error", err)
	}

	log.Debug("Subagent announced result", "id", task.ID, "to", task.OriginCh+":"+task.OriginID)
}

// buildSubagentPrompt builds a focused system prompt for the subagent
func (m *subagentMgr) buildSubagentPrompt(task string) string {
	name := m.cfg.Identity.Name
	if name == "" {
		name = "subagent"
	}
	title := m.cfg.Identity.Title
	if title == "" {
		title = "a subagent spawned by the main agent"
	}
	prompt := m.cfg.Identity.Prompt
	if prompt == "" {
		prompt = "You are a subagent spawned by the main agent to complete a specific task."
	}

	return fmt.Sprintf("# %s\n\nYou are %s, %s\n\n## Your Task\n%s\n\n## Rules\n1. Stay focused - complete only the assigned task, nothing else\n2. Your final response will be reported back to the main agent\n3. Do not initiate conversations or take on side tasks\n4. Be concise but informative in your findings\n\n## What You Can Do\n- Read and write files in the workspace\n- Execute shell commands\n- Search the web and fetch web pages\n- Complete the task thoroughly\n\n## What You Cannot Do\n- Send messages directly to users (no message tool available)\n- Spawn other subagents\n- Access the main agent's conversation history\n\n## Workspace\nYour workspace is at: %s\n\nWhen you have completed the task, provide a clear summary of your findings or actions.",
		name, name, title, task, m.cfg.Workspace)
}

// GetRunningCount returns the number of currently running subagents
func (m *subagentMgr) GetRunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.running)
}

// Helper functions
func getToolDefs(registry *tools.ToolRegistry) []providers.ToolDefinition {
	defs := []providers.ToolDefinition{}
	for _, tool := range registry.List() {
		defs = append(defs, providers.ToolDefinition{Name: tool.Name(), Description: tool.Description(), Parameters: tool.Parameters()})
	}
	return defs
}

func (m *subagentMgr) addToolCalls(messages []providers.Message, content string, toolCalls []providers.ToolCall) []providers.Message {
	tcMap := make([]map[string]interface{}, len(toolCalls))
	for i, tc := range toolCalls {
		argsJSON, _ := json.Marshal(tc.Arguments)
		tcMap[i] = map[string]interface{}{
			"id":   tc.ID,
			"type": "function",
			"function": map[string]interface{}{
				"name":      tc.Name,
				"arguments": string(argsJSON),
			},
		}
	}
	messages = append(messages, providers.Message{Role: "assistant", Content: content})
	return messages
}

func (m *subagentMgr) addToolResult(messages []providers.Message, toolCallID, toolName, result string) []providers.Message {
	return append(messages, providers.Message{Role: "tool", Content: result, ToolID: toolCallID, ToolResult: result})
}

func executeToolFromRegistry(registry *tools.ToolRegistry, ctx context.Context, name string, args map[string]interface{}) (string, error) {
	tool, ok := registry.Get(name)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	return tool.Execute(ctx, args)
}
