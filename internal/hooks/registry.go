package hooks

import (
	"context"
	"fmt"
	"regexp"
	"sync"
)

// HookRegistry manages hook registration and execution
type HookRegistry struct {
	mu     sync.RWMutex
	hooks  map[HookEventType][]Hook
	config *Config
}

// NewHookRegistry creates a new hook registry
func NewHookRegistry(config *Config) *HookRegistry {
	return &HookRegistry{
		hooks:  make(map[HookEventType][]Hook),
		config: config,
	}
}

// Register adds a hook to the registry
func (r *HookRegistry) Register(hook Hook) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	eventType := hook.EventType()
	r.hooks[eventType] = append(r.hooks[eventType], hook)

	return nil
}

// Unregister removes a hook from the registry
func (r *HookRegistry) Unregister(name string, eventType HookEventType) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hooks := r.hooks[eventType]
	for i, hook := range hooks {
		if hook.Name() == name {
			// Remove the hook
			r.hooks[eventType] = append(hooks[:i], hooks[i+1:]...)
			return
		}
	}
}

// GetHooks returns all hooks for a given event type
func (r *HookRegistry) GetHooks(eventType HookEventType) []Hook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.hooks[eventType]
}

// GetMatchingHooks returns hooks that match the given tool name for PreToolUse/PostToolUse events
func (r *HookRegistry) GetMatchingHooks(eventType HookEventType, toolName string) []Hook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matching []Hook
	for _, hook := range r.hooks[eventType] {
		matcher := hook.Matcher()
		if matcher == "" {
			// Empty matcher means match all
			matching = append(matching, hook)
			continue
		}

		// Check if tool name matches the regex pattern
		matched, err := regexp.MatchString(matcher, toolName)
		if err == nil && matched {
			matching = append(matching, hook)
		}
	}

	return matching
}

// ExecutePreToolUse executes all PreToolUse hooks for a tool
func (r *HookRegistry) ExecutePreToolUse(ctx context.Context, toolName string, toolInput map[string]interface{}) (HookOutput, []HookError) {
	if !r.config.Enabled {
		return HookOutput{Decision: HookAllow}, nil
	}

	var errors []HookError
	output := HookOutput{Decision: HookAllow}

	for _, hook := range r.GetMatchingHooks(PreToolUse, toolName) {
		hookOutput, err := r.executeHook(ctx, hook, HookInput{
			Event:     PreToolUse,
			ToolName:  toolName,
			ToolInput: toolInput,
		})

		if err != nil {
			errors = append(errors, HookError{
				HookName: hook.Name(),
				Error:    err,
			})
			continue
		}

		// Apply the hook's decision
		switch hookOutput.Decision {
		case HookDeny:
			// Deny takes precedence
			output = hookOutput
			return output, errors
		case HookModify:
			// Modify the input and continue
			output = hookOutput
			// Update toolInput with modified values
			for k, v := range hookOutput.Modified {
				toolInput[k] = v
			}
		case HookAllow:
			// Allow continues to next hook
			if output.Decision != HookModify {
				// Don't override a previous modify decision
				output = hookOutput
			}
		}
	}

	return output, errors
}

// ExecutePostToolUse executes all PostToolUse hooks after a tool succeeds
func (r *HookRegistry) ExecutePostToolUse(ctx context.Context, toolName string, result string) []HookError {
	if !r.config.Enabled {
		return nil
	}

	var errors []HookError
	for _, hook := range r.GetMatchingHooks(PostToolUse, toolName) {
		_, err := r.executeHook(ctx, hook, HookInput{
			Event:    PostToolUse,
			ToolName: toolName,
			Result:   result,
		})

		if err != nil {
			errors = append(errors, HookError{
				HookName: hook.Name(),
				Error:    err,
			})
		}
	}

	return errors
}

// ExecutePostToolUseFailure executes all PostToolUseFailure hooks after a tool fails
func (r *HookRegistry) ExecutePostToolUseFailure(ctx context.Context, toolName string, err error) []HookError {
	if !r.config.Enabled {
		return nil
	}

	var errors []HookError
	for _, hook := range r.GetMatchingHooks(PostToolUseFailure, toolName) {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}

		_, herr := r.executeHook(ctx, hook, HookInput{
			Event:    PostToolUseFailure,
			ToolName: toolName,
			Error:    errMsg,
		})

		if herr != nil {
			errors = append(errors, HookError{
				HookName: hook.Name(),
				Error:    herr,
			})
		}
	}

	return errors
}

