package enhanced

import (
	"regexp"
	"strings"
)

// QuestionType represents the type of question
type QuestionType string

const (
	QuestionYesNo        QuestionType = "yes_no"
	QuestionSingleChoice QuestionType = "single_choice"
	QuestionMultipleChoice QuestionType = "multiple_choice"
	QuestionCheckList    QuestionType = "checklist"
	QuestionOpenEnded    QuestionType = "open_ended"
)

// QuestionDetector detects questions in LLM responses
type QuestionDetector struct {
	patterns map[QuestionType][]string
}

// NewQuestionDetector creates a new question detector
func NewQuestionDetector() *QuestionDetector {
	return &QuestionDetector{
		patterns: map[QuestionType][]string{
			QuestionYesNo: {
				`是否`,
				`确定要`,
				`确认`,
				`Do you want`,
				`Are you sure`,
				`Confirm`,
				`Would you like`,
				`Should I`,
			},
			QuestionSingleChoice: {
				`请选择`,
				`选择.*?种`,
				`Which.*?one`,
				`Choose.*?option`,
				`Select.*?from`,
			},
			QuestionMultipleChoice: {
				`多选`,
				`可以多选`,
				`Select multiple`,
				`Choose.*?options`,
				`Pick several`,
			},
			QuestionCheckList: {
				`需要.*?功能`,
				`包含.*?项`,
				`Which features`,
				`What items`,
				`需要包含`,
			},
		},
	}
}

// DetectQuestion detects the type of question in the text
// Returns the question type and the extracted structured card (if applicable)
func (d *QuestionDetector) DetectQuestion(text string) (QuestionType, *InteractiveCard) {
	// 1. Check if there's a question
	if !d.ContainsQuestion(text) {
		return QuestionOpenEnded, nil
	}

	// 2. Detect question type
	qType := d.DetectQuestionType(text)

	// 3. Parse options if possible
	options := d.ParseOptions(text, qType)

	// 4. Generate interactive card
	card := &InteractiveCard{
		Type:     d.MapToCardType(qType),
		Question: text,
		Options:  options,
	}

	return qType, card
}

// ContainsQuestion checks if the text contains any question patterns
func (d *QuestionDetector) ContainsQuestion(text string) bool {
	// Check for question marks
	if strings.Contains(text, "？") || strings.Contains(text, "?") {
		return true
	}

	// Check for question patterns
	for _, patterns := range d.patterns {
		for _, pattern := range patterns {
			if matched, _ := regexp.MatchString(pattern, text); matched {
				return true
			}
		}
	}

	return false
}

// DetectQuestionType determines the type of question
func (d *QuestionDetector) DetectQuestionType(text string) QuestionType {
	// Check patterns in order of specificity
	for qType, patterns := range d.patterns {
		for _, pattern := range patterns {
			if matched, _ := regexp.MatchString(pattern, text); matched {
				// Additional checks for specificity
				if qType == QuestionMultipleChoice || qType == QuestionCheckList {
					if d.HasMultipleOptions(text) {
						return qType
					}
				} else {
					return qType
				}
			}
		}
	}

	return QuestionOpenEnded
}

// ParseOptions extracts options from the text
func (d *QuestionDetector) ParseOptions(text string, qType QuestionType) []CardOption {
	var options []CardOption

	switch qType {
	case QuestionYesNo:
		options = []CardOption{
			{ID: "yes", Label: "是", Value: "yes", Icon: "✓"},
			{ID: "no", Label: "否", Value: "no", Icon: "✗"},
		}
	case QuestionSingleChoice:
		options = d.extractSingleChoiceOptions(text)
	case QuestionMultipleChoice:
		options = d.extractMultipleChoiceOptions(text)
	case QuestionCheckList:
		options = d.extractChecklistOptions(text)
	}

	return options
}

// MapToCardType maps question type to card type
func (d *QuestionDetector) MapToCardType(qType QuestionType) CardType {
	switch qType {
	case QuestionYesNo:
		return CardTypeConfirm
	case QuestionSingleChoice:
		return CardTypeSingleChoice
	case QuestionMultipleChoice:
		return CardTypeMultipleChoice
	case QuestionCheckList:
		return CardTypeChecklist
	default:
		return ""
	}
}

// HasMultipleOptions checks if the text suggests multiple options
func (d *QuestionDetector) HasMultipleOptions(text string) bool {
	// Check for numbered lists, bullet points, etc.
	numberedList := regexp.MustCompile(`\d+[.、].*`)
	bulletList := regexp.MustCompile(`[•●○▪]\s*`)

	return numberedList.MatchString(text) || bulletList.MatchString(text)
}

// extractSingleChoiceOptions extracts options for single choice questions
func (d *QuestionDetector) extractSingleChoiceOptions(text string) []CardOption {
	var options []CardOption

	// Try to match numbered list: 1. Option1, 2. Option2, etc.
	numberedRE := regexp.MustCompile(`(\d+)[.、]\s*(\S.*?)\s*(?:\n|$)`)
	matches := numberedRE.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			options = append(options, CardOption{
				ID:    match[1],
				Label: strings.TrimSpace(match[2]),
				Value: strings.TrimSpace(match[2]),
			})
		}
	}

	// Try to match bullet list: • Option1, • Option2, etc.
	if len(options) == 0 {
		bulletRE := regexp.MustCompile(`[•●○▪]\s*(\S.*?)\s*(?:\n|$)`)
		matches = bulletRE.FindAllStringSubmatch(text, -1)

		for i, match := range matches {
			if len(match) >= 2 {
				options = append(options, CardOption{
					ID:    string(rune('a' + i)),
					Label: strings.TrimSpace(match[1]),
					Value: strings.TrimSpace(match[1]),
				})
			}
		}
	}

	return options
}

// extractMultipleChoiceOptions extracts options for multiple choice questions
func (d *QuestionDetector) extractMultipleChoiceOptions(text string) []CardOption {
	options := d.extractSingleChoiceOptions(text)

	// For multiple choice, all options are valid
	return options
}

// extractChecklistOptions extracts options for checklist questions
func (d *QuestionDetector) extractChecklistOptions(text string) []CardOption {
	var options []CardOption

	// Try to match: [ ] Option1, [ ] Option2
	checklistRE := regexp.MustCompile(`\[\s*\]\s*(.+?)(?:\n|$)`)
	matches := checklistRE.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			options = append(options, CardOption{
				ID:      match[1],
				Label:   strings.TrimSpace(match[1]),
				Value:   strings.TrimSpace(match[1]),
				Default: false,
			})
		}
	}

	// Also try to match: ☐ Option1, ○ Option2
	if len(options) == 0 {
		checkboxRE := regexp.MustCompile(`[☐○]\s*(.+?)(?:\n|$)`)
		matches = checkboxRE.FindAllStringSubmatch(text, -1)

		for _, match := range matches {
			if len(match) >= 2 {
				options = append(options, CardOption{
					ID:      match[1],
					Label:   strings.TrimSpace(match[1]),
					Value:   strings.TrimSpace(match[1]),
					Default: false,
				})
			}
		}
	}

	return options
}

// IsConfirmQuestion checks if it's a confirmation question
func (d *QuestionDetector) IsConfirmQuestion(text string) bool {
	return d.DetectQuestionType(text) == QuestionYesNo
}

// IsMultiSelectQuestion checks if it's a multi-select question
func (d *QuestionDetector) IsMultiSelectQuestion(text string) bool {
	qType := d.DetectQuestionType(text)
	return qType == QuestionMultipleChoice || qType == QuestionCheckList
}
