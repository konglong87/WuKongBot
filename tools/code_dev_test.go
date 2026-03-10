package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeDevTool_Description(t *testing.T) {
	tool := &CodeDevTool{}
	desc := tool.Description()

	// 验证描述包含关键触发词
	assert.Contains(t, desc, "opencode", "Description should contain opencode")
	assert.Contains(t, desc, "cursor", "Description should contain cursor")
	assert.Contains(t, desc, "claude", "Description should contain claude")
	assert.Contains(t, desc, "用opencode", "Description should trigger on 用opencode")
	assert.Contains(t, desc, "用claude", "Description should trigger on 用claude")
	assert.Contains(t, desc, "用xx编程", "Description should trigger on 用xx编程")
}

func TestCodeDevTool_Parameters(t *testing.T) {
	tool := &CodeDevTool{}
	params := tool.Parameters()

	// 验证参数包含 claude 选项
	assert.Contains(t, string(params), "claude", "Parameters should include claude option")
	assert.Contains(t, string(params), "opencode", "Parameters should include opencode option")
	assert.Contains(t, string(params), "cursor", "Parameters should include cursor option")
}

func TestClaudeExecutor_Name(t *testing.T) {
	exec := NewClaudeExecutor()
	assert.Equal(t, "claude", exec.Name(), "ClaudeExecutor name should be 'claude'")
}

func TestClaudeExecutor_Template(t *testing.T) {
	exec := NewClaudeExecutor()
	template := exec.Template()
	assert.Equal(t, "opencode \"{task}\"", template, "ClaudeExecutor should use opencode command")
}

func TestCodeDevTool_WithClaude(t *testing.T) {
	executors := map[string]ToolExecutor{
		"claude": NewClaudeExecutor(),
	}

	tool := NewCodeDevTool("/tmp", 300, executors)

	// 检查工具是否可用
	if exec, ok := tool.executors["claude"]; ok {
		assert.Equal(t, "claude", exec.Name())
		assert.Equal(t, "opencode \"{task}\"", exec.Template())
	} else {
		t.Error("Claude executor not found in tool")
	}
}