// ExecuteSessionStart executes all SessionStart hooks
func (r *HookRegistry) ExecuteSessionStart(ctx context.Context, data map[string]interface{}) []HookError {
	if !r.config.Enabled {
		return nil
	}

	var errors []HookError
	for _, hook := range r.GetHooks(SessionStart) {
		_, err := r.executeHook(ctx, hook, HookInput{
			Event: SessionStart,
			Data:  data,
		})

		if err != nil {
			errors = append(errors, HookError{
				HookName: hook.Name(),
				Error:    err,
			})
		}
	}

	return errors
}

// ExecuteSessionEnd executes all SessionEnd hooks
func (r *HookRegistry) ExecuteSessionEnd(ctx context.Context, data map[string]interface{}) []HookError {
	if !r.config.Enabled {
		return nil
	}

	var errors []HookError
	for _, hook := range r.GetHooks(SessionEnd) {
		_, err := r.executeHook(ctx, hook, HookInput{
			Event: SessionEnd,
			Data:  data,
		})

		if err != nil {
			errors = append(errors, HookError{
				HookName: hook.Name(),
				Error:    err,
			})
		}
	}

	return errors
}

// ExecuteCodeDevProgress executes all CodeDevProgress hooks
func (r *HookRegistry) ExecuteCodeDevProgress(ctx context.Context, taskID, tool string, progress float64, currentFile string, files []string) []HookError {
	if !r.config.Enabled {
		return nil
	}

	var errors []HookError
	for _, hook := range r.GetHooks(CodeDevProgress) {
		_, err := r.executeHook(ctx, hook, HookInput{
			Event:       CodeDevProgress,
			TaskID:      taskID,
			CodingTool:  tool,
			Progress:    progress,
			CurrentFile: currentFile,
			Files:       files,
		})

		if err != nil {
			errors = append(errors, HookError{
				HookName: hook.Name(),
				Error:    err,
			})
		}
	}

	return errors
}

// ExecuteUserPromptSubmit executes all UserPromptSubmit hooks
// This is used by codedev handler to trigger task events
func (r *HookRegistry) ExecuteUserPromptSubmit(ctx context.Context, data map[string]interface{}) (HookOutput, []HookError) {
	if !r.config.Enabled {
		return HookOutput{Decision: HookAllow}, nil
	}

	var errors []HookError
	output := HookOutput{Decision: HookAllow}

	for _, hook := range r.GetHooks(UserPromptSubmit) {
		hookOutput, err := r.executeHook(ctx, hook, HookInput{
			Event: UserPromptSubmit,
			Data:  data,
		})

		if err != nil {
			errors = append(errors, HookError{
				HookName: hook.Name(),
				Error:    err,
			})
		}

		// Apply the hook's decision
		switch hookOutput.Decision {
		case HookDeny:
			output = hookOutput
			return output, errors
		case HookModify:
			output = hookOutput
		case HookAllow:
			if output.Decision != HookModify {
				output = hookOutput
			}
		}
	}

	return output, errors
}

// executeHook executes a single hook with timeout handling
func (r *HookRegistry) executeHook(ctx context.Context, hook Hook, input HookInput) (HookOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, hook.Timeout())
	defer cancel()

	resultChan := make(chan struct {
		output HookOutput
		err    error
	}, 1)

	go func() {
		output, err := hook.Execute(ctx, input)
		resultChan <- struct {
			output HookOutput
			err    error
		}{output, err}
	}()

	select {
	case <-ctx.Done():
		return HookOutput{Decision: HookAllow}, fmt.Errorf("hook %s timed out", hook.Name())
	case result := <-resultChan:
		return result.output, result.err
	}
}

// HookError represents an error that occurred during hook execution
type HookError struct {
	HookName string
	Error    error
}

// HookStats returns statistics about registered hooks
func (r *HookRegistry) HookStats() map[HookEventType]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[HookEventType]int)
	for eventType, hooks := range r.hooks {
		stats[eventType] = len(hooks)
	}

	return stats
}
