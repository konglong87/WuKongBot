package tools

import (
	"os"
	"testing"
	"time"
)

func TestGuardCommand_DenyPatterns(t *testing.T) {
	tool := NewExecTool(30, "/tmp/test", false)

	denyPatterns := []string{
		"rm -rf /",
		"rm -r /home",
		"rm -fr /var",
		"del /f C:\\",
		"del /q C:\\",
		"rmdir /s C:\\",
		"format c:",
		"mkfs.ext4 /dev/sda1",
		"diskpart",
		"dd if=/dev/zero of=/dev/sda",
		"echo test > /dev/sda",
		"shutdown now",
		"reboot",
		"poweroff",
		":(){ :|:& };:",
	}

	for _, pattern := range denyPatterns {
		t.Run(pattern, func(t *testing.T) {
			err := tool.guardCommand(pattern, "/tmp/test", "") // Workspace restriction disabled
			if err == nil {
				t.Errorf("Expected error for command: %s", pattern)
			}
			if _, ok := err.(*GuardError); !ok {
				t.Errorf("Expected GuardError for command: %s, got: %T", pattern, err)
			}
		})
	}
}

func TestGuardCommand_AllowPatterns(t *testing.T) {
	tool := NewExecTool(30, "/tmp/test", false)

	allowPatterns := []string{
		"ls -la",
		"pwd",
		"echo hello",
		"cat file.txt",
		"grep pattern file.txt",
		"find . -name '*.go'",
	}

	for _, pattern := range allowPatterns {
		t.Run(pattern, func(t *testing.T) {
			err := tool.guardCommand(pattern, "/tmp/test", "") // Workspace restriction disabled
			if err != nil {
				t.Errorf("Expected no error for command: %s, got: %v", pattern, err)
			}
		})
	}
}

func TestGuardCommand_PathTraversal(t *testing.T) {
	tool := NewExecTool(30, "/tmp/test", true)

	// Create test working directory
	testDir := "/tmp/wukongbot-test"
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	traversalPatterns := []string{
		"cat ../../../etc/passwd",
		"ls ../../../../",
		"cat ..\\..\\windows\\system32", // Windows style
	}

	for _, pattern := range traversalPatterns {
		t.Run(pattern, func(t *testing.T) {
			err := tool.guardCommand(pattern, testDir, testDir) // Use testDir as workspace
			if err == nil {
				t.Errorf("Expected error for path traversal in command: %s", pattern)
			}
			if err != nil && err.Error() != "Command blocked by safety guard (path traversal detected)" {
				t.Errorf("Expected path traversal error for command: %s, got: %v", pattern, err)
			}
		})
	}
}

func TestGuardCommand_CWithURL(t *testing.T) {
	tool := NewExecTool(30, "/tmp/test", true)

	urlPatterns := []string{
		"curl -s https://api.example.com/data",
		"curl http://localhost:8080/health",
		"wget -O file.txt https://example.com/file.txt",
		"curl -s -I https://www.google.com",
	}

	for _, pattern := range urlPatterns {
		t.Run(pattern, func(t *testing.T) {
			err := tool.guardCommand(pattern, "/tmp/test", "/tmp/test")
			if err != nil {
				t.Errorf("Expected no error for URL command: %s, got: %v", pattern, err)
			}
		})
	}
}

func TestGuardCommand_CurlWithoutProtocol(t *testing.T) {
	tool := NewExecTool(30, "/tmp/test", true)

	// Test curl with domain without protocol (like wttr.in)
	curlPatterns := []string{
		"curl -s wttr.in/Beijing",
		"curl wttr.in/Shanghai?format=%l:+%c+%t",
		"curl -s \"wttr.in/Tokyo?format=%l:+%c+%t+%h+%w&lang=zh\"",
		"curl api.example.com/data",
		"wget wttr.in/Beijing",
		"wget https://wttr.in/Beijing",
	}

	for _, pattern := range curlPatterns {
		t.Run(pattern, func(t *testing.T) {
			err := tool.guardCommand(pattern, "/tmp/test", "/tmp/test")
			if err != nil {
				t.Errorf("Expected no error for curl/wget with domain: %s, got: %v", pattern, err)
			}
		})
	}
}

