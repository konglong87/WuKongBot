package handlers

import (
	"runtime"

	"context"

	"github.com/charmbracelet/log"
	h "github.com/konglong87/wukongbot/internal/hooks"
)

// NewPreCodeDevHook creates a monitoring hook that runs before code_dev tool execution
// This hook only logs the start of the task, does not intercept
func NewPreCodeDevHook() h.Hook {
	return h.NewInlineHook(
		"pre-code-dev-monitor",
		h.PreToolUse,
		"code_dev",
		5, // 5 second timeout
		func(ctx context.Context, input h.HookInput) (h.HookOutput, error) {
			pc, _, _, _ := runtime.Caller(0)
			fnName := runtime.FuncForPC(pc).Name()

			// Extract tool parameters
			task, ok := input.ToolInput["task"].(string)
			tool, ok2 := input.ToolInput["tool"].(string)

			if ok && ok2 {
				log.Info(fnName, "tool", tool, "task", task)
			} else {
				log.Debug(fnName, "params", input.ToolInput)
			}

			// Always allow - this is a monitoring hook
			return h.HookOutput{Decision: h.HookAllow}, nil
		},
	)
}

// NewPostCodeDevHook creates a monitoring hook that runs after code_dev tool execution
// This hook logs the result, does not intercept
func NewPostCodeDevHook() h.Hook {
	return h.NewInlineHook(
		"post-code-dev-monitor",
		h.PostToolUse,
		"code_dev",
		5,
		func(ctx context.Context, input h.HookInput) (h.HookOutput, error) {
			pc, _, _, _ := runtime.Caller(0)
			fnName := runtime.FuncForPC(pc).Name()

			result := input.Result

			// Log execution result
			if len(result) > 2048 {
				log.Info(fnName, "result_length", len(result))
			} else {
				log.Info(fnName, "result", result)
			}

			// Always allow - this is a monitoring hook
			return h.HookOutput{Decision: h.HookAllow}, nil
		},
	)
}

// NewCodeDevProgressHook creates a hook for progress tracking
// This can be triggered externally to update progress
func NewCodeDevProgressHook() h.Hook {
	return h.NewInlineHook(
		"code-dev-progress",
		h.CodeDevProgress,
		"", // Match all
		5,
		func(ctx context.Context, input h.HookInput) (h.HookOutput, error) {
			pc, _, _, _ := runtime.Caller(0)
			fnName := runtime.FuncForPC(pc).Name()

			taskID := input.TaskID
			tool := input.CodingTool
			progress := input.Progress
			currentFile := input.CurrentFile
			files := input.Files

			// Log progress
			log.Info(fnName,
				"task_id", taskID,
				"tool", tool,
				"progress", progress,
				"current_file", currentFile,
				"files_count", len(files),
			)

			// Always allow
			return h.HookOutput{Decision: h.HookAllow}, nil
		},
	)
}
