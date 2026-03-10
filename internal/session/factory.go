package session

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/konglong87/wukongbot/internal/config"
)

// StorageInterface defines the interface for storage implementations
type StorageInterface interface {
	Close() error
	GetConfig(key string) (string, error)
	SetConfig(key, value string) error
	GetOrCreateSession(sessionKey string) (*Session, error)
	AddMessage(sessionID int64, role, content string) error
	GetMessages(sessionID int64, limit int) ([]Message, error)
	GetLastNMessages(sessionID int64, n int) ([]Message, error)
	DeleteSession(sessionID int64) error
	SaveCronJob(job *CronJob) error
	GetCronJob(jobID string) (*CronJob, error)
	GetAllCronJobs() ([]*CronJob, error)
	DeleteCronJob(jobID string) error
	SaveExecutionLog(log *ExecutionLog) error
	GetExecutionLogs(jobID string, limit int) ([]*ExecutionLog, error)

	// SubagentRun methods
	CreateSubagentRun(run *SubagentRun) error
	UpdateSubagentRunStatus(runID string, status string, startedAt, endedAt *time.Time) error
	RecordSubagentOutcome(runID string, outcome *SubagentRunOutcome) error
	GetSubagentRun(runID string) (*SubagentRun, error)
	GetRunningSubagentRuns() ([]*SubagentRun, error)
	GetSubagentRunHistory(page, limit int) ([]*SubagentRun, error)
	CleanupExpiredSubagentRuns(olderThan time.Time) (int, error)

	// TmuxSession methods
	CreateTmuxSession(ctx context.Context, session *TmuxSession) error
	GetTmuxSession(ctx context.Context, sessionID string) (*TmuxSession, error)
	ListTmuxSessions(ctx context.Context, userID string, state TmuxSessionState) ([]*TmuxSession, error)
	UpdateTmuxSession(ctx context.Context, session *TmuxSession) error
	DeleteTmuxSession(ctx context.Context, sessionID string) error
}

// NewStorage creates a new storage instance based on the configuration
func NewStorage(cfg *config.Config) (StorageInterface, error) {
	// Set default database type to sqlite if not specified
	dbType := cfg.Database.Type
	if dbType == "" {
		dbType = "sqlite"
	}

	switch dbType {
	case "sqlite":
		// Use SQLite storage
		dbPath := cfg.Database.Path
		if dbPath == "" {
			// Default SQLite database path
			workspace := cfg.WorkspacePath()
			dbPath = filepath.Join(workspace, "wukongbot.db")
		}
		return NewSQLiteStorage(dbPath)
	case "mysql":
		// Use MySQL storage with GORM
		dsn := buildMySQLDSN(cfg)
		return NewGORMStorage(dsn)
	default:
		return nil, fmt.Errorf("unsupported database type: %s (supported types: sqlite, mysql)", dbType)
	}
}

// buildMySQLDSN builds a MySQL DSN from the configuration
func buildMySQLDSN(cfg *config.Config) string {
	host := cfg.Database.Host
	if host == "" {
		host = "localhost"
	}

	port := cfg.Database.Port
	if port == 0 {
		port = 3306
	}

	database := cfg.Database.Database
	if database == "" {
		database = "wukongbot"
	}

	username := cfg.Database.Username
	if username == "" {
		username = "root"
	}

	// Build DSN: username:password@tcp(host:port)/database?charset=utf8mb4&parseTime=True&loc=Local
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		username,
		cfg.Database.Password,
		host,
		port,
		database,
	)

	return dsn
}
