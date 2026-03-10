package toolcontext

import (
	"time"
)

// ToolContext provides execution context for tool hooks
type ToolContext struct {
	UserID      string
	ChannelID   string
	SessionID   string
	ToolName    string
	ToolCallID  string
	Params      map[string]interface{}
	CardSession *CardSession

	// Dependencies
	Bus     MessageSender
	Adapter CardAdapter
	Storage CardStorage

	// Execution tracking
	StartTime time.Time
	Metadata  map[string]string
}

// CardAdapter interface for sending cards to channels
type CardAdapter interface {
	SendCard(userID, channelID string, card *CardContent) error
	UpdateCard(userID, channelID, sessionID string, card *CardContent) error
	DeleteCard(userID, channelID, sessionID string) error
}

// CardStorage interface for card session persistence
type CardStorage interface {
	SaveCardSession(session *CardSession) error
	GetCardSession(id string) (*CardSession, error)
	GetActiveCardSession(userID, channelID, toolName, toolCallID string) (*CardSession, error)
	UpdateCardSession(session *CardSession) error
	DeleteCardSession(id string) error
	ExpireCardSessions(expiresBefore time.Time) error
}

// MessageSender interface for sending messages
type MessageSender interface {
	SendOutbound(channelID, senderID, content string) error
}

// ToolAction represents the action to take after a tool hook
type ToolAction int

const (
	// ActionContinue continues with tool execution
	ActionContinue ToolAction = iota
	// ActionWaitCard waits for card response before continuing
	ActionWaitCard
	// ActionSkip skips tool execution
	ActionSkip
	// ActionCancel cancels tool execution
	ActionCancel
)

// ToolDecision represents a decision made by tool hooks
type ToolDecision struct {
	Action      ToolAction
	CardNeeded  bool
	CardType    CardType
	CardContent *CardContent
	SessionID   string
	Timeout     time.Duration
	UserAnswers []string
	AnsweredAt  time.Time
}

// CardType represents the type of interactive card
type CardType int

const (
	// CardTypeConfirm is a simple yes/no confirmation card
	CardTypeConfirm CardType = iota
	// CardTypeSingleChoice allows selecting one option
	CardTypeSingleChoice
	// CardTypeMultipleChoice allows selecting multiple options
	CardTypeMultipleChoice
	// CardTypeInput allows text input
	CardTypeInput
	// CardTypeProgress shows progress updates
	CardTypeProgress
	// CardTypeResult shows final results
	CardTypeResult
)

// CardContent represents the content of an interactive card
type CardContent struct {
	Title       string
	Description string
	Question    string
	Options     []*CardOption
	WarnLevel   string // "low", "medium", "high"
	Extra       map[string]interface{}
}

// CardOption represents an option in a card
type CardOption struct {
	Label       string
	Value       string
	Description string
	Selected    bool
}

// CardSession represents an active interactive card session
type CardSession struct {
	ID         string
	UserID     string
	ChannelID  string
	ToolName   string
	ToolCallID string
	State      CardSessionState

	// Timestamps
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ExpiresAt  time.Time
	AnsweredAt time.Time

	// Card content
	CardType    CardType
	CardContent *CardContent
	UserAnswers []string

	// Tool parameters
	ToolParams map[string]interface{}
	Metadata   map[string]string
}

// CardSessionState represents the state of a card session
type CardSessionState int

const (
	// CardStatePending card is waiting for user response
	CardStatePending CardSessionState = iota
	// CardStateAnswered card has been answered by user
	CardStateAnswered
	// CardStateExpired card has expired without response
	CardStateExpired
	// CardStateCancelled card was cancelled
	CardStateCancelled
	// CardStateCompleted card session completed successfully
	CardStateCompleted
)

// IsExpired checks if the card session has expired
func (s *CardSession) IsExpired() bool {
	if s.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(s.ExpiresAt)
}

// SetAnswer sets the user's answer to the card
func (s *CardSession) SetAnswer(answers []string) {
	s.UserAnswers = answers
	s.State = CardStateAnswered
	s.AnsweredAt = time.Now()
	s.UpdatedAt = time.Now()
}
