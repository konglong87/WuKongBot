package session

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// GORMStorage handles database operations using GORM
type GORMStorage struct {
	db *gorm.DB
}

// ConfigModel represents a config entry in the database
type ConfigModel struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

// SessionModel represents a session in the database
type SessionModel struct {
	ID        int64                  `gorm:"primaryKey;autoIncrement"`
	Key       string                 `gorm:"uniqueIndex;not null;column:session_key"`
	CreatedAt time.Time              `gorm:"column:created_at"`
	UpdatedAt time.Time              `gorm:"column:updated_at"`
	Metadata  map[string]interface{} `gorm:"type:json"`
}

// MessageModel represents a message in the database
type MessageModel struct {
	ID        int64        `gorm:"primaryKey;autoIncrement"`
	SessionID int64        `gorm:"not null;index:idx_messages_session;column:session_id"`
	Role      string       `gorm:"not null;size:50"`
	Content   string       `gorm:"type:text;not null"`
	Timestamp time.Time    `gorm:"index:idx_messages_timestamp;column:timestamp"`
	Session   SessionModel `gorm:"foreignKey:SessionID"`
}

// CronJobModel represents a scheduled cron job in the database
type CronJobModel struct {
	ID             string                 `gorm:"primaryKey;size:255"`
	Name           string                 `gorm:"not null;size:255"`
	Enabled        bool                   `gorm:"not null;column:enabled"`
	Schedule       map[string]interface{} `gorm:"type:json;not null"`
	Payload        map[string]interface{} `gorm:"type:json;not null"`
	State          map[string]interface{} `gorm:"type:json;not null"`
	CreatedAt      time.Time              `gorm:"column:created_at"`
	UpdatedAt      time.Time              `gorm:"column:updated_at"`
	DeleteAfterRun bool                   `gorm:"not null;column:delete_after_run"`
}

// ExecutionLogModel represents a cron job execution log entry in the database
type ExecutionLogModel struct {
	ID         int64        `gorm:"primaryKey;autoIncrement"`
	JobID      string       `gorm:"not null;size:255;index:idx_execution_logs_job;column:job_id"`
	JobName    string       `gorm:"not null;size:255;column:job_name"`
	Status     string       `gorm:"not null;size:20"`
	Error      string       `gorm:"type:text"`
	ExecutedAt time.Time    `gorm:"not null;index:idx_execution_logs_executed_at;column:executed_at"`
	CronJob    CronJobModel `gorm:"foreignKey:JobID"`
}

// TableName specifies the table name for ExecutionLogModel
func (ExecutionLogModel) TableName() string {
	return "execution_logs"
}

// TableName specifies the table name for SessionModel
func (SessionModel) TableName() string {
	return "sessions"
}

// TableName specifies the table name for MessageModel
func (MessageModel) TableName() string {
	return "messages"
}

// TableName specifies the table name for CronJobModel
func (CronJobModel) TableName() string {
	return "cron_jobs"
}

// TableName specifies the table name for ConfigModel
func (ConfigModel) TableName() string {
	return "config"
}

// SubagentRunModel represents a subagent run in the database
type SubagentRunModel struct {
	ID                  int64      `gorm:"primaryKey;autoIncrement"`
	RunID               string     `gorm:"uniqueIndex;not null;column:run_id"`
	ChildSessionKey     string     `gorm:"not null;column:child_session_key"`
	RequesterSessionKey string     `gorm:"not null;column:requester_session_key"`
	Task                string     `gorm:"type:text;not null"`
	Label               string     `gorm:"size:255"`
	Status              string     `gorm:"not null;size:20;column:status"`
	Outcome             string     `gorm:"type:json;column:outcome"`
	OriginChannel       string     `gorm:"not null;size:255;column:origin_channel"`
	OriginChatID        string     `gorm:"not null;size:255;column:origin_chat_id"`
	CreatedAt           time.Time  `gorm:"not null;index:idx_subagent_runs_created;column:created_at"`
	StartedAt           *time.Time `gorm:"column:started_at"`
	EndedAt             *time.Time `gorm:"column:ended_at"`
	DurationSeconds     int        `gorm:"column:duration_seconds"`
	CleanupPolicy       string     `gorm:"default:'keep';size:50;column:cleanup_policy"`
	ArchivedAt          *time.Time `gorm:"index:idx_subagent_runs_archived;column:archived_at"`
}

