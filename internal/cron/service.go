package cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/konglong87/wukongbot/internal/bus"
	"github.com/konglong87/wukongbot/internal/session"
	"github.com/robfig/cron/v3"
)

// Schedule represents a cron schedule
type Schedule struct {
	Kind     string
	EveryMs  int64
	CronExpr string
}

// Service manages scheduled tasks
type Service struct {
	cron    *cron.Cron
	storage session.StorageInterface
	bus     bus.MessageBus
	mu      sync.RWMutex
	jobs    map[string]*JobEntry
	channel string
	to      string
}

// JobEntry represents a scheduled job entry
type JobEntry struct {
	ID             string
	Name           string
	Schedule       Schedule
	Message        string
	Channel        string
	To             string
	DeleteAfterRun bool
	DirectSend     bool // If true, send directly to user without LLM processing
	CronExpr       string
	EntryID        cron.EntryID
	lastRun        time.Time
}

// NewService creates a new cron service
func NewService(storage session.StorageInterface, bus bus.MessageBus, channel, to string) *Service {
	s := &Service{
		cron:    cron.New(cron.WithSeconds()),
		storage: storage,
		bus:     bus,
		jobs:    make(map[string]*JobEntry),
		channel: channel,
		to:      to,
	}
	s.cron.Start()
	return s
}

// AddJob adds a new scheduled job with "one_time" parameter
func (s *Service) AddJob(name, scheduleKind string, everyMs int64, cronExpr, message, channel, to string, oneTime, directSend bool) *JobEntry {
	id := uuid.New().String()[:8]

	scheduleStr := cronExpr
	if scheduleKind == "every" && everyMs > 0 {
		seconds := everyMs / 1000
		if seconds > 0 {
			scheduleStr = fmt.Sprintf("@every %ds", seconds)
		}
	}

	job := &JobEntry{
		ID:             id,
		Name:           name,
		Schedule:       Schedule{Kind: scheduleKind, EveryMs: everyMs, CronExpr: cronExpr},
		Message:        message,
		Channel:        channel,
		To:             to,
		DirectSend:     directSend, // 新增
		CronExpr:       scheduleStr,
		DeleteAfterRun: oneTime,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// For one-time jobs, use a wrapper that removes the job after execution
	entryID, err := s.cron.AddFunc(scheduleStr, func() {
		s.executeJob(job)
		if job.DeleteAfterRun {
			s.cron.Remove(job.EntryID)
			s.mu.Lock()
			delete(s.jobs, id)
			s.storage.DeleteCronJob(id)
			s.mu.Unlock()
			log.Info("Removed one-time cron job", "id", id, "name", name)
		}
	})
	if err != nil {
		log.Error("Failed to add cron job", "error", err)
		return nil
	}

	job.EntryID = entryID
	job.lastRun = time.Now()
	s.jobs[id] = job

	err = s.storage.SaveCronJob(&session.CronJob{
		ID:             id,
		Name:           name,
		Enabled:        true,
		Schedule:       map[string]interface{}{"kind": scheduleKind, "every_ms": everyMs, "expr": cronExpr},
		Payload:        map[string]interface{}{"message": message, "channel": channel, "to": to, "direct_send": directSend},
		State:          map[string]interface{}{"entry_id": int(entryID)},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		DeleteAfterRun: oneTime,
	})
	if err != nil {
		log.Error("AddJob Failed to save cron job", "error", err)
	} else {
		log.Info("AddJob Saved cron job ok ", "id", id, "name", name, "schedule", scheduleStr)
	}

	jobType := "Recurring"
	if oneTime {
		jobType = "One-time"
	}
	log.Info("Added cron job", "id", id, "name", name, "type", jobType, "schedule", scheduleStr)
	return job
}

// ListJobs returns all scheduled jobs
func (s *Service) ListJobs() []*JobEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*JobEntry, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// RemoveJob removes a job by ID
func (s *Service) RemoveJob(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return false
	}

	s.cron.Remove(job.EntryID)
	delete(s.jobs, id)
	s.storage.DeleteCronJob(id)

	log.Info("Removed cron job", "id", id, "name", job.Name)
	return true
}

