package enhanced

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// CardGenerator generates Feishu interactive cards
type CardGenerator struct {
	theme string // "default", "dark", "colorful"
}

// NewCardGenerator creates a new card generator
func NewCardGenerator() *CardGenerator {
	return &CardGenerator{
		theme: "default",
	}
}

// SetTheme sets the card theme
func (g *CardGenerator) SetTheme(theme string) {
	g.theme = theme
}

// GenerateFromLLMResponse generates a card from LLM response
func (g *CardGenerator) GenerateFromLLMResponse(response string, sessionID string) (*FeishuCard, error) {
	detector := NewQuestionDetector()
	_, interactiveCard := detector.DetectQuestion(response)

	if interactiveCard == nil || interactiveCard.Type == "" {
		return nil, fmt.Errorf("no interactive card needed for this response")
	}

	interactiveCard.SessionID = sessionID

	switch interactiveCard.Type {
	case CardTypeConfirm:
		return g.CreateConfirmCard(interactiveCard.Question, interactiveCard.WarnLevel)
	case CardTypeSingleChoice:
		return g.CreateSingleChoiceCard(interactiveCard.Question, interactiveCard.Options)
	case CardTypeMultipleChoice:
		return g.CreateMultipleChoiceCard(interactiveCard.Question, interactiveCard.Options)
	case CardTypeChecklist:
		return g.CreateChecklistCard(interactiveCard.Question, interactiveCard.Options)
	default:
		return nil, fmt.Errorf("unsupported card type: %s", interactiveCard.Type)
	}
}

// CreateConfirmCard creates a confirmation dialog card
func (g *CardGenerator) CreateConfirmCard(message string, warnLevel AlertLevel) (*FeishuCard, error) {
	// Determine button style based on warn level
	buttonType := "primary"
	if warnLevel == AlertLevelDanger {
		buttonType = "danger"
	}

	// Create buttons
	buttons := []map[string]interface{}{
		{
			"tag": "button",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "确认",
			},
			"type": buttonType,
			"value": map[string]interface{}{
				"type":  "confirm",
				"value": "yes",
			},
		},
		{
			"tag": "button",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "取消",
			},
			"type": "default",
			"value": map[string]interface{}{
				"type":  "confirm",
				"value": "no",
			},
		},
	}

	// Determine title based on warn level
	title := "请确认"
	switch warnLevel {
	case AlertLevelWarning:
		title = "⚠️ 警告"
	case AlertLevelDanger:
		title = "🚨 注意"
	}

	return &FeishuCard{
		Config: map[string]interface{}{
			"wide_screen_mode": true,
		},
		Header: FeishuCardHeader{
			Title: FeishuCardContent{
				Tag:     "plain_text",
				Content: title,
			},
		},
		Elements: []interface{}{
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":      "lark_md",
					"content": message,
				},
			},
			map[string]interface{}{
				"tag":     "action",
				"actions": buttons,
			},
		},
	}, nil
}

// CreateSingleChoiceCard creates a single choice card
func (g *CardGenerator) CreateSingleChoiceCard(question string, options []CardOption) (*FeishuCard, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("no options provided for single choice card")
	}

	// Create buttons for each option
	buttons := make([]map[string]interface{}, len(options))
	for i, opt := range options {
		buttonType := "default"
		if i == 0 {
			buttonType = "primary"
		}

		label := opt.Label
		if opt.Icon != "" {
			label = fmt.Sprintf("%s %s", opt.Icon, label)
		}

		buttons[i] = map[string]interface{}{
			"tag": "button",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": label,
			},
			"type": buttonType,
			"value": map[string]interface{}{
				"type":  "single_choice",
				"value": opt.Value,
			},
		}
	}

	return &FeishuCard{
		Config: map[string]interface{}{
			"wide_screen_mode": true,
		},
		Header: FeishuCardHeader{
			Title: FeishuCardContent{
				Tag:     "plain_text",
				Content: "请选择",
			},
		},
		Elements: []interface{}{
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":      "lark_md",
					"content": question,
				},
			},
			map[string]interface{}{
				"tag":     "action",
				"actions": buttons,
			},
		},
	}, nil
}

