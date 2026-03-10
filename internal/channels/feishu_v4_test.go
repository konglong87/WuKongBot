package channels

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConvertToMarkdownContentV4_Basic tests basic markdown conversion
func TestConvertToMarkdownContentV4_Basic(t *testing.T) {
	channel := &FeishuChannel{}

	text := "**mention user:**<at user_id=\"ou_xxxxxx\">Tom</at>\n**href:**[Open Platform](https://open.feishu.cn)"

	result := channel.convertToMarkdownContentV4(text)

	// Verify structure - no "post" wrapper
	zhCN, ok := result["zh_cn"].(map[string]interface{})
	assert.True(t, ok, "zh_cn field should be map[string]interface{}")

	title, ok := zhCN["title"].(string)
	assert.True(t, ok, "title field should be string")
	assert.Equal(t, "", title, "title should be empty")

	content, ok := zhCN["content"].([][]map[string]interface{})
	assert.True(t, ok, "content field should be [][]map[string]interface{}")
	assert.Len(t, content, 1, "content should have 1 block")

	assert.Len(t, content[0], 1, "block should have 1 line")

	element := content[0][0]
	assert.Equal(t, "md", element["tag"], "tag should be 'md'")
	assert.Equal(t, text, element["text"], "text should match input")
}

// TestConvertToMarkdownContentV4_WithTitle tests conversion with title
func TestConvertToMarkdownContentV4_WithTitle(t *testing.T) {
	// Create post content directly to test structure (no "post" wrapper)
	postContent := map[string]interface{}{
		"zh_cn": map[string]interface{}{
			"title": "我是一个标题",
			"content": [][][]map[string]interface{}{
				{
					{
						{
							"tag":  "md",
							"text": "**mention user:**<at user_id=\"ou_xxxxxx\">Tom</at>\n**href:**[Open Platform](https://open.feishu.cn)\n**code block:**\n```GO\nfunc main() int64 {\n\treturn 0\n}```",
						},
					},
				},
			},
		},
	}

	// Verify structure - no "post" wrapper
	zhCN, ok := postContent["zh_cn"].(map[string]interface{})
	assert.True(t, ok)

	title, ok := zhCN["title"].(string)
	assert.True(t, ok)
	assert.Equal(t, "我是一个标题", title)

	content, ok := zhCN["content"].([][][]map[string]interface{})
	assert.True(t, ok)
	assert.Len(t, content, 1)

	element := content[0][0][0]
	assert.Equal(t, "md", element["tag"])
	assert.Contains(t, element["text"], "mention user")
	assert.Contains(t, element["text"], "Open Platform")
	assert.Contains(t, element["text"], "code block")
}

// TestConvertToMarkdownContentV4_ComplexMarkdown tests complex markdown with various elements
func TestConvertToMarkdownContentV4_ComplexMarkdown(t *testing.T) {
	channel := &FeishuChannel{}

	text := "**mention user:**<at user_id=\"ou_xxxxxx\">Tom</at>\n" +
		"**href:**[Open Platform](https://open.feishu.cn)\n" +
		"**code block:**\n" +
		"```GO\nfunc main() int64 {\n\treturn 0\n}```\n" +
		"**text styles:** **bold**, *italic*, ***bold and italic***, ~underline~,~~lineThrough~~\n" +
		"> quote content\n\n" +
		"1. item1\n1. item1.1\n\n" +
		"2. item2.2\n2. item2\n" +
		"---\n" +
		"- item1\n    - item1.1\n    - item2.2\n- item2"

	result := channel.convertToMarkdownContentV4(text)

	// Verify the markdown text is preserved exactly (no "post" wrapper)
	zhCN, _ := result["zh_cn"].(map[string]interface{})
	content, _ := zhCN["content"].([][]map[string]interface{})
	element := content[0][0]

	assert.Equal(t, "md", element["tag"])
	assert.Equal(t, text, element["text"], "Markdown text should be preserved exactly")
}

// TestConvertToMarkdownContentV4_WithJSONSerialization tests that the content can be serialized to JSON
func TestConvertToMarkdownContentV4_WithJSONSerialization(t *testing.T) {
	channel := &FeishuChannel{}

	text := "**bold** text and *italic* text"
	result := channel.convertToMarkdownContentV4(text)

	// Try to serialize to JSON
	jsonBytes, err := json.Marshal(result)
	assert.NoError(t, err, "Should be able to marshal to JSON")

	// Verify it can be unmarshaled back
	var unmarshaled map[string]interface{}
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	assert.NoError(t, err, "Should be able to unmarshal JSON")

	// Verify structure is preserved (no "post" wrapper)
	zhCN, ok := unmarshaled["zh_cn"].(map[string]interface{})
	assert.True(t, ok)

	content, ok := zhCN["content"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, content, 1)
}

// TestConvertToMarkdownContentV4_EmptyText tests empty text
func TestConvertToMarkdownContentV4_EmptyText(t *testing.T) {
	channel := &FeishuChannel{}

	result := channel.convertToMarkdownContentV4("")

	// Verify structure still exists (no "post" wrapper)
	zhCN, ok := result["zh_cn"].(map[string]interface{})
	assert.True(t, ok)

	content, ok := zhCN["content"].([][]map[string]interface{})
	assert.True(t, ok)
	assert.Len(t, content, 1)

	element := content[0][0]
	assert.Equal(t, "md", element["tag"])
	assert.Equal(t, "", element["text"])
}

