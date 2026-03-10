package tools

import (
	"context"
	"encoding/json"

	"github.com/konglong87/wukongbot/internal/toolcontext"
)

// Tool defines the interface for executable tools
type Tool interface {
	// Name returns the tool name
	Name() string

	// Description returns a human-readable description of the tool
	Description() string

	// Parameters returns the JSON schema for the tool parameters
	Parameters() json.RawMessage

	// Execute runs the tool with the given arguments and returns the result
	Execute(ctx context.Context, args map[string]interface{}) (string, error)

	// ConcurrentSafe returns whether the tool can be executed concurrently with other tools
	// Tools that modify shared state (e.g., file writes) should return false
	ConcurrentSafe() bool
}

// ToolWithHooks identifies tools that implement execution hooks
// Tools implementing this interface can intercept execution before and after
// the actual Execute method is called, allowing for interactive cards, validation,
// and other custom behaviors.
type ToolWithHooks interface {
	Tool

	// BeforeExecute is called before the tool's Execute method.
	// It returns a ToolDecision that controls whether to continue, skip, or cancel execution.
	// This is useful for:
	// - Showing confirmation cards before dangerous operations
	// - Validating parameters before execution
	// - Requesting user input through interactive cards
	BeforeExecute(ctx *toolcontext.ToolContext) (*toolcontext.ToolDecision, error)

	// AfterExecute is called after the tool's Execute method completes.
	// It receives the execution context, result, and any error.
	// This is useful for:
	// - Post-execution validation
	// - Updating interactive cards with results
	// - Logging or analytics
	// - Detecting interactive prompts in output
	AfterExecute(ctx *toolcontext.ToolContext, result string, err error) error
}

// ToolRegistry manages available tools
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// List returns all registered tools
func (r *ToolRegistry) List() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// Names returns all registered tool names
func (r *ToolRegistry) Names() []string {
	result := make([]string, 0, len(r.tools))
	for name := range r.tools {
		result = append(result, name)
	}
	return result
}

// Has checks if a tool is registered
func (r *ToolRegistry) Has(name string) bool {
	_, exists := r.tools[name]
	return exists
}
