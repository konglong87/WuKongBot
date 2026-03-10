package hooks

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Config represents the hooks configuration
type Config struct {
	Enabled            bool          `yaml:"enabled"`
	Timeout            int           `yaml:"timeout"` // default timeout in seconds
	PreToolUse         []HookConfig  `yaml:"pre_tool_use"`
	PostToolUse        []HookConfig  `yaml:"post_tool_use"`
	PostToolUseFailure []HookConfig  `yaml:"post_tool_use_failure"`
	SessionStart       []HookConfig  `yaml:"session_start"`
	SessionEnd         []HookConfig  `yaml:"session_end"`
	CodeDevelopment    CodeDevConfig `yaml:"code_development"`
}

// CodeDevConfig represents code development specific hooks configuration
type CodeDevConfig struct {
	Enabled     bool                          `yaml:"enabled"`
	Timeout     int                           `yaml:"timeout"`      // Command execution timeout in seconds
	DefaultTool string                        `yaml:"default_tool"` // Default tool: "opencode", "cursor"
	Executors   map[string]ToolExecutorConfig `yaml:"executors"`    // Tool configurations
	PreWrite    []HookConfig                  `yaml:"pre_write"`
	PostWrite   []HookConfig                  `yaml:"post_write"`
	AutoTest    bool                          `yaml:"auto_test"`
	TestCommand string                        `yaml:"test_command"`
	TaskHooks   []HookConfig                  `yaml:"task_hooks"` // Progress tracking hooks
}

// ToolExecutorConfig represents configuration for an external coding tool
type ToolExecutorConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Command  string `yaml:"command"`       // e.g., "opencode", "cursor"
	Template string `yaml:"template"`      // Command template with {task} placeholder
	CheckCmd string `yaml:"check_command"` // Command to check if tool is available
}

// DefaultConfig returns the default hooks configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:            false,
		Timeout:            10,
		PreToolUse:         []HookConfig{},
		PostToolUse:        []HookConfig{},
		PostToolUseFailure: []HookConfig{},
		SessionStart:       []HookConfig{},
		SessionEnd:         []HookConfig{},
		CodeDevelopment: CodeDevConfig{
			Enabled:     false,
			Timeout:     300, // 5 minutes
			DefaultTool: "opencode",
			Executors: map[string]ToolExecutorConfig{
				"opencode": {
					Enabled:  true,
					Command:  "opencode",
					Template: "opencode \"{task}\"",
					CheckCmd: "opencode --version",
				},
				"cursor": {
					Enabled:  false,
					Command:  "cursor",
					Template: "cursor ai \"{task}\"",
					CheckCmd: "cursor --version",
				},
			},
			AutoTest:    false,
			TestCommand: "go test ./...",
		},
	}
}

// LoadHooksFromConfig loads hooks from configuration and registers them
func LoadHooksFromConfig(config *Config, workspace string) ([]Hook, error) {
	var hooks []Hook

	// Load PreToolUse hooks
	for _, cfg := range config.PreToolUse {
		hook, err := configToHook(cfg, PreToolUse, workspace)
		if err != nil {
			return nil, fmt.Errorf("failed to load PreToolUse hook %s: %w", cfg.Name, err)
		}
		if hook != nil {
			hooks = append(hooks, hook)
		}
	}

	// Load PostToolUse hooks
	for _, cfg := range config.PostToolUse {
		hook, err := configToHook(cfg, PostToolUse, workspace)
		if err != nil {
			return nil, fmt.Errorf("failed to load PostToolUse hook %s: %w", cfg.Name, err)
		}
		if hook != nil {
			hooks = append(hooks, hook)
		}
	}

	// Load PostToolUseFailure hooks
	for _, cfg := range config.PostToolUseFailure {
		hook, err := configToHook(cfg, PostToolUseFailure, workspace)
		if err != nil {
			return nil, fmt.Errorf("failed to load PostToolUseFailure hook %s: %w", cfg.Name, err)
		}
		if hook != nil {
			hooks = append(hooks, hook)
		}
	}

	// Load SessionStart hooks
	for _, cfg := range config.SessionStart {
		hook, err := configToHook(cfg, SessionStart, workspace)
		if err != nil {
			return nil, fmt.Errorf("failed to load SessionStart hook %s: %w", cfg.Name, err)
		}
		if hook != nil {
			hooks = append(hooks, hook)
		}
	}

	// Load SessionEnd hooks
	for _, cfg := range config.SessionEnd {
		hook, err := configToHook(cfg, SessionEnd, workspace)
		if err != nil {
			return nil, fmt.Errorf("failed to load SessionEnd hook %s: %w", cfg.Name, err)
		}
		if hook != nil {
			hooks = append(hooks, hook)
		}
	}

	// Load code development hooks
	if config.CodeDevelopment.Enabled {
		// Add pre-write hooks
		for _, cfg := range config.CodeDevelopment.PreWrite {
			cfg.Matcher = "write_file" // Force matcher
			hook, err := configToHook(cfg, PreToolUse, workspace)
			if err != nil {
				return nil, fmt.Errorf("failed to load code dev pre-write hook %s: %w", cfg.Name, err)
			}
			if hook != nil {
				hooks = append(hooks, hook)
			}
		}

		// Add post-write hooks (for auto-test)
		if config.CodeDevelopment.AutoTest {
			testHook := NewInlineHook(
				"auto-test-after-write",
				PostToolUse,
				"write_file",
				60, // 1 minute timeout
				func(ctx context.Context, input HookInput) (HookOutput, error) {
					testCmd := config.CodeDevelopment.TestCommand
					if testCmd == "" {
						testCmd = "go test ./..."
					}

					// Extract the file path from tool input
					path, ok := input.ToolInput["path"].(string)
					if !ok {
						return HookOutput{Decision: HookAllow}, nil
					}

					// Check if it's a Go source file
					if !strings.HasSuffix(path, ".go") {
						return HookOutput{Decision: HookAllow}, nil
					}

					cmd := exec.CommandContext(ctx, "sh", "-c", testCmd)
					output, err := cmd.CombinedOutput()

					if err != nil {
						return HookOutput{
							Decision: HookAllow,
							Message:  fmt.Sprintf("⚠️ Tests failed:\n%s", string(output)),
						}, nil
					}

					return HookOutput{
						Decision: HookAllow,
						Message:  "✅ Tests passed",
					}, nil
				},
			)
			hooks = append(hooks, testHook)
		}
	}

	return hooks, nil
}

// configToHook converts a HookConfig to a Hook implementation
func configToHook(cfg HookConfig, eventType HookEventType, workspace string) (Hook, error) {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	switch cfg.Type {
	case "inline":
		// Inline hooks require a handler function
		// For now, we'll skip inline hooks from config as they need Go code
		// These will be registered programmatically
		return nil, nil

	case "command":
		if cfg.Command == "" {
			return nil, fmt.Errorf("command hook requires 'command' field")
		}
		return NewCommandHook(cfg.Name, eventType, cfg.Matcher, timeout, cfg.Command), nil

	default:
		return nil, fmt.Errorf("unknown hook type: %s", cfg.Type)
	}
}
