package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TmuxSessionState represents the state of a tmux session
type TmuxSessionState string

const (
	StateCreating TmuxSessionState = "creating" // Session is being created
	StateActive   TmuxSessionState = "active"   // Session is running and being monitored
	StateIdle     TmuxSessionState = "idle"     // Session is idle (no output change)
	StateError    TmuxSessionState = "error"    // Session encountered an error
	StateClosed   TmuxSessionState = "closed"   // Session has been closed
)

// TmuxSession represents a tmux session managed by the system
type TmuxSession struct {
	ID              string                 `json:"id"`               // Unique identifier: "userID:timestamp"
	UserID          string                 `json:"user_id"`          // User ID (Feishu open_id)
	SocketPath      string                 `json:"socket_path"`      // tmux socket path
	SessionName     string                 `json:"session_name"`     // tmux session name
	Target          string                 `json:"target"`           // Full target: "session:window.pane"
	Command         string                 `json:"command"`          // Startup command
	State           TmuxSessionState       `json:"state"`            // Current state
	CreatedAt       time.Time              `json:"created_at"`       // Creation timestamp
	LastActivity    time.Time              `json:"last_activity"`    // Last activity timestamp
	CronJobID       string                 `json:"cron_job_id"`      // Associated cron job ID for monitoring
	MonitorInterval int64                  `json:"monitor_interval"` // Monitoring interval in seconds
	LastOutput      string                 `json:"last_output"`      // Last captured output (for change detection)
	LastOutputHash  string                 `json:"last_output_hash"` // Hash of last output for quick comparison
	Metadata        map[string]interface{} `json:"metadata"`         // Additional metadata
	ClosedAt        *time.Time             `json:"closed_at"`        // Closure timestamp (if closed)
}

// Storage handles SQLite database operations
type Storage struct {
	db *sql.DB
}

// Message represents a message in the database
type Message struct {
	ID        int64     `json:"id"`
	SessionID int64     `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Session represents a chat session in the database
type Session struct {
	ID        int64                  `json:"id"`
	Key       string                 `json:"session_key"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// CronJob represents a scheduled cron job
type CronJob struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Enabled        bool                   `json:"enabled"`
	Schedule       map[string]interface{} `json:"schedule"`
	Payload        map[string]interface{} `json:"payload"`
	State          map[string]interface{} `json:"state"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	DeleteAfterRun bool                   `json:"delete_after_run"`
}

// ExecutionLog represents a cron job execution log entry
type ExecutionLog struct {
	ID         int64     `json:"id"`
	JobID      string    `json:"job_id"`
	JobName    string    `json:"job_name"`
	Status     string    `json:"status"` // "success", "error"
	Error      string    `json:"error"`  // error message if Status is "error"
	ExecutedAt time.Time `json:"executed_at"`
}

// SubagentRun represents a subagent execution record
type SubagentRun struct {
	RunID               string              `json:"run_id"`
	ChildSessionKey     string              `json:"child_session_key"`
	RequesterSessionKey string              `json:"requester_session_key"`
	Task                string              `json:"task"`
	Label               string              `json:"label"`
	Status              string              `json:"status"` // "pending", "running", "completed", "failed", "timeout"
	Outcome             *SubagentRunOutcome `json:"outcome,omitempty"`
	OriginChannel       string              `json:"origin_channel"`
	OriginChatID        string              `json:"origin_chat_id"`
	CreatedAt           time.Time           `json:"created_at"`
	StartedAt           *time.Time          `json:"started_at,omitempty"`
	EndedAt             *time.Time          `json:"ended_at,omitempty"`
	DurationSeconds     int                 `json:"duration_seconds"`
	CleanupPolicy       string              `json:"cleanup_policy"`
	ArchivedAt          *time.Time          `json:"archived_at,omitempty"`
}

// SubagentRunOutcome represents the outcome of a subagent run
type SubagentRunOutcome struct {
	Status    string     `json:"status"` // "ok", "error", "timeout"
	Result    string     `json:"result"`
	Error     string     `json:"error,omitempty"`
	ToolCalls int        `json:"tool_calls"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string) (*Storage, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &Storage{db: db}

	// Initialize schema
	if err := storage.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return storage, nil
}

