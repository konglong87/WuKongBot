package enhanced

import (
	"testing"
)

func TestClaudeInteractiveParser_ParseSelectQuestion(t *testing.T) {
	parser := NewClaudeInteractiveParser()

	tests := []struct {
		name            string
		output          string
		sessionID       string
		expectedType    string
		expectedPrompt  string
		expectedOptions []string
		expectMatch     bool
	}{
		{
			name: "Multiple select options",
			output: `? Choose database type:
  [ ] PostgreSQL
  [ ] MySQL
  [ ] SQLite`,
			sessionID:       "session123",
			expectedType:    "select",
			expectedPrompt:  "database type",
			expectedOptions: []string{"PostgreSQL", "MySQL", "SQLite"},
			expectMatch:     true,
		},
		{
			name: "Single select option",
			output: `? Choose theme:
  [ ] dark`,
			sessionID:       "session456",
			expectedType:    "select",
			expectedPrompt:  "theme",
			expectedOptions: []string{"dark"},
			expectMatch:     true,
		},
		{
			name:        "No select options",
			output:      `? Choose option:`,
			sessionID:   "session789",
			expectMatch: false,
		},
		{
			name:        "Not a select question",
			output:      `This is just regular text`,
			sessionID:   "session000",
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			question, ok := parser.ParseInteractiveQuestion(tt.output, tt.sessionID)

			if tt.expectMatch && !ok {
				t.Fatal("Expected to parse select question")
			}

			if !tt.expectMatch && ok {
				t.Fatal("Expected NOT to match, but got a match")
			}

			if ok {
				if question.Type != tt.expectedType {
					t.Errorf("Expected type '%s', got '%s'", tt.expectedType, question.Type)
				}

				if question.Prompt != tt.expectedPrompt {
					t.Errorf("Expected prompt '%s', got '%s'", tt.expectedPrompt, question.Prompt)
				}

				if tt.expectedOptions != nil {
					if len(question.Options) != len(tt.expectedOptions) {
						t.Errorf("Expected %d options, got %d", len(tt.expectedOptions), len(question.Options))
					}

					for i, expectedOption := range tt.expectedOptions {
						if i < len(question.Options) && question.Options[i] != expectedOption {
							t.Errorf("Option %d: expected '%s', got '%s'", i, expectedOption, question.Options[i])
						}
					}
				}

				if question.SessionID != tt.sessionID {
					t.Errorf("Expected sessionID '%s', got '%s'", tt.sessionID, question.SessionID)
				}
			}
		})
	}
}

func TestClaudeInteractiveParser_ParseInputQuestion(t *testing.T) {
	parser := NewClaudeInteractiveParser()

	tests := []struct {
		name           string
		output         string
		sessionID      string
		expectedType   string
		expectedPrompt string
		expectMatch    bool
	}{
		{
			name:           "Simple input question",
			output:         `? Enter your project name: `,
			sessionID:      "session456",
			expectedType:   "input",
			expectedPrompt: "your project name",
			expectMatch:    true,
		},
		{
			name:           "Complex input prompt",
			output:         `? Enter the full path to your configuration file (absolute or relative): `,
			sessionID:      "session789",
			expectedType:   "input",
			expectedPrompt: "the full path to your configuration file (absolute or relative)",
			expectMatch:    true,
		},
		{
			name:        "Not an input question",
			output:      `This is just regular text`,
			sessionID:   "session000",
			expectMatch: false,
		},
		{
			name:        "Empty output",
			output:      "",
			sessionID:   "session111",
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			question, ok := parser.ParseInteractiveQuestion(tt.output, tt.sessionID)

			if tt.expectMatch && !ok {
				t.Fatal("Expected to parse input question")
			}

			if !tt.expectMatch && ok {
				t.Fatal("Expected NOT to match, but got a match")
			}

			if ok {
				if question.Type != tt.expectedType {
					t.Errorf("Expected type '%s', got '%s'", tt.expectedType, question.Type)
				}

				if question.Prompt != tt.expectedPrompt {
					t.Errorf("Expected prompt '%s', got '%s'", tt.expectedPrompt, question.Prompt)
				}

				if question.SessionID != tt.sessionID {
					t.Errorf("Expected sessionID '%s', got '%s'", tt.sessionID, question.SessionID)
				}
			}
		})
	}
}

