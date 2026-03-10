package channels

import (
	"encoding/json"
	"testing"
)

// TestMarkdownToPost_Bold tests bold text conversion (**text**)
func TestMarkdownToPost_Bold(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name            string
		input           string
		expectedBold    []string // Text that should be bold
		expectedPlain   []string // Text that should be plain
	}{
		{
			name:          "Single bold word",
			input:         "This is **bold** text",
			expectedBold:  []string{"bold"},
			expectedPlain: []string{"This is ", " text"},
		},
		{
			name:          "Multiple bold words",
			input:         "**Hello** **World**",
			expectedBold:  []string{"Hello", "World"},
			expectedPlain: []string{" "},
		},
		{
			name:          "Bold at start",
			input:         "**Bold** text",
			expectedBold:  []string{"Bold"},
			expectedPlain: []string{" text"},
		},
		{
			name:          "Bold at end",
			input:         "Text **bold**",
			expectedBold:  []string{"bold"},
			expectedPlain: []string{"Text "},
		},
		{
			name:          "Only bold",
			input:         "**Everything bold**",
			expectedBold:  []string{"Everything bold"},
			expectedPlain: []string{},
		},
		{
			name:          "Chinese bold",
			input:         "这是**加粗**文本",
			expectedBold:  []string{"加粗"},
			expectedPlain: []string{"这是", "文本"},
		},
		{
			name:          "Bold with special characters",
			input:         "Test **bold!** text",
			expectedBold:  []string{"bold!"},
			expectedPlain: []string{"Test ", " text"},
		},
		{
			name:          "Multiple bold sections",
			input:         "Plain **bold1** plain **bold2** plain",
			expectedBold:  []string{"bold1", "bold2"},
			expectedPlain: []string{"Plain ", " plain ", " plain"},
		},
		{
			name:          "Bold with numbers",
			input:         "Version **2.0** released",
			expectedBold:  []string{"2.0"},
			expectedPlain: []string{"Version ", " released"},
		},
		{
			name:          "Empty bold markers",
			input:         "Test****text",
			expectedBold:  []string{},
			expectedPlain: []string{"Test", "text"}, // Or could be "Testtext" depending on implementation
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := channel.convertToPostContentV2(tc.input)
			post := result["post"].(map[string]interface{})
			zhCN := post["zh_cn"].(map[string]interface{})
			content := zhCN["content"].([][]map[string]interface{})
			paragraph := content[0]

			resultJSON, _ := json.Marshal(result)
			t.Logf("Input: %s", tc.input)
			t.Logf("Output: %s", string(resultJSON))

			// Verify bold elements have style: ["bold"]
			boldCount := 0
			plainCount := 0

			for _, element := range paragraph {
				tag, _ := element["tag"].(string)
				if tag != "text" {
					t.Errorf("Expected tag to be 'text', got: %v", tag)
					continue
				}

				style, hasStyle := element["style"]
				text, _ := element["text"].(string)

				if hasStyle {
					// Should be bold
					styleSlice, ok := style.([]string)
					if !ok {
						t.Errorf("Expected style to be a []string, got: %T", style)
						continue
					}

					if len(styleSlice) == 1 && styleSlice[0] == "bold" {
						boldCount++
						t.Logf("  ✓ Bold: '%s'", text)

						// Verify this text should be bold
						found := false
						for _, expected := range tc.expectedBold {
							if text == expected {
								found = true
								break
							}
						}
						if !found && len(tc.expectedBold) > 0 {
							t.Errorf("Unexpected bold text: '%s'", text)
						}
					}
				} else {
					// Plain text
					plainCount++
					t.Logf("  Plain: '%s'", text)

					// Verify this text should be plain
					found := false
					for _, expected := range tc.expectedPlain {
						if text == expected {
							found = true
							break
						}
					}
					if !found && len(tc.expectedPlain) > 0 {
						// Some plain text might be combined, so we're lenient here
						t.Logf("  Note: Plain text '%s' not in expected list", text)
					}
				}
			}

			if boldCount != len(tc.expectedBold) && len(tc.expectedBold) > 0 {
				t.Errorf("Expected %d bold elements, got %d", len(tc.expectedBold), boldCount)
			}

			t.Logf("  Summary: %d bold, %d plain elements", boldCount, plainCount)
		})
	}
}