// initSchema creates the database schema if it doesn't exist
func (s *Storage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT
	);

	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_key TEXT UNIQUE NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		metadata TEXT
	);

	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		FOREIGN KEY (session_id) REFERENCES sessions(id)
	);

	CREATE TABLE IF NOT EXISTS cron_jobs (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		enabled INTEGER NOT NULL,
		schedule TEXT NOT NULL,
		payload TEXT NOT NULL,
		state TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		delete_after_run INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS execution_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id TEXT NOT NULL,
		job_name TEXT NOT NULL,
		status TEXT NOT NULL,
		error TEXT,
		executed_at INTEGER NOT NULL,
		FOREIGN KEY (job_id) REFERENCES cron_jobs(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS subagent_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id TEXT UNIQUE NOT NULL,
		child_session_key TEXT NOT NULL,
		requester_session_key TEXT NOT NULL,
		task TEXT NOT NULL,
		label TEXT,
		status TEXT NOT NULL,
		outcome TEXT,
		origin_channel TEXT NOT NULL,
		origin_chat_id TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		started_at INTEGER,
		ended_at INTEGER,
		duration_seconds INTEGER DEFAULT 0,
		cleanup_policy TEXT DEFAULT 'keep',
		archived_at INTEGER
	);

	CREATE TABLE IF NOT EXISTS tmux_sessions (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		socket_path TEXT NOT NULL,
		session_name TEXT NOT NULL,
		target TEXT NOT NULL,
		command TEXT NOT NULL,
		state TEXT NOT NULL,
		cron_job_id TEXT,
		monitor_interval INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		last_activity INTEGER NOT NULL,
		last_output TEXT,
		last_output_hash TEXT,
		metadata TEXT,
		closed_at INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_key ON sessions(session_key);
	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
	CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
	CREATE INDEX IF NOT EXISTS idx_execution_logs_job ON execution_logs(job_id);
	CREATE INDEX IF NOT EXISTS idx_execution_logs_executed_at ON execution_logs(executed_at);
	CREATE INDEX IF NOT EXISTS idx_subagent_runs_created ON subagent_runs(created_at);
	CREATE INDEX IF NOT EXISTS idx_subagent_runs_archived ON subagent_runs(archived_at);
	CREATE INDEX IF NOT EXISTS idx_subagent_runs_status ON subagent_runs(status);
	CREATE INDEX IF NOT EXISTS idx_tmux_sessions_user ON tmux_sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_tmux_sessions_state ON tmux_sessions(state);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// GetConfig retrieves a config value by key
func (s *Storage) GetConfig(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetConfig stores a config value
func (s *Storage) SetConfig(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO config (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

// GetOrCreateSession gets an existing session or creates a new one
func (s *Storage) GetOrCreateSession(sessionKey string) (*Session, error) {
	// Try to get existing session
	var id int64
	var metadata string
	var createdAt, updatedAt int64
	err := s.db.QueryRow(`
		SELECT id, created_at, updated_at, metadata
		FROM sessions WHERE session_key = ?
	`, sessionKey).Scan(&id, &createdAt, &updatedAt, &metadata)

	if err == nil {
		// Session exists, update timestamp
		now := time.Now().Unix()
		_, err = s.db.Exec("UPDATE sessions SET updated_at = ? WHERE id = ?", now, id)
		if err != nil {
			return nil, err
		}

		session := &Session{
			ID:        id,
			Key:       sessionKey,
			CreatedAt: time.Unix(createdAt, 0),
			UpdatedAt: time.Unix(now, 0),
		}
		if metadata != "" {
			json.Unmarshal([]byte(metadata), &session.Metadata)
		}
		return session, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create new session
	now := time.Now().Unix()
	result, err := s.db.Exec(`
		INSERT INTO sessions (session_key, created_at, updated_at, metadata)
		VALUES (?, ?, ?, '{}')
	`, sessionKey, now, now)
	if err != nil {
		return nil, err
	}

	id, err = result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Session{
		ID:        id,
		Key:       sessionKey,
		CreatedAt: time.Unix(now, 0),
		UpdatedAt: time.Unix(now, 0),
		Metadata:  make(map[string]interface{}),
	}, nil
}

// AddMessage adds a message to a session
func (s *Storage) AddMessage(sessionID int64, role, content string) error {
	timestamp := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT INTO messages (session_id, role, content, timestamp)
		VALUES (?, ?, ?, ?)
	`, sessionID, role, content, timestamp)
	return err
}

// GetMessages retrieves messages for a session
func (s *Storage) GetMessages(sessionID int64, limit int) ([]Message, error) {
	query := "SELECT id, session_id, role, content, timestamp FROM messages WHERE session_id = ? ORDER BY timestamp DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var timestamp int64
		err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &timestamp)
		if err != nil {
			return nil, err
		}
		msg.Timestamp = time.Unix(timestamp, 0)
		messages = append(messages, msg)
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, rows.Err()
}

// GetLastNMessages retrieves the last N messages from a session
func (s *Storage) GetLastNMessages(sessionID int64, n int) ([]Message, error) {
	query := `
		SELECT id, session_id, role, content, timestamp
		FROM messages
		WHERE session_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := s.db.Query(query, sessionID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var timestamp int64
		err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &timestamp)
		if err != nil {
			return nil, err
		}
		msg.Timestamp = time.Unix(timestamp, 0)
		messages = append(messages, msg)
	}

	// Reverse to get chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, rows.Err()
}

// DeleteSession deletes a session and all its messages
func (s *Storage) DeleteSession(sessionID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Delete messages
	_, err = tx.Exec("DELETE FROM messages WHERE session_id = ?", sessionID)
	if err != nil {
		return err
	}

	// Delete session
	_, err = tx.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// SaveCronJob saves or updates a cron job
func (s *Storage) SaveCronJob(job *CronJob) error {
	scheduleJSON, err := json.Marshal(job.Schedule)
	if err != nil {
		return err
	}

	payloadJSON, err := json.Marshal(job.Payload)
	if err != nil {
		return err
	}

	stateJSON, err := json.Marshal(job.State)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	enabled := 0
	if job.Enabled {
		enabled = 1
	}
	deleteAfterRun := 0
	if job.DeleteAfterRun {
		deleteAfterRun = 1
	}

	_, err = s.db.Exec(`
		INSERT INTO cron_jobs (id, name, enabled, schedule, payload, state, created_at, updated_at, delete_after_run)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			enabled = excluded.enabled,
			schedule = excluded.schedule,
			payload = excluded.payload,
			state = excluded.state,
			updated_at = excluded.updated_at,
			delete_after_run = excluded.delete_after_run
	`, job.ID, job.Name, enabled, string(scheduleJSON), string(payloadJSON), string(stateJSON), now, now, deleteAfterRun)

	return err
}

// GetCronJob retrieves a cron job by ID
func (s *Storage) GetCronJob(jobID string) (*CronJob, error) {
	var job CronJob
	var scheduleJSON, payloadJSON, stateJSON string
	var enabled, deleteAfterRun int
	var createdAt, updatedAt int64

	err := s.db.QueryRow(`
		SELECT id, name, enabled, schedule, payload, state, created_at, updated_at, delete_after_run
		FROM cron_jobs WHERE id = ?
	`, jobID).Scan(
		&job.ID, &job.Name, &enabled, &scheduleJSON, &payloadJSON, &stateJSON,
		&createdAt, &updatedAt, &deleteAfterRun,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	job.Enabled = enabled == 1
	job.DeleteAfterRun = deleteAfterRun == 1
	job.CreatedAt = time.Unix(createdAt, 0)
	job.UpdatedAt = time.Unix(updatedAt, 0)

	json.Unmarshal([]byte(scheduleJSON), &job.Schedule)
	json.Unmarshal([]byte(payloadJSON), &job.Payload)
	json.Unmarshal([]byte(stateJSON), &job.State)

	return &job, nil
}

// GetAllCronJobs retrieves all cron jobs
func (s *Storage) GetAllCronJobs() ([]*CronJob, error) {
	rows, err := s.db.Query(`
		SELECT id, name, enabled, schedule, payload, state, created_at, updated_at, delete_after_run
		FROM cron_jobs ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*CronJob
	for rows.Next() {
		var job CronJob
		var scheduleJSON, payloadJSON, stateJSON string
		var enabled, deleteAfterRun int
		var createdAt, updatedAt int64

		err := rows.Scan(
			&job.ID, &job.Name, &enabled, &scheduleJSON, &payloadJSON, &stateJSON,
			&createdAt, &updatedAt, &deleteAfterRun,
		)
		if err != nil {
			return nil, err
		}

		job.Enabled = enabled == 1
		job.DeleteAfterRun = deleteAfterRun == 1
		job.CreatedAt = time.Unix(createdAt, 0)
		job.UpdatedAt = time.Unix(updatedAt, 0)

		json.Unmarshal([]byte(scheduleJSON), &job.Schedule)
		json.Unmarshal([]byte(payloadJSON), &job.Payload)
		json.Unmarshal([]byte(stateJSON), &job.State)

		jobs = append(jobs, &job)
	}

	return jobs, rows.Err()
}

// DeleteCronJob deletes a cron job
func (s *Storage) DeleteCronJob(jobID string) error {
	_, err := s.db.Exec("DELETE FROM cron_jobs WHERE id = ?", jobID)
	return err
}

// SaveExecutionLog saves an execution log entry
func (s *Storage) SaveExecutionLog(log *ExecutionLog) error {
	result, err := s.db.Exec(`
		INSERT INTO execution_logs (job_id, job_name, status, error, executed_at)
		VALUES (?, ?, ?, ?, ?)
	`, log.JobID, log.JobName, log.Status, log.Error, log.ExecutedAt.Unix())
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	log.ID = id
	return nil
}

// GetExecutionLogs retrieves execution logs for a specific job
func (s *Storage) GetExecutionLogs(jobID string, limit int) ([]*ExecutionLog, error) {
	query := `
		SELECT id, job_id, job_name, status, error, executed_at
		FROM execution_logs
		WHERE job_id = ?
		ORDER BY executed_at DESC
		LIMIT ?
	`

	if limit <= 0 {
		limit = 100 // default limit
	}

	rows, err := s.db.Query(query, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*ExecutionLog
	for rows.Next() {
		log := &ExecutionLog{}
		var executedAtUnix int64

		err := rows.Scan(&log.ID, &log.JobID, &log.JobName, &log.Status, &log.Error, &executedAtUnix)
		if err != nil {
			return nil, err
		}

		log.ExecutedAt = time.Unix(executedAtUnix, 0)
		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// GetAllExecutionLogs retrieves all execution logs (with optional limit)
func (s *Storage) GetAllExecutionLogs(limit int) ([]*ExecutionLog, error) {
	query := `
		SELECT id, job_id, job_name, status, error, executed_at
		FROM execution_logs
		ORDER BY executed_at DESC
		LIMIT ?
	`

	if limit <= 0 {
		limit = 100 // default limit
	}

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*ExecutionLog
	for rows.Next() {
		log := &ExecutionLog{}
		var executedAtUnix int64

		err := rows.Scan(&log.ID, &log.JobID, &log.JobName, &log.Status, &log.Error, &executedAtUnix)
		if err != nil {
			return nil, err
		}

		log.ExecutedAt = time.Unix(executedAtUnix, 0)
		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// CreateSubagentRun creates a new subagent run record
func (s *Storage) CreateSubagentRun(run *SubagentRun) error {
	outcomeJSON := ""
	if run.Outcome != nil {
		data, err := json.Marshal(run.Outcome)
		if err == nil {
			outcomeJSON = string(data)
		}
	}

	startedAt := int64(0)
	endedAt := int64(0)
	archivedAt := int64(0)
	if run.StartedAt != nil {
		startedAt = run.StartedAt.Unix()
	}
	if run.EndedAt != nil {
		endedAt = run.EndedAt.Unix()
	}
	if run.ArchivedAt != nil {
		archivedAt = run.ArchivedAt.Unix()
	}

	_, err := s.db.Exec(`
		INSERT INTO subagent_runs (
			run_id, child_session_key, requester_session_key, task, label, status,
			outcome, origin_channel, origin_chat_id, created_at, started_at, ended_at,
			duration_seconds, cleanup_policy, archived_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, run.RunID, run.ChildSessionKey, run.RequesterSessionKey, run.Task, run.Label, run.Status,
		outcomeJSON, run.OriginChannel, run.OriginChatID, run.CreatedAt.Unix(), startedAt, endedAt,
		run.DurationSeconds, run.CleanupPolicy, archivedAt)

	return err
}

// UpdateSubagentRunStatus updates the status of a subagent run
func (s *Storage) UpdateSubagentRunStatus(runID string, status string, startedAt, endedAt *time.Time) error {
	updates := []string{}
	args := []interface{}{}

	updates = append(updates, "status = ?")
	args = append(args, status)

	if startedAt != nil {
		updates = append(updates, "started_at = ?")
		args = append(args, startedAt.Unix())
	}

	if endedAt != nil {
		updates = append(updates, "ended_at = ?")
		args = append(args, endedAt.Unix())
		if startedAt != nil {
			duration := int(endedAt.Sub(*startedAt).Seconds())
			updates = append(updates, "duration_seconds = ?")
			args = append(args, duration)
		}
	}

	args = append(args, runID)

	query := "UPDATE subagent_runs SET " + strings.Join(updates, ", ") + " WHERE run_id = ?"
	_, err := s.db.Exec(query, args...)
	return err
}

// RecordSubagentOutcome records the execution outcome
func (s *Storage) RecordSubagentOutcome(runID string, outcome *SubagentRunOutcome) error {
	outcomeJSON, _ := json.Marshal(outcome)

	updates := []string{"outcome = ?"}
	args := []interface{}{string(outcomeJSON)}

	if outcome.EndedAt != nil {
		updates = append(updates, "ended_at = ?, status = ?")
		args = append(args, outcome.EndedAt.Unix(), outcome.Status)
	}

	args = append(args, runID)

	query := "UPDATE subagent_runs SET " + strings.Join(updates, ", ") + " WHERE run_id = ?"
	_, err := s.db.Exec(query, args...)
	return err
}

// GetSubagentRun retrieves a subagent run by run ID
func (s *Storage) GetSubagentRun(runID string) (*SubagentRun, error) {
	var run SubagentRun
	var outcomeSQL string
	var startedAt, endedAt, archivedAt int64
	var createdAt int64

	err := s.db.QueryRow(`
		SELECT run_id, child_session_key, requester_session_key, task, label, status,
		       outcome, origin_channel, origin_chat_id, created_at, started_at, ended_at,
		       duration_seconds, cleanup_policy, archived_at
		FROM subagent_runs WHERE run_id = ?
	`, runID).Scan(
		&run.RunID, &run.ChildSessionKey, &run.RequesterSessionKey, &run.Task, &run.Label,
		&run.Status, &outcomeSQL, &run.OriginChannel, &run.OriginChatID,
		&createdAt, &startedAt, &endedAt, &run.DurationSeconds, &run.CleanupPolicy, &archivedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	run.CreatedAt = time.Unix(createdAt, 0)

	if startedAt > 0 {
		t := time.Unix(startedAt, 0)
		run.StartedAt = &t
	}
	if endedAt > 0 {
		t := time.Unix(endedAt, 0)
		run.EndedAt = &t
	}
	if archivedAt > 0 {
		t := time.Unix(archivedAt, 0)
		run.ArchivedAt = &t
	}

	if outcomeSQL != "" {
		var outcome SubagentRunOutcome
		if err := json.Unmarshal([]byte(outcomeSQL), &outcome); err == nil {
			run.Outcome = &outcome
		}
	}

	return &run, nil
}

// GetRunningSubagentRuns retrieves all currently running subagent runs
func (s *Storage) GetRunningSubagentRuns() ([]*SubagentRun, error) {
	rows, err := s.db.Query(`
		SELECT run_id, child_session_key, requester_session_key, task, label, status,
		       outcome, origin_channel, origin_chat_id, created_at, started_at, ended_at,
		       duration_seconds, cleanup_policy, archived_at
		FROM subagent_runs WHERE status IN (?, ?)
	`, "pending", "running")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*SubagentRun
	for rows.Next() {
		var run SubagentRun
		var outcomeSQL string
		var startedAt, endedAt, archivedAt int64
		var createdAt int64

		err := rows.Scan(
			&run.RunID, &run.ChildSessionKey, &run.RequesterSessionKey, &run.Task, &run.Label,
			&run.Status, &outcomeSQL, &run.OriginChannel, &run.OriginChatID,
			&createdAt, &startedAt, &endedAt, &run.DurationSeconds, &run.CleanupPolicy, &archivedAt,
		)
		if err != nil {
			return nil, err
		}

		run.CreatedAt = time.Unix(createdAt, 0)

		if startedAt > 0 {
			t := time.Unix(startedAt, 0)
			run.StartedAt = &t
		}
		if endedAt > 0 {
			t := time.Unix(endedAt, 0)
			run.EndedAt = &t
		}
		if archivedAt > 0 {
			t := time.Unix(archivedAt, 0)
			run.ArchivedAt = &t
		}

		if outcomeSQL != "" {
			var outcome SubagentRunOutcome
			if err := json.Unmarshal([]byte(outcomeSQL), &outcome); err == nil {
				run.Outcome = &outcome
			}
		}

		runs = append(runs, &run)
	}

	return runs, rows.Err()
}

// GetSubagentRunHistory retrieves historical subagent runs with pagination
func (s *Storage) GetSubagentRunHistory(page, limit int) ([]*SubagentRun, error) {
	offset := (page - 1) * limit
	rows, err := s.db.Query(`
		SELECT run_id, child_session_key, requester_session_key, task, label, status,
		       outcome, origin_channel, origin_chat_id, created_at, started_at, ended_at,
		       duration_seconds, cleanup_policy, archived_at
		FROM subagent_runs
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*SubagentRun
	for rows.Next() {
		var run SubagentRun
		var outcomeSQL string
		var startedAt, endedAt, archivedAt int64
		var createdAt int64

		err := rows.Scan(
			&run.RunID, &run.ChildSessionKey, &run.RequesterSessionKey, &run.Task, &run.Label,
			&run.Status, &outcomeSQL, &run.OriginChannel, &run.OriginChatID,
			&createdAt, &startedAt, &endedAt, &run.DurationSeconds, &run.CleanupPolicy, &archivedAt,
		)
		if err != nil {
			return nil, err
		}

		run.CreatedAt = time.Unix(createdAt, 0)

		if startedAt > 0 {
			t := time.Unix(startedAt, 0)
			run.StartedAt = &t
		}
		if endedAt > 0 {
			t := time.Unix(endedAt, 0)
			run.EndedAt = &t
		}
		if archivedAt > 0 {
			t := time.Unix(archivedAt, 0)
			run.ArchivedAt = &t
		}

		if outcomeSQL != "" {
			var outcome SubagentRunOutcome
			if err := json.Unmarshal([]byte(outcomeSQL), &outcome); err == nil {
				run.Outcome = &outcome
			}
		}

		runs = append(runs, &run)
	}

	return runs, rows.Err()
}

// CleanupExpiredSubagentRuns removes expired subagent runs
func (s *Storage) CleanupExpiredSubagentRuns(olderThan time.Time) (int, error) {
	result, err := s.db.Exec(`
		DELETE FROM subagent_runs WHERE archived_at IS NOT NULL AND archived_at < ?
	`, olderThan.Unix())
	if err != nil {
		return 0, err
	}

	count, err := result.RowsAffected()
	return int(count), err
}

// ExecuteQuery executes a custom query for advanced operations
func (s *Storage) ExecuteQuery(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

// ExecuteExec executes a custom exec statement
func (s *Storage) ExecuteExec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

// CreateTmuxSession creates a new tmux session record
func (s *Storage) CreateTmuxSession(ctx context.Context, session *TmuxSession) error {
	metadataJSON, _ := json.Marshal(session.Metadata)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tmux_sessions (
			id, user_id, socket_path, session_name, target, command, state,
			cron_job_id, monitor_interval, created_at, last_activity,
			last_output, last_output_hash, metadata, closed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.SocketPath, session.SessionName, session.Target,
		session.Command, string(session.State), session.CronJobID, session.MonitorInterval,
		session.CreatedAt.Unix(), session.LastActivity.Unix(), session.LastOutput, session.LastOutputHash,
		string(metadataJSON), nil)

	return err
}

// GetTmuxSession retrieves a tmux session by ID
func (s *Storage) GetTmuxSession(ctx context.Context, sessionID string) (*TmuxSession, error) {
	var session TmuxSession
	var state, metadataJSON string
	var createdAt, lastActivity int64
	var closedAt *int64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, socket_path, session_name, target, command, state,
			   cron_job_id, monitor_interval, created_at, last_activity,
			   last_output, last_output_hash, metadata, closed_at
		FROM tmux_sessions WHERE id = ?
	`, sessionID).Scan(
		&session.ID, &session.UserID, &session.SocketPath, &session.SessionName, &session.Target,
		&session.Command, &state, &session.CronJobID, &session.MonitorInterval,
		&createdAt, &lastActivity, &session.LastOutput, &session.LastOutputHash,
		&metadataJSON, &closedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	session.State = TmuxSessionState(state)
	session.CreatedAt = time.Unix(createdAt, 0)
	session.LastActivity = time.Unix(lastActivity, 0)

	if metadataJSON != "" {
		json.Unmarshal([]byte(metadataJSON), &session.Metadata)
	}

	if closedAt != nil {
		t := time.Unix(*closedAt, 0)
		session.ClosedAt = &t
	}

	return &session, nil
}

// ListTmuxSessions lists tmux sessions with optional filters
func (s *Storage) ListTmuxSessions(ctx context.Context, userID string, state TmuxSessionState) ([]*TmuxSession, error) {
	query := `
		SELECT id, user_id, socket_path, session_name, target, command, state,
			   cron_job_id, monitor_interval, created_at, last_activity,
			   last_output, last_output_hash, metadata, closed_at
		FROM tmux_sessions WHERE 1=1
	`
	args := []interface{}{}

	if userID != "" {
		query += " AND user_id = ?"
		args = append(args, userID)
	}

	if state != "" {
		query += " AND state = ?"
		args = append(args, string(state))
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*TmuxSession
	for rows.Next() {
		var session TmuxSession
		var stateStr, metadataJSON string
		var createdAt, lastActivity int64
		var closedAt *int64

		err := rows.Scan(
			&session.ID, &session.UserID, &session.SocketPath, &session.SessionName, &session.Target,
			&session.Command, &stateStr, &session.CronJobID, &session.MonitorInterval,
			&createdAt, &lastActivity, &session.LastOutput, &session.LastOutputHash,
			&metadataJSON, &closedAt,
		)
		if err != nil {
			return nil, err
		}

		session.State = TmuxSessionState(stateStr)
		session.CreatedAt = time.Unix(createdAt, 0)
		session.LastActivity = time.Unix(lastActivity, 0)

		if metadataJSON != "" {
			json.Unmarshal([]byte(metadataJSON), &session.Metadata)
		}

		if closedAt != nil {
			t := time.Unix(*closedAt, 0)
			session.ClosedAt = &t
		}

		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

// UpdateTmuxSession updates a tmux session
func (s *Storage) UpdateTmuxSession(ctx context.Context, session *TmuxSession) error {
	metadataJSON, _ := json.Marshal(session.Metadata)

	var closedAt interface{}
	if session.ClosedAt != nil {
		closedAt = session.ClosedAt.Unix()
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE tmux_sessions SET
			socket_path = ?, session_name = ?, target = ?, command = ?, state = ?,
			cron_job_id = ?, monitor_interval = ?, last_activity = ?,
			last_output = ?, last_output_hash = ?, metadata = ?, closed_at = ?
		WHERE id = ?
	`, session.SocketPath, session.SessionName, session.Target, session.Command, string(session.State),
		session.CronJobID, session.MonitorInterval, session.LastActivity.Unix(),
		session.LastOutput, session.LastOutputHash, string(metadataJSON), closedAt,
		session.ID)

	return err
}

// DeleteTmuxSession deletes a tmux session record
func (s *Storage) DeleteTmuxSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM tmux_sessions WHERE id = ?", sessionID)
	return err
}
