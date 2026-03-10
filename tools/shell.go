package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ExecTool executes shell commands
type ExecTool struct {
	timeout             time.Duration
	workingDir          string
	restrictToWorkspace bool
}

// NewExecTool creates a new exec tool
func NewExecTool(timeoutSeconds int, workingDir string, restrictToWorkspace bool) *ExecTool {
	return &ExecTool{
		timeout:             time.Duration(timeoutSeconds) * time.Second,
		workingDir:          workingDir,
		restrictToWorkspace: restrictToWorkspace,
	}
}

// Name returns the tool name
func (t *ExecTool) Name() string {
	return "exec"
}

// Description returns the tool description
func (t *ExecTool) Description() string {
	return "Execute a shell command and return its output. Use with caution."
}

// Parameters returns the JSON schema for parameters
func (t *ExecTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The shell command to execute"
			},
			"working_dir": {
				"type": "string",
				"description": "Optional working directory for the command"
			}
		},
		"required": ["command"]
	}`)
}

// Execute executes the command
func (t *ExecTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "Error: command is required", nil
	}

	workingDir := t.workingDir
	if wd, ok := args["working_dir"].(string); ok && wd != "" {
		workingDir = wd
	}

	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}

	// Guard check - use configured workspace as the security boundary
	if err := t.guardCommand(command, workingDir, t.workingDir); err != nil {
		return err.Error(), nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	// Build the command
	var cmd *exec.Cmd
	if strings.Contains(command, "\n") || strings.Contains(command, ";") || strings.Contains(command, "&&") {
		// Multi-line or chained command
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = workingDir

	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return "Error: command timed out after " + formatDuration(t.timeout), nil
	}

	var result strings.Builder

	if len(output) > 0 {
		result.WriteString(string(output))
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if result.Len() > 0 {
				result.WriteString("\n")
			}
			result.WriteString("Exit code: ")
			result.WriteString(formatExitCode(exitErr.ExitCode()))
		}
	}

	outputStr := result.String()
	if outputStr == "" {
		outputStr = "(no output)"
	}

	// Truncate very long output
	maxLen := 10000
	if len(outputStr) > maxLen {
		truncated := len(outputStr) - maxLen
		outputStr = outputStr[:maxLen] + "\n... (truncated, " + fmt.Sprintf("%d", truncated) + " more chars)"
	}

	return outputStr, nil
}

// ConcurrentSafe returns false - shell commands may modify shared state
func (t *ExecTool) ConcurrentSafe() bool {
	return false
}

// guardCommand checks for dangerous patterns
// cwd: current working directory for command execution
// workspacePath: configured workspace path (security boundary)
func (t *ExecTool) guardCommand(command, cwd, workspacePath string) error {
	cmd := strings.TrimSpace(command)
	lower := strings.ToLower(cmd)

	// Deny patterns for dangerous commands
	// Note: patterns are order-sensitive, more specific patterns should come first
	denyPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\brm\s+-[rf]{1,2}\b`),            // rm -r, rm -rf, rm -fr
		regexp.MustCompile(`\bdel\s+/[fq]\b`),                // del /f, del /q
		regexp.MustCompile(`\brmdir\s+/s\b`),                 // rmdir /s
		regexp.MustCompile(`\bdd\s+if=`),                     // dd
		regexp.MustCompile(`>\s*/dev/sd`),                    // write to disk
		regexp.MustCompile(`\b(shutdown|reboot|poweroff)\b`), // system power
		regexp.MustCompile(`:\(\)\s*\{.*\};\s*:`),            // fork bomb
		// format/mkfs/diskpart should only match as commands, not in URLs or parameters
		// Match when at start of line or after spaces/semicolon
		regexp.MustCompile(`(^|[\s;])format\s`),   // format c: or ; format c:
		regexp.MustCompile(`(^|[\s;])mkfs`),       // mkfs.ext4 or mkfs -t ext4
		regexp.MustCompile(`(^|[\s;])diskpart\b`), // diskpart or ; diskpart
	}

	for _, pattern := range denyPatterns {
		if pattern.MatchString(lower) {
			return &GuardError{Message: "Command blocked by safety guard (dangerous pattern detected)"}
		}
	}

	// Workspace restriction - use configured workspace path as security boundary
	if t.restrictToWorkspace && workspacePath != "" {
		// Check for path traversal
		if strings.Contains(cmd, "..\\") || strings.Contains(cmd, "../") {
			return &GuardError{Message: "Command blocked by safety guard (path traversal detected)"}
		}

		workspaceAbs, err := filepath.Abs(workspacePath)
		if err == nil {
			workspaceResolved, _ := filepath.EvalSymlinks(workspaceAbs)

			// Quick check: if cwd (working_dir parameter) is within workspace, allow it
			// This handles cases where the user explicitly provides a valid working_dir
			if cwd != "" {
				cwdAbs, err := filepath.Abs(cwd)
				if err == nil {
					cwdResolved, _ := filepath.EvalSymlinks(cwdAbs)
					sep := string(filepath.Separator)
					isCwdInWorkspace := strings.HasPrefix(cwdResolved, workspaceResolved+sep) || cwdResolved == workspaceResolved
					if isCwdInWorkspace {
						// cwd is within workspace, no need for further path checks
						return nil
					}
				}
			}

			workspaceResolved, _ = filepath.EvalSymlinks(workspaceAbs)
			// Skip URL-based commands entirely (curl, wget with URLs or domain-like arguments)
			// Check for curl/wget commands first
			if strings.Contains(cmd, "curl ") || strings.Contains(cmd, "wget ") {
				// Allow curl/wget if the URL contains http:// or https://
				if strings.Contains(cmd, "http://") || strings.Contains(cmd, "https://") {
					return nil
				}
				// For curl/wget with domain-only URLs (no protocol), check common patterns
				// Match: curl domain.com/path or curl "domain.com/path" or curl -s "domain.com/path?query=..."
				// Extended pattern to support options and query parameters
				curlURLPattern := regexp.MustCompile(`curl\s+[\-a-zA-Z0-9\s]*["']?[a-zA-Z0-9\-]+\.[a-zA-Z]{2,}`)
				wgetURLPattern := regexp.MustCompile(`wget\s+[\-a-zA-Z0-9\s]*["']?[a-zA-Z0-9\-]+\.[a-zA-Z]{2,}`)
				if curlURLPattern.MatchString(cmd) || wgetURLPattern.MatchString(cmd) {
					return nil // Allow curl/wget with domain names
				}
			}

			// Check for absolute paths outside workspace
			// Use more precise regex to avoid matching false positives like "path" in osascript
			// Match absolute paths (starting with /) that look like real file paths
			// Include: /home, /Users, /tmp, /var, /etc, /opt, /root, /mnt, etc.
			// Or paths with directory separators (one or more '/')
			absPathPattern := regexp.MustCompile(`/(?:home|Users|tmp|var|etc|opt|root|mnt|usr|lib|bin|data|proc|sys|dev|(?:tmp|work|home)\/[^\s"'\)\\]+)(?:[^\s"'\)]*)?`)
			// Also match patterns after flags like -output, -o, --output, -f, etc.
			flagOutputPattern := regexp.MustCompile(`--output[=\s]+([^\s"'\)]+)|-o[=\s]+([^\s"'\)]+)|-f[=\s]+([^\s"'\)]+)|>-?\s*([^\s"'\)]+)`)

			var pathsToCheck []string
			if absPaths := absPathPattern.FindAllStringSubmatch(cmd, -1); absPaths != nil {
				for _, match := range absPaths {
					if len(match) > 0 {
						pathsToCheck = append(pathsToCheck, match[0])
					}
				}
			}
			if flagMatches := flagOutputPattern.FindAllStringSubmatch(cmd, -1); flagMatches != nil {
				for _, match := range flagMatches {
					for i := 1; i < len(match); i++ {
						if match[i] != "" && (strings.HasPrefix(match[i], "/") || strings.Contains(match[i], "/")) {
							pathsToCheck = append(pathsToCheck, match[i])
						}
					}
				}
			}

			for _, raw := range pathsToCheck {
				// Skip URLs (contain ://)
				if strings.Contains(raw, "://") {
					continue
				}
				// Skip if command contains http/https - this is a URL command
				if strings.Contains(cmd, "http://") || strings.Contains(cmd, "https://") {
					// Only allow if the path clearly looks like a URL
					if regexp.MustCompile(`https?://`).MatchString(raw) {
						continue
					}
				}
				// Skip if the path looks like a domain name (e.g., /domain.tld/path)
				if regexp.MustCompile(`^/[a-zA-Z0-9\-]+\.[a-zA-Z]{2,}(/|$)`).MatchString(raw) {
					continue
				}
				// Skip short paths (likely not real paths)
				if len(raw) < 3 {
					continue
				}

				p, err := filepath.Abs(raw)
				if err == nil {
					pResolved, _ := filepath.EvalSymlinks(p)

					// For mkdir/cp/mv/touch/cat/echo/python/node commands, allow creating paths in workspace
					allowCreation := strings.Contains(cmd, "mkdir") ||
						strings.Contains(cmd, "cp ") ||
						strings.Contains(cmd, "mv ") ||
						strings.Contains(cmd, "touch ") ||
						strings.Contains(cmd, "cat ") ||
						strings.Contains(cmd, "echo ") ||
						strings.Contains(cmd, "python ") ||
						strings.Contains(cmd, "node ") ||
						strings.Contains(cmd, "ruby ")

					// Check if path is directly in workspace
					isInWorkspace := strings.HasPrefix(pResolved, workspaceResolved+string(filepath.Separator)) || pResolved == workspaceResolved

					if !isInWorkspace {
						// For creation commands, check if any parent path would be in workspace
						// This allows creating subdirectories within workspace even if they don't exist yet
						if allowCreation {
							// Check if workspace is a prefix of the target path
							// This allows: /workspace/subdir when workspace is /workspace
							if strings.HasPrefix(pResolved, workspaceResolved) {
								// Path starts with workspace, it's OK (e.g., creating workspace/subdir)
							} else {
								return &GuardError{Message: "Command blocked by safety guard (path outside working dir)"}
							}
						} else {
							// For other commands, only block if the path exists and is outside workspace
							_, statErr := os.Stat(p)
							if statErr == nil {
								return &GuardError{Message: "Command blocked by safety guard (path outside working dir)"}
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// GuardError represents a safety guard error
type GuardError struct {
	Message string
}

func (e *GuardError) Error() string {
	return e.Message
}

// Helper functions
func formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	}
	hours := minutes / 60
	return fmt.Sprintf("%d hours", hours)
}

func formatExitCode(code int) string {
	return fmt.Sprintf("%d", code)
}
