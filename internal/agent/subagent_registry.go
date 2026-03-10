package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	sessionpkg "github.com/konglong87/wukongbot/internal/session"
)

// SubagentRegistry manages subagent runs with persistence
type SubagentRegistry struct {
	storage       sessionpkg.StorageInterface
	policy        *ToolPolicy
	mu            sync.RWMutex
	running       map[string]*SubagentTask
	sweeperStop   chan struct{}
	onRunComplete func(runID string, record *sessionpkg.SubagentRun)
}

// NewSubagentRegistry creates a new subagent registry
func NewSubagentRegistry(storage sessionpkg.StorageInterface, policy *ToolPolicy) *SubagentRegistry {
	registry := &SubagentRegistry{
		storage:     storage,
		policy:      policy,
		running:     make(map[string]*SubagentTask),
		sweeperStop: make(chan struct{}),
	}

	// Start sweeper goroutine
	go registry.sweeper()

	// Recover running tasks on startup
	registry.recoverRunningTasks()

	return registry
}

// recoverRunningTasks recovers any tasks that were running before a restart
func (r *SubagentRegistry) recoverRunningTasks() {
	runs, err := r.storage.GetRunningSubagentRuns()
	if err != nil {
		log.Error("Failed to recover running tasks", "error", err)
		return
	}

	for _, run := range runs {
		// Mark as failed since the process was terminated
		if run.Status == "running" || run.Status == "pending" {
			outcome := &sessionpkg.SubagentRunOutcome{
				Status:  "error",
				Error:   "Process terminated unexpectedly",
				EndedAt: &time.Time{},
			}
			now := time.Now()
			outcome.EndedAt = &now

			if err := r.storage.RecordSubagentOutcome(run.RunID, outcome); err != nil {
				log.Error("Failed to update recovered task", "run_id", run.RunID, "error", err)
			}
		}
	}
}

// CreateRun creates a new subagent run record
func (r *SubagentRegistry) CreateRun(task *SubagentTask, sessionKey string) error {
	run := &sessionpkg.SubagentRun{
		RunID:               task.ID,
		ChildSessionKey:     sessionKey,
		RequesterSessionKey: task.OriginCh + ":" + task.OriginID,
		Task:                task.Task,
		Label:               task.Label,
		Status:              "pending",
		OriginChannel:       task.OriginCh,
		OriginChatID:        task.OriginID,
		CreatedAt:           time.Now(),
		CleanupPolicy:       "keep",
	}

	return r.storage.CreateSubagentRun(run)
}

// UpdateRunStatus updates the run status
func (r *SubagentRegistry) UpdateRunStatus(runID string, status string, startedAt, endedAt *time.Time) error {
	return r.storage.UpdateSubagentRunStatus(runID, status, startedAt, endedAt)
}

// RecordOutcome records the execution outcome
func (r *SubagentRegistry) RecordOutcome(runID string, outcome *sessionpkg.SubagentRunOutcome) error {
	err := r.storage.RecordSubagentOutcome(runID, outcome)

	// Trigger completion callback
	if err == nil && r.onRunComplete != nil {
		record, getErr := r.storage.GetSubagentRun(runID)
		if getErr == nil {
			r.onRunComplete(runID, record)
		}
	}

	return err
}

// GetRun retrieves a run record
func (r *SubagentRegistry) GetRun(runID string) (*sessionpkg.SubagentRun, error) {
	return r.storage.GetSubagentRun(runID)
}

// GetRunningRuns retrieves all currently running runs
func (r *SubagentRegistry) GetRunningRuns() ([]*sessionpkg.SubagentRun, error) {
	return r.storage.GetRunningSubagentRuns()
}

// GetHistory retrieves historical runs
func (r *SubagentRegistry) GetHistory(page, limit int) ([]*sessionpkg.SubagentRun, error) {
	return r.storage.GetSubagentRunHistory(page, limit)
}

// RegisterTask registers a running task in memory
func (r *SubagentRegistry) RegisterTask(task *SubagentTask) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running[task.ID] = task
}

// UnregisterTask unregisters a task
func (r *SubagentRegistry) UnregisterTask(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.running, taskID)
}

// GetRunningCount returns the count of running tasks
func (r *SubagentRegistry) GetRunningCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.running)
}

// CanSpawn checks if a new subagent can be spawned based on policy
func (r *SubagentRegistry) CanSpawn() bool {
	if r.policy == nil {
		return true // No policy restriction
	}
	return r.policy.ValidateResourceUsage(r.GetRunningCount())
}

// GetPolicy returns the current policy
func (r *SubagentRegistry) GetPolicy() *ToolPolicy {
	return r.policy
}

// SetCompletionCallback sets a callback for run completions
func (r *SubagentRegistry) SetCompletionCallback(callback func(runID string, record *sessionpkg.SubagentRun)) {
	r.onRunComplete = callback
}

// sweeper periodically cleans up old runs
func (r *SubagentRegistry) sweeper() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Clean up runs older than 7 days
			cutoff := time.Now().AddDate(0, 0, -7)
			count, err := r.storage.CleanupExpiredSubagentRuns(cutoff)
			if err != nil {
				log.Error("Failed to cleanup expired runs", "error", err)
			} else if count > 0 {
				log.Info("Cleaned up expired runs", "count", count)
			}
		case <-r.sweeperStop:
			return
		}
	}
}

// Close stops the registry
func (r *SubagentRegistry) Close() {
	close(r.sweeperStop)
}

// Spawn implements SubagentManager interface
func (r *SubagentRegistry) Spawn(ctx context.Context, task, label, originChannel, originChatID string) (string, error) {
	if !r.CanSpawn() {
		return "", fmt.Errorf("cannot spawn subagent: max concurrent limit reached (%d)", r.policy.GetMaxConcurrent())
	}

	taskID := uuid.New().String()[:8]
	if len(label) == 0 {
		if len(task) > 30 {
			label = task[:30] + "..."
		} else {
			label = task
		}
	} else if len(label) > 30 {
		label = label[:30] + "..."
	}

	subagent := &SubagentTask{
		ID:       taskID,
		Label:    label,
		Task:     task,
		OriginCh: originChannel,
		OriginID: originChatID,
		Done:     make(chan struct{}),
	}

	// Create persistent record
	if err := r.CreateRun(subagent, originChannel+":"+originChatID); err != nil {
		return "", fmt.Errorf("failed to create run record: %w", err)
	}

	// Register in memory
	r.RegisterTask(subagent)

	// Spawn completion handler
	go func() {
		<-subagent.Done
		r.UnregisterTask(taskID)
	}()

	log.Info("Spawned subagent", "id", taskID, "label", label)
	return taskID, nil
}

// AnnounceResult announces the result to the main agent (kept for compatibility)
func (r *SubagentRegistry) AnnounceResult(ctx context.Context, task *SubagentTask, result, status string, resultBus interface{}) {
	log.Debug("Subagent announced result", "id", task.ID, "status", status, "to", task.OriginCh+":"+task.OriginID)
	// This is a placeholder - actual implementation would publish to the message bus
}

// GetMaxConcurrent returns the max concurrent limit
func (p *ToolPolicy) GetMaxConcurrent() int {
	if p == nil || p.Resources.MaxConcurrent <= 0 {
		return 5 // Default
	}
	return p.Resources.MaxConcurrent
}
