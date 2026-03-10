package agentteam

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// TaskCoordinator manages task distribution to agents
type TaskCoordinator struct {
	registry *AgentRegistry
	queue    *TaskQueue
	mu       sync.Mutex
	provider LLMProvider
}

// NewTaskCoordinator creates a new task coordinator
func NewTaskCoordinator(registry *AgentRegistry, queue *TaskQueue, provider LLMProvider) *TaskCoordinator {
	return &TaskCoordinator{
		registry: registry,
		queue:    queue,
		provider: provider,
	}
}

// ExecuteTask executes a task by decomposing it and distributing to agents
func (c *TaskCoordinator) ExecuteTask(ctx context.Context, request *TaskRequest) (*TaskResult, error) {
	log.Info("ExecuteTask", "starting_task", "task_title", request.Title)

	// Create main task
	mainTask := &Task{
		ID:          generateTaskID(0),
		Title:       request.Title,
		Description: request.Description,
		Type:        request.Type,
		State:       StatePending,
		CreatedAt:   time.Now(),
		Metadata: TaskMetadata{
			Priority: 1,
		},
	}

	// Decompose task into subtasks
	decomposer := NewDecomposer(c.provider)
	decompResult, err := decomposer.DecomposeTask(ctx, mainTask)
	if err != nil {
		return nil, fmt.Errorf("task decomposition failed: %w", err)
	}

	// Add subtasks to queue
	for _, subtask := range decompResult.Tasks {
		c.queue.Enqueue(&subtask)
	}

	// Build dependency graph
	taskGraph := buildTaskGraph(decompResult.Tasks, decompResult.Dependencies)

	// Execute tasks following dependency order
	log.Info("ExecuteTask", "executing_tasks", "main_task", mainTask.ID, "subtasks", len(decompResult.Tasks))

	result := &TaskResult{
		TaskID:   mainTask.ID,
		Title:    mainTask.Title,
		State:    StateInProgress,
		Subtasks: make([]SubtaskResult, 0, len(decompResult.Tasks)),
		Summary:  decompResult.Summary,
	}

	// Process tasks in dependency order
	startTime := time.Now()

	for _, taskID := range taskGraph {
		task := c.queue.Get(taskID)
		if task == nil {
			log.Error("ExecuteTask", "task_not_found", "task_id", taskID)
			continue
		}

		log.Info("ExecuteTask", "processing_subtask", "task_id", task.ID, "title", task.Title)

		// Assign to agent
		task.State = StateInProgress
		startedAt := time.Now()
		task.StartedAt = &startedAt

		agent, err := c.assignTask(ctx, task)
		if err != nil {
			log.Error("ExecuteTask", "agent_assignment_failed", "task_id", task.ID, "error", err)
			task.State = StateFailed
			task.Error = fmt.Sprintf("Failed to find suitable agent: %v", err)
		} else {
			// Execute task
			subtaskResult := c.executeSubtask(ctx, task, agent)
			result.Subtasks = append(result.Subtasks, *subtaskResult)
			c.registry.DecrementLoad(agent.ID)
		}

		if task.State != StateCompleted && task.State != StateFailed {
			log.Error("ExecuteTask", "subtask_failed", "task_id", task.ID, "error", task.Error)
			break
		}
	}

	// Mark main task as completed
	if allTasksCompleted(result.Subtasks) {
		mainTask.State = StateCompleted
		result.State = StateCompleted
		result.Success = true
		result.Summary = decompResult.Summary
	} else {
		mainTask.State = StateFailed
		result.State = StateFailed
		result.Summary = "Some subtasks failed"
		result.Error = fmt.Sprintf("%d of %d subtasks failed", countFailed(result.Subtasks), len(result.Subtasks))
	}

	completedAt := time.Now()
	mainTask.CompletedAt = &completedAt
	result.Duration = int(completedAt.Sub(startTime).Seconds())

	log.Info("ExecuteTask", "task_completed", "task_id", mainTask.ID, "state", mainTask.State, "duration", result.Duration)

	return result, nil
}

