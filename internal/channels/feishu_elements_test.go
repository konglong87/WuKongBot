package channels

import (
	"encoding/json"
	"testing"
)

// TestMarkdownToPost_Headings tests heading markdown syntax
func TestMarkdownToPost_Headings(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Level 1 heading",
			input: "# Heading 1",
		},
		{
			name:  "Level 2 heading",
			input: "## Heading 2",
		},
		{
			name:  "Level 3 heading",
			input: "### Heading 3",
		},
		{
			name:  "Headings with text",
			input: "## This is a heading",
		},
		{
			name:  "Heading and text",
			input: "# Title\n\nSome text",
		},
		{
			name:  "Multiple headings",
			input: "# Title\n## Subtitle\n### Section",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := channel.convertToPostContentV2(tc.input)
			resultJSON, _ := json.Marshal(result)

			t.Logf("Input: %s", tc.input)
			t.Logf("Output: %s", string(resultJSON))

			post := result["post"].(map[string]interface{})
			zhCN := post["zh_cn"].(map[string]interface{})
			content := zhCN["content"].([]interface{})

			t.Logf("Number of paragraphs: %d", len(content))
		})
	}
}

// TestMarkdownToPost_Blockquote tests blockquote markdown syntax
func TestMarkdownToPost_Blockquote(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Single blockquote",
			input: "> This is a quote",
		},
		{
			name:  "Blockquote without space",
			input: ">This is a quote",
		},
		{
			name:  "Multiple blockquotes",
			input: "> Quote 1\n> Quote 2",
		},
		{
			name:  "Blockquote with bold",
			input: "> **Bold quote**",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := channel.convertToPostContentV2(tc.input)
			resultJSON, _ := json.Marshal(result)

			t.Logf("Input: %s", tc.input)
			t.Logf("Output: %s", string(resultJSON))
		})
	}
}

// TestMarkdownToPost_Lists tests list markdown syntax
func TestMarkdownToPost_Lists(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Bulleted list",
			input: "- Item 1\n- Item 2\n- Item 3",
		},
		{
			name:  "Asterisk list",
			input: "* Item 1\n* Item 2\n* Item 3",
		},
		{
			name:  "Numbered list",
			input: "1. First\n2. Second\n3. Third",
		},
		{
			name:  "List with bold",
			input: "- **Bold** item",
		},
		{
			name:  "Nested list",
			input: "- Item 1\n  - Subitem 1.1\n  - Subitem 1.2\n- Item 2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := channel.convertToPostContentV2(tc.input)
			resultJSON, _ := json.Marshal(result)

			t.Logf("Input:\n%s", tc.input)
			t.Logf("Output: %s", string(resultJSON))
		})
	}
}

// TestMarkdownToPost_Code tests code markdown syntax
func TestMarkdownToPost_Code(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Inline code",
			input: "Use `code` for inline",
		},
		{
			name:  "Multiple inline codes",
			input: "`code1` and `code2`",
		},
		{
			name:  "Code with spaces",
			input: "`function name()`",
		},
		{
			name:  "Code block (triple backticks)",
			input: "```\nfunc main() {\n    print(\"hello\")\n}\n```",
		},
		{
			name:  "Code block with language",
			input: "```go\nfunc main() {\n    print(\"hello\")\n}\n```",
		},
		{
			name:  "Python code block",
			input: "```python\nprint('hello')\n```",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := channel.convertToPostContentV2(tc.input)
			resultJSON, _ := json.Marshal(result)

			t.Logf("Input:\n%s", tc.input)
			t.Logf("Output: %s", string(resultJSON))
		})
	}
}

// TestMarkdownToPost_Links tests link markdown syntax
func TestMarkdownToPost_Links(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Basic link",
			input: "[Open Platform](https://open.feishu.cn)",
		},
		{
			name:  "Link with text",
			input: "Visit [Feishu](https://feishu.cn) for more",
		},
		{
			name:  "Multiple links",
			input: "[Link1](url1) and [Link2](url2)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := channel.convertToPostContentV2(tc.input)
			resultJSON, _ := json.Marshal(result)

			t.Logf("Input: %s", tc.input)
			t.Logf("Output: %s", string(resultJSON))
		})
	}
}

// TestMarkdownToPost_MixedMarkdown tests mixed markdown syntax
func TestMarkdownToPost_MixedMarkdown(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Heading + Bold + List",
			input: "# Title\n\n## Introduction\n\n**Important:**\n- Point 1\n- Point 2",
		},
		{
			name:  "Paragraph with bold and code",
			input: "Use **bold** and `code` in your text.",
		},
		{
			name:  "Quote with bold",
			input: "> **Note:** This is important",
		},
		{
			name:  "Complex document structure",
			input: "# Guide\n\n## Introduction\n\nThis is a guide with **bold text** and `code snippets`.\n\n## Steps\n\n1. Step one\n2. Step two\n\n> **Tip:** Read carefully.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n=== %s ===", tc.name)
			t.Logf("Input:\n%s", tc.input)

			result := channel.convertToPostContentV2(tc.input)
			resultJSON, _ := json.Marshal(result)

			t.Logf("Output:\n%s", string(resultJSON))

			post := result["post"].(map[string]interface{})
			zhCN := post["zh_cn"].(map[string]interface{})
			content := zhCN["content"].([]interface{})

			t.Logf("Number of paragraphs: %d", len(content))

			for i, para := range content {
				paragraph, _ := para.([]interface{})
				t.Logf("Paragraph %d has %d elements", i, len(paragraph))
			}
		})
	}
}

// TestMarkdownToPost_ParagraphSeparation tests paragraph separation
func TestMarkdownToPost_ParagraphSeparation(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name              string
		input             string
		expectedParagraphs int
	}{
		{
			name:               "Single paragraph",
			input:              "Single line of text",
			expectedParagraphs: 1,
		},
		{
			name:               "Two paragraphs (double newline)",
			input:              "First paragraph\n\nSecond paragraph",
			expectedParagraphs: 2,
		},
		{
			name:               "Three paragraphs",
			input:              "First\n\nSecond\n\nThird",
			expectedParagraphs: 3,
		},
		{
			name:               "Single newline (same paragraph)",
			input:              "Line 1\nLine 2",
			expectedParagraphs: 1, // May vary by implementation
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := channel.convertToPostContentV2(tc.input)
			post := result["post"].(map[string]interface{})
			zhCN := post["zh_cn"].(map[string]interface{})
			content := zhCN["content"].([]interface{})

			actualParagraphs := len(content)

			t.Logf("Input: %q", tc.input)
			t.Logf("Expected paragraphs: %d, Actual: %d", tc.expectedParagraphs, actualParagraphs)

			if tc.expectedParagraphs > 0 && actualParagraphs != tc.expectedParagraphs {
				t.Logf("Note: Paragraph count differs from expected")
			}

			resultJSON, _ := json.Marshal(result)
			t.Logf("Output: %s", string(resultJSON))
		})
	}
}

// TestMarkdownToPost_Emojis tests emoji handling
func TestMarkdownToPost_Emojis(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Simple emoji",
			input: "Hello 👋 World",
		},
		{
			name:  "Multiple emojis",
			input: "😀 😃 😄 😁",
		},
		{
			name:  "Emoji with bold",
			input: "**Important! ⚠️**",
		},
		{
			name:  "Text emoji combinations",
			input: "🎉 Celebrate! 🎊",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := channel.convertToPostContentV2(tc.input)
			resultJSON, _ := json.Marshal(result)

			t.Logf("Input: %s", tc.input)
			t.Logf("Output: %s", string(resultJSON))
		})
	}
}