func TestGuardCommand_PathOutsideWorkspace(t *testing.T) {
	tool := NewExecTool(30, "/tmp/test", true)

	// Create test directories
	workspaceDir := "/tmp/wukongbot-workspace"
	outsideDir := "/tmp/wukongbot-outside"
	os.RemoveAll(workspaceDir)
	os.RemoveAll(outsideDir)
	os.MkdirAll(workspaceDir, 0755)
	os.MkdirAll(outsideDir, 0755)
	defer os.RemoveAll(workspaceDir)
	defer os.RemoveAll(outsideDir)

	// Test accessing file outside workspace
	outsideFile := outsideDir + "/secret.txt"
	os.WriteFile(outsideFile, []byte("secret data"), 0644)

	patterns := []string{
		"cat /tmp/wukongbot-outside/secret.txt",
		"ls /tmp/wukongbot-outside",
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			err := tool.guardCommand(pattern, workspaceDir, workspaceDir) // workspaceDir is the security boundary
			if err == nil {
				t.Errorf("Expected error for path outside workspace in command: %s", pattern)
			}
			if err != nil && err.Error() != "Command blocked by safety guard (path outside working dir)" {
				t.Errorf("Expected path outside working dir error for command: %s, got: %v", pattern, err)
			}
		})
	}
}

func TestGuardCommand_PathInsideWorkspace(t *testing.T) {
	tool := NewExecTool(30, "/tmp/test", true)

	// Create test directories
	workspaceDir := "/tmp/wukongbot-workspace-2"
	os.RemoveAll(workspaceDir)
	os.MkdirAll(workspaceDir, 0755)
	os.MkdirAll(workspaceDir+"/subdir", 0755)
	defer os.RemoveAll(workspaceDir)

	// Create test file
	testFile := workspaceDir + "/file.txt"
	os.WriteFile(testFile, []byte("test data"), 0644)

	patterns := []string{
		"cat /tmp/wukongbot-workspace-2/file.txt",
		"ls /tmp/wukongbot-workspace-2/subdir",
		"cd " + workspaceDir + " && pwd",
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			err := tool.guardCommand(pattern, workspaceDir, workspaceDir) // workspaceDir is the security boundary
			if err != nil {
				t.Errorf("Expected no error for path inside workspace in command: %s, got: %v", pattern, err)
			}
		})
	}
}

func TestNewExecTool(t *testing.T) {
	tool := NewExecTool(60, "/workspace", true)

	if tool == nil {
		t.Fatal("Expected non-nil tool")
	}

	if tool.timeout != 60*time.Second {
		t.Errorf("Expected timeout 60 seconds, got: %v", tool.timeout)
	}

	if tool.workingDir != "/workspace" {
		t.Errorf("Expected working dir /workspace, got: %s", tool.workingDir)
	}

	if !tool.restrictToWorkspace {
		t.Error("Expected restrictToWorkspace to be true")
	}
}

func TestExecTool_Name(t *testing.T) {
	tool := NewExecTool(30, "", false)
	name := tool.Name()
	if name != "exec" {
		t.Errorf("Expected name 'exec', got: %s", name)
	}
}

func TestExecTool_Description(t *testing.T) {
	tool := NewExecTool(30, "", false)
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestExecTool_ConcurrentSafe(t *testing.T) {
	tool := NewExecTool(30, "", false)
	if tool.ConcurrentSafe() {
		t.Error("Expected ExecTool to not be concurrent safe")
	}
}

func TestGuardError_Error(t *testing.T) {
	msg := "test error"
	err := &GuardError{Message: msg}
	if err.Error() != msg {
		t.Errorf("Expected error message '%s', got: '%s'", msg, err.Error())
	}
}

func TestExecute_CommandRequired(t *testing.T) {
	tool := NewExecTool(30, "", false)
	result, err := tool.Execute(nil, map[string]interface{}{})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result != "Error: command is required" {
		t.Errorf("Expected 'command is required' error, got: %s", result)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds int
		expect  string
	}{
		{30, "30 seconds"},
		{60, "1 minutes"},
		{120, "2 minutes"},
		{3600, "1 hours"},
		{7200, "2 hours"},
	}

	for _, tt := range tests {
		t.Run(tt.expect, func(t *testing.T) {
			d := time.Duration(tt.seconds) * time.Second
			result := formatDuration(d)
			if result != tt.expect {
				t.Errorf("Expected '%s', got: '%s'", tt.expect, result)
			}
		})
	}
}

func TestFormatExitCode(t *testing.T) {
	tests := []int{0, 1, 127, 255}
	for _, code := range tests {
		t.Run(string(rune(code)), func(t *testing.T) {
			result := formatExitCode(code)
			expected := string(rune('0' + code))
			if code >= 0 && code <= 9 && result != expected {
				t.Errorf("Expected '%s', got: '%s'", expected, result)
			}
			// For codes > 9, we just check it returns something
			if result == "" {
				t.Error("Expected non-empty result")
			}
		})
	}
}
