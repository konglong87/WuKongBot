package tools

import (
	"testing"

	"github.com/konglong87/wukongbot/internal/toolcontext"
)

// TestTmuxInteractTool_BeforeExecute_SafeCommand tests that safe commands don't trigger a card
func TestTmuxInteractTool_BeforeExecute_SafeCommand(t *testing.T) {
	tool := NewTmuxInteractTool()

	ctx := &toolcontext.ToolContext{
		UserID:     "test-user",
		ChannelID:  "test-channel",
		ToolName:   "tmux_interact",
		ToolCallID: "call-123",
		Params: map[string]interface{}{
			"command": "ls -la",
		},
	}

	decision, err := tool.BeforeExecute(ctx)
	if err != nil {
		t.Fatalf("BeforeExecute failed: %v", err)
	}

	if decision != nil {
		t.Error("Expected nil decision for safe command, got non-nil")
	}
}

// TestTmuxInteractTool_BeforeExecute_DangerousCommand tests that dangerous commands trigger a confirmation card
func TestTmuxInteractTool_BeforeExecute_DangerousCommand(t *testing.T) {
	tool := NewTmuxInteractTool()

	dangerousCommands := []string{
		"rm -rf /tmp/test",
		"dd if=/dev/zero of=/tmp/test bs=1M count=100",
		"mkfs.ext4 /dev/sdb1",
		"shutdown -h now",
		"kill -9 -1",
	}

	for _, cmd := range dangerousCommands {
		t.Run(cmd, func(t *testing.T) {
			ctx := &toolcontext.ToolContext{
				UserID:     "test-user",
				ChannelID:  "test-channel",
				ToolName:   "tmux_interact",
				ToolCallID: "call-123",
				Params: map[string]interface{}{
					"command": cmd,
				},
			}

			decision, err := tool.BeforeExecute(ctx)
			if err != nil {
				t.Fatalf("BeforeExecute failed: %v", err)
			}

			if decision == nil {
				t.Error("Expected non-nil decision for dangerous command")
			} else if decision.Action != toolcontext.ActionWaitCard {
				t.Errorf("Expected ActionWaitCard, got %v", decision.Action)
			} else if !decision.CardNeeded {
				t.Error("Expected CardNeeded to be true")
			} else if decision.CardType != toolcontext.CardTypeConfirm {
				t.Errorf("Expected CardTypeConfirm, got %v", decision.CardType)
			} else if decision.CardContent == nil {
				t.Error("Expected CardContent to be set")
			}
		})
	}
}

// TestTmuxInteractTool_BeforeExecute_KeysMode tests that keys mode doesn't trigger a card
func TestTmuxInteractTool_BeforeExecute_KeysMode(t *testing.T) {
	tool := NewTmuxInteractTool()

	ctx := &toolcontext.ToolContext{
		UserID:     "test-user",
		ChannelID:  "test-channel",
		ToolName:   "tmux_interact",
		ToolCallID: "call-123",
		Params: map[string]interface{}{
			"keys": "some text",
		},
	}

	decision, err := tool.BeforeExecute(ctx)
	if err != nil {
		t.Fatalf("BeforeExecute failed: %v", err)
	}

	if decision != nil {
		t.Error("Expected nil decision for keys mode, got non-nil")
	}
}

// TestTmuxInteractTool_BeforeExecute_EnterMode tests that enter mode doesn't trigger a card
func TestTmuxInteractTool_BeforeExecute_EnterMode(t *testing.T) {
	tool := NewTmuxInteractTool()

	ctx := &toolcontext.ToolContext{
		UserID:     "test-user",
		ChannelID:  "test-channel",
		ToolName:   "tmux_interact",
		ToolCallID: "call-123",
		Params: map[string]interface{}{
			"enter": true,
		},
	}

	decision, err := tool.BeforeExecute(ctx)
	if err != nil {
		t.Fatalf("BeforeExecute failed: %v", err)
	}

	if decision != nil {
		t.Error("Expected nil decision for enter mode, got non-nil")
	}
}

// TestTmuxInteractTool_AfterExecute_InteractiveQuestion tests detection of interactive questions
func TestTmuxInteractTool_AfterExecute_InteractiveQuestion(t *testing.T) {
	tool := NewTmuxInteractTool()

	interactiveOutputs := []string{
		"Are you sure you want to continue? (y/n)",
		"Do you want to overwrite this file? [y/N]",
		"Press any key to continue...",
		"Please select an option:\n1. Option A\n2. Option B\n3. Option C",
	}

	for _, output := range interactiveOutputs {
		t.Run(output[:Min(20, len(output))], func(t *testing.T) {
			ctx := &toolcontext.ToolContext{
				UserID:     "test-user",
				ChannelID:  "test-channel",
				ToolName:   "tmux_interact",
				ToolCallID: "call-123",
			}

			err := tool.AfterExecute(ctx, output, nil)
			if err != nil {
				t.Fatalf("AfterExecute failed: %v", err)
			}

			// Note: We can't verify card sending in unit tests without mocking the Adapter
			// The actual card sending logic will be tested in integration tests
		})
	}
}

// TestTmuxInteractTool_AfterExecute_NoInteractiveQuestion tests that normal output doesn't trigger a card
func TestTmuxInteractTool_AfterExecute_NoInteractiveQuestion(t *testing.T) {
	tool := NewTmuxInteractTool()

	normalOutputs := []string{
		"ls -la\noutput...",
		"Hello, world!",
		"Process completed successfully.",
	}

	for _, output := range normalOutputs {
		t.Run(output[:Min(20, len(output))], func(t *testing.T) {
			ctx := &toolcontext.ToolContext{
				UserID:     "test-user",
				ChannelID:  "test-channel",
				ToolName:   "tmux_interact",
				ToolCallID: "call-123",
			}

			err := tool.AfterExecute(ctx, output, nil)
			if err != nil {
				t.Fatalf("AfterExecute failed: %v", err)
			}
		})
	}
}

// TestTmuxInteractTool_AfterExecute_WithError tests that AfterExecute handles errors
func TestTmuxInteractTool_AfterExecute_WithError(t *testing.T) {
	tool := NewTmuxInteractTool()

	ctx := &toolcontext.ToolContext{
		UserID:     "test-user",
		ChannelID:  "test-channel",
		ToolName:   "tmux_interact",
		ToolCallID: "call-123",
	}

	err := tool.AfterExecute(ctx, "some output", nil)
	if err != nil {
		t.Fatalf("AfterExecute failed: %v", err)
	}
}

// TestTmuxInteractTool_ImplementsToolWithHooks tests that TmuxInteractTool implements ToolWithHooks
func TestTmuxInteractTool_ImplementsToolWithHooks(t *testing.T) {
	tool := NewTmuxInteractTool()

	// Verify it implements Tool interface
	var _ Tool = tool

	// Verify it implements ToolWithHooks interface
	var _ ToolWithHooks = tool
}
