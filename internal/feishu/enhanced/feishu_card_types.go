package enhanced

import "encoding/json"

// CardType represents the type of interactive card
type CardType string

const (
	CardTypeSingleChoice   CardType = "single_choice"   // 单选按钮
	CardTypeMultipleChoice CardType = "multiple_choice" // 多选按钮
	CardTypeChecklist      CardType = "checklist"       // 复选框
	CardTypeConfirm        CardType = "confirm"         // 确认对话框
)

// AlertLevel represents the alert/warning level for confirm cards
type AlertLevel string

const (
	AlertLevelInfo    AlertLevel = "info"
	AlertLevelWarning AlertLevel = "warning"
	AlertLevelDanger  AlertLevel = "danger"
)

// InteractiveCard represents an interactive card for Feishu
type InteractiveCard struct {
	Type      CardType      `json:"type"`
	Title     string        `json:"title"`
	Question  string        `json:"question"`
	Options   []CardOption  `json:"options,omitempty"`
	Message   string        `json:"message,omitempty"`
	WarnLevel AlertLevel    `json:"warn_level,omitempty"`
	SessionID string        `json:"session_id"`
}

// CardOption represents an option in the interactive card
type CardOption struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Value   string `json:"value"`
	Icon    string `json:"icon,omitempty"`
	Default bool   `json:"default"`
}

// FeishuCard represents the complete Feishu card structure
// Based on Feishu Card API specification
type FeishuCard struct {
	Config map[string]interface{} `json:"config"`
	Header FeishuCardHeader       `json:"header"`
	Elements []interface{}        `json:"elements"`
}

// FeishuCardHeader represents the card header
type FeishuCardHeader struct {
	Title FeishuCardContent `json:"title"`
}

// FeishuCardContent represents content in the card
type FeishuCardContent struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

// FeishuCardButton represents a button element
type FeishuCardButton struct {
	Tag      string                      `json:"tag"`
	Text     FeishuCardContent          `json:"text"`
	Type     string                      `json:"type"`
	Value    map[string]interface{}      `json:"value"`
}

// FeishuCardCheckbox represents a checkbox element
type FeishuCardCheckbox struct {
	Tag   string                      `json:"tag"`
	Text  FeishuCardContent          `json:"text"`
	Value map[string]interface{}      `json:"value"`
}

// FeishuCardDiv represents a div element for text
type FeishuCardDiv struct {
	Tag  string              `json:"tag"`
	Text FeishuCardContent  `json:"text"`
}

// FeishuCardAction represents an action module
type FeishuCardAction struct {
	Tag     string       `json:"tag"`
	Actions []interface{} `json:"actions"`
}

// ToJSON converts the card to JSON for API call
func (c *FeishuCard) ToJSON() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FeishuCardCallback represents a callback from Feishu card interaction
type FeishuCardCallback struct {
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id"`
	UserID    string                 `json:"user_id"`
	Values    map[string]interface{} `json:"values"`
	Timestamp int64                  `json:"timestamp"`
}

// FeishuElement represents a generic card element
type FeishuElement interface {
	IsValid() bool
}

// IsValid validates if the element is properly configured
func (b *FeishuCardButton) IsValid() bool {
	return b.Tag == "button" && b.Text.Tag == "plain_text" && b.Text.Content != ""
}

// IsValid validates if the element is properly configured
func (c *FeishuCardCheckbox) IsValid() bool {
	return c.Tag == "checkbox" && c.Text.Tag == "plain_text" && c.Text.Content != ""
}

// IsValid validates if the element is properly configured
func (d *FeishuCardDiv) IsValid() bool {
	return d.Tag == "div" && (d.Text.Tag == "lark_md" || d.Text.Tag == "plain_text")
}