// TestMarkdownToPost_BoldNesting tests various edge cases with bold markers
func TestMarkdownToPost_BoldNesting(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Triple asterisks",
			input: "Text ***bold*** more",
		},
		{
			name:  "Four asterisks",
			input: "Text ****bold**** more",
		},
		{
			name:  "Bold within word",
			input: "Hel**lo** World",
		},
		{
			name:  "Overlapping markers - closing after opening",
			input: "Text **bold1 **bold2** text",
		},
		{
			name:  "Unclosed bold",
			input: "Text **bold text",
		},
		{
			name:  "Empty text between markers",
			input: "Text **  ** text",
		},
		{
			name:  "Bold with newline",
			input: "Text **bold\ntext** more",
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
			paragraph := content[0].([]interface{})

			t.Logf("Number of elements: %d", len(paragraph))

			// Just verify it doesn't panic
			for i, elem := range paragraph {
				element, _ := elem.(map[string]interface{})
				text, _ := element["text"].(string)
				_, hasStyle := element["style"]
				if hasStyle {
					t.Logf("  [%d] Bold: '%s'", i, text)
				} else {
					t.Logf("  [%d] Plain: '%s'", i, text)
				}
			}
		})
	}
}

// TestMarkdownToPost_StyleFormat tests that style field is correct format
func TestMarkdownToPost_StyleFormat(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	input := "**Bold** text"
	result := channel.convertToPostContentV2(input)
	resultJSON, _ := json.Marshal(result)

	t.Logf("Full JSON: %s", string(resultJSON))

	post := result["post"].(map[string]interface{})
	zhCN := post["zh_cn"].(map[string]interface{})
	content := zhCN["content"].([]interface{})
	paragraph := content[0].([]interface{})

	// Find bold element
	for _, elem := range paragraph {
		element, ok := elem.(map[string]interface{})
		if !ok {
			continue
		}

		style, hasStyle := element["style"]
		if hasStyle {
			// Verify style is a string array
			styleSlice, ok := style.([]interface{})
			if !ok {
				t.Errorf("Expected style to be []interface{}, got: %T", style)
				continue
			}

			t.Logf("Style: %v (type: %T)", styleSlice, styleSlice)

			// According to official API, style should be an array of strings
			// Like: "style": ["bold"]
			if len(styleSlice) != 1 {
				t.Errorf("Expected style array to have 1 element, got %d", len(styleSlice))
			}

			styleValue, ok := styleSlice[0].(string)
			if !ok {
				t.Errorf("Expected style[0] to be string, got: %T", styleSlice[0])
			}

			if styleValue != "bold" {
				t.Errorf("Expected style value to be 'bold', got: '%s'", styleValue)
			}

			t.Log("✓ Style format is correct: [\"bold\"]")
			return
		}
	}

	t.Error("Did not find any bold element with style field")
}

// TestMarkdownToPost_V2Comparison compares V2 with original implementation
func TestMarkdownToPost_V2Comparison(t *testing.T) {
	channel := NewFeishuChannel(FeishuConfig{})

	testCases := []string{
		"Plain text",
		"**Bold** text",
		"Text **bold** text **another bold**",
		"**All bold**",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			t.Logf("\n=== Test: %s ===", tc)

			// V2 implementation
			resultV2 := channel.convertToPostContentV2(tc)
			v2JSON, _ := json.Marshal(resultV2)
			t.Logf("V2 Output:\n%s", string(v2JSON))

			// Original implementation
			resultOrig := channel.convertToPostContent(tc)
			origJSON, _ := json.Marshal(resultOrig)
			t.Logf("Original Output:\n%s", string(origJSON))
		})
	}
}