// assignTask finds and assigns an agent to a task
func (c *TaskCoordinator) assignTask(ctx context.Context, task *Task) (*Agent, error) {
	// Build criteria from task metadata
	criteria := AgentCapabilities{
		Language:      task.Metadata.Language,
		Domain:        task.Metadata.Domain,
		RequiredTools: []string{},
	}

	// Parse tool hints if provided
	if task.Metadata.ToolHints != "" {
		criteria.RequiredTools = parseToolHints(task.Metadata.ToolHints)
	}

	// Find matching agents
	agents := c.registry.FindAgents(criteria)
	if len(agents) == 0 {
		return nil, fmt.Errorf("no available agents matching criteria (language=%s, domain=%s)", criteria.Language, criteria.Domain)
	}

	// Select best agent (lowest load)
	var selectedAgent *Agent
	minLoad := -1

	for _, agent := range agents {
		if minLoad == -1 || agent.Load < minLoad {
			selectedAgent = agent
			minLoad = agent.Load
		}
	}

	// Increment load
	if err := c.registry.IncrementLoad(selectedAgent.ID); err != nil {
		return nil, err
	}

	// Assign agent to task
	task.Agents = []Agent{*selectedAgent}

	log.Info("Task assigned to agent", "task_id", task.ID, "agent", selectedAgent.Name, "agent_load", selectedAgent.Load)

	return selectedAgent, nil
}

// executeSubtask executes a subtask using the assigned agent
func (c *TaskCoordinator) executeSubtask(ctx context.Context, task *Task, agent *Agent) *SubtaskResult {
	log.Info("executeSubtask", "task_id", task.ID, "agent", agent.Name, "model", agent.Model)

	startTime := time.Now()

	// For MVP, we delegate to existing subagent manager
	// This would normally involve calling the agent via LLM with the task description

	resultText := fmt.Sprintf("Subtask completed by %s: %s", agent.Name, task.Description)

	// Mark task as completed
	task.State = StateCompleted
	task.Result = resultText
	completedAt := time.Now()
	task.CompletedAt = &completedAt

	// Return result
	subtaskResult := &SubtaskResult{
		SubtaskID:  task.ID,
		Title:      task.Title,
		State:      task.State,
		AssignedTo: agent.Name,
		Result:     resultText,
		Duration:   int(time.Since(startTime).Seconds()),
	}

	log.Info("ExecuteTask", "subtask_completed", "task_id", task.ID, "duration", subtaskResult.Duration)

	return subtaskResult
}

// buildTaskGraph constructs task execution order based on dependencies
func buildTaskGraph(tasks []Task, dependencies [][]string) []string {
	// Simple topological sort implementation
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	for _, task := range tasks {
		graph[task.ID] = []string{}
		inDegree[task.ID] = 0
	}

	for _, dependency := range dependencies {
		if len(dependency) < 2 {
			continue
		}
		fromTaskID := dependency[0]
		for _, toTaskID := range dependency[1:] {
			graph[fromTaskID] = append(graph[fromTaskID], toTaskID)
			inDegree[toTaskID]++
		}
	}

	// Topological sort
	var result []string
	queue := make([]string, 0)

	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	for _, taskID := range queue {
		result = append(result, taskID)

		for _, successorID := range graph[taskID] {
			inDegree[successorID]--
			if inDegree[successorID] == 0 {
				queue = append(queue, successorID)
			}
		}
	}

	log.Debug("TaskGraph built", "order", result, "tasks", len(tasks))

	return result
}

func countFailed(subtasks []SubtaskResult) int {
	count := 0
	for _, st := range subtasks {
		if st.State == StateFailed {
			count++
		}
	}
	return count
}

func allTasksCompleted(subtasks []SubtaskResult) bool {
	for _, st := range subtasks {
		if st.State != StateCompleted {
			return false
		}
	}
	return true
}

func parseToolHints(hints string) []string {
	// Simple parsing - split by comma
	return nil // MVP: ignore tool hints for now
}
