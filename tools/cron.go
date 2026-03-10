package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CronJob represents a scheduled job
type CronJob struct {
	ID      string
	Name    string
	Kind    string // "every" or "cron"
	EveryMs int64
	Expr    string
	Message string
	Channel string
	To      string
}

// CronService interface for cron operations
type CronService interface {
	AddJob(name, scheduleKind string, everyMs int64, cronExpr, message, channel, to string, oneTime, directSend bool) *JobEntry
	UpdateJob(id string, scheduleKind string, everyMs int64, cronExpr, message string, oneTime, directSend bool) *JobEntry
	ListJobs() []*JobEntry
	RemoveJob(id string) bool
	EnableJob(id string) bool
	DisableJob(id string) bool
	GetExecutionLogs(jobID string, limit int) string
}

// JobEntry represents a scheduled job entry
type JobEntry struct {
	ID             string
	Name           string
	Schedule       CronSchedule
	Message        string
	Channel        string
	To             string
	DeleteAfterRun bool
	CronExpr       string
	EntryID        int
	lastRun        time.Time
}

// CronSchedule represents a cron schedule
type CronSchedule struct {
	Kind     string
	EveryMs  int64
	CronExpr string
}

// CronTool schedules reminders and recurring tasks
type CronTool struct {
	cronService CronService
	mu          sync.RWMutex
	channel     string
	chatID      string
}

// NewCronTool creates a new cron tool
func NewCronTool(cronService CronService) *CronTool {
	return &CronTool{
		cronService: cronService,
	}
}

// Name returns the tool name
func (t *CronTool) Name() string {
	return "cron"
}

// Description returns the tool description
func (t *CronTool) Description() string {
	return "Create SCHEDULED/Delayed messages and reminders. This tool sends messages AFTER a specified time delay (NOT immediately). For immediate messages, use the 'message' tool. Supports: one-time reminders (one_time=true + every_seconds), recurring reminders (every_seconds), and cron expressions (cron_expr). Actions: add, list, remove, update, enable, disable."
}

