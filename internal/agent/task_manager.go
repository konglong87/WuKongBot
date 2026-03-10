package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	agentteam "github.com/konglong87/wukongbot/internal/agentteam"
	bus "github.com/konglong87/wukongbot/internal/bus"
	sessionpkg "github.com/konglong87/wukongbot/internal/session"
)

// TaskManager manages task lifecycle, dependencies, and tracking
type TaskManager struct {
	registry       *SubagentRegistry
	executor       *ParallelExecutor
	taskGraph      map[string][]string // dependency graph: taskID -> []dependentTaskIDs
	topoOrder      []string            // topological order
	mu             sync.RWMutex
	completionChan chan *TaskCompletion
}

// TaskCompletion represents task completion event
type TaskCompletion struct {
	TaskID    string
	Task      *agentteam.Task
	Result    *agentteam.SubtaskResult
	Timestamp time.Time
}

// NewTaskManager creates a new task manager
func NewTaskManager(registry *SubagentRegistry, executor *ParallelExecutor) *TaskManager {
	tm := &TaskManager{
		registry:       registry,
		executor:       executor,
		taskGraph:      make(map[string][]string),
		completionChan: make(chan *TaskCompletion, 100),
	}

	go tm.processCompletions()

	return tm
}

// ExecuteWithDeps executes tasks respecting dependencies
func (tm *TaskManager) ExecuteWithDeps(ctx context.Context, tasks []*agentteam.Task, dependencies [][]string) (*agentteam.TaskResult, error) {
	log.Info("TaskManager", "executing_tasks", len(tasks), "with_deps", len(dependencies))

	// Build dependency graph
	if err := tm.buildDependencyGraph(tasks, dependencies); err != nil {
		return nil, err
	}

	// Get topological order
	topoOrder, err := tm.getTopologicalOrder()
	if err != nil {
		return nil, err
	}

	// Execute in topological order
	taskMap := make(map[string]*agentteam.Task)
	for _, task := range tasks {
		taskMap[task.ID] = task
	}

	allResults := make([]agentteam.SubtaskResult, 0, len(tasks))
	startTime := time.Now()

	for _, taskID := range topoOrder {
		task, ok := taskMap[taskID]
		if !ok {
			continue
		}

		log.Info("TaskManager", "executing_task", task.ID, "title", task.Title)

		// Execute single task
		results, err := tm.executor.ExecuteParallel(ctx, []*agentteam.Task{task})
		if err != nil {
			log.Error("TaskManager", "execute_failed", task.ID, "error", err)
			continue
		}
		if len(results) > 0 && results[0] != nil {
			result := *results[0]
			allResults = append(allResults, result)

			// Notify completion
			tm.completionChan <- &TaskCompletion{
				TaskID:    task.ID,
				Task:      task,
				Result:    &result,
				Timestamp: time.Now(),
			}

			// Update task state
			task.State = result.State
			if result.State == agentteam.StateFailed {
				// Mark dependent tasks as blocked
				tm.markDependentsBlocked(task.ID)
				break // Stop on failure
			}
		}
	}

	// Build final result
	mainTaskID := tasks[0].ParentID
	if mainTaskID == "" {
		mainTaskID = tasks[0].ID
	}

	finalResult := &agentteam.TaskResult{
		TaskID:   mainTaskID,
		Title:    tasks[0].Title,
		State:    agentteam.StateCompleted,
		Subtasks: allResults,
		Summary:  fmt.Sprintf("Completed %d subtasks", len(allResults)),
		Duration: int(time.Since(startTime).Seconds()),
	}

	// Check for failures
	failedCount := 0
	for _, r := range allResults {
		if r.State == agentteam.StateFailed {
			failedCount++
		}
	}

	if failedCount > 0 {
		finalResult.State = agentteam.StateFailed
		finalResult.Success = false
		finalResult.Summary = fmt.Sprintf("Completed %d/%d subtasks (%d failed)", len(allResults)-failedCount, len(allResults), failedCount)
		finalResult.Error = fmt.Sprintf("%d subtasks failed", failedCount)
	} else {
		finalResult.Success = true
	}

	return finalResult, nil
}

// buildDependencyGraph builds the dependency graph
func (tm *TaskManager) buildDependencyGraph(tasks []*agentteam.Task, dependencies [][]string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Clear previous graph
	tm.taskGraph = make(map[string][]string)

	// Initialize graph nodes
	for _, task := range tasks {
		tm.taskGraph[task.ID] = []string{}
	}

	// Add edges from dependencies
	for _, dep := range dependencies {
		if len(dep) < 2 {
			continue
		}

		fromTask := dep[0]
		for i := 1; i < len(dep); i++ {
			toTask := dep[i]
			if _, exists := tm.taskGraph[toTask]; exists {
				tm.taskGraph[toTask] = append(tm.taskGraph[toTask], fromTask)
			}
		}
	}

	// Check for cycles
	if tm.hasCycle() {
		return fmt.Errorf("dependency graph contains cycles")
	}

	return nil
}

// getTopologicalOrder returns tasks in topological order
func (tm *TaskManager) getTopologicalOrder() ([]string, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Build in-degree map
	inDegree := make(map[string]int)
	for taskID := range tm.taskGraph {
		inDegree[taskID] = 0
	}

	for _, deps := range tm.taskGraph {
		for _, dep := range deps {
			inDegree[dep]++
		}
	}

	// Find tasks with no dependencies
	queue := make([]string, 0)
	for taskID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, taskID)
		}
	}

	// Topological sort
	var order []string
	for len(queue) > 0 {
		taskID := queue[0]
		queue = queue[1:]
		order = append(order, taskID)

		for _, neighbor := range tm.taskGraph[taskID] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(order) != len(tm.taskGraph) {
		return nil, fmt.Errorf("dependency graph has cycles")
	}

	return order, nil
}

