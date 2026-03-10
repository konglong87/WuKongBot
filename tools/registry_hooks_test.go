package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/konglong87/wukongbot/internal/toolcontext"
)

// mockToolWithHooks is a mock tool that implements ToolWithHooks
type mockToolWithHooks struct {
	beforeCalled bool
	afterCalled  bool
	beforeResult *toolcontext.ToolDecision
	afterError   error
}

func (m *mockToolWithHooks) Name() string {
	return "mock_tool_with_hooks"
}

func (m *mockToolWithHooks) Description() string {
	return "Mock tool with hooks for testing"
}

func (m *mockToolWithHooks) Parameters() json.RawMessage {
	return nil
}

func (m *mockToolWithHooks) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return "mock result", nil
}

func (m *mockToolWithHooks) ConcurrentSafe() bool {
	return true
}

func (m *mockToolWithHooks) BeforeExecute(ctx *toolcontext.ToolContext) (*toolcontext.ToolDecision, error) {
	m.beforeCalled = true
	return m.beforeResult, nil
}

func (m *mockToolWithHooks) AfterExecute(ctx *toolcontext.ToolContext, result string, err error) error {
	m.afterCalled = true
	return m.afterError
}

// mockToolWithoutHooks is a mock tool that doesn't implement hooks
type mockToolWithoutHooks struct{}

func (m *mockToolWithoutHooks) Name() string {
	return "mock_tool_without_hooks"
}

func (m *mockToolWithoutHooks) Description() string {
	return "Mock tool without hooks for testing"
}

func (m *mockToolWithoutHooks) Parameters() json.RawMessage {
	return nil
}

func (m *mockToolWithoutHooks) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return "mock result", nil
}

func (m *mockToolWithoutHooks) ConcurrentSafe() bool {
	return true
}

// TestToolWithHooksInterfaceDetection tests that tools implementing hooks are detected correctly
func TestToolWithHooksInterfaceDetection(t *testing.T) {
	var toolWithHooks Tool = &mockToolWithHooks{}
	var toolWithoutHooks Tool = &mockToolWithoutHooks{}

	// Test tool with hooks implements ToolWithHooks
	_, ok := toolWithHooks.(ToolWithHooks)
	if !ok {
		t.Error("Expected tool with hooks to implement ToolWithHooks interface")
	}

	// Test tool without hooks does not implement ToolWithHooks
	_, ok = toolWithoutHooks.(ToolWithHooks)
	if ok {
		t.Error("Expected tool without hooks to NOT implement ToolWithHooks interface")
	}

	// Both should implement Tool interface
	_, ok = toolWithHooks.(Tool)
	if !ok {
		t.Error("Expected tool with hooks to implement Tool interface")
	}

	_, ok = toolWithoutHooks.(Tool)
	if !ok {
		t.Error("Expected tool without hooks to implement Tool interface")
	}
}

// TestToolWithHooksInvocation tests that BeforeExecute and AfterExecute are called
func TestToolWithHooksInvocation(t *testing.T) {
	tool := &mockToolWithHooks{
		beforeResult: &toolcontext.ToolDecision{
			Action: toolcontext.ActionContinue,
		},
	}

	ctx := &toolcontext.ToolContext{
		UserID:    "test-user",
		ChannelID: "test-channel",
		ToolName:  "mock_tool_with_hooks",
		Params:    map[string]interface{}{"key": "value"},
	}

	// Call BeforeExecute
	decision, err := tool.BeforeExecute(ctx)
	if err != nil {
		t.Errorf("BeforeExecute returned error: %v", err)
	}
	if !tool.beforeCalled {
		t.Error("BeforeExecute was not called")
	}
	if decision.Action != toolcontext.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", decision.Action)
	}

	// Call AfterExecute
	err = tool.AfterExecute(ctx, "test result", nil)
	if err != nil {
		t.Errorf("AfterExecute returned error: %v", err)
	}
	if !tool.afterCalled {
		t.Error("AfterExecute was not called")
	}
}

// TestToolWithHooksDecisionSkip tests that tools can return Skip decision
func TestToolWithHooksDecisionSkip(t *testing.T) {
	tool := &mockToolWithHooks{
		beforeResult: &toolcontext.ToolDecision{
			Action: toolcontext.ActionSkip,
		},
	}

	ctx := &toolcontext.ToolContext{}

	decision, err := tool.BeforeExecute(ctx)
	if err != nil {
		t.Errorf("BeforeExecute returned error: %v", err)
	}
	if decision.Action != toolcontext.ActionSkip {
		t.Errorf("Expected ActionSkip, got %v", decision.Action)
	}
}

