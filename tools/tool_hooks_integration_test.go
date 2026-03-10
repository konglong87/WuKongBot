package tools

import (
	"context"
	"fmt"
	"testing"

	"github.com/konglong87/wukongbot/internal/toolcontext"
)

// TestToolWithHooksIntegration tests the complete ToolWithHooks interface integration
func TestToolWithHooksIntegration(t *testing.T) {
	// Create a tool that implements ToolWithHooks
	tool := &mockToolWithHooks{
		beforeResult: &toolcontext.ToolDecision{
			Action: toolcontext.ActionContinue,
		},
	}

	// Verify tool implements both interfaces
	var _ Tool = tool
	var _ ToolWithHooks = tool

	// Test BeforeExecute
	ctx := &toolcontext.ToolContext{
		UserID:    "test-user",
		ChannelID: "test-channel",
		ToolName:  "mock_tool_with_hooks",
		Params:    map[string]interface{}{"key": "value"},
	}

	decision, err := tool.BeforeExecute(ctx)
	if err != nil {
		t.Fatalf("BeforeExecute failed: %v", err)
	}
	if decision.Action != toolcontext.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", decision.Action)
	}

	// Test AfterExecute
	err = tool.AfterExecute(ctx, "test result", nil)
	if err != nil {
		t.Fatalf("AfterExecute failed: %v", err)
	}

	// Test tool execution
	result, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "mock result" {
		t.Errorf("Expected 'mock result', got '%s'", result)
	}

	fmt.Println("✓ ToolWithHooks integration test passed")
}
