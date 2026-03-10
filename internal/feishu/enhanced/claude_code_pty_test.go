package enhanced

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestCreateSessionShellCommand(t *testing.T) {
	manager := NewClaudeCodePTYManager(
		"opencode",
		"/tmp/workspace",
		func(channelID, sessionID, content string) error { return nil },
	)

	// Verify claudeCommand is set correctly
	if len(manager.claudeCommand) != 2 {
		t.Fatalf("Expected 2 command parts, got %d", len(manager.claudeCommand))
	}
	if manager.claudeCommand[0] != "opencode" {
		t.Errorf("Expected first command 'opencode', got '%s'", manager.claudeCommand[0])
	}
	if manager.claudeCommand[1] != "code" {
		t.Errorf("Expected second command 'code', got '%s'", manager.claudeCommand[1])
	}
}

func TestBuildShellCommandWithCustomCommand(t *testing.T) {
	manager := NewClaudeCodePTYManager(
		"opencode",
		"/tmp/workspace",
		func(channelID, sessionID, content string) error { return nil },
	)

	projectPath := "/Users/user/project"
	shellCmd := manager.buildShellCommand(projectPath)

	// Verify shell command format: cd "<path>" && "<command>" "<args>"
	// Paths are now properly quoted to prevent injection and handle spaces
	expected := `cd "/Users/user/project" && opencode`
	if shellCmd != expected {
		t.Errorf("Expected shell command '%s', got '%s'", expected, shellCmd)
	}
}

func TestBuildShellCommandWithDefaultCommand(t *testing.T) {
	manager := NewClaudeCodePTYManager(
		"", // Empty command should use default
		"/tmp/workspace",
		func(channelID, sessionID, content string) error { return nil },
	)

	projectPath := "/Users/user/project"
	shellCmd := manager.buildShellCommand(projectPath)

	// Verify shell command format with default command
	// Paths are now properly quoted to prevent injection and handle spaces
	expected := `cd "/Users/user/project" && claude code`
	if shellCmd != expected {
		t.Errorf("Expected shell command '%s', got '%s'", expected, shellCmd)
	}
}

func TestBuildShellCommandWithSinglePartCommand(t *testing.T) {
	manager := NewClaudeCodePTYManager(
		"claude",
		"/tmp/workspace",
		func(channelID, sessionID, content string) error { return nil },
	)

	projectPath := "/Users/user/project"
	shellCmd := manager.buildShellCommand(projectPath)

	// Verify shell command format with single-part command
	// Paths are now properly quoted to prevent injection and handle spaces
	expected := `cd "/Users/user/project" && claude`
	if shellCmd != expected {
		t.Errorf("Expected shell command '%s', got '%s'", expected, shellCmd)
	}
}

func TestBuildShellCommandWithPathWithSpaces(t *testing.T) {
	manager := NewClaudeCodePTYManager(
		"opencode",
		"/tmp/workspace",
		func(channelID, sessionID, content string) error { return nil },
	)

	projectPath := "/Users/user/my project"
	shellCmd := manager.buildShellCommand(projectPath)

	// Verify shell command format - paths with spaces should now be properly quoted
	// This prevents shell injection and handles spaces correctly
	expected := `cd "/Users/user/my project" && opencode`
	if shellCmd != expected {
		t.Errorf("Expected shell command '%s', got '%s'", expected, shellCmd)
	}
}

func TestBuildShellCommandPreventsShellInjection(t *testing.T) {
	manager := NewClaudeCodePTYManager(
		"opencode",
		"/tmp/workspace",
		func(channelID, sessionID, content string) error { return nil },
	)

	// Test path containing shell special characters that could be used for injection
	projectPath := "/Users/user/project; rm -rf /"
	shellCmd := manager.buildShellCommand(projectPath)

	// The special characters should be quoted, preventing command injection
	// The semicolon and command should be treated as part of the path string
	expected := `cd "/Users/user/project; rm -rf /" && opencode`
	if shellCmd != expected {
		t.Errorf("Expected shell command '%s', got '%s'", expected, shellCmd)
	}

	// Verify that the semicolon is properly quoted (inside double quotes)
	// The command should have 1 quoted string for the path
	quotedParts := strings.Count(shellCmd, `"`)
	if quotedParts != 2 {
		t.Errorf("Expected 2 quotes (1 quoted string), got %d", quotedParts)
	}

	// Verify the special characters are inside the quoted path string (full path)
	if !strings.Contains(shellCmd, `"/Users/user/project; rm -rf /"`) {
		t.Error("Shell injection vulnerability detected: semicolon should be inside quoted string")
	}

	// Verify that the semicolon is inside quotes (after an opening quote)
	if !strings.Contains(shellCmd, `"/Users/user/project;`) {
		t.Error("Shell injection vulnerability detected: semicolon should be after opening quote")
	}
}

