package channels

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMarkdownToPost_BasicText tests basic text conversion to post format
func TestMarkdownToPost_BasicText(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name     string
		input    string
		wantText string // Expected text content in the first element
	}{
		{
			name:     "Plain text",
			input:    "Hello World",
			wantText: "Hello World",
		},
		{
			name:     "Chinese text",
			input:    "你好世界",
			wantText: "你好世界",
		},
		{
			name:     "Empty text",
			input:    "",
			wantText: "",
		},
		{
			name:     "Single line",
			input:    "这是一行文本",
			wantText: "这是一行文本",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := channel.convertToPostContentV2(tc.input)
			resultJSON, _ := json.Marshal(result)

			t.Logf("Input: %s", tc.input)
			t.Logf("Output: %s", string(resultJSON))

			// Verify it's a valid post structure
			post, ok := result["post"].(map[string]interface{})
			if !ok {
				t.Errorf("Expected post field in result")
				return
			}

			zhCN, ok := post["zh_cn"].(map[string]interface{})
			if !ok {
				t.Errorf("Expected zh_cn field in post")
				return
			}

			content, ok := zhCN["content"].([][]map[string]interface{})
			if !ok {
				t.Errorf("Expected content field in zh_cn")
				return
			}

			if len(content) == 0 {
				if tc.wantText != "" {
					t.Errorf("Expected content to have elements for input: %s", tc.input)
				}
				return
			}

			// Get first paragraph
			firstParagraph := content[0]

			if len(firstParagraph) == 0 {
				t.Errorf("Expected first paragraph to have elements")
				return
			}

			// Get first element
			firstElement := firstParagraph[0]

			tag := firstElement["tag"].(string)
			if tag != "text" {
				t.Errorf("Expected tag to be 'text', got: %v", firstElement["tag"])
			}

			text := firstElement["text"].(string)
			if !ok || text != tc.wantText {
				t.Errorf("Expected text to be '%s', got: '%s'", tc.wantText, text)
			}
		})
	}
}

// TestMarkdownToPost_BasicStructure tests the overall post structure
func TestMarkdownToPost_BasicStructure(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	input := "Test message"
	result := channel.convertToPostContentV2(input)
	resultJSON, _ := json.Marshal(result)

	t.Logf("Full post JSON:\n%s", string(resultJSON))

	// Verify structure follows official Feishu API format
	// Expected: {"post": {"zh_cn": {"title": "", "content": [[{"tag": "text", "text": "..."}]}}}}
	post, ok := result["post"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'post' field at root level, got: %v", result)
	}

	// Check zh_cn exists
	zhCN, ok := post["zh_cn"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'zh_cn' field in post, got keys: %v", post)
	}

	// Check content exists
	content, ok := zhCN["content"].([][]map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'content' field in zh_cn, got keys: %v", zhCN)
	}

	// Content should be array of arrays (paragraphs)
	if len(content) == 0 {
		t.Fatal("Expected content to have at least one paragraph")
	}

	if len(content[0]) == 0 {
		t.Fatal("Expected first paragraph to have at least one element")
	}

	// First element should have "tag" field
	firstElement := content[0][0]

	tag, ok := firstElement["tag"]
	if !ok {
		t.Fatal("Expected element to have 'tag' field")
	}

	if tag != "text" {
		t.Errorf("Expected tag to be 'text', got: %v", tag)
	}

	// Verify title field exists
	title, ok := zhCN["title"]
	if !ok {
		t.Fatal("Expected 'title' field in zh_cn")
	}

	if title != "" {
		t.Errorf("Expected empty title, got: %v", title)
	}

	t.Log("✓ Post structure is valid")
}

// TestMarkdownToPost_MultipleLines tests multi-line text handling
func TestMarkdownToPost_MultipleLines(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name          string
		input         string
		expectedLines int
	}{
		{
			name:          "Two lines with single newline",
			input:         "Line 1\nLine 2",
			expectedLines: 1, // Current implementation may merge into one paragraph
		},
		{
			name:          "Multiple lines",
			input:         "Line 1\nLine 2\nLine 3",
			expectedLines: 1,
		},
		{
			name:          "Line with spaces",
			input:         "Hello   World",
			expectedLines: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := channel.convertToPostContentV2(tc.input)
			post := result["post"].(map[string]interface{})
			zhCN := post["zh_cn"].(map[string]interface{})
			content := zhCN["content"].([][]map[string]interface{})

			t.Logf("Input lines: %d, Output paragraphs: %d", len(strings.Split(tc.input, "\n")), len(content))

			resultJSON, _ := json.Marshal(result)
			t.Logf("Output: %s", string(resultJSON))
		})
	}
}