// TableName specifies the table name for SubagentRunModel
func (SubagentRunModel) TableName() string {
	return "subagent_runs"
}

// TmuxSessionModel represents a tmux session in the database
type TmuxSessionModel struct {
	ID              string                 `gorm:"primaryKey;size:255"`
	UserID          string                 `gorm:"not null;size:255;index:idx_tmux_sessions_user;column:user_id"`
	SocketPath      string                 `gorm:"not null;size:500;column:socket_path"`
	SessionName     string                 `gorm:"not null;size:255;column:session_name"`
	Target          string                 `gorm:"not null;size:255;column:target"`
	Command         string                 `gorm:"not null;type:text;column:command"`
	State           string                 `gorm:"not null;size:20;index:idx_tmux_sessions_state;column:state"`
	CronJobID       string                 `gorm:"size:255;column:cron_job_id"`
	MonitorInterval int64                  `gorm:"not null;column:monitor_interval"`
	CreatedAt       time.Time              `gorm:"not null;column:created_at"`
	LastActivity    time.Time              `gorm:"not null;column:last_activity"`
	LastOutput      string                 `gorm:"type:text;column:last_output"`
	LastOutputHash  string                 `gorm:"size:64;column:last_output_hash"`
	Metadata        map[string]interface{} `gorm:"type:json;column:metadata"`
	ClosedAt        *time.Time             `gorm:"column:closed_at"`
}

// TableName specifies the table name for TmuxSessionModel
func (TmuxSessionModel) TableName() string {
	return "tmux_sessions"
}

// NewGORMStorage creates a new GORM storage instance
func NewGORMStorage(dsn string) (*GORMStorage, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto migrate the schema
	if err := db.AutoMigrate(&ConfigModel{}, &SessionModel{}, &MessageModel{}, &CronJobModel{}, &ExecutionLogModel{}, &SubagentRunModel{}, &TmuxSessionModel{}); err != nil {
		return nil, err
	}

	storage := &GORMStorage{db: db}
	return storage, nil
}

// Close closes the database connection
func (s *GORMStorage) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GetConfig retrieves a config value by key
func (s *GORMStorage) GetConfig(key string) (string, error) {
	var config ConfigModel
	result := s.db.Where("key = ?", key).First(&config)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", result.Error
	}
	return config.Value, nil
}

// SetConfig stores a config value
func (s *GORMStorage) SetConfig(key, value string) error {
	config := ConfigModel{
		Key:   key,
		Value: value,
	}
	return s.db.Save(&config).Error
}

// GetOrCreateSession gets an existing session or creates a new one
func (s *GORMStorage) GetOrCreateSession(sessionKey string) (*Session, error) {
	var sessionModel SessionModel
	now := time.Now()

	// Try to find existing session
	result := s.db.Where("session_key = ?", sessionKey).First(&sessionModel)

	if result.Error == nil {
		// Session exists, update timestamp
		sessionModel.UpdatedAt = now
		if err := s.db.Save(&sessionModel).Error; err != nil {
			return nil, err
		}

		return &Session{
			ID:        sessionModel.ID,
			Key:       sessionModel.Key,
			CreatedAt: sessionModel.CreatedAt,
			UpdatedAt: sessionModel.UpdatedAt,
			Metadata:  sessionModel.Metadata,
		}, nil
	}

	if result.Error != gorm.ErrRecordNotFound {
		return nil, result.Error
	}

	// Create new session
	sessionModel = SessionModel{
		Key:       sessionKey,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]interface{}),
	}

	if err := s.db.Create(&sessionModel).Error; err != nil {
		return nil, err
	}

	return &Session{
		ID:        sessionModel.ID,
		Key:       sessionModel.Key,
		CreatedAt: sessionModel.CreatedAt,
		UpdatedAt: sessionModel.UpdatedAt,
		Metadata:  sessionModel.Metadata,
	}, nil
}

