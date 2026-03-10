package agentteam

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// Decomposer handles task decomposition using LLM
type Decomposer struct {
	provider   LLMProvider
	classifier *TaskClassifier
	complexity *ComplexityEstimator
}

// LLMProvider interface for LLM calls (minimal interface)
type LLMProvider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// ChatRequest represents an LLM chat request
type ChatRequest struct {
	Messages    []Message `json:"messages"`
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

// ChatResponse represents an LLM chat response
type ChatResponse struct {
	Content      string `json:"content"`
	FinishReason string `json:"finish_reason"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}

// DecompositionResult represents the result of task decomposition
type DecompositionResult struct {
	Tasks        []Task     `json:"tasks"`
	Dependencies [][]string `json:"dependencies"` // Dependencies as [from_task_id][to_task_ids]
	Summary      string     `json:"summary"`
}

// DecomposeTask decomposes a complex task into subtasks using LLM
func (d *Decomposer) DecomposeTask(ctx context.Context, task *Task) (*DecompositionResult, error) {
	log.Debug("DecomposeTask", "task_id", task.ID, "task_type", task.Type)

	// Fallback to simple decomposition if no provider
	if d.provider == nil {
		return d.simpleDecomposition(task)
	}

	// Build LLM decomposition prompt
	prompt := d.buildDecompositionPrompt(task)

	// Call LLM
	req := ChatRequest{
		Messages: []Message{
			{
				Role: "system",
				Content: `You are a task decomposition expert. Your job is to break down complex tasks into smaller, manageable subtasks.

For each subtask, you must:
1. Give it a descriptive title
2. Describe what needs to be done
3. Identify dependencies (which subtasks must complete first)
4. Estimate time to complete
5. Identify required skills (language, domain)

Format your output as JSON:
{
  "subtasks": [
    {
      "id": "unique_id",
      "title": "Subtask title",
      "description": "What needs to be done",
      "dependencies": ["id1", "id2"],  // IDs of tasks this depends on (empty if none)
      "language": "Python",  // Programming language
      "domain": "backend",     // Domain specialty
      "estimated_time": "30s", // Time estimate
      "priority": 1
    }
  ],
  "summary": "Brief summary of the decomposition"
}

Ensure all "id" fields are unique in 4-8 character alphanumeric format (e.g., "task001").`,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Model:       d.getModelName(),
		MaxTokens:   4000,
		Temperature: 0.3,
	}

	resp, err := d.provider.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLM decomposition failed: %w", err)
	}

	log.Info("DecomposeTask", "LLM response received", "content_length", len(resp.Content), "finish_reason", resp.FinishReason)

	return d.parseDecomposition(resp.Content, task)
}

// simpleDecomposition provides a basic decomposition without LLM
func (d *Decomposer) simpleDecomposition(task *Task) (*DecompositionResult, error) {
	// Create a simple single-task decomposition by default
	subtask := Task{
		ID:          generateTaskID(1),
		ParentID:    task.ID,
		Title:       fmt.Sprintf("Execute: %s", task.Title),
		Description: task.Description,
		Type:        TaskTypeDevelopment,
		State:       StatePending,
		CreatedAt:   time.Now(),
		Metadata: TaskMetadata{
			Language:      task.Metadata.Language,
			LanguageTags:  task.Metadata.LanguageTags,
			Domain:        task.Metadata.Domain,
			EstimatedTime: task.Metadata.EstimatedTime,
			Priority:      task.Metadata.Priority,
		},
	}

	return &DecompositionResult{
		Tasks:        []Task{subtask},
		Dependencies: [][]string{{subtask.ID}},
		Summary:      fmt.Sprintf("Task %s will be executed as a single unit", task.ID),
	}, nil
}

// GetModelName returns the LLM model for task decomposition
func (d *Decomposer) getModelName() string {
	return ""
}

// buildDecompositionPrompt creates the prompt for LLM decomposition
func (d *Decomposer) buildDecompositionPrompt(task *Task) string {
	return fmt.Sprintf(`Break down the following task into 3-5 subtasks:

**Task: %s**

**Description:** %s

**Type:** %s

Requirements:
1. Decompose into logical subtasks
2. Identify dependencies between subtasks
3. For code development, consider: database, API, frontend, testing phases
4. Make subtasks roughly equal in complexity when possible
5. Each subtask must be independently completable by a single agent

Return your decomposition in JSON format with the structure specified in the system prompt.`,
		task.Title,
		task.Description,
		task.Type)
}

// parseDecomposition parses the LLM response into structured data
func (d *Decomposer) parseDecomposition(llmOutput string, parentTask *Task) (*DecompositionResult, error) {
	log.Debug("parseDecomposition", "output_length", len(llmOutput))

	// Try to extract JSON from markdown code blocks
	jsonStr := extractJSON(llmOutput)
	if jsonStr == "" {
		jsonStr = llmOutput
	}

	log.Debug("parseDecomposition", "extracted_json_length", len(jsonStr))

	var result struct {
		Subtasks []struct {
			ID            string   `json:"id"`
			Title         string   `json:"title"`
			Description   string   `json:"description"`
			Dependencies  []string `json:"dependencies"`
			Language      string   `json:"language"`
			Domain        string   `json:"domain"`
			EstimatedTime string   `json:"estimated_time"`
			Priority      int      `json:"priority"`
		} `json:"subtasks"`
		Summary string `json:"summary"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		preview := jsonStr
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("failed to parse LLM decomposition output: %w (extracted: %s)", err, preview)
	}

	// Check if subtasks exist
	if len(result.Subtasks) == 0 {
		return nil, fmt.Errorf("no subtasks found in decomposition output (parsed %d bytes)", len(jsonStr))
	}

	// Check if subtasks contain valid data
	for _, st := range result.Subtasks {
		if st.Title == "" {
			return nil, fmt.Errorf("subtask missing required field 'title'")
		}
	}

	// Convert to Task format
	tasks := make([]Task, 0, len(result.Subtasks))
	dependencies := make([][]string, 0, len(result.Subtasks))

	for i, st := range result.Subtasks {
		if st.ID == "" {
			st.ID = generateTaskID(i + 1)
		}

		task := Task{
			ID:          st.ID,
			ParentID:    parentTask.ID,
			Title:       st.Title,
			Description: st.Description,
			Type:        TaskTypeDevelopment,
			State:       StatePending,
			CreatedAt:   time.Now(),
			Metadata: TaskMetadata{
				Language:      st.Language,
				LanguageTags:  st.Language,
				Domain:        st.Domain,
				EstimatedTime: st.EstimatedTime,
				Priority:      st.Priority,
			},
		}
		tasks = append(tasks, task)
		dependencies = append(dependencies, st.Dependencies)
	}

	log.Debug("parseDecomposition", "task_decomposed", "task_id", parentTask.ID, "subtasks", len(tasks))

	return &DecompositionResult{
		Tasks:        tasks,
		Dependencies: dependencies,
		Summary:      result.Summary,
	}, nil
}

