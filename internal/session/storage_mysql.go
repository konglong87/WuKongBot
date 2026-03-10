package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLStorage handles MySQL database operations
type MySQLStorage struct {
	db *sql.DB
}

// NewMySQLStorage creates a new MySQL storage instance
func NewMySQLStorage(dsn string) (*MySQLStorage, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	storage := &MySQLStorage{db: db}

	// Initialize schema
	if err := storage.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return storage, nil
}

// initSchema creates the database schema if it doesn't exist
func (s *MySQLStorage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS config (
		key VARCHAR(255) PRIMARY KEY,
		value TEXT
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

	CREATE TABLE IF NOT EXISTS sessions (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		session_key VARCHAR(255) UNIQUE NOT NULL,
		created_at BIGINT NOT NULL,
		updated_at BIGINT NOT NULL,
		metadata JSON,
		INDEX idx_sessions_key (session_key)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

	CREATE TABLE IF NOT EXISTS messages (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		session_id BIGINT NOT NULL,
		role VARCHAR(50) NOT NULL,
		content TEXT NOT NULL,
		timestamp BIGINT NOT NULL,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
		INDEX idx_messages_session (session_id),
		INDEX idx_messages_timestamp (timestamp)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

	CREATE TABLE IF NOT EXISTS cron_jobs (
		id VARCHAR(255) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		enabled TINYINT NOT NULL,
		schedule JSON NOT NULL,
		payload JSON NOT NULL,
		state JSON NOT NULL,
		created_at BIGINT NOT NULL,
		updated_at BIGINT NOT NULL,
		delete_after_run TINYINT NOT NULL
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection
func (s *MySQLStorage) Close() error {
	return s.db.Close()
}

// GetConfig retrieves a config value by key
func (s *MySQLStorage) GetConfig(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetConfig stores a config value
func (s *MySQLStorage) SetConfig(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO config (key, value) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE value = VALUES(value)
	`, key, value)
	return err
}

// GetOrCreateSession gets an existing session or creates a new one
func (s *MySQLStorage) GetOrCreateSession(sessionKey string) (*Session, error) {
	// Try to get existing session
	var id int64
	var metadata sql.NullString
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
		if metadata.Valid && metadata.String != "" {
			json.Unmarshal([]byte(metadata.String), &session.Metadata)
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
func (s *MySQLStorage) AddMessage(sessionID int64, role, content string) error {
	timestamp := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT INTO messages (session_id, role, content, timestamp)
		VALUES (?, ?, ?, ?)
	`, sessionID, role, content, timestamp)
	return err
}

// GetMessages retrieves messages for a session
func (s *MySQLStorage) GetMessages(sessionID int64, limit int) ([]Message, error) {
	query := "SELECT id, session_id, role, content, timestamp FROM messages WHERE session_id = ? ORDER BY timestamp DESC"
	if limit > 0 {
		query += " LIMIT ?"
	}

	var rows *sql.Rows
	var err error

	if limit > 0 {
		rows, err = s.db.Query(query, sessionID, limit)
	} else {
		rows, err = s.db.Query(query, sessionID)
	}

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
func (s *MySQLStorage) GetLastNMessages(sessionID int64, n int) ([]Message, error) {
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
func (s *MySQLStorage) DeleteSession(sessionID int64) error {
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
func (s *MySQLStorage) SaveCronJob(job *CronJob) error {
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
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			enabled = VALUES(enabled),
			schedule = VALUES(schedule),
			payload = VALUES(payload),
			state = VALUES(state),
			updated_at = VALUES(updated_at),
			delete_after_run = VALUES(delete_after_run)
	`, job.ID, job.Name, enabled, string(scheduleJSON), string(payloadJSON), string(stateJSON), now, now, deleteAfterRun)

	return err
}

// GetCronJob retrieves a cron job by ID
func (s *MySQLStorage) GetCronJob(jobID string) (*CronJob, error) {
	var job CronJob
	var scheduleJSON, payloadJSON, stateJSON sql.NullString
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

	if scheduleJSON.Valid {
		json.Unmarshal([]byte(scheduleJSON.String), &job.Schedule)
	}
	if payloadJSON.Valid {
		json.Unmarshal([]byte(payloadJSON.String), &job.Payload)
	}
	if stateJSON.Valid {
		json.Unmarshal([]byte(stateJSON.String), &job.State)
	}

	return &job, nil
}

// GetAllCronJobs retrieves all cron jobs
func (s *MySQLStorage) GetAllCronJobs() ([]*CronJob, error) {
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
		var scheduleJSON, payloadJSON, stateJSON sql.NullString
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

		if scheduleJSON.Valid {
			json.Unmarshal([]byte(scheduleJSON.String), &job.Schedule)
		}
		if payloadJSON.Valid {
			json.Unmarshal([]byte(payloadJSON.String), &job.Payload)
		}
		if stateJSON.Valid {
			json.Unmarshal([]byte(stateJSON.String), &job.State)
		}

		jobs = append(jobs, &job)
	}

	return jobs, rows.Err()
}

// DeleteCronJob deletes a cron job by ID
func (s *MySQLStorage) DeleteCronJob(jobID string) error {
	_, err := s.db.Exec("DELETE FROM cron_jobs WHERE id = ?", jobID)
	return err
}