// AddMessage adds a message to a session
func (s *GORMStorage) AddMessage(sessionID int64, role, content string) error {
	message := MessageModel{
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	return s.db.Create(&message).Error
}

// GetMessages retrieves messages for a session
func (s *GORMStorage) GetMessages(sessionID int64, limit int) ([]Message, error) {
	var messageModels []MessageModel
	query := s.db.Where("session_id = ?", sessionID).Order("timestamp ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&messageModels).Error; err != nil {
		return nil, err
	}

	messages := make([]Message, len(messageModels))
	for i, msgModel := range messageModels {
		messages[i] = Message{
			ID:        msgModel.ID,
			SessionID: msgModel.SessionID,
			Role:      msgModel.Role,
			Content:   msgModel.Content,
			Timestamp: msgModel.Timestamp,
		}
	}

	return messages, nil
}

// GetLastNMessages retrieves the last N messages from a session
func (s *GORMStorage) GetLastNMessages(sessionID int64, n int) ([]Message, error) {
	var messageModels []MessageModel
	if err := s.db.Where("session_id = ?", sessionID).
		Order("timestamp DESC").
		Limit(n).
		Find(&messageModels).Error; err != nil {
		return nil, err
	}

	messages := make([]Message, len(messageModels))
	for i, msgModel := range messageModels {
		messages[i] = Message{
			ID:        msgModel.ID,
			SessionID: msgModel.SessionID,
			Role:      msgModel.Role,
			Content:   msgModel.Content,
			Timestamp: msgModel.Timestamp,
		}
	}

	// Reverse to get chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// DeleteSession deletes a session and all its messages
func (s *GORMStorage) DeleteSession(sessionID int64) error {
	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete messages
	if err := tx.Where("session_id = ?", sessionID).Delete(&MessageModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete session
	if err := tx.Delete(&SessionModel{}, sessionID).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// SaveCronJob saves or updates a cron job
func (s *GORMStorage) SaveCronJob(job *CronJob) error {
	cronJobModel := CronJobModel{
		ID:             job.ID,
		Name:           job.Name,
		Enabled:        job.Enabled,
		Schedule:       job.Schedule,
		Payload:        job.Payload,
		State:          job.State,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      time.Now(),
		DeleteAfterRun: job.DeleteAfterRun,
	}

	return s.db.Save(&cronJobModel).Error
}

// GetCronJob retrieves a cron job by ID
func (s *GORMStorage) GetCronJob(jobID string) (*CronJob, error) {
	var cronJobModel CronJobModel
	result := s.db.Where("id = ?", jobID).First(&cronJobModel)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}

	return &CronJob{
		ID:             cronJobModel.ID,
		Name:           cronJobModel.Name,
		Enabled:        cronJobModel.Enabled,
		Schedule:       cronJobModel.Schedule,
		Payload:        cronJobModel.Payload,
		State:          cronJobModel.State,
		CreatedAt:      cronJobModel.CreatedAt,
		UpdatedAt:      cronJobModel.UpdatedAt,
		DeleteAfterRun: cronJobModel.DeleteAfterRun,
	}, nil
}

// GetAllCronJobs retrieves all cron jobs
func (s *GORMStorage) GetAllCronJobs() ([]*CronJob, error) {
	var cronJobModels []CronJobModel
	if err := s.db.Order("created_at ASC").Find(&cronJobModels).Error; err != nil {
		return nil, err
	}

	jobs := make([]*CronJob, len(cronJobModels))
	for i, model := range cronJobModels {
		jobs[i] = &CronJob{
			ID:             model.ID,
			Name:           model.Name,
			Enabled:        model.Enabled,
			Schedule:       model.Schedule,
			Payload:        model.Payload,
			State:          model.State,
			CreatedAt:      model.CreatedAt,
			UpdatedAt:      model.UpdatedAt,
			DeleteAfterRun: model.DeleteAfterRun,
		}
	}

	return jobs, nil
}

// DeleteCronJob deletes a cron job by ID
func (s *GORMStorage) DeleteCronJob(jobID string) error {
	return s.db.Where("id = ?", jobID).Delete(&CronJobModel{}).Error
}

// SaveExecutionLog saves an execution log entry
func (s *GORMStorage) SaveExecutionLog(log *ExecutionLog) error {
	model := &ExecutionLogModel{
		JobID:      log.JobID,
		JobName:    log.JobName,
		Status:     log.Status,
		Error:      log.Error,
		ExecutedAt: log.ExecutedAt,
	}
	return s.db.Create(model).Error
}

// GetExecutionLogs retrieves execution logs for a specific job
func (s *GORMStorage) GetExecutionLogs(jobID string, limit int) ([]*ExecutionLog, error) {
	var models []ExecutionLogModel
	query := s.db.Where("job_id = ?", jobID).Order("executed_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	logs := make([]*ExecutionLog, len(models))
	for i, m := range models {
		logs[i] = &ExecutionLog{
			ID:         m.ID,
			JobID:      m.JobID,
			JobName:    m.JobName,
			Status:     m.Status,
			Error:      m.Error,
			ExecutedAt: m.ExecutedAt,
		}
	}
	return logs, nil
}

// CreateSubagentRun creates a new subagent run record
func (s *GORMStorage) CreateSubagentRun(run *SubagentRun) error {
	model := &SubagentRunModel{
		RunID:               run.RunID,
		ChildSessionKey:     run.ChildSessionKey,
		RequesterSessionKey: run.RequesterSessionKey,
		Task:                run.Task,
		Label:               run.Label,
		Status:              run.Status,
		OriginChannel:       run.OriginChannel,
		OriginChatID:        run.OriginChatID,
		CreatedAt:           run.CreatedAt,
		CleanupPolicy:       run.CleanupPolicy,
	}
	return s.db.Create(model).Error
}

// UpdateSubagentRunStatus updates the status of a subagent run
func (s *GORMStorage) UpdateSubagentRunStatus(runID string, status string, startedAt, endedAt *time.Time) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if startedAt != nil {
		updates["started_at"] = *startedAt
	}
	if endedAt != nil {
		updates["ended_at"] = *endedAt
		if startedAt != nil {
			duration := int(endedAt.Sub(*startedAt).Seconds())
			updates["duration_seconds"] = duration
		}
	}
	return s.db.Model(&SubagentRunModel{}).Where("run_id = ?", runID).Updates(updates).Error
}

// RecordSubagentOutcome records the execution outcome
func (s *GORMStorage) RecordSubagentOutcome(runID string, outcome *SubagentRunOutcome) error {
	outcomeJSON, _ := json.Marshal(outcome)
	updates := map[string]interface{}{
		"outcome": string(outcomeJSON),
	}
	if outcome.EndedAt != nil {
		updates["ended_at"] = outcome.EndedAt
		updates["status"] = outcome.Status
	}
	return s.db.Model(&SubagentRunModel{}).Where("run_id = ?", runID).Updates(updates).Error
}

// GetSubagentRun retrieves a subagent run by run ID
func (s *GORMStorage) GetSubagentRun(runID string) (*SubagentRun, error) {
	var model SubagentRunModel
	result := s.db.Where("run_id = ?", runID).First(&model)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}

	run := &SubagentRun{
		RunID:               model.RunID,
		ChildSessionKey:     model.ChildSessionKey,
		RequesterSessionKey: model.RequesterSessionKey,
		Task:                model.Task,
		Label:               model.Label,
		Status:              model.Status,
		OriginChannel:       model.OriginChannel,
		OriginChatID:        model.OriginChatID,
		CreatedAt:           model.CreatedAt,
		StartedAt:           model.StartedAt,
		EndedAt:             model.EndedAt,
		DurationSeconds:     model.DurationSeconds,
		CleanupPolicy:       model.CleanupPolicy,
		ArchivedAt:          model.ArchivedAt,
	}

	if model.Outcome != "" {
		var outcome SubagentRunOutcome
		if err := json.Unmarshal([]byte(model.Outcome), &outcome); err == nil {
			run.Outcome = &outcome
		}
	}

	return run, nil
}

// GetRunningSubagentRuns retrieves all currently running subagent runs
func (s *GORMStorage) GetRunningSubagentRuns() ([]*SubagentRun, error) {
	var models []SubagentRunModel
	if err := s.db.Where("status IN ?", []string{"pending", "running"}).Find(&models).Error; err != nil {
		return nil, err
	}

	runs := make([]*SubagentRun, len(models))
	for i, model := range models {
		run := &SubagentRun{
			RunID:               model.RunID,
			ChildSessionKey:     model.ChildSessionKey,
			RequesterSessionKey: model.RequesterSessionKey,
			Task:                model.Task,
			Label:               model.Label,
			Status:              model.Status,
			OriginChannel:       model.OriginChannel,
			OriginChatID:        model.OriginChatID,
			CreatedAt:           model.CreatedAt,
			StartedAt:           model.StartedAt,
			EndedAt:             model.EndedAt,
			DurationSeconds:     model.DurationSeconds,
			CleanupPolicy:       model.CleanupPolicy,
			ArchivedAt:          model.ArchivedAt,
		}

		if model.Outcome != "" {
			var outcome SubagentRunOutcome
			if err := json.Unmarshal([]byte(model.Outcome), &outcome); err == nil {
				run.Outcome = &outcome
			}
		}

		runs[i] = run
	}

	return runs, nil
}

// GetSubagentRunHistory retrieves historical subagent runs with pagination
func (s *GORMStorage) GetSubagentRunHistory(page, limit int) ([]*SubagentRun, error) {
	var models []SubagentRunModel
	offset := (page - 1) * limit
	if err := s.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, err
	}

	runs := make([]*SubagentRun, len(models))
	for i, model := range models {
		run := &SubagentRun{
			RunID:               model.RunID,
			ChildSessionKey:     model.ChildSessionKey,
			RequesterSessionKey: model.RequesterSessionKey,
			Task:                model.Task,
			Label:               model.Label,
			Status:              model.Status,
			OriginChannel:       model.OriginChannel,
			OriginChatID:        model.OriginChatID,
			CreatedAt:           model.CreatedAt,
			StartedAt:           model.StartedAt,
			EndedAt:             model.EndedAt,
			DurationSeconds:     model.DurationSeconds,
			CleanupPolicy:       model.CleanupPolicy,
			ArchivedAt:          model.ArchivedAt,
		}

		if model.Outcome != "" {
			var outcome SubagentRunOutcome
			if err := json.Unmarshal([]byte(model.Outcome), &outcome); err == nil {
				run.Outcome = &outcome
			}
		}

		runs[i] = run
	}

	return runs, nil
}

// CleanupExpiredSubagentRuns removes expired subagent runs based on policy
func (s *GORMStorage) CleanupExpiredSubagentRuns(olderThan time.Time) (int, error) {
	result := s.db.Where("archived_at IS NOT NULL AND archived_at < ?", olderThan).Delete(&SubagentRunModel{})
	return int(result.RowsAffected), result.Error
}

// CreateTmuxSession creates a new tmux session record
func (s *GORMStorage) CreateTmuxSession(ctx context.Context, session *TmuxSession) error {
	model := &TmuxSessionModel{
		ID:              session.ID,
		UserID:          session.UserID,
		SocketPath:      session.SocketPath,
		SessionName:     session.SessionName,
		Target:          session.Target,
		Command:         session.Command,
		State:           string(session.State),
		CronJobID:       session.CronJobID,
		MonitorInterval: session.MonitorInterval,
		CreatedAt:       session.CreatedAt,
		LastActivity:    session.LastActivity,
		LastOutput:      session.LastOutput,
		LastOutputHash:  session.LastOutputHash,
		Metadata:        session.Metadata,
		ClosedAt:        session.ClosedAt,
	}
	return s.db.WithContext(ctx).Create(model).Error
}

// GetTmuxSession retrieves a tmux session by ID
func (s *GORMStorage) GetTmuxSession(ctx context.Context, sessionID string) (*TmuxSession, error) {
	var model TmuxSessionModel
	result := s.db.WithContext(ctx).Where("id = ?", sessionID).First(&model)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}

	return &TmuxSession{
		ID:              model.ID,
		UserID:          model.UserID,
		SocketPath:      model.SocketPath,
		SessionName:     model.SessionName,
		Target:          model.Target,
		Command:         model.Command,
		State:           TmuxSessionState(model.State),
		CronJobID:       model.CronJobID,
		MonitorInterval: model.MonitorInterval,
		CreatedAt:       model.CreatedAt,
		LastActivity:    model.LastActivity,
		LastOutput:      model.LastOutput,
		LastOutputHash:  model.LastOutputHash,
		Metadata:        model.Metadata,
		ClosedAt:        model.ClosedAt,
	}, nil
}

