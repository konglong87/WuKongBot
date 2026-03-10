package providers

import (
	"encoding/json"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestKimiHandler_ParseToolCalls_NoNestedRaw(t *testing.T) {
	handler := NewKimiHandler()

	// 模拟 Kimi 返回的第一次工具调用
	choice := openai.ChatCompletionChoice{
		Message: openai.ChatCompletionMessage{
			ToolCalls: []openai.ToolCall{
				{
					ID:   "call_1",
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      "exec",
						Arguments: `{"command": "ls -la"}`,
					},
				},
			},
		},
	}

	// 第一次解析
	toolCalls := handler.ParseToolCalls(choice)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	// 检查 Arguments 不应该有嵌套的 raw
	args := toolCalls[0].Arguments
	if _, ok := args["raw"]; ok {
		t.Errorf("Arguments should not have 'raw' wrapper, got: %v", args)
	}

	// 检查是否正确解析
	if cmd, ok := args["command"].(string); !ok || cmd != "ls -la" {
		t.Errorf("Expected command='ls -la', got: %v", args)
	}
}

func TestKimiHandler_ParseToolCalls_RoundTrip(t *testing.T) {
	handler := NewKimiHandler()

	// 模拟完整往返：
	// 1. Kimi 返回工具调用
	// 2. ParseToolCalls 解析
	// 3. addAssistantMessage 序列化
	// 4. ConvertMessages 发送给 Kimi
	// 5. Kimi 再次返回（不应该嵌套）

	// 第一步：解析 Kimi 的返回
	choice1 := openai.ChatCompletionChoice{
		Message: openai.ChatCompletionMessage{
			ToolCalls: []openai.ToolCall{
				{
					ID:   "call_1",
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      "exec",
						Arguments: `{"command": "ls"}`,
					},
				},
			},
		},
	}

	toolCalls := handler.ParseToolCalls(choice1)

	// 第二步：模拟 addAssistantMessage 的序列化
	argsJSON, _ := json.Marshal(toolCalls[0].Arguments)
	argumentsStr := string(argsJSON)

	t.Logf("After serialization: %s", argumentsStr)

	// 第三步：模拟 ConvertMessages 发送给 Kimi
	// Kimi 会把 argumentsStr 原样返回

	// 第四步：Kimi 再次返回工具调用（使用序列化后的字符串）
	choice2 := openai.ChatCompletionChoice{
		Message: openai.ChatCompletionMessage{
			ToolCalls: []openai.ToolCall{
				{
					ID:   "call_2",
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      "exec",
						Arguments: argumentsStr, // 使用上次序列化的结果
					},
				},
			},
		},
	}

	// 第五步：再次解析
	toolCalls2 := handler.ParseToolCalls(choice2)

	// 检查不应该有嵌套的 raw
	args2 := toolCalls2[0].Arguments

	// 统计 raw 嵌套深度
	rawCount := countNestedRaw(args2)
	if rawCount > 0 {
		t.Errorf("Arguments should not have nested 'raw' fields, got %d levels, args: %v", rawCount, args2)
	}

	// 应该能正确解析出 command
	if cmd, ok := args2["command"].(string); !ok || cmd != "ls" {
		t.Errorf("Expected command='ls', got: %v", args2)
	}
}

func countNestedRaw(args map[string]interface{}) int {
	count := 0
	if raw, ok := args["raw"]; ok {
		count = 1
		if rawMap, ok := raw.(map[string]interface{}); ok {
			count += countNestedRaw(rawMap)
		}
	}
	return count
}