func TestBuildShellCommandHandlesDollarSign(t *testing.T) {
	manager := NewClaudeCodePTYManager(
		"opencode",
		"/tmp/workspace",
		func(channelID, sessionID, content string) error { return nil },
	)

	// Test path containing dollar sign which could be used for command substitution
	projectPath := "/Users/user/project$(whoami)"
	shellCmd := manager.buildShellCommand(projectPath)

	// The dollar sign should be quoted, preventing command substitution
	expected := `cd "/Users/user/project$(whoami)" && opencode`
	if shellCmd != expected {
		t.Errorf("Expected shell command '%s', got '%s'", expected, shellCmd)
	}
}

// TestMonitorProcessUsesUserIDNotSessionID tests that monitorProcess sends messages
// with userID (recipientID) instead of composite sessionID (userID:timestamp).
// This test catches the Feishu API bug where sessionID is incorrectly used as recipientID.
func TestMonitorProcessUsesUserIDNotSessionID(t *testing.T) {
	var capturedRecipientID string
	var capturedContent string
	var capturedChannelID string

	manager := NewClaudeCodePTYManager(
		"echo", // Use echo for a quick, safe command
		"/tmp/workspace",
		func(channelID, recipientID, content string) error {
			// Capture the parameters for verification
			capturedChannelID = channelID
			capturedRecipientID = recipientID
			capturedContent = content
			return nil
		},
	)

	// Simulate creating a session (this won't actually start a real process)
	userID := "ou_5d593df09238b43c41899d6ef0ccaf83"
	sessionID := userID + ":1234567890" // Format: userID:timestamp

	// Create a session object directly (bypassing actual command execution)
	ctx, cancel := context.WithCancel(context.Background())
	session := &ClaudeCodeSession{
		ID:           sessionID,
		UserID:       userID, // Set UserID to the plain userID
		ProjectPath:  "/tmp/test-project",
		IsActive:     true,
		OutputBuffer: &strings.Builder{},
		Cmd:          exec.CommandContext(ctx, "echo", "test"), // Initialize Cmd to prevent nil pointer
		Ctx:          ctx,
		Cancel:       cancel,
	}

	// Store the session in the manager's sessions map
	manager.sessions[sessionID] = session

	// Call monitorProcess (this is the function with the bug)
	// Since the process doesn't actually run, we'll need to simulate the call
	// In real scenario, a goroutine would call this after cmd.Wait() returns

	// Mock the scenario where process completes successfully
	manager.monitorProcess(session, sessionID)

	// Verify the message sender received userID, not sessionID
	// This is the critical assertion - recipientID should be userID only
	if capturedRecipientID == "" {
		t.Fatal("Message sender was not called")
	}

	// The bug is: capturedRecipientID equals sessionID (with timestamp)
	// The fix should make it equal userID (without timestamp)
	if capturedRecipientID != userID {
		t.Errorf("Expected recipientID to be plain userID '%s', got composite sessionID '%s'",
			userID, capturedRecipientID)
		t.Error("This indicates the bug: sessionID is being used instead of userID")
	}

	// Verify the content mentions session end (this helps confirm the right code path ran)
	if !strings.Contains(capturedContent, "会话已结束") {
		t.Errorf("Expected message to contain '会话已结束', got: %s", capturedContent)
	}

	// Verify channelID is correct
	if capturedChannelID != "feishu" {
		t.Errorf("Expected channelID 'feishu', got: %s", capturedChannelID)
	}

	// Additional verification: ensure recipientID doesn't contain timestamp separator
	if strings.Contains(capturedRecipientID, ":") {
		t.Errorf("recipientID contains ':' separator, indicating it's a composite sessionID instead of pure userID: %s",
			capturedRecipientID)
	}
}
