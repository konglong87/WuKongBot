package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/konglong87/wukongbot/internal/agentteam"
)

// ParallelExecutor manages parallel execution of tasks
type ParallelExecutor struct {
	registry      AgentRegistry
	policy        *ToolPolicy
	maxConcurrent int
	semaphore     chan struct{}
}

// AgentRegistry interface for agent management
type AgentRegistry interface {
	FindAgents(criteria agentteam.AgentCapabilities) []*agentteam.Agent
	IncrementLoad(agentID string) error
	DecrementLoad(agentID string)
}

// NewParallelExecutor creates a new parallel executor
func NewParallelExecutor(registry AgentRegistry, policy *ToolPolicy, maxConcurrent int) *ParallelExecutor {
	if maxConcurrent <= 0 {
		maxConcurrent = 3 // Default
	}

	return &ParallelExecutor{
		registry:      registry,
		policy:        policy,
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}
}

// ExecuteParallel executes tasks in parallel where possible
func (e *ParallelExecutor) ExecuteParallel(ctx context.Context, tasks []*agentteam.Task) ([]*agentteam.SubtaskResult, error) {
	log.Info("ParallelExecutor", "executing_tasks", len(tasks), "max_concurrent", e.maxConcurrent)

	// Phase 1: Identify which tasks can run in parallel
	parallelGroups := e.groupTasksByParallel(tasks)

	// Track results by task ID
	resultMap := sync.Map{}
	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	firstError := &sync.Once{}

	// Phase 2: Execute groups in dependency order
	for groupIdx, group := range parallelGroups {
		for i, task := range group {
			// Acquire semaphore
			e.semaphore <- struct{}{}
			wg.Add(1)

			go func(taskIdx int, t *agentteam.Task, gIdx int) {
				defer func() {
					<-e.semaphore
					wg.Done()
				}()

				result, err := e.executeTask(ctx, t)
				if err != nil {
					firstError.Do(func() {
						log.Error("ParallelExecutor", "task_failed", t.ID, "error", err)
						errChan <- err
					})
					return
				}

				resultMap.Store(t.ID, result)
				log.Debug("ParallelExecutor", "task_completed", t.ID, "group", gIdx, "task", taskIdx)
			}(i, task, groupIdx)
		}

		// Wait for all tasks in this group before proceeding to next group
		wg.Wait()
	}

	// Wait for any error
	select {
	case <-errChan:
		// Error occurred, return partial results
	default:
	}

	// Collect results in original order
	results := make([]*agentteam.SubtaskResult, len(tasks))
	for i, task := range tasks {
		if val, ok := resultMap.Load(task.ID); ok {
			results[i] = val.(*agentteam.SubtaskResult)
		}
	}

	return results, nil
}

// groupTasksByParallel groups tasks that can run in parallel
func (e *ParallelExecutor) groupTasksByParallel(tasks []*agentteam.Task) [][]*agentteam.Task {
	// Simple implementation: no dependencies means all can run in parallel
	// Complex implementation would build DAG and group by dependency level

	// Check if any task has dependencies
	hasDependencies := false
	for _, task := range tasks {
		if len(task.Dependencies) > 0 {
			hasDependencies = true
			break
		}
	}

	if !hasDependencies {
		// All tasks can run in parallel
		return [][]*agentteam.Task{tasks}
	}

	// Build dependency graph and group
	// For now, return sequential execution
	groups := make([][]*agentteam.Task, len(tasks))
	for i, task := range tasks {
		groups[i] = []*agentteam.Task{task}
	}

	return groups
}

// executeTask executes a single task
func (e *ParallelExecutor) executeTask(ctx context.Context, task *agentteam.Task) (*agentteam.SubtaskResult, error) {
	startTime := time.Now()

	// Find suitable agent
	agent, err := e.assignAgent(task)
	if err != nil {
		return nil, err
	}

	// Execute the task (this is a placeholder - actual implementation would call the agent)
	resultText := fmt.Sprintf("Task completed by %s: %s", agent.Name, task.Description)

	// Execute with timeout
	timeout := e.policy.GetTimeout()
	_ = timeout // Will use later

	taskCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Simulate execution (replace with actual agent invocation)
	select {
	case <-taskCtx.Done():
		return &agentteam.SubtaskResult{
			SubtaskID:  task.ID,
			Title:      task.Title,
			State:      agentteam.StateFailed,
			AssignedTo: agent.Name,
			Error:      "Task timeout",
			Duration:   int(time.Since(startTime).Seconds()),
		}, nil
	case <-time.After(100 * time.Millisecond):
		// Task completed
	}

	return &agentteam.SubtaskResult{
		SubtaskID:  task.ID,
		Title:      task.Title,
		State:      agentteam.StateCompleted,
		AssignedTo: agent.Name,
		Result:     resultText,
		Duration:   int(time.Since(startTime).Seconds()),
	}, nil
}

// assignAgent finds and assigns an agent to a task
func (e *ParallelExecutor) assignAgent(task *agentteam.Task) (*agentteam.Agent, error) {
	criteria := agentteam.AgentCapabilities{
		Language:      task.Metadata.Language,
		Domain:        task.Metadata.Domain,
		RequiredTools: parseToolHints(task.Metadata.ToolHints),
	}

	agents := e.registry.FindAgents(criteria)
	if len(agents) == 0 {
		return nil, fmt.Errorf("no available agents matching criteria")
	}

	// Select best agent (lowest load)
	selected := agents[0]
	for _, agent := range agents {
		if agent.Load < selected.Load {
			selected = agent
		}
	}

	// Increment load
	if err := e.registry.IncrementLoad(selected.ID); err != nil {
		return nil, err
	}

	return selected, nil
}

// canRunParallel checks if a task can run in parallel with others
func (e *ParallelExecutor) canRunParallel(task *agentteam.Task) bool {
	// Check concurrency safety of required tools
	toolList := parseToolHints(task.Metadata.ToolHints)
	if len(toolList) == 0 {
		return true // No specific tools, assume safe
	}

	// Check if all tools are concurrency-safe
	for _, tool := range toolList {
		if !e.isToolConcurrencySafe(tool) {
			return false
		}
	}

	return true
}

// isToolConcurrencySafe checks if a tool is safe for parallel execution
func (e *ParallelExecutor) isToolConcurrencySafe(tool string) bool {
	// Known concurrency-unsafe tools
	unsafeTools := map[string]bool{
		"exec":       false, // Shell commands might conflict
		"spawn":      false, // Spawning should be serial
		"cron":       false,
		"sessions_*": false,
	}

	// Check exact match
	if safe, ok := unsafeTools[tool]; ok {
		return safe
	}

	// Check wildcards
	for pattern, safe := range unsafeTools {
		if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
			prefix := pattern[:len(pattern)-1]
			if len(tool) >= len(prefix) && tool[:len(prefix)] == prefix {
				return safe
			}
		}
	}

	// Default to safe
	return true
}

// parseToolHints parses tool hints from string
func parseToolHints(hints string) []string {
	// Simple parsing - split by comma
	if hints == "" {
		return []string{}
	}

	// MVP: ignore tool hints for now
	return []string{}
}

// IDtoInt is a helper to convert task ID to index
func IDtoInt(task *agentteam.Task, index int) int {
	return index
}