// TestToolWithHooksDecisionCancel tests that tools can return Cancel decision
func TestToolWithHooksDecisionCancel(t *testing.T) {
	tool := &mockToolWithHooks{
		beforeResult: &toolcontext.ToolDecision{
			Action: toolcontext.ActionCancel,
		},
	}

	ctx := &toolcontext.ToolContext{}

	decision, err := tool.BeforeExecute(ctx)
	if err != nil {
		t.Errorf("BeforeExecute returned error: %v", err)
	}
	if decision.Action != toolcontext.ActionCancel {
		t.Errorf("Expected ActionCancel, got %v", decision.Action)
	}
}

// TestToolWithHooksDecisionWaitCard tests that tools can return WaitCard decision with card content
func TestToolWithHooksDecisionWaitCard(t *testing.T) {
	cardContent := &toolcontext.CardContent{
		Title:       "Test Card",
		Description: "This is a test card",
		Question:    "Are you sure?",
		Options: []*toolcontext.CardOption{
			{Label: "Yes", Value: "yes"},
			{Label: "No", Value: "no"},
		},
		WarnLevel: "medium",
	}

	tool := &mockToolWithHooks{
		beforeResult: &toolcontext.ToolDecision{
			Action:      toolcontext.ActionWaitCard,
			CardNeeded:  true,
			CardType:    toolcontext.CardTypeConfirm,
			CardContent: cardContent,
			SessionID:   "test-session-id",
			Timeout:     300000000000, // 5 minutes
		},
	}

	ctx := &toolcontext.ToolContext{}

	decision, err := tool.BeforeExecute(ctx)
	if err != nil {
		t.Errorf("BeforeExecute returned error: %v", err)
	}
	if decision.Action != toolcontext.ActionWaitCard {
		t.Errorf("Expected ActionWaitCard, got %v", decision.Action)
	}
	if !decision.CardNeeded {
		t.Error("Expected CardNeeded to be true")
	}
	if decision.CardContent == nil {
		t.Error("Expected CardContent to be set")
	}
	if decision.CardContent.Title != "Test Card" {
		t.Errorf("Expected card title 'Test Card', got '%s'", decision.CardContent.Title)
	}
	if len(decision.CardContent.Options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(decision.CardContent.Options))
	}
}

// TestToolWithHooksAfterExecuteError tests that AfterExecute can handle errors
func TestToolWithHooksAfterExecuteError(t *testing.T) {
	expectedError := "after execute error"
	tool := &mockToolWithHooks{
		afterError: errors.New(expectedError),
	}

	ctx := &toolcontext.ToolContext{}

	err := tool.AfterExecute(ctx, "test result", nil)
	if err == nil {
		t.Error("Expected AfterExecute to return error")
	}
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}

// TestToolWithHooksAfterExecuteWithToolError tests that AfterExecute receives tool execution errors
func TestToolWithHooksAfterExecuteWithToolError(t *testing.T) {
	tool := &mockToolWithHooks{}

	ctx := &toolcontext.ToolContext{}
	toolError := errors.New("tool execution failed")

	// AfterExecute should receive the tool error
	err := tool.AfterExecute(ctx, "", toolError)
	if err != nil {
		t.Errorf("AfterExecute should not return error when receiving tool error: %v", err)
	}
	if !tool.afterCalled {
		t.Error("AfterExecute was not called")
	}
}

// TestToolWithHooksBackwardCompatibility tests that existing tools without hooks still work
func TestToolWithHooksBackwardCompatibility(t *testing.T) {
	var tool Tool = &mockToolWithoutHooks{}

	// Tool should still implement Tool interface
	if _, ok := tool.(Tool); !ok {
		t.Error("Expected backward compatible tool to implement Tool interface")
	}

	// Tool should NOT implement ToolWithHooks
	if _, ok := tool.(ToolWithHooks); ok {
		t.Error("Expected backward compatible tool to NOT implement ToolWithHooks interface")
	}

	// Execute should still work
	ctx := context.Background()
	result, err := tool.Execute(ctx, map[string]interface{}{})
	if err != nil {
		t.Errorf("Execute returned error: %v", err)
	}
	if result != "mock result" {
		t.Errorf("Expected result 'mock result', got '%s'", result)
	}
}