// UpdateJob modifies an existing scheduled job
func (s *Service) UpdateJob(id string, scheduleKind string, everyMs int64, cronExpr, message string, oneTime, directSend bool) *JobEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return nil
	}

	// Remove the old job from cron scheduler
	s.cron.Remove(cron.EntryID(job.EntryID))

	// Update job fields
	if scheduleKind != "" {
		job.Schedule.Kind = scheduleKind
		job.Schedule.EveryMs = everyMs
		job.Schedule.CronExpr = cronExpr

		// Build new schedule string
		scheduleStr := cronExpr
		if scheduleKind == "every" && everyMs > 0 {
			seconds := everyMs / 1000
			if seconds > 0 {
				scheduleStr = fmt.Sprintf("@every %ds", seconds)
			}
		}
		job.CronExpr = scheduleStr
	}

	if message != "" {
		job.Message = message
	}

	job.DirectSend = directSend // 新增

	if oneTime {
		job.DeleteAfterRun = true
	}

	// Re-add the job to cron scheduler with updated settings
	entryID, err := s.cron.AddFunc(job.CronExpr, func() {
		s.executeJob(job)
		if job.DeleteAfterRun {
			s.cron.Remove(job.EntryID)
			s.mu.Lock()
			delete(s.jobs, id)
			s.storage.DeleteCronJob(id)
			s.mu.Unlock()
			log.Info("Removed one-time cron job", "id", id, "name", job.Name)
		}
	})
	if err != nil {
		log.Error("Failed to update cron job", "id", id, "error", err)
		// Revert the job
		job.EntryID = 0
		return nil
	}

	job.EntryID = entryID

	// Update in storage
	s.storage.SaveCronJob(&session.CronJob{
		ID:             id,
		Name:           job.Name,
		Enabled:        true,
		Schedule:       map[string]interface{}{"kind": job.Schedule.Kind, "every_ms": job.Schedule.EveryMs, "expr": job.Schedule.CronExpr},
		Payload:        map[string]interface{}{"message": job.Message, "channel": job.Channel, "to": job.To, "direct_send": directSend},
		State:          map[string]interface{}{"entry_id": int(entryID)},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		DeleteAfterRun: job.DeleteAfterRun,
	})

	log.Info("Updated cron job", "id", id, "name", job.Name, "schedule", job.CronExpr)
	return job
}

// EnableJob enables a disabled job
func (s *Service) EnableJob(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return false
	}

	// Add back to cron scheduler
	scheduleStr := job.CronExpr
	if job.Schedule.Kind == "every" && job.Schedule.EveryMs > 0 {
		seconds := job.Schedule.EveryMs / 1000
		if seconds > 0 {
			scheduleStr = fmt.Sprintf("@every %ds", seconds)
		}
	}

	entryID, err := s.cron.AddFunc(scheduleStr, func() {
		s.executeJob(job)
		if job.DeleteAfterRun {
			s.cron.Remove(job.EntryID)
			s.mu.Lock()
			delete(s.jobs, id)
			s.storage.DeleteCronJob(id)
			s.mu.Unlock()
			log.Info("Removed one-time cron job", "id", id, "name", job.Name)
		}
	})
	if err != nil {
		log.Error("Failed to enable cron job", "id", id, "error", err)
		return false
	}

	job.EntryID = entryID
	job.lastRun = time.Now()

	// Update in storage
	s.storage.SaveCronJob(&session.CronJob{
		ID:             id,
		Name:           job.Name,
		Enabled:        true,
		Schedule:       map[string]interface{}{"kind": job.Schedule.Kind, "every_ms": job.Schedule.EveryMs, "expr": job.Schedule.CronExpr},
		Payload:        map[string]interface{}{"message": job.Message, "channel": job.Channel, "to": job.To},
		State:          map[string]interface{}{"entry_id": int(entryID)},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		DeleteAfterRun: job.DeleteAfterRun,
	})

	log.Info("Enabled cron job", "id", id, "name", job.Name)
	return true
}

// DisableJob disables a job without deleting it
func (s *Service) DisableJob(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return false
	}

	// Remove from cron scheduler
	s.cron.Remove(cron.EntryID(job.EntryID))
	job.EntryID = 0

	// Update in storage
	s.storage.SaveCronJob(&session.CronJob{
		ID:             id,
		Name:           job.Name,
		Enabled:        false,
		Schedule:       map[string]interface{}{"kind": job.Schedule.Kind, "every_ms": job.Schedule.EveryMs, "expr": job.Schedule.CronExpr},
		Payload:        map[string]interface{}{"message": job.Message, "channel": job.Channel, "to": job.To},
		State:          map[string]interface{}{"entry_id": int(0)},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		DeleteAfterRun: job.DeleteAfterRun,
	})

	log.Info("Disabled cron job", "id", id, "name", job.Name)
	return true
}