// Parameters returns the JSON schema for parameters
func (t *CronTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["add", "list", "remove", "update", "enable", "disable", "logs"],
				"description": "Action: add (create SCHEDULED/delayed reminder), list (show all scheduled jobs), remove (delete scheduled job), update (modify job message/schedule), enable (enable a disabled job), disable (disable a job without deleting), logs (show execution history)"
			},
			"job_id": {
				"type": "string",
				"description": "The job ID (required for remove, update, enable, disable, logs actions, obtained from list action)"
			},
			"message": {
				"type": "string",
				"description": "The message text that will be sent AFTER the delay/schedule (NOT immediately). Required for add action, optional for update action."
			},
			"every_seconds": {
				"type": "integer",
				"description": "Delay in seconds BEFORE sending the message (e.g., 30 = wait 30s then send, 300 = wait 5min, 3600 = wait 1hour). Required for add action, optional for update action."
			},
			"cron_expr": {
				"type": "string",
				"description": "Cron expression for complex recurring schedules (e.g., '0 8 * * *' for daily at 8am, '0 */5 * * *' for every 5min). Optional for add and update actions."
			},
			"one_time": {
				"type": "boolean",
				"description": "If true, sends message once at the specified delay then deletes itself. Use with every_seconds for delayed one-time messages. Optional for add and update actions."
			}
		},
		"required": ["action"]
	}`)
}

// SetContext sets the current session context for delivery
func (t *CronTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channel = channel
	t.chatID = chatID
}

// Execute performs the cron action
func (t *CronTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, ok := args["action"].(string)
	if !ok {
		return "Error: action is required", nil
	}

	switch action {
	case "add":
		return t.addJob(args)
	case "list":
		return t.listJobs()
	case "remove":
		return t.removeJob(args)
	case "update":
		return t.updateJob(args)
	case "enable":
		return t.enableJob(args)
	case "disable":
		return t.disableJob(args)
	case "logs":
		return t.getExecutionLogs(args)
	default:
		return "Unknown action: " + action, nil
	}
}

// ConcurrentSafe returns false - cron operations modify shared state
func (t *CronTool) ConcurrentSafe() bool {
	return false
}

func (t *CronTool) addJob(args map[string]interface{}) (string, error) {
	message, ok := args["message"].(string)
	if !ok || message == "" {
		return "Error: message is required for add", nil
	}

	t.mu.RLock()
	channel := t.channel
	chatID := t.chatID
	cronService := t.cronService
	t.mu.RUnlock()

	if channel == "" || chatID == "" {
		return "Error: no session context (channel/chat_id)", nil
	}

	everySeconds := int64(0)
	cronExpr := ""
	oneTime := false

	// Check for one_time parameter
	if ot, ok := args["one_time"].(bool); ok {
		oneTime = ot
	}

	if es, ok := args["every_seconds"].(float64); ok && es > 0 {
		everySeconds = int64(es)
		if oneTime && everySeconds > 0 {
			// For one-time reminders, we create a one-time job
			// We'll handle this by setting DeleteAfterRun
		}
	} else if ce, ok := args["cron_expr"].(string); ok && ce != "" {
		cronExpr = ce
	} else {
		return "Error: either every_seconds or cron_expr is required", nil
	}

	var scheduleKind string
	var everyMs int64
	if everySeconds > 0 {
		scheduleKind = "every"
		everyMs = everySeconds * 1000
	} else {
		scheduleKind = "cron"
	}

	// Build job name from message (truncate to 50 chars)
	name := message
	if len(name) > 50 {
		name = name[:46] + "..."
	}
	if oneTime {
		name = "[ONCE] " + name
	}

	job := cronService.AddJob(name, scheduleKind, everyMs, cronExpr, message, channel, chatID, oneTime, false)

	if job == nil {
		return "Error: failed to create job", nil
	}

	if oneTime {
		return fmt.Sprintf("Created one-time reminder '%s' (id: %s) - will trigger once and be deleted", job.Name, job.ID), nil
	}
	return fmt.Sprintf("Created recurring reminder '%s' (id: %s)", job.Name, job.ID), nil
}

func (t *CronTool) listJobs() (string, error) {
	t.mu.RLock()
	cronService := t.cronService
	t.mu.RUnlock()

	if cronService == nil {
		return "Error: cron service not configured", nil
	}

	jobs := cronService.ListJobs()
	if len(jobs) == 0 {
		return "No scheduled jobs.", nil
	}

	var lines []string
	for _, job := range jobs {
		jobType := "Recurring"
		if job.DeleteAfterRun {
			jobType = "One-time"
		}
		lines = append(lines, fmt.Sprintf("- %s (id: %s, type: %s)", job.Name, job.ID, jobType))
	}

	return "Scheduled jobs:\n" + joinLines(lines), nil
}

func (t *CronTool) removeJob(args map[string]interface{}) (string, error) {
	jobID, ok := args["job_id"].(string)
	if !ok || jobID == "" {
		return "Error: job_id is required for remove", nil
	}

	t.mu.RLock()
	cronService := t.cronService
	t.mu.RUnlock()

	if cronService == nil {
		return "Error: cron service not configured", nil
	}

	if cronService.RemoveJob(jobID) {
		return "Removed job " + jobID, nil
	}
	return "Job " + jobID + " not found", nil
}

func (t *CronTool) updateJob(args map[string]interface{}) (string, error) {
	jobID, ok := args["job_id"].(string)
	if !ok || jobID == "" {
		return "Error: job_id is required for update", nil
	}

	t.mu.RLock()
	cronService := t.cronService
	t.mu.RUnlock()

	if cronService == nil {
		return "Error: cron service not configured", nil
	}

	// Parse optional parameters
	var scheduleKind string
	var everyMs int64
	var cronExpr string
	var message string
	var oneTime bool

	// Get message (optional for update)
	if msg, ok := args["message"].(string); ok {
		message = msg
	}

	// Get every_seconds (optional for update)
	if es, ok := args["every_seconds"].(float64); ok && es > 0 {
		everyMs = int64(es)
		scheduleKind = "every"
	}

	// Get cron_expr (optional for update)
	if ce, ok := args["cron_expr"].(string); ok && ce != "" {
		cronExpr = ce
		scheduleKind = "cron"
	}

	// Get one_time (optional for update)
	if ot, ok := args["one_time"].(bool); ok {
		oneTime = ot
	}

	job := cronService.UpdateJob(jobID, scheduleKind, everyMs, cronExpr, message, oneTime, false)
	if job == nil {
		return "Error: job not found or update failed", nil
	}

	return fmt.Sprintf("Updated job '%s' (id: %s)", job.Name, job.ID), nil
}

func (t *CronTool) enableJob(args map[string]interface{}) (string, error) {
	jobID, ok := args["job_id"].(string)
	if !ok || jobID == "" {
		return "Error: job_id is required for enable", nil
	}

	t.mu.RLock()
	cronService := t.cronService
	t.mu.RUnlock()

	if cronService == nil {
		return "Error: cron service not configured", nil
	}

	if cronService.EnableJob(jobID) {
		return "Enabled job " + jobID, nil
	}
	return "Job " + jobID + " not found", nil
}

func (t *CronTool) disableJob(args map[string]interface{}) (string, error) {
	jobID, ok := args["job_id"].(string)
	if !ok || jobID == "" {
		return "Error: job_id is required for disable", nil
	}

	t.mu.RLock()
	cronService := t.cronService
	t.mu.RUnlock()

	if cronService == nil {
		return "Error: cron service not configured", nil
	}

	if cronService.DisableJob(jobID) {
		return "Disabled job " + jobID, nil
	}
	return "Job " + jobID + " not found", nil
}

func (t *CronTool) getExecutionLogs(args map[string]interface{}) (string, error) {
	jobID, ok := args["job_id"].(string)
	if !ok || jobID == "" {
		return "Error: job_id is required for logs action", nil
	}

	t.mu.RLock()
	cronService := t.cronService
	t.mu.RUnlock()

	if cronService == nil {
		return "Error: cron service not configured", nil
	}

	// Limit parameter (default 50)
	limit := 50
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	return cronService.GetExecutionLogs(jobID, limit), nil
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

// WaitForJob waits for a cron job to complete
func WaitForJob(ctx context.Context, cronService CronService, jobID string, timeout time.Duration) (string, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			jobs := cronService.ListJobs()
			found := false
			for _, job := range jobs {
				if job.ID == jobID {
					found = true
					break
				}
			}
			if !found {
				return "Job completed", nil
			}
			if time.Now().After(deadline) {
				return "", fmt.Errorf("timeout waiting for job %s", jobID)
			}
		}
	}
}