// generateTaskID generates a unique task ID
func generateTaskID(index int) string {
	return fmt.Sprintf("tsk%03d", index)
}

// extractJSON extracts JSON from markdown code blocks or plain text
func extractJSON(text string) string {
	// Try to find JSON in markdown code blocks
	text = strings.TrimSpace(text)

	// Try ````json ... ````
	if idx := strings.Index(text, "```json"); idx >= 0 {
		start := idx + 7
		end := strings.Index(text[start:], "```")
		if end >= 0 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Try ``` ... ```
	if idx := strings.Index(text, "```"); idx >= 0 {
		start := idx + 3
		end := strings.Index(text[start:], "```")
		if end >= 0 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Try to find JSON object boundaries
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "{") && strings.HasSuffix(text, "}") {
		// Count braces to find the complete JSON object
		braceCount := 0
		for i, ch := range text {
			if ch == '{' {
				braceCount++
			} else if ch == '}' {
				braceCount--
				if braceCount == 0 {
					return text[:i+1]
				}
			}
		}
		// Return entire text if we can't parse it
		return text
	}

	return text
}

// TaskClassifier analyzes and classifies tasks
type TaskClassifier struct {
	provider LLMProvider
}

// Classify analyzes a task and returns its classification
func (tc *TaskClassifier) Classify(description string) (TaskType, TaskMetadata) {
	if tc.provider == nil {
		return TaskTypeGeneral, TaskMetadata{
			EstimatedTime: "1m",
			Priority:      1,
		}
	}

	// Build classification prompt
	prompt := fmt.Sprintf(`Analyze the following task and return your analysis as JSON:

Task Description: %s

Return JSON format:
{
  "type": "development|testing|deployment|general|code",
  "language": "Primary programming language (Python, Go, JavaScript, etc.)",
  "domain": "Primary domain (backend, frontend, database, api, testing, etc.)",
  "tool_hints": "Comma-separated list of tools likely needed",
  "complexity": 1-5 (1 = simple, 5 = very complex),
  "estimated_time": "Estimated time (e.g., 30s, 2m, 5m)"
}

Rules:
- If task involves writing/modifying code, type is "code" or "development"
- If task involves testing, type is "testing"
- If task involves deployment, type is "deployment"
- Otherwise type is "general"
- Time estimates should be realistic for LLM execution
`, description)

	req := ChatRequest{
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a task classification expert. Analyze tasks and return structured JSON.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Model:       "",
		MaxTokens:   500,
		Temperature: 0.1,
	}

	resp, err := tc.provider.Chat(context.Background(), req)
	if err != nil {
		log.Error("Task classification failed", "error", err)
		return TaskTypeGeneral, TaskMetadata{
			EstimatedTime: "1m",
			Priority:      1,
		}
	}

	return tc.parseClassification(resp.Content)
}

// parseClassification parses the LLM classification response
func (tc *TaskClassifier) parseClassification(content string) (TaskType, TaskMetadata) {
	jsonStr := extractJSON(content)

	var result struct {
		Type          string `json:"type"`
		Language      string `json:"language"`
		Domain        string `json:"domain"`
		ToolHints     string `json:"tool_hints"`
		Complexity    int    `json:"complexity"`
		EstimatedTime string `json:"estimated_time"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Error("Failed to parse classification", "error", err)
		return TaskTypeGeneral, TaskMetadata{
			EstimatedTime: "1m",
			Priority:      1,
		}
	}

	taskType := TaskTypeGeneral
	switch result.Type {
	case "development", "code":
		taskType = TaskTypeCode
	case "testing":
		taskType = TaskTypeTesting
	case "deployment":
		taskType = TaskTypeDeployment
	}

	metadata := TaskMetadata{
		Language:      result.Language,
		LanguageTags:  result.Language,
		Domain:        result.Domain,
		ToolHints:     result.ToolHints,
		EstimatedTime: result.EstimatedTime,
		Priority:      result.Complexity,
	}

	if metadata.EstimatedTime == "" {
		metadata.EstimatedTime = "1m"
	}

	return taskType, metadata
}

// ComplexityEstimator estimates task complexity
type ComplexityEstimator struct {
	provider LLMProvider
}

// Estimate returns complexity and estimated duration
func (ce *ComplexityEstimator) Estimate(task *Task) (int, time.Duration) {
	if ce.provider == nil {
		return 2, 60 * time.Second
	}

	prompt := fmt.Sprintf(`Estimate the complexity and time for this task:

Task: %s
Description: %s

Return JSON:
{
  "complexity": 1-5 (1 = simple, 5 = very complex),
  "estimated_seconds": Estimated time in seconds for agent execution
}

Consider:
- Number of steps required
- Potential for issues or retries
- Amount of analysis needed
- Tool invocation count
`, task.Title, task.Description)

	req := ChatRequest{
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a task estimation expert. Provide realistic time estimates.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Model:       "",
		MaxTokens:   300,
		Temperature: 0.1,
	}

	resp, err := ce.provider.Chat(context.Background(), req)
	if err != nil {
		return 2, 60 * time.Second
	}

	return ce.parseEstimation(resp.Content)
}

// parseEstimation parses the estimation response
func (ce *ComplexityEstimator) parseEstimation(content string) (int, time.Duration) {
	jsonStr := extractJSON(content)

	var result struct {
		Complexity       int `json:"complexity"`
		EstimatedSeconds int `json:"estimated_seconds"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return 2, 60 * time.Second
	}

	if result.Complexity < 1 {
		result.Complexity = 1
	}
	if result.Complexity > 5 {
		result.Complexity = 5
	}

	duration := time.Duration(result.EstimatedSeconds) * time.Second
	if duration < 30*time.Second {
		duration = 30 * time.Second
	}
	if duration > 10*time.Minute {
		duration = 10 * time.Minute
	}

	return result.Complexity, duration
}

// NewDecomposer creates a new decomposer with classifiers
func NewDecomposer(provider LLMProvider) *Decomposer {
	return &Decomposer{
		provider:   provider,
		classifier: &TaskClassifier{provider: provider},
		complexity: &ComplexityEstimator{provider: provider},
	}
}

// ClassifyTask classifies a task using the internal classifier
func (d *Decomposer) ClassifyTask(description string) (TaskType, TaskMetadata) {
	return d.classifier.Classify(description)
}

// EstimateTaskComplexity estimates task complexity
func (d *Decomposer) EstimateTaskComplexity(task *Task) (int, time.Duration) {
	return d.complexity.Estimate(task)
}

// ShouldDecompose decides if a task should be decomposed
func (d *Decomposer) ShouldDecompose(task *Task) bool {
	complexity, _ := d.complexity.Estimate(task)
	if complexity <= 2 {
		return false
	}

	switch task.Type {
	case TaskTypeGeneral:
		return len(task.Description) > 200
	case TaskTypeTesting:
		return false
	case TaskTypeDeployment:
		return false
	default:
		return true
	}
}
