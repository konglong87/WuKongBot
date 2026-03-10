package agentteam

import (
	"time"
)

// TaskType defines types of tasks
type TaskType string

const (
	TaskTypeGeneral     TaskType = "general"
	TaskTypeDevelopment TaskType = "development"
	TaskTypeCode        TaskType = "code"
	TaskTypeTesting     TaskType = "testing"
	TaskTypeDeployment  TaskType = "deployment"
)

// TaskState represents the state of a task in execution
type TaskState string

const (
	StatePending    TaskState = "pending"
	StateInProgress TaskState = "in_progress"
	StateBlocked    TaskState = "blocked" // waiting for dependencies
	StateCompleted  TaskState = "completed"
	StateFailed     TaskState = "failed"
)

// Task represents a main task or subtask
type Task struct {
	ID           string       `json:"id"`
	ParentID     string       `json:"parent_id,omitempty"`
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	Type         TaskType     `json:"type"`
	State        TaskState    `json:"state"`
	Agents       []Agent      `json:"agents,omitempty"`       // Assigned agents
	Dependencies []string     `json:"dependencies,omitempty"` // Task IDs this task depends on
	CreatedAt    time.Time    `json:"created_at"`
	StartedAt    *time.Time   `json:"started_at,omitempty"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	Metadata     TaskMetadata `json:"metadata"`

	// Execution results
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// TaskMetadata contains additional task information
type TaskMetadata struct {
	Language      string `json:"language,omitempty"`       // Programming language
	LanguageTags  string `json:"language_tags,omitempty"`  // Language tags for matching
	Domain        string `json:"domain,omitempty"`         // Domain (database, frontend, testing, etc.)
	ToolHints     string `json:"tool_hints,omitempty"`     // Hints about required tools
	EstimatedTime string `json:"estimated_time,omitempty"` // Time estimate (e.g., "30s", "1m5s")
	Priority      int    `json:"priority,omitempty"`       // Priority for execution ordering (higher = more urgent)
}

// Agent represents an AI agent with capabilities
type Agent struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Capabilities Capabilities `json:"capabilities"`
	Model        string       `json:"model"`  // LLM model to use for this agent
	Active       bool         `json:"active"` // Whether agent is available
	Load         int          `json:"load"`   // Current workload (number of assigned tasks)
}

// Capabilities defines what an agent can do
type Capabilities struct {
	Languages     []string `json:"languages"`      // Coding languages
	Domains       []string `json:"domains"`        // Expertise areas
	Tools         []string `json:"tools"`          // Tools (opencode, cursor, testing frameworks)
	Skills        []string `json:"skills"`         // Specific skills
	MaxConcurrent int      `json:"max_concurrent"` // Max concurrent tasks
}

// TaskRequest represents a request to create and execute an agent team task
type TaskRequest struct {
	ParentTaskID string          // For nested agent teams (optional)
	Title        string          // Task title
	Description  string          // What needs to be done
	Type         TaskType        // Task type
	Agents       string          // Preferred agents (optional, comma-separated)
	Preferences  TaskPreferences `json:"preferences"`
}

// TaskPreferences defines execution preferences
type TaskPreferences struct {
	MaxConcurrentTasks int  `json:"max_concurrent_tasks"` // Max parallel tasks (default: 3)
	Timeout            int  `json:"timeout_seconds"`      // Overall timeout (default: 600)
	AutoRetry          bool `json:"auto_retry"`           // Retry failed subtasks (default: false)
}

// TaskResult represents the final result from an agent team execution
type TaskResult struct {
	TaskID   string          `json:"task_id"`
	Title    string          `json:"title"`
	State    TaskState       `json:"state"`
	Success  bool            `json:"success"`
	Subtasks []SubtaskResult `json:"subtasks"`
	Summary  string          `json:"summary"`
	Error    string          `json:"error,omitempty"`
	Duration int             `json:"duration_seconds"`
}

// SubtaskResult represents the result of a single subtask
type SubtaskResult struct {
	SubtaskID  string    `json:"subtask_id"`
	Title      string    `json:"title"`
	State      TaskState `json:"state"`
	AssignedTo string    `json:"assigned_to"` // Agent name
	Result     string    `json:"result,omitempty"`
	Error      string    `json:"error,omitempty"`
	Duration   int       `json:"duration_seconds"`
}

// AgentCapabilities represents agent selection criteria
type AgentCapabilities struct {
	Language      string   `json:"language,omitempty"`       // Target language (e.g., "Python", "JavaScript")
	RequiredTools []string `json:"required_tools,omitempty"` // Specific tools needed
	Domain        string   `json:"domain,omitempty"`         // Domain expertise
}
