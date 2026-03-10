package hooks

import (
	"context"
	"encoding/json"
	"time"
)

// HookEventType defines the type of hook event
type HookEventType string

const (
	// PreToolUse fires before a tool is executed
	PreToolUse HookEventType = "PreToolUse"
	// PostToolUse fires after a tool succeeds
	PostToolUse HookEventType = "PostToolUse"
	// PostToolUseFailure fires after a tool fails
	PostToolUseFailure HookEventType = "PostToolUseFailure"
	// SessionStart fires when a session begins
	SessionStart HookEventType = "SessionStart"
	// SessionEnd fires when a session ends
	SessionEnd HookEventType = "SessionEnd"
	// UserPromptSubmit fires when user submits a prompt
	UserPromptSubmit HookEventType = "UserPromptSubmit"
	// CodeDevProgress fires during code development task execution
	CodeDevProgress HookEventType = "CodeDevProgress"
)

// HookDecision is the decision returned by a hook
type HookDecision string

const (
	// HookAllow allows the operation to proceed
	HookAllow HookDecision = "allow"
	// HookDeny blocks the operation
	HookDeny HookDecision = "deny"
	// HookModify modifies the parameters and proceeds
	HookModify HookDecision = "modify"
)

// HookInput is the input provided to a hook
type HookInput struct {
	Event       HookEventType          `json:"event"`
	ToolName    string                 `json:"tool_name,omitempty"`
	ToolInput   map[string]interface{} `json:"tool_input,omitempty"`
	Result      string                 `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	// Code development specific fields
	TaskID      string                 `json:"task_id,omitempty"`
	CodingTool  string                 `json:"coding_tool,omitempty"`
	Progress    float64                `json:"progress,omitempty"`
	CurrentFile string                 `json:"current_file,omitempty"`
	Files       []string               `json:"files,omitempty"`
}

// HookOutput is the output returned by a hook
type HookOutput struct {
	Decision HookDecision           `json:"decision"`
	Reason   string                 `json:"reason,omitempty"`
	Message  string                 `json:"message,omitempty"`
	Modified map[string]interface{} `json:"modified,omitempty"`
}

// Hook represents a hook that can be called at specific events
type Hook interface {
	// Name returns the hook name
	Name() string

	// EventType returns the event type this hook listens to
	EventType() HookEventType

	// Matcher returns the regex pattern for matching (e.g., tool name)
	// Empty string means match all
	Matcher() string

	// Execute runs the hook with the given input and returns the decision
	Execute(ctx context.Context, input HookInput) (HookOutput, error)

	// Timeout returns the maximum time to allow for hook execution
	Timeout() time.Duration
}

// BaseHook provides common hook functionality
type BaseHook struct {
	name      string
	eventType HookEventType
	matcher   string
	timeout   time.Duration
}

// NewBaseHook creates a new base hook
func NewBaseHook(name string, eventType HookEventType, matcher string, timeout time.Duration) *BaseHook {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &BaseHook{
		name:      name,
		eventType: eventType,
		matcher:   matcher,
		timeout:   timeout,
	}
}

// Name returns the hook name
func (h *BaseHook) Name() string {
	return h.name
}

// EventType returns the event type
func (h *BaseHook) EventType() HookEventType {
	return h.eventType
}

// Matcher returns the matcher pattern
func (h *BaseHook) Matcher() string {
	return h.matcher
}

// Timeout returns the timeout duration
func (h *BaseHook) Timeout() time.Duration {
	return h.timeout
}

// InlineHook is a hook defined by inline Go code
type InlineHook struct {
	*BaseHook
	handler func(context.Context, HookInput) (HookOutput, error)
}

// NewInlineHook creates a new inline hook with a Go handler function
func NewInlineHook(name string, eventType HookEventType, matcher string, timeout time.Duration, handler func(context.Context, HookInput) (HookOutput, error)) *InlineHook {
	return &InlineHook{
		BaseHook: NewBaseHook(name, eventType, matcher, timeout),
		handler:  handler,
	}
}

// Execute runs the inline hook handler
func (h *InlineHook) Execute(ctx context.Context, input HookInput) (HookOutput, error) {
	return h.handler(ctx, input)
}

// CommandHook is a hook that executes a shell command
type CommandHook struct {
	*BaseHook
	command string
}

// NewCommandHook creates a new command hook
func NewCommandHook(name string, eventType HookEventType, matcher string, timeout time.Duration, command string) *CommandHook {
	return &CommandHook{
		BaseHook: NewBaseHook(name, eventType, matcher, timeout),
		command:  command,
	}
}

// Execute runs the command hook
func (h *CommandHook) Execute(ctx context.Context, input HookInput) (HookOutput, error) {
	// This will be implemented in the next stage
	// For now, return a placeholder
	return HookOutput{
		Decision: HookAllow,
	}, nil
}

// Command returns the command string
func (h *CommandHook) Command() string {
	return h.command
}

// HookConfig represents a hook configuration from YAML
type HookConfig struct {
	Name    string                 `yaml:"name"`
	Matcher string                 `yaml:"matcher"`
	Type    string                 `yaml:"type"`    // "inline" or "command"
	Code    string                 `yaml:"code"`    // inline Go code
	Command string                 `yaml:"command"` // shell command
	Timeout int                    `yaml:"timeout"` // seconds (default: 10)
	Data    map[string]interface{} `yaml:"data"`    // additional data
}

// ToJSON converts HookInput to JSON
func (h *HookInput) ToJSON() ([]byte, error) {
	return json.Marshal(h)
}

// FromJSON creates HookInput from JSON
func (h *HookInput) FromJSON(data []byte) error {
	return json.Unmarshal(data, h)
}

// ToJSON converts HookOutput to JSON
func (h *HookOutput) ToJSON() ([]byte, error) {
	return json.Marshal(h)
}

// FromJSON creates HookOutput from JSON
func (h *HookOutput) FromJSON(data []byte) error {
	return json.Unmarshal(data, h)
}