// TestConvertToMarkdownContentV4_WithNewlines tests that newlines are preserved
func TestConvertToMarkdownContentV4_WithNewlines(t *testing.T) {
	channel := &FeishuChannel{}

	text := "Line 1\nLine 2\nLine 3"
	result := channel.convertToMarkdownContentV4(text)

	zhCN, _ := result["zh_cn"].(map[string]interface{})
	content, ok := zhCN["content"].([][]map[string]interface{})
	assert.True(t, ok, "content should have correct type")

	element := content[0][0]

	assert.Equal(t, text, element["text"], "Newlines should be preserved in markdown")
}

// TestFeishuChannel_V4FormatStructure verifies the nested array structure
func TestFeishuChannel_V4FormatStructure(t *testing.T) {
	channel := &FeishuChannel{}

	text := "Test message"
	result := channel.convertToMarkdownContentV4(text)

	// The structure should be: content is [][]map[string]interface{}
	// - First level: array of blocks (paragraphs)
	// - Second level: array of elements within a block
	// No "post" wrapper
	zhCN, _ := result["zh_cn"].(map[string]interface{})
	content, ok := zhCN["content"].([][]map[string]interface{})

	assert.True(t, ok, "content should have correct type")
	assert.NotNil(t, content)

	// Verify we can access the element
	assert.NotEmpty(t, content, "content should not be empty")
	assert.NotEmpty(t, content[0], "first block should not be empty")
	assert.NotEmpty(t, content[0][0], "first element should not be empty")
}

// This is a conceptual test showing how to test sendTextV4
// In a real implementation, you would mock the Feishu client
/*
func TestSendTextV4(t *testing.T) {
	// Note: This test would require mocking the Feishu client
	// For now, we're testing the format conversion only

	channel := &FeishuChannel{
		client: nil, // Would be mock client
		ctx:    context.Background(),
	}

	msg := OutboundMessage{
		RecipientID: "ou_test123",
		Content:     "**Bold** text and *italic* text",
	}

	// Would test actual send logic here
	// err := channel.sendTextV4(context.Background(), msg)
	// assert.NoError(t, err)
}
*/

// TestParseMixedContent tests content parsing with markdown tags only
func TestParseMixedContent(t *testing.T) {
	channel := &FeishuChannel{}

	// Content with multiple blocks separated by \n\n
	input := "**根据目录内容，按文件后缀整理如下：**\n\n| 文件类型 | 数量 | 文件名 |\n|---------|------|--------|\n| 文件夹 | 7 | ecommerce/ |\n\n---\n\n**总计**: 16 个文件/文件夹"

	content := channel.parseMixedContent(input)

	// All blocks should use md tag (including tables as markdown)
	assert.Len(t, content, 4, "Should have 4 content blocks")

	// All blocks should use md tag
	for i, block := range content {
		assert.Equal(t, "md", block[0]["tag"], "Block %d should use md tag", i)
	}

	// First block should contain title
	assert.Contains(t, content[0][0]["text"], "根据目录内容", "First block text should match")

	// Second block should contain table (as markdown)
	assert.Contains(t, content[1][0]["text"], "| 文件类型 |", "Second block should contain table")

	// Third block should be separator
	assert.Equal(t, "---", content[2][0]["text"], "Third block should be separator")

	// Fourth block should contain summary
	assert.Contains(t, content[3][0]["text"], "总计", "Fourth block text should match")
}

// TestParseMixedContent_WithoutSeparator tests content parsing without separator
func TestParseMixedContent_WithoutSeparator(t *testing.T) {
	channel := &FeishuChannel{}

	// Without the --- separator, the summary is directly after table
	input := "**根据目录内容，按文件后缀整理如下：**\n\n| 文件类型 | 数量 | 文件名 |\n|---------|------|--------|\n| 文件夹 | 7 | ecommerce/ |\n\n**总计**: 16 个文件/文件夹"

	content := channel.parseMixedContent(input)

	assert.Len(t, content, 3, "Should have 3 content blocks")

	// All blocks should use md tag
	for i, block := range content {
		assert.Equal(t, "md", block[0]["tag"], "Block %d should use md tag", i)
	}

	// First block should contain title
	assert.Contains(t, content[0][0]["text"], "根据目录内容", "First block text should match")

	// Second block should contain table (as markdown)
	assert.Contains(t, content[1][0]["text"], "| 文件类型 |", "Second block should contain table")

	// Third block should contain summary
	assert.Contains(t, content[2][0]["text"], "总计", "Third block text should match")
}