// ListTmuxSessions lists tmux sessions with optional filters
func (s *GORMStorage) ListTmuxSessions(ctx context.Context, userID string, state TmuxSessionState) ([]*TmuxSession, error) {
	query := s.db.WithContext(ctx).Model(&TmuxSessionModel{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if state != "" {
		query = query.Where("state = ?", string(state))
	}

	var models []TmuxSessionModel
	if err := query.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}

	sessions := make([]*TmuxSession, len(models))
	for i, model := range models {
		sessions[i] = &TmuxSession{
			ID:              model.ID,
			UserID:          model.UserID,
			SocketPath:      model.SocketPath,
			SessionName:     model.SessionName,
			Target:          model.Target,
			Command:         model.Command,
			State:           TmuxSessionState(model.State),
			CronJobID:       model.CronJobID,
			MonitorInterval: model.MonitorInterval,
			CreatedAt:       model.CreatedAt,
			LastActivity:    model.LastActivity,
			LastOutput:      model.LastOutput,
			LastOutputHash:  model.LastOutputHash,
			Metadata:        model.Metadata,
			ClosedAt:        model.ClosedAt,
		}
	}

	return sessions, nil
}

// UpdateTmuxSession updates a tmux session
func (s *GORMStorage) UpdateTmuxSession(ctx context.Context, session *TmuxSession) error {
	return s.db.WithContext(ctx).Model(&TmuxSessionModel{}).Where("id = ?", session.ID).Updates(map[string]interface{}{
		"socket_path":       session.SocketPath,
		"session_name":      session.SessionName,
		"target":            session.Target,
		"command":           session.Command,
		"state":             string(session.State),
		"cron_job_id":       session.CronJobID,
		"monitor_interval":  session.MonitorInterval,
		"last_activity":     session.LastActivity,
		"last_output":       session.LastOutput,
		"last_output_hash":  session.LastOutputHash,
		"metadata":          session.Metadata,
		"closed_at":         session.ClosedAt,
	}).Error
}

// DeleteTmuxSession deletes a tmux session record
func (s *GORMStorage) DeleteTmuxSession(ctx context.Context, sessionID string) error {
	return s.db.WithContext(ctx).Where("id = ?", sessionID).Delete(&TmuxSessionModel{}).Error
}
