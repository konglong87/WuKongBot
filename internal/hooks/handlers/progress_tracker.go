package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	h "github.com/konglong87/wukongbot/internal/hooks"
)

// ProgressTracker tracks and reports code development progress
type ProgressTracker struct {
}

// NewProgressTrackerHook creates a progress tracking hook
func NewProgressTrackerHook() h.Hook {
	return h.NewInlineHook(
		"progress-reporter",
		h.CodeDevProgress,
		"",            // Match all
		5*time.Second, // 5 second timeout
		func(ctx context.Context, input h.HookInput) (h.HookOutput, error) {
			// Extract progress information
			taskID := input.TaskID
			tool := input.CodingTool
			progress := input.Progress
			currentFile := input.CurrentFile
			files := input.Files

			// Log progress
			log.Info("Code dev progress",
				"task_id", taskID,
				"tool", tool,
				"progress", fmt.Sprintf("%.0f%%", progress*100),
				"current_file", currentFile,
				"files_modified", len(files),
			)

			// Build progress message
			message := buildProgressMessage(tool, progress, currentFile, files)

			return h.HookOutput{
				Decision: h.HookAllow,
				Message:  message,
			}, nil
		},
	)
}

// buildProgressMessage builds a human-readable progress message
func buildProgressMessage(tool string, progress float64, currentFile string, files []string) string {
	progressPercent := int(progress * 100)

	if currentFile != "" {
		return fmt.Sprintf("[%s 正在编码... %d%%] 当前: %s", tool, progressPercent, currentFile)
	}

	if len(files) > 0 {
		return fmt.Sprintf("[%s 编码完成 %d%%] 已修改 %d 个文件", tool, progressPercent, len(files))
	}

	if progress >= 1.0 {
		return fmt.Sprintf("[%s 编码任务完成]", tool)
	}

	return fmt.Sprintf("[%s 正在编码... %d%%]", tool, progressPercent)
}

// FileChangeLogger logs file changes during development
func NewFileChangeLogger() h.Hook {
	return h.NewInlineHook(
		"file-change-logger",
		h.CodeDevProgress,
		"", // Match all
		5*time.Second,
		func(ctx context.Context, input h.HookInput) (h.HookOutput, error) {
			files := input.Files
			if len(files) == 0 {
				return h.HookOutput{Decision: h.HookAllow}, nil
			}

			// Log files that were modified
			for _, file := range files {
				log.Debug("File modified", "file", file, "task_id", input.TaskID)
			}

			return h.HookOutput{Decision: h.HookAllow}, nil
		},
	)
}

// TaskCompletionTracker tracks task completion
func NewTaskCompletionTracker() h.Hook {
	return h.NewInlineHook(
		"task-completion",
		h.CodeDevProgress,
		"", // Match all
		5*time.Second,
		func(ctx context.Context, input h.HookInput) (h.HookOutput, error) {
			if input.Progress >= 1.0 {
				log.Info("Code development task completed",
					"task_id", input.TaskID,
					"tool", input.CodingTool,
					"files_modified", len(input.Files),
				)
			}
			return h.HookOutput{Decision: h.HookAllow}, nil
		},
	)
}
