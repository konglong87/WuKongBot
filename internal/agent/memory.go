package agent

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MemoryStore manages the agent's memory system
type MemoryStore struct {
	workspace  string
	memoryDir  string
	memoryFile string
}

// NewMemoryStore creates a new memory store
func NewMemoryStore(workspace string) *MemoryStore {
	memoryDir := filepath.Join(workspace, "memory")
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")
	os.MkdirAll(memoryDir, 0755)
	return &MemoryStore{workspace: workspace, memoryDir: memoryDir, memoryFile: memoryFile}
}

// GetTodayFile returns the path to today's memory file
func (m *MemoryStore) GetTodayFile() string {
	return filepath.Join(m.memoryDir, time.Now().Format("2006-01-02")+".md")
}

// ReadToday reads today's memory notes
func (m *MemoryStore) ReadToday() string {
	content, _ := os.ReadFile(m.GetTodayFile())
	return string(content)
}

// AppendToday appends content to today's memory notes
func (m *MemoryStore) AppendToday(content string) error {
	todayFile := m.GetTodayFile()
	var finalContent string
	if _, err := os.Stat(todayFile); os.IsNotExist(err) {
		finalContent = "# " + time.Now().Format("2006-01-02") + "\n\n" + content
	} else {
		existing, _ := os.ReadFile(todayFile)
		finalContent = string(existing) + "\n" + content
	}
	return os.WriteFile(todayFile, []byte(finalContent), 0644)
}

// ReadLongTerm reads long-term memory (MEMORY.md)
func (m *MemoryStore) ReadLongTerm() string {
	content, _ := os.ReadFile(m.memoryFile)
	return string(content)
}

// WriteLongTerm writes to long-term memory (MEMORY.md)
func (m *MemoryStore) WriteLongTerm(content string) error {
	return os.WriteFile(m.memoryFile, []byte(content), 0644)
}

// GetRecentMemories gets memories from the last N days
func (m *MemoryStore) GetRecentMemories(days int) string {
	memories := []string{}
	today := time.Now()
	for i := 0; i < days; i++ {
		date := today.AddDate(0, 0, -i)
		filePath := filepath.Join(m.memoryDir, date.Format("2006-01-02")+".md")
		if content, err := os.ReadFile(filePath); err == nil {
			memories = append(memories, string(content))
		}
	}
	return strings.Join(memories, "\n\n---\n\n")
}

// ListMemoryFiles lists all memory files sorted by date
func (m *MemoryStore) ListMemoryFiles() []string {
	files, _ := os.ReadDir(m.memoryDir)
	var result []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".md") && f.Name() != "MEMORY.md" {
			result = append(result, f.Name())
		}
	}
	return result
}

// GetMemoryContext returns memory context for the agent
func (m *MemoryStore) GetMemoryContext() string {
	parts := []string{}
	longTerm := m.ReadLongTerm()
	if longTerm != "" {
		parts = append(parts, "## Long-term Memory\n"+longTerm)
	}
	today := m.ReadToday()
	if today != "" {
		parts = append(parts, "## Today's Notes\n"+today)
	}
	return strings.Join(parts, "\n\n")
}