// TestConvertToMarkdownContentV4_WithTable tests conversion with markdown table
func TestConvertToMarkdownContentV4_WithTable(t *testing.T) {
	channel := &FeishuChannel{}

	text := "**Title**\n\n| Col 1 | Col 2 |\n|-------|-------|\n| A     | B     |"

	result := channel.convertToMarkdownContentV4(text)

	// The result should NOT have "post" wrapper
	zhCN, ok := result["zh_cn"].(map[string]interface{})
	assert.True(t, ok, "zh_cn field should exist")

	content, ok := zhCN["content"].([][]map[string]interface{})
	assert.True(t, ok, "content field should exist")
	assert.Len(t, content, 2, "Should have 2 blocks")

	// Both blocks should use md tag (tables are rendered as markdown)
	assert.Equal(t, "md", content[0][0]["tag"])
	assert.Equal(t, "md", content[1][0]["tag"])
}

// TestConvertToMarkdownContentV4_ComplexMarkdown2 tests complex markdown with various elements
func TestConvertToMarkdownContentV4_ComplexMarkdown2(t *testing.T) {
	channel := &FeishuChannel{}

	text := "**mention user:**<at user_id=\"ou_xxxxxx\">Tom</at>\n" +
		"**href:**[Open Platform](https://open.feishu.cn)\n" +
		"**code block:**\n" +
		"```GO\nfunc main() int64 {\n\treturn 0\n}```\n" +
		"**text styles:** **bold**, *italic*, ***bold and italic***, ~underline~,~~lineThrough~~\n" +
		"> quote content\n\n" +
		"1. item1\n1. item1.1\n\n" +
		"2. item2.2\n2. item2\n" +
		"---\n" +
		"- item1\n    - item1.1\n    - item2.2\n- item2"

	result := channel.convertToMarkdownContentV4(text)

	// The markdown text should be preserved exactly (no "post" wrapper)
	zhCN, _ := result["zh_cn"].(map[string]interface{})
	content, _ := zhCN["content"].([][]map[string]interface{})
	element := content[0][0]

	assert.Equal(t, "md", element["tag"])
	assert.Equal(t, text, element["text"], "Markdown text should be preserved exactly")
}

// TestContainsMarkdownTable tests the table detection function
func TestContainsMarkdownTable(t *testing.T) {
	channel := &FeishuChannel{}

	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "Simple table with header and data",
			text:     "| Name | Age |\n|------|-----|\n| Alice | 25 |",
			expected: true,
		},
		{
			name:     "Table with header and separator only",
			text:     "| Column 1 | Column 2 |\n|----------|----------|",
			expected: true,
		},
		{
			name:     "Text with single pipe line (not a table)",
			text:     "This is not | a table",
			expected: false,
		},
		{
			name:     "Plain text without pipes",
			text:     "This is just plain text with no table",
			expected: false,
		},
		{
			name:     "Markdown list",
			text:     "- Item 1\n- Item 2\n- Item 3",
			expected: false,
		},
		{
			name:     "Empty text",
			text:     "",
			expected: false,
		},
		{
			name:     "Table with multiple rows",
			text:     "| ID | Name | Status |\n|----|------|--------|\n| 1 | Task 1 | Done |\n| 2 | Task 2 | Pending |",
			expected: true,
		},
		{
			name:     "Text with pipes but not table format",
			text:     "Here is some text | and more text\nBut this is not a table",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := channel.containsMarkdownTable(tt.text)
			assert.Equal(t, tt.expected, result, "containsMarkdownTable(%q) = %v, expected %v", tt.text, result, tt.expected)
		})
	}
}

// TestConvertToMarkdownContentV4_TableDetection tests smart table detection
func TestConvertToMarkdownContentV4_TableDetection(t *testing.T) {
	channel := &FeishuChannel{}

	t.Run("With table - should use block format", func(t *testing.T) {
		// Use table text with \n\n separator to create multiple blocks
		text := "**Table Title**\n\n| Column 1 | Column 2 |\n|----------|----------|\n| Data 1   | Data 2   |"
		result := channel.convertToMarkdownContentV4(text)

		zhCN, ok := result["zh_cn"].(map[string]interface{})
		assert.True(t, ok, "Should have zh_cn key")

		content, ok := zhCN["content"].([][]map[string]interface{})
		assert.True(t, ok, "Should have content array")

		// Should have multiple blocks (tables use block format)
		assert.Greater(t, len(content), 1, "Table content should have multiple blocks")

		// Should detect table and use parseMixedContent (which adds hr separators)
		assert.Equal(t, "hr", content[0][1]["tag"], "Table blocks should have hr separator")
	})

	t.Run("Without table - should use single block", func(t *testing.T) {
		text := "This is plain text without any table\nIt should be sent as a single block"
		result := channel.convertToMarkdownContentV4(text)

		zhCN, ok := result["zh_cn"].(map[string]interface{})
		assert.True(t, ok, "Should have zh_cn key")

		content, ok := zhCN["content"].([][]map[string]interface{})
		assert.True(t, ok, "Should have content array")

		// Should have exactly one block (plain text uses single block format)
		assert.Equal(t, 1, len(content), "Plain text should have exactly one block")
		assert.Equal(t, 1, len(content[0]), "Plain text block should have 1 element")
		assert.Equal(t, "md", content[0][0]["tag"], "Single block should use md tag")
		assert.Equal(t, text, content[0][0]["text"], "Text should match input")
	})
}