// CreateMultipleChoiceCard creates a multiple choice card
func (g *CardGenerator) CreateMultipleChoiceCard(question string, options []CardOption) (*FeishuCard, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("no options provided for multiple choice card")
	}

	// Select buttons for multiple choice
	selectButtons := make([]map[string]interface{}, len(options))
	for i, opt := range options {
		label := opt.Label
		if opt.Icon != "" {
			label = fmt.Sprintf("%s %s", opt.Icon, label)
		}

		selectButtons[i] = map[string]interface{}{
			"tag": "button",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": label,
			},
			"type": "default",
			"value": map[string]interface{}{
				"type":  "multiple_choice",
				"value": opt.Value,
			},
		}
	}

	// Add submit and reset buttons
	submitResetButtons := []map[string]interface{}{
		{
			"tag": "button",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "提交",
			},
			"type": "primary",
			"value": map[string]interface{}{
				"type":  "action",
				"value": "submit",
			},
		},
		{
			"tag": "button",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "重置",
			},
			"type": "default",
			"value": map[string]interface{}{
				"type":  "action",
				"value": "reset",
			},
		},
	}

	return &FeishuCard{
		Config: map[string]interface{}{
			"wide_screen_mode": true,
		},
		Header: FeishuCardHeader{
			Title: FeishuCardContent{
				Tag:     "plain_text",
				Content: "请选择(可多选)",
			},
		},
		Elements: []interface{}{
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":      "lark_md",
					"content": question,
				},
			},
			map[string]interface{}{
				"tag":     "action",
				"actions": selectButtons,
			},
			map[string]interface{}{
				"tag":     "action",
				"actions": submitResetButtons,
			},
		},
	}, nil
}

// CreateChecklistCard creates a checklist card
func (g *CardGenerator) CreateChecklistCard(question string, items []CardOption) (*FeishuCard, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items provided for checklist card")
	}

	// Create checkboxes
	checkboxes := make([]map[string]interface{}, len(items))
	for i, item := range items {
		checkboxID := item.ID
		if checkboxID == "" {
			checkboxID = g.generateID()
		}

		checkboxes[i] = map[string]interface{}{
			"tag": "checkbox",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": item.Label,
			},
			"value": map[string]interface{}{
				"type":  "checklist",
				"value": item.Value,
				"id":    checkboxID,
			},
		}
	}

	// Add submit button
	submitButton := []map[string]interface{}{
		{
			"tag": "button",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "确定",
			},
			"type": "primary",
			"value": map[string]interface{}{
				"type":  "action",
				"value": "submit",
			},
		},
	}

	return &FeishuCard{
		Config: map[string]interface{}{
			"wide_screen_mode": true,
		},
		Header: FeishuCardHeader{
			Title: FeishuCardContent{
				Tag:     "plain_text",
				Content: "请选择项",
			},
		},
		Elements: append([]interface{}{
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":      "lark_md",
					"content": question,
				},
			},
		}, append(convertToInterfaceSlice(checkboxes), map[string]interface{}{
			"tag":     "action",
			"actions": submitButton,
		})...),
	}, nil
}

// CreateProgressCard creates a progress notification card
func (g *CardGenerator) CreateProgressCard(title, message string, progress int) (*FeishuCard, error) {
	// Progress bar (using text characters)
	progressBar := g.generateProgressBar(progress)

	return &FeishuCard{
		Config: map[string]interface{}{
			"wide_screen_mode": true,
		},
		Header: FeishuCardHeader{
			Title: FeishuCardContent{
				Tag:     "plain_text",
				Content: title,
			},
		},
		Elements: []interface{}{
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":      "lark_md",
					"content": fmt.Sprintf("**进度**: %d%%", progress),
				},
			},
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":      "lark_md",
					"content": progressBar,
				},
			},
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":      "lark_md",
					"content": message,
				},
			},
		},
	}, nil
}

// generateProgressBar generates a text-based progress bar
func (g *CardGenerator) generateProgressBar(progress int) string {
	if progress < 0 {
		progress = 0
	} else if progress > 100 {
		progress = 100
	}

	barWidth := 20
	filled := int(float64(progress) / 100 * float64(barWidth))
	empty := barWidth - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("[%s]", bar)
}

// generateID generates a unique ID
func (g *CardGenerator) generateID() string {
	rand.Seed(time.Now().UnixNano())
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// convertToInterfaceSlice converts a slice to []interface{}
func convertToInterfaceSlice(slice []map[string]interface{}) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}
