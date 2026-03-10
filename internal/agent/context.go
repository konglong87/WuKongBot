package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/konglong87/wukongbot/internal/config"
	"github.com/konglong87/wukongbot/internal/providers"
)

// ContextBuilder builds the context (system prompt + messages) for the agent
type ContextBuilder struct {
	workspace string
	memory    *MemoryStore
	skills    *SkillsLoader
	identity  *config.IdentityConfig
}

// NewContextBuilder creates a new context builder
func NewContextBuilder(workspace string, identity *config.IdentityConfig) *ContextBuilder {
	return &ContextBuilder{
		workspace: workspace,
		memory:    NewMemoryStore(workspace),
		skills:    NewSkillsLoader(workspace),
		identity:  identity,
	}
}

// BuildSystemPrompt builds the system prompt from bootstrap files, memory, and skills
func (b *ContextBuilder) BuildSystemPrompt(skillNames []string) string {
	parts := []string{}

	parts = append(parts, b.getIdentity())

	bootstrap := b.loadBootstrapFiles()
	if bootstrap != "" {
		parts = append(parts, bootstrap)
	}

	memory := b.memory.GetMemoryContext()
	if memory != "" {
		parts = append(parts, "# Memory\n\n"+memory)
	}

	alwaysSkills := b.skills.GetAlwaysSkills()
	if len(alwaysSkills) > 0 {
		alwaysContent := b.skills.LoadSkillsForContext(alwaysSkills)
		if alwaysContent != "" {
			parts = append(parts, "# Active Skills\n\n"+alwaysContent)
		}
	}

	skillsSummary := b.skills.BuildSkillsSummary()
	if skillsSummary != "" {
		parts = append(parts, "# Skills\n\nThe following skills extend your capabilities.\n\n## CRITICAL: How to Use Skills\n\n**YOU MUST use the read_file tool to load a skill BEFORE using it.** Do NOT attempt to use skills without first reading their SKILL.md file.\n\n### Steps to use a skill:\n\n1. **First**: Read the skill file using read_file:\n   read_file(path=\"skills/{skill-name}/SKILL.md\")\n\n2. **Then**: Use the skill based on the guidance in the file\n\n### Example:\n- User asks: \"Help me with GitHub issues\"\n- You MUST call: read_file(path=\"skills/github/SKILL.md\")\n- Then use the GitHub skill as described\n\n### Important Rules:\n- Do NOT load all skills at once - only load the one you need\\n- Do NOT attempt to use skills without reading their documentation\\n- If a skill requires external tools (like gh), they must be installed\\n\n"+skillsSummary)
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// BuildMessages builds the complete message list for an LLM call
func (b *ContextBuilder) BuildMessages(history []providers.Message, currentMessage string, skillNames, media []string, channel, chatID string) []providers.Message {
	messages := []providers.Message{}

	systemPrompt := b.BuildSystemPrompt(skillNames)
	if channel != "" && chatID != "" {
		systemPrompt += "\n\n## Current Session\nChannel: " + channel + "\nChat ID: " + chatID
	}
	messages = append(messages, providers.Message{Role: "system", Content: systemPrompt})

	messages = append(messages, history...)

	//messages = append(messages, providers.Message{Role: "user", Content: currentMessage})

	return messages
}

// AddToolResult adds a tool result to the message list
func (b *ContextBuilder) AddToolResult(messages []providers.Message, toolCallID, toolName, result string) []providers.Message {
	messages = append(messages, providers.Message{
		Role:       "tool",
		Content:    result,
		ToolID:     toolCallID,
		ToolResult: result,
	})
	return messages
}

// AddAssistantMessage adds an assistant message to the message list
func (b *ContextBuilder) AddAssistantMessage(messages []providers.Message, content string, toolCalls []providers.ToolCall) []providers.Message {
	msg := providers.Message{Role: "assistant", Content: content}
	messages = append(messages, msg)
	return messages
}

// getIdentity returns the core identity section
func (b *ContextBuilder) getIdentity() string {
	now := time.Now().Format("2006-01-02 15:04 (Monday)")
	workspacePath := b.workspace

	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macOS"
	}
	runtimeInfo := osName + " " + runtime.GOARCH

	// Use configured identity or fallback to defaults
	name := b.identity.Name
	if name == "" {
		name = "wukongbot"
	}

	title := b.identity.Title
	if title == "" {
		title = "a helpful AI assistant"
	}

	prompt := b.identity.Prompt
	if prompt == "" {
		prompt = "You have access to tools that allow you to:\n- Read, write, and edit files\n- Execute shell commands\n- Search the web and fetch web pages\n- Send messages to users on chat channels\n- Spawn subagents for complex background tasks"
	}

	tools := b.identity.Tools
	if tools == "" {
		tools = "read_file, write_file, edit_file, list_files, exec, web_search, web_fetch, message, generate_image, analyze_image"
	}

	behavior := b.identity.Behavior
	if behavior == "" {
		behavior = "IMPORTANT: When responding to direct questions or conversations, reply directly with your text response.\nOnly use the 'message' tool when you need to send a message to a specific chat channel (like WhatsApp).\nFor normal conversation, just respond with text - do not call the message tool.\n\nAlways be helpful, accurate, and concise. When using tools, explain what you're doing.\nWhen remembering something, write to " + workspacePath + "/memory/MEMORY.md"
	}

	soul := b.identity.Soul
	if soul != "" {
		behavior += "\n\n## Soul (性格)\n" + soul
	}

	return fmt.Sprintf("# %s 🐈🐶\n\nYou are %s, %s\n\n%s\n\n## Current Time\n%s\n\n## Runtime\n%s, Go %s\n\n## Workspace\nYour workspace is at: %s\n- Memory files: %s/memory/MEMORY.md\n- Daily notes: %s/memory/YYYY-MM-DD.md\n- Custom skills: %s/skills/{skill-name}/SKILL.md\n\n%s",
		name, name, title, prompt, now, runtimeInfo, runtime.Version(), workspacePath, workspacePath, workspacePath, workspacePath, behavior)
}

// loadBootstrapFiles loads all bootstrap files from workspace and returns their content
func (b *ContextBuilder) loadBootstrapFiles() string {
	bootstrapFiles := []string{"AGENTS.md", "SOUL.md", "USER.md", "TOOLS.md", "IDENTITY.md"}
	parts := []string{}

	for _, filename := range bootstrapFiles {
		filePath := filepath.Join(b.workspace, filename)
		content, err := os.ReadFile(filePath)
		if err == nil {
			parts = append(parts, "## "+filename+"\n\n"+string(content))
		}
	}

	return strings.Join(parts, "\n\n")
}
