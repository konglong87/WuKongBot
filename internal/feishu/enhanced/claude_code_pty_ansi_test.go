package enhanced

import (
	"testing"
)

func TestRemoveANSICodes(t *testing.T) {
	manager := NewClaudeCodePTYManager("", "test", func(_, _, _ string) error { return nil })

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "No ANSI codes",
			input:    "Hello, World!",
			expected: "Hello, World!",
		},
		{
			name:     "Simple color codes",
			input:    "\x1b[31mRed text\x1b[0m",
			expected: "Red text",
		},
		{
			name:     "Bracketed paste mode enable",
			input:    "\x1b[?2004h",
			expected: "",
		},
		{
			name:     "Bracketed paste mode disable",
			input:    "\x1b[?2004l",
			expected: "",
		},
		{
			name:     "Brace mode enable",
			input:    "\x1b[?2026h",
			expected: "",
		},
		{
			name:     "Brace mode disable - from log issue",
			input:    "\x1b[?2026l",
			expected: "",
		},
		{
			name:     "Mixed ANSI codes - colors and modes",
			input:    "\x1b[31mHello\x1b[?2026l World\x1b[0m",
			expected: "Hello World",
		},
		{
			name:     "Cursor movement",
			input:    "Text\x1b[A\x1b[2KMore text",
			expected: "TextMore text",
		},
		{
			name:     "Text with multiple ANSI sequences",
			input:    "\x1b[0K\x1b[1mBold\x1b[0m \x1b[?2026ltext",
			expected: "Bold text",
		},
		{
			name:     "Invalid sequence without terminator",
			input:    "Text\x1b[more text",
			expected: "Textore text", // Remove ESC character when no valid terminator found
		},
		{
			name:     "Non-CSI escape sequence",
			input:    "Text\x1bMore",
			expected: "TextMore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.removeANSICodes(tt.input)
			if result != tt.expected {
				t.Errorf("removeANSICodes(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertANSIToFeishu(t *testing.T) {
	manager := NewClaudeCodePTYManager("", "test", func(_, _, _ string) error { return nil })

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple text with ANSI",
			input:    "\x1b[31mHello\x1b[0m",
			expected: "Hello",
		},
		{
			name:     "Text with bracketed mode",
			input:    "\x1b[?2026lHello World",
			expected: "Hello World",
		},
		{
			name:     "Empty after ANSI removal",
			input:    "\x1b[?2026l",
			expected: "",
		},
		{
			name:     "Claude Code prefix",
			input:    "\x1b[31mClaude Code:\x1b[0m Hello",
			expected: "Hello",
		},
		{
			name:     "Preserves markdown code blocks",
			input:    "\x1b[0m```go\nhello()\n```",
			expected: "```go\nhello()\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.convertANSIToFeishu(tt.input)
			if result != tt.expected {
				t.Errorf("convertANSIToFeishu(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