func TestClaudeInteractiveParser_ParseConfirmQuestion(t *testing.T) {
	parser := NewClaudeInteractiveParser()

	tests := []struct {
		name            string
		output          string
		sessionID       string
		expectedType    string
		expectedPrompt  string
		expectedOptions []string
		expectMatch     bool
	}{
		{
			name:            "Simple confirm question",
			output:          `? Do you want to continue? (y/n)`,
			sessionID:       "session123",
			expectedType:    "confirm",
			expectedPrompt:  "to continue?",
			expectedOptions: []string{"yes", "no"},
			expectMatch:     true,
		},
		{
			name:            "Confirm with longer prompt",
			output:          `? Do you want to create a new project? This will create directories and files. (y/n)`,
			sessionID:       "session456",
			expectedType:    "confirm",
			expectedPrompt:  "to create a new project? This will create directories and files.",
			expectedOptions: []string{"yes", "no"},
			expectMatch:     true,
		},
		{
			name:        "Not a confirm question",
			output:      `This is just regular text`,
			sessionID:   "session000",
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			question, ok := parser.ParseInteractiveQuestion(tt.output, tt.sessionID)

			if tt.expectMatch && !ok {
				t.Fatal("Expected to parse confirm question")
			}

			if !tt.expectMatch && ok {
				t.Fatal("Expected NOT to match, but got a match")
			}

			if ok {
				if question.Type != tt.expectedType {
					t.Errorf("Expected type '%s', got '%s'", tt.expectedType, question.Type)
				}

				if question.Prompt != tt.expectedPrompt {
					t.Errorf("Expected prompt '%s', got '%s'", tt.expectedPrompt, question.Prompt)
				}

				if len(question.Options) != len(tt.expectedOptions) {
					t.Errorf("Expected %d options, got %d", len(tt.expectedOptions), len(question.Options))
				}

				for i, expectedOption := range tt.expectedOptions {
					if i < len(question.Options) && question.Options[i] != expectedOption {
						t.Errorf("Option %d: expected '%s', got '%s'", i, expectedOption, question.Options[i])
					}
				}

				if question.SessionID != tt.sessionID {
					t.Errorf("Expected sessionID '%s', got '%s'", tt.sessionID, question.SessionID)
				}
			}
		})
	}
}

func TestClaudeInteractiveParser_FormatAnswer_Select(t *testing.T) {
	parser := NewClaudeInteractiveParser()

	question := &InteractiveQuestion{
		Type:    "select",
		Options: []string{"Option A", "Option B", "Option C"},
	}

	t.Run("Select first option", func(t *testing.T) {
		answer := parser.FormatAnswer(question, 0)
		if answer != "1" {
			t.Errorf("Expected answer '1', got '%s'", answer)
		}
	})

	t.Run("Select second option", func(t *testing.T) {
		answer := parser.FormatAnswer(question, 1)
		if answer != "2" {
			t.Errorf("Expected answer '2', got '%s'", answer)
		}
	})

	t.Run("Select third option", func(t *testing.T) {
		answer := parser.FormatAnswer(question, 2)
		if answer != "3" {
			t.Errorf("Expected answer '3', got '%s'", answer)
		}
	})

	t.Run("Invalid index - negative", func(t *testing.T) {
		answer := parser.FormatAnswer(question, -1)
		if answer != "" {
			t.Errorf("Expected empty string, got '%s'", answer)
		}
	})

	t.Run("Invalid index - out of range", func(t *testing.T) {
		answer := parser.FormatAnswer(question, 5)
		if answer != "" {
			t.Errorf("Expected empty string, got '%s'", answer)
		}
	})
}

func TestClaudeInteractiveParser_FormatAnswer_Input(t *testing.T) {
	parser := NewClaudeInteractiveParser()

	question := &InteractiveQuestion{
		Type: "input",
	}

	t.Run("Simple text input", func(t *testing.T) {
		answer := parser.FormatAnswer(question, "my-project")
		if answer != "my-project" {
			t.Errorf("Expected 'my-project', got '%s'", answer)
		}
	})

	t.Run("Path input", func(t *testing.T) {
		answer := parser.FormatAnswer(question, "/home/user/project")
		if answer != "/home/user/project" {
			t.Errorf("Expected '/home/user/project', got '%s'", answer)
		}
	})

	t.Run("Empty input", func(t *testing.T) {
		answer := parser.FormatAnswer(question, "")
		if answer != "" {
			t.Errorf("Expected empty string, got '%s'", answer)
		}
	})

	t.Run("Wrong type for input", func(t *testing.T) {
		answer := parser.FormatAnswer(question, 123)
		if answer != "" {
			t.Errorf("Expected empty string, got '%s'", answer)
		}
	})
}

