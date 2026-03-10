package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileTool reads file contents
type ReadFileTool struct{}

// NewReadFileTool creates a new read file tool
func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{}
}

// Name returns the tool name
func (t *ReadFileTool) Name() string {
	return "read_file"
}

// Description returns the tool description
func (t *ReadFileTool) Description() string {
	return "Read the contents of a file at the given path."
}

// Parameters returns the JSON schema for parameters
func (t *ReadFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "The file path to read"
			}
		},
		"required": ["path"]
	}`)
}

// Execute reads the file
func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "Error: path is required", nil
	}

	filePath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "Error: invalid path", nil
	}

	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return "Error: file not found: " + path, nil
	}
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	if info.IsDir() {
		return "Error: not a file: " + path, nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsPermission(err) {
			return "Error: permission denied: " + path, nil
		}
		return "Error reading file: " + err.Error(), nil
	}

	return string(content), nil
}

// ConcurrentSafe returns false - file operations should be sequential
func (t *ReadFileTool) ConcurrentSafe() bool {
	return false
}

// WriteFileTool writes content to a file
type WriteFileTool struct {
	workspace string
}

// NewWriteFileTool creates a new write file tool
func NewWriteFileTool(workspace string) *WriteFileTool {
	return &WriteFileTool{workspace: workspace}
}

// Name returns the tool name
func (t *WriteFileTool) Name() string {
	return "write_file"
}

// Description returns the tool description
func (t *WriteFileTool) Description() string {
	return "Write content to a file at the given path. Creates parent directories if needed."
}

// Parameters returns the JSON schema for parameters
func (t *WriteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "The file path to write to"
			},
			"content": {
				"type": "string",
				"description": "The content to write"
			}
		},
		"required": ["path", "content"]
	}`)
}

// Execute writes the file
func (t *WriteFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "Error: path is required", nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return "Error: content is required", nil
	}

	filePath := expandPath(path)

	// Create parent directories
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "Error: failed to create directory: " + err.Error(), nil
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		if os.IsPermission(err) {
			return "Error: permission denied: " + path, nil
		}
		return "Error writing file: " + err.Error(), nil
	}

	return "Successfully wrote " + formatBytes(len(content)) + " to " + path, nil
}

// ConcurrentSafe returns false - file write operations should be sequential
func (t *WriteFileTool) ConcurrentSafe() bool {
	return false
}

// EditFileTool edits a file by replacing text
type EditFileTool struct{}

// NewEditFileTool creates a new edit file tool
func NewEditFileTool() *EditFileTool {
	return &EditFileTool{}
}

// Name returns the tool name
func (t *EditFileTool) Name() string {
	return "edit_file"
}

// Description returns the tool description
func (t *EditFileTool) Description() string {
	return "Edit a file by replacing old_text with new_text. The old_text must exist exactly in the file."
}

// Parameters returns the JSON schema for parameters
func (t *EditFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "The file path to edit"
			},
			"old_text": {
				"type": "string",
				"description": "The exact text to find and replace"
			},
			"new_text": {
				"type": "string",
				"description": "The text to replace with"
			}
		},
		"required": ["path", "old_text", "new_text"]
	}`)
}

// Execute edits the file
func (t *EditFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "Error: path is required", nil
	}

	oldText, ok := args["old_text"].(string)
	if !ok {
		return "Error: old_text is required", nil
	}

	newText, ok := args["new_text"].(string)
	if !ok {
		return "Error: new_text is required", nil
	}

	filePath := expandPath(path)

	content, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return "Error: file not found: " + path, nil
	}
	if err != nil {
		return "Error: " + err.Error(), nil
	}

	oldContent := string(content)
	if !strings.Contains(oldContent, oldText) {
		return "Error: old_text not found in file. Make sure it matches exactly.", nil
	}

	// Count occurrences
	count := strings.Count(oldContent, oldText)
	if count > 1 {
		return "Warning: old_text appears " + formatCount(count) + " times. Please provide more context to make it unique.", nil
	}

	newContent := strings.Replace(oldContent, oldText, newText, 1)

	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return "Error editing file: " + err.Error(), nil
	}

	return "Successfully edited " + path, nil
}

// ConcurrentSafe returns false - file edit operations should be sequential
func (t *EditFileTool) ConcurrentSafe() bool {
	return false
}

// ListDirTool lists directory contents
type ListDirTool struct{}

// NewListDirTool creates a new list directory tool
func NewListDirTool() *ListDirTool {
	return &ListDirTool{}
}

// Name returns the tool name
func (t *ListDirTool) Name() string {
	return "list_dir"
}

// Description returns the tool description
func (t *ListDirTool) Description() string {
	return "List the contents of a directory."
}

// Parameters returns the JSON schema for parameters
func (t *ListDirTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "The directory path to list"
			}
		},
		"required": ["path"]
	}`)
}

// Execute lists the directory
func (t *ListDirTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "Error: path is required", nil
	}

	dirPath := expandPath(path)

	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return "Error: directory not found: " + path, nil
	}
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	if !info.IsDir() {
		return "Error: not a directory: " + path, nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsPermission(err) {
			return "Error: permission denied: " + path, nil
		}
		return "Error listing directory: " + err.Error(), nil
	}

	var items []string
	for _, entry := range entries {
		if entry.IsDir() {
			items = append(items, "📁 "+entry.Name())
		} else {
			items = append(items, "📄 "+entry.Name())
		}
	}

	if len(items) == 0 {
		return "Directory " + path + " is empty", nil
	}

	return strings.Join(items, "\n"), nil
}

// ConcurrentSafe returns true - directory listing is read-only and safe to run concurrently
func (t *ListDirTool) ConcurrentSafe() bool {
	return true
}

// Helper functions
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

func formatBytes(n int) string {
	return fmt.Sprintf("%d bytes", n)
}

func formatCount(n int) string {
	return fmt.Sprintf("%d", n)
}
