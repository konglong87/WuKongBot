package enhanced

import (
	"testing"
)

func TestQuestionDetector_DetectQuestion(t *testing.T) {
	detector := NewQuestionDetector()

	tests := []struct {
		name      string
		input     string
		wantType  QuestionType
		wantCard  bool
	}{
		{
			name:     "Yes/No question - Chinese",
			input:    "你是否要执行此操作？",
			wantType: QuestionYesNo,
			wantCard: true,
		},
		{
			name:     "Yes/No question - English",
			input:    "Do you want to proceed?",
			wantType: QuestionYesNo,
			wantCard: true,
		},
		{
			name:     "Single choice question",
			input:    "请选择一种认证方式：1. JWT 2. Session 3. OAuth",
			wantType: QuestionSingleChoice,
			wantCard: true,
		},
		{
			name:     "Multiple choice question",
			input:    "可以多选以下功能：• 登录 • 注册 • 忘记密码",
			wantType: QuestionMultipleChoice,
			wantCard: true,
		},
		{
			name:     "Checklist question",
			input:    "需要包含哪些功能？",
			wantType: QuestionOpenEnded,
			wantCard: true,
		},
		{
			name:     "Open-ended statement",
			input:    "这是一个普通的陈述句，不需要交互卡。",
			wantType: QuestionOpenEnded,
			wantCard: false,
		},
		{
			name:     "No question mark but has question patterns",
			input:    "请确认你想要执行",
			wantType: QuestionYesNo,
			wantCard: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qType, card := detector.DetectQuestion(tt.input)

			if qType != tt.wantType {
				t.Errorf("DetectQuestion() type = %v, want %v", qType, tt.wantType)
			}

			if (card != nil) != tt.wantCard {
				t.Errorf("DetectQuestion() card = %v, wantCard %v", card, tt.wantCard)
			}

			if tt.wantCard && card != nil {
				if card.Question == "" {
					t.Errorf("DetectQuestion() card.Question empty, want non-empty")
				}
			}
		})
	}
}

func TestQuestionDetector_ContainsQuestion(t *testing.T) {
	detector := NewQuestionDetector()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "Question with ?",
			input: "Do you want to proceed?",
			want:  true,
		},
		{
			name:  "Question with ？",
			input: "你是否要执行？",
			want:  true,
		},
		{
			name:  "Statement with no question mark",
			input: "This is a statement.",
			want:  false,
		},
		{
			name:  "Question pattern with confirm",
			input: "请确认执行",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detector.ContainsQuestion(tt.input); got != tt.want {
				t.Errorf("ContainsQuestion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuestionDetector_ExtractSingleChoiceOptions(t *testing.T) {
	detector := NewQuestionDetector()

	// Test basic functionality with newline-separated options
	input := "1. Option A\n2. Option B\n3. Option C"
	options := detector.extractSingleChoiceOptions(input)

	if len(options) != 3 {
		t.Errorf("extractSingleChoiceOptions() count = %d, want 3", len(options))
	}
}

func TestQuestionDetector_IsConfirmQuestion(t *testing.T) {
	detector := NewQuestionDetector()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "Yes/No question",
			input: "Do you want to continue?",
			want:  true,
		},
		{
			name:  "Chinese confirm",
			input: "确定要删除吗？",
			want:  true,
		},
		{
			name:  "Not confirm - single choice",
			input: "Select one: A or B",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detector.IsConfirmQuestion(tt.input); got != tt.want {
				t.Errorf("IsConfirmQuestion() = %v, want %v", got, tt.want)
			}
		})
	}
}