func TestClaudeInteractiveParser_FormatAnswer_Confirm(t *testing.T) {
	parser := NewClaudeInteractiveParser()

	question := &InteractiveQuestion{
		Type: "confirm",
	}

	t.Run("Confirm yes lowercase", func(t *testing.T) {
		answer := parser.FormatAnswer(question, "yes")
		if answer != "y" {
			t.Errorf("Expected 'y', got '%s'", answer)
		}
	})

	t.Run("Confirm yes uppercase", func(t *testing.T) {
		answer := parser.FormatAnswer(question, "YES")
		if answer != "y" {
			t.Errorf("Expected 'y', got '%s'", answer)
		}
	})

	t.Run("Confirm no lowercase", func(t *testing.T) {
		answer := parser.FormatAnswer(question, "no")
		if answer != "n" {
			t.Errorf("Expected 'n', got '%s'", answer)
		}
	})

	t.Run("Confirm no uppercase", func(t *testing.T) {
		answer := parser.FormatAnswer(question, "NO")
		if answer != "n" {
			t.Errorf("Expected 'n', got '%s'", answer)
		}
	})

	t.Run("Invalid input - defaults to no", func(t *testing.T) {
		answer := parser.FormatAnswer(question, "maybe")
		if answer != "n" {
			t.Errorf("Expected 'n' (default), got '%s'", answer)
		}
	})

	t.Run("Empty input - defaults to no", func(t *testing.T) {
		answer := parser.FormatAnswer(question, "")
		if answer != "n" {
			t.Errorf("Expected 'n' (default), got '%s'", answer)
		}
	})
}

func TestClaudeInteractiveParser_FormatAnswer_InvalidType(t *testing.T) {
	parser := NewClaudeInteractiveParser()

	tests := []struct {
		name     string
		dataType string
		question *InteractiveQuestion
		answer   interface{}
		expected string
	}{
		{
			name:     "Unknown type",
			dataType: "unknown",
			question: &InteractiveQuestion{Type: "unknown"},
			answer:   "some answer",
			expected: "",
		},
		{
			name:     "Empty type",
			dataType: "",
			question: &InteractiveQuestion{Type: ""},
			answer:   "some answer",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answer := parser.FormatAnswer(tt.question, tt.answer)
			if answer != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, answer)
			}
		})
	}
}

func TestClaudeInteractiveParser_DetectQuestionType(t *testing.T) {
	parser := NewClaudeInteractiveParser()

	tests := []struct {
		name         string
		output       string
		expectedType string
		expectDetect bool
	}{
		{
			name:         "Detect select question",
			output:       `? Choose database type:\n  [ ] PostgreSQL`,
			expectedType: "select",
			expectDetect: true,
		},
		{
			name:         "Detect input question",
			output:       `? Enter your project name: `,
			expectedType: "input",
			expectDetect: true,
		},
		{
			name:         "Detect confirm question",
			output:       `? Do you want to continue? (y/n)`,
			expectedType: "confirm",
			expectDetect: true,
		},
		{
			name:         "No question detected - regular text",
			output:       `This is just regular conversation output`,
			expectedType: "",
			expectDetect: false,
		},
		{
			name:         "No question detected - empty",
			output:       "",
			expectedType: "",
			expectDetect: false,
		},
		{
			name:         "Multiple questions in output - first one detected",
			output:       `? Choose database type:\n  [ ] PostgreSQL\n\n? Enter your project name: `,
			expectedType: "select",
			expectDetect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qType, detected := parser.DetectQuestionType(tt.output)

			if tt.expectDetect && !detected {
				t.Fatal("Expected to detect a question")
			}

			if !tt.expectDetect && detected {
				t.Fatal("Expected NOT to detect, but got a detection")
			}

			if detected && qType != tt.expectedType {
				t.Errorf("Expected type '%s', got '%s'", tt.expectedType, qType)
			}
		})
	}
}

func TestClaudeInteractiveParser_EdgeCases(t *testing.T) {
	parser := NewClaudeInteractiveParser()

	t.Run("Malformed select - no prompt", func(t *testing.T) {
		output := `? Choose:`
		question, ok := parser.ParseInteractiveQuestion(output, "session123")
		if ok {
			t.Fatal("Expected NOT to match, but got a match")
		}
		if question != nil {
			t.Fatal("Expected nil question, got non-nil")
		}
	})

	t.Run("Malformed select - only prompt newline", func(t *testing.T) {
		output := `? Choose database type:\n`
		question, ok := parser.ParseInteractiveQuestion(output, "session123")
		if ok {
			t.Fatal("Expected NOT to match, but got a match")
		}
		if question != nil {
			t.Fatal("Expected nil question, got non-nil")
		}
	})

	t.Run("Mixed content - question in the middle", func(t *testing.T) {
		output := "This is some content\n? Enter your project name: \nMore content"
		question, ok := parser.ParseInteractiveQuestion(output, "session456")
		if !ok {
			t.Fatal("Expected to parse input question")
		}
		if question.Type != "input" {
			t.Errorf("Expected type 'input', got '%s'", question.Type)
		}
	})

	t.Run("Select with extra whitespace", func(t *testing.T) {
		output := "?   Choose    database    type:  \n  [ ]   PostgreSQL"
		question, ok := parser.ParseInteractiveQuestion(output, "session789")
		if !ok {
			t.Fatal("Expected to parse select question")
		}
		// Only trims outer whitespace, preserves internal spaces in prompt
		if question.Prompt != "database    type" {
			t.Errorf("Expected prompt 'database    type', got '%s'", question.Prompt)
		}
	})
}
