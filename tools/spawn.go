package tools

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// SpawnResult represents the result of a spawned subagent
type SpawnResult struct {
	TaskID string
	Label  string
	Result string
	Error  error
	Done   chan struct{}
}

// SpawnTool spawns background subagents
type SpawnTool struct {
	manager       SubagentManager
	mu            sync.RWMutex
	originChannel string
	originChatID  string
}

// SubagentManager interface for subagent operations
type SubagentManager interface {
	Spawn(ctx context.Context, task, label, originChannel, originChatID string) (string, error)
}

// NewSpawnTool creates a new spawn tool
func NewSpawnTool(manager SubagentManager) *SpawnTool {
	return &SpawnTool{
		manager: manager,
	}
}

// Name returns the tool name
func (t *SpawnTool) Name() string {
	return "spawn"
}

// Description returns the tool description
func (t *SpawnTool) Description() string {
	return "Spawn a subagent to handle a task in the background. Use this for complex or time-consuming tasks that can run independently. The subagent will complete the task and report back when done."
}

// Parameters returns the JSON schema for parameters
func (t *SpawnTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"task": {
				"type": "string",
				"description": "The task for the subagent to complete"
			},
			"label": {
				"type": "string",
				"description": "Optional short label for the task (for display)"
			}
		},
		"required": ["task"]
	}`)
}

// SetContext sets the origin context for subagent announcements
func (t *SpawnTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.originChannel = channel
	t.originChatID = chatID
}

// Execute spawns the subagent
func (t *SpawnTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	task, ok := args["task"].(string)
	if !ok {
		return "Error: task is required", nil
	}

	label := ""
	if l, ok := args["label"].(string); ok {
		label = l
	}

	t.mu.RLock()
	originChannel := t.originChannel
	originChatID := t.originChatID
	manager := t.manager
	t.mu.RUnlock()

	if manager == nil {
		return "Error: subagent manager not configured", nil
	}

	taskID, err := manager.Spawn(ctx, task, label, originChannel, originChatID)
	if err != nil {
		return "Error spawning subagent: " + err.Error(), nil
	}

	if label != "" {
		return "Spawned subagent for task '" + label + "' (id: " + taskID + ")", nil
	}
	return "Spawned subagent (id: " + taskID + ")", nil
}

// ConcurrentSafe returns true - spawning subagents is stateless and safe to run concurrently
func (t *SpawnTool) ConcurrentSafe() bool {
	return true
}

// SubagentFuture wraps an async subagent result
type SubagentFuture struct {
	Result *SpawnResult
	once   sync.Once
}

// NewSubagentFuture creates a new subagent future
func NewSubagentFuture() *SubagentFuture {
	return &SubagentFuture{
		Result: &SpawnResult{
			Done: make(chan struct{}),
		},
	}
}

// Wait waits for the subagent to complete
func (f *SubagentFuture) Wait(timeout time.Duration) error {
	select {
	case <-f.Result.Done:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}

// SetResult sets the result when the subagent completes
func (f *SubagentFuture) SetResult(result string, err error) {
	f.once.Do(func() {
		f.Result.Result = result
		f.Result.Error = err
		close(f.Result.Done)
	})
}