// hasCycle checks if the graph has cycles
func (tm *TaskManager) hasCycle() bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for node := range tm.taskGraph {
		if !visited[node] {
			if tm.cycleDFS(node, visited, recStack) {
				return true
			}
		}
	}

	return false
}

// cycleDFS performs DFS for cycle detection
func (tm *TaskManager) cycleDFS(node string, visited, recStack map[string]bool) bool {
	visited[node] = true
	recStack[node] = true

	for _, neighbor := range tm.taskGraph[node] {
		if !visited[neighbor] {
			if tm.cycleDFS(neighbor, visited, recStack) {
				return true
			}
		} else if recStack[neighbor] {
			return true
		}
	}

	recStack[node] = false
	return false
}

// markDependentsBlocked marks dependent tasks as blocked
func (tm *TaskManager) markDependentsBlocked(failedTaskID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Find all tasks that depend on this task
	toBlock := []string{failedTaskID}
	for i := 0; i < len(toBlock); i++ {
		currentID := toBlock[i]
		for _, neighbor := range tm.taskGraph[currentID] {
			toBlock = append(toBlock, neighbor)
		}
	}

	log.Info("TaskManager", "marking_blocked", "failed_task", failedTaskID, "count", len(toBlock)-1)
}

// processCompletions processes task completion events
func (tm *TaskManager) processCompletions() {
	for completion := range tm.completionChan {
		log.Debug("TaskManager", "task_completed", completion.TaskID,
			"state", completion.Result.State, "duration", completion.Result.Duration)
	}
}

// GetTaskStatus returns the status of a task
func (tm *TaskManager) GetTaskStatus(taskID string) (*TaskStatus, error) {
	// Query from registry
	run, err := tm.registry.GetRun(taskID)
	if err != nil {
		return nil, err
	}

	if run == nil {
		return nil, fmt.Errorf("task not found")
	}

	return &TaskStatus{
		RunID:     run.RunID,
		Label:     run.Label,
		Status:    run.Status,
		Task:      run.Task,
		CreatedAt: run.CreatedAt,
		StartedAt: run.StartedAt,
		EndedAt:   run.EndedAt,
		Duration:  run.DurationSeconds,
		Outcome:   run.Outcome,
	}, nil
}

// TaskStatus represents status of a task
type TaskStatus struct {
	RunID     string
	Label     string
	Status    string
	Task      string
	CreatedAt time.Time
	StartedAt *time.Time
	EndedAt   *time.Time
	Duration  int
	Outcome   *sessionpkg.SubagentRunOutcome
}

// AnnounceResult provides intelligent result announcement
func (tm *TaskManager) AnnounceResult(ctx context.Context, task *SubagentTask, result, status string, resultBus bus.MessageBus) {
	announceType := "task"
	statusLabel := "completed"
	if status == "error" {
		statusLabel = "failed"
	}
	if status == "timeout" {
		statusLabel = "timed out"
	}

	// Intelligent announcement template
	announceContent := fmt.Sprintf(`A %s "%s" just %s.

Findings:
%s

Summarize this naturally for the user. Keep it brief (1-2 sentences).
Flow it into the conversation naturally.
Do not mention technical details like tokens, stats, or that this was a %s.

You can respond with NO_REPLY if no announcement is needed.`,
		announceType, task.Label, statusLabel, result, announceType)

	msg := bus.InboundMessage{
		ChannelID: "system",
		SenderID:  "subagent",
		Content:   announceContent,
		Timestamp: time.Now(),
	}

	if err := resultBus.PublishInbound(ctx, msg); err != nil {
		log.Error("Failed to announce subagent result", "error", err)
	}

	log.Debug("TaskManager", "announced_result", "task_id", task.ID, "status", status)
}

// FormatResult formats a subagent result for user display
func (tm *TaskManager) FormatResult(task *SubagentTask, result string, completion *sessionpkg.SubagentRun) string {
	if completion == nil {
		return result
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**%s**\n\n", task.Label))

	// Add summary if outcome exists
	if completion.Outcome != nil {
		sb.WriteString(tm.formatOutcome(completion.Outcome))
	}

	// Add duration
	if completion.DurationSeconds > 0 {
		sb.WriteString(fmt.Sprintf("\n*Completed in %s*\n", formatDuration(completion.DurationSeconds)))
	}

	return sb.String()
}

// formatOutcome formats the outcome for display
func (tm *TaskManager) formatOutcome(outcome *sessionpkg.SubagentRunOutcome) string {
	var sb strings.Builder

	switch outcome.Status {
	case "ok":
		sb.WriteString("✅ ")
	case "error":
		sb.WriteString("❌ ")
	case "timeout":
		sb.WriteString("⏱️ ")
	}

	sb.WriteString(outcome.Result)

	if outcome.Error != "" {
		sb.WriteString(fmt.Sprintf("\n\n*Error:* %s", outcome.Error))
	}

	if outcome.ToolCalls > 0 {
		sb.WriteString(fmt.Sprintf("\n\n*Used %d tool calls*", outcome.ToolCalls))
	}

	return sb.String()
}

// formatDuration formats seconds as human-readable duration
func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		minutes := seconds / 60
		sec := seconds % 60
		if sec == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm %ds", minutes, sec)
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// IsNoReply checks if to reply is NO_REPLY
func (tm *TaskManager) IsNoReply(content string) bool {
	return strings.TrimSpace(strings.ToUpper(content)) == "NO_REPLY"
}