// executeJob runs a job
func (s *Service) executeJob(job *JobEntry) {
	jobType := "Recurring"
	if job.DeleteAfterRun {
		jobType = "One-time"
	}
	log.Info("Executing cron job", "id", job.ID, "name", job.Name, "type", jobType, "direct_send", job.DirectSend)

	// Create execution log
	execLog := &session.ExecutionLog{
		JobID:      job.ID,
		JobName:    job.Name,
		Status:     "success",
		Error:      "",
		ExecutedAt: time.Now(),
	}

	if job.DirectSend {
		// Direct send mode - send directly to user without LLM processing
		outboundMsg := bus.OutboundMessage{
			MessageID:   uuid.New().String(),
			ChannelID:   job.Channel,
			RecipientID: job.To,
			Content:     job.Message,
			Timestamp:   time.Now(),
		}

		if err := s.bus.PublishOutbound(context.Background(), outboundMsg); err != nil {
			log.Error("Failed to send direct cron message", "error", err)
			execLog.Status = "error"
			execLog.Error = err.Error()
		} else {
			log.Info("Cron message sent directly to user", "id", job.ID, "to", job.To, "message_preview", job.Message[:min(100, len(job.Message))])
		}
	} else {
		// LLM processing mode - send through Agent Loop
		content := `【定时任务提醒】
这是一条用户预设的定时任务消息，请根据内容判断：
- 如需调用工具（如查询天气、获取数据等），请调用相应工具后返回结果
- 如是简单提醒或通知，请直接返回给用户

任务内容：` + job.Message

		msg := bus.InboundMessage{
			MessageID:  uuid.New().String(),
			ChannelID:  job.Channel,
			SenderID:   job.To, // Use real user ID as sender for proper routing
			SenderName: "Scheduled Task",
			Content:    content,
			Timestamp:  time.Now(),
			TraceId:    uuid.New().String(),
		}

		if err := s.bus.PublishInbound(context.Background(), msg); err != nil {
			log.Error("Failed to publish cron message", "error", err)
			execLog.Status = "error"
			execLog.Error = err.Error()
		} else {
			log.Info("Cron message sent to Agent Loop", "id", job.ID, "to", job.To, "message_preview", job.Message[:min(100, len(job.Message))])
		}
	}

	// Save execution log to storage
	if err := s.storage.SaveExecutionLog(execLog); err != nil {
		log.Error("Failed to save execution log", "id", job.ID, "error", err)
	}

	job.lastRun = time.Now()
}

// LoadJobs loads jobs from storage
func (s *Service) LoadJobs() error {
	jobs, err := s.storage.GetAllCronJobs()
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if !job.Enabled {
			continue
		}

		schedule := job.Schedule
		kind := schedule["kind"].(string)

		var scheduleStr string
		if kind == "every" {
			if everyMs, ok := schedule["every_ms"].(int64); ok && everyMs > 0 {
				seconds := everyMs / 1000
				scheduleStr = fmt.Sprintf("@every %ds", seconds)
			}
		} else if expr, ok := schedule["expr"].(string); ok {
			scheduleStr = expr
		}

		if scheduleStr == "" {
			continue
		}

		s.mu.Lock()
		entryID, err := s.cron.AddFunc(scheduleStr, func() {
			// 处理 direct_send 向后兼容
			var directSend bool
			if v, ok := job.Payload["direct_send"].(bool); ok {
				directSend = v
			}

			s.executeJob(&JobEntry{
				ID:         job.ID,
				Name:       job.Name,
				Channel:    job.Payload["channel"].(string),
				To:         job.Payload["to"].(string),
				Message:    job.Payload["message"].(string),
				DirectSend: directSend, // 新增
			})
		})
		s.mu.Unlock()

		if err != nil {
			log.Error("Failed to add cron job on load", "error", err)
			continue
		}

		// 处理 direct_send 向后兼容
		var directSend bool
		if v, ok := job.Payload["direct_send"].(bool); ok {
			directSend = v
		}

		s.jobs[job.ID] = &JobEntry{
			ID:             job.ID,
			Name:           job.Name,
			Schedule:       Schedule{Kind: kind},
			Channel:        job.Payload["channel"].(string),
			To:             job.Payload["to"].(string),
			Message:        job.Payload["message"].(string),
			DirectSend:     directSend, // 新增
			CronExpr:       scheduleStr,
			EntryID:        entryID,
			DeleteAfterRun: job.DeleteAfterRun,
		}

		log.Info("Loaded cron job", "id", job.ID, "name", job.Name)
	}

	return nil
}

// GetExecutionLogs retrieves execution history for a specific job
func (s *Service) GetExecutionLogs(jobID string, limit int) string {
	logs, err := s.storage.GetExecutionLogs(jobID, limit)
	if err != nil {
		log.Error("Failed to get execution logs", "job_id", jobID, "error", err)
		return fmt.Sprintf("Error: failed to get execution logs: %v", err)
	}

	if len(logs) == 0 {
		return fmt.Sprintf("No execution logs found for job %s", jobID)
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Execution history for job %s (showing %d most recent):", jobID, len(logs)))
	for _, log := range logs {
		statusIcon := "✅"
		if log.Status == "error" {
			statusIcon = "❌"
		}
		lines = append(lines, fmt.Sprintf("%s %s - %s", statusIcon, log.ExecutedAt.Format("2006-01-02 15:04:05"), log.JobName))
		if log.Error != "" {
			lines = append(lines, fmt.Sprintf("   Error: %s", log.Error))
		}
	}

	// Join lines
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

// Close shuts down the cron service
func (s *Service) Close() {
	s.cron.Stop()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
