// Package enhanced provides enhanced parsing capabilities for Feishu/Lark messages.
package enhanced

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// InteractiveQuestion 交互式问题 (本地定义，避免循环导入)
type InteractiveQuestion struct {
	Type      string                 // select, input, confirm
	Prompt    string                 // 问题提示
	Options   []string               // 选项（仅选择题）
	SessionID string                 // 关联的会话 ID
	CreatedAt time.Time              // 创建时间
	Metadata  map[string]interface{} // 额外元数据
}

// ClaudeInteractiveParser 解析 Claude Code 的交互式输出
type ClaudeInteractiveParser struct {
	patterns map[string]*regexp.Regexp
	logger   *log.Logger
}

// NewClaudeInteractiveParser 创建交互式解析器
func NewClaudeInteractiveParser() *ClaudeInteractiveParser {
	p := &ClaudeInteractiveParser{
		patterns: make(map[string]*regexp.Regexp),
		logger:   log.Default(),
	}

	// 初始化正则模式
	p.initPatterns()

	return p
}

// initPatterns 初始化检测模式
func (p *ClaudeInteractiveParser) initPatterns() {
	// 选择题模式: ? Choose [X] option
	p.patterns["select"] = regexp.MustCompile(`\?[\s]*Choose[\s]*(.+)[:\n](.+)`)

	// 输入模式: ? Enter your
	p.patterns["input"] = regexp.MustCompile(`\?[\s]*Enter[\s]*(.+)[:\s]*`)

	// 确认模式: ? Do you want to
	p.patterns["confirm"] = regexp.MustCompile(`\?[\s]*Do you want[\s]*(.+)[:\s]*\(y/n\)`)

	// 选项匹配模式: [ ] option 或 [X] option
	p.patterns["option"] = regexp.MustCompile(`\[\s*\w?\s*\]\s*(.+)`)
}

// ParseInteractiveQuestion 解析 Claude Code 输出中的交互式问题，
func (p *ClaudeInteractiveParser) ParseInteractiveQuestion(output, sessionID string) (*InteractiveQuestion, bool) {
	// 先检测选择问题（需要处理多行）
	// 检查是否包含 "?"、"Choose" 和 "["（有选项）
	if strings.Contains(output, "?") && strings.Contains(output, "Choose") && strings.Contains(output, "[") {
		if question, ok := p.parseSelectQuestion(output, sessionID); ok {
			return question, true
		}
	}

	// 逐行检测输入和确认问题
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// 检测输入问题: ? Enter your project name:
		if strings.Contains(line, "?") && strings.Contains(line, "Enter") {
			if question, ok := p.parseInputQuestion(line, sessionID); ok {
				return question, true
			}
		}
		// 检测确认问题: ? Do you want to continue? (y/n)
		if strings.Contains(line, "?") && strings.Contains(line, "Do you want") && strings.Contains(line, "(y/n)") {
			if question, ok := p.parseConfirmQuestion(line, sessionID); ok {
				return question, true
			}
		}
	}

	return nil, false
}

// parseSelectQuestion 解析选择题
func (p *ClaudeInteractiveParser) parseSelectQuestion(line, sessionID string) (*InteractiveQuestion, bool) {
	matches := p.patterns["select"].FindStringSubmatch(line)
	if len(matches) < 3 {
		return nil, false
	}

	// matches[1] 捕获从 "Choose" 之后到冒号或换行之前的内容
	prompt := strings.TrimSpace(matches[1])
	// 移除末尾的冒号（如果存在）
	prompt = strings.TrimSuffix(prompt, ":")

	// 由于 spec 使用 [:\n]，第二组可能只捕获第一行
	// 需要从原始行中提取匹配位置之后的所有内容
	matchIndex := p.patterns["select"].FindStringSubmatchIndex(line)
	if matchIndex == nil || len(matchIndex) < 4 {
		return nil, false
	}
	// Get the starting position of the second capture group
	optionsStart := matchIndex[4]
	optionsText := line[optionsStart:]

	// 解析选项
	optionsLines := strings.Split(optionsText, "\n")
	var options []string

	for _, optLine := range optionsLines {
		// 匹配 [ ] option 或 [X] option
		optMatches := p.patterns["option"].FindStringSubmatch(optLine)
		if len(optMatches) >= 2 {
			options = append(options, strings.TrimSpace(optMatches[1]))
		}
	}

	if len(options) == 0 {
		return nil, false
	}

	return &InteractiveQuestion{
		Type:      "select",
		Prompt:    prompt,
		Options:   options,
		SessionID: sessionID,
		CreatedAt: time.Now(),
	}, true
}

// parseInputQuestion 解析输入题
func (p *ClaudeInteractiveParser) parseInputQuestion(line, sessionID string) (*InteractiveQuestion, bool) {
	matches := p.patterns["input"].FindStringSubmatch(line)
	if len(matches) < 2 {
		return nil, false
	}

	// matches[1] 捕获从 "Enter" 之后到冒号之前的内容
	prompt := strings.TrimSpace(matches[1])
	// 移除末尾的冒号（如果存在）
	prompt = strings.TrimSuffix(prompt, ":")

	return &InteractiveQuestion{
		Type:      "input",
		Prompt:    prompt,
		SessionID: sessionID,
		CreatedAt: time.Now(),
	}, true
}

// parseConfirmQuestion 解析确认题
func (p *ClaudeInteractiveParser) parseConfirmQuestion(line, sessionID string) (*InteractiveQuestion, bool) {
	matches := p.patterns["confirm"].FindStringSubmatch(line)
	if len(matches) < 2 {
		return nil, false
	}

	// matches[1] 捕获从 "Do you want" 之后到冒号之前的内容
	prompt := strings.TrimSpace(matches[1])

	return &InteractiveQuestion{
		Type:      "confirm",
		Prompt:    prompt,
		Options:   []string{"yes", "no"},
		SessionID: sessionID,
		CreatedAt: time.Now(),
	}, true
}

// FormatAnswer 格式化用户答案为 Claude Code 可识别格式
func (p *ClaudeInteractiveParser) FormatAnswer(question *InteractiveQuestion, answer interface{}) string {
	// Nil check to prevent panic
	if question == nil {
		p.logger.Error("cannot format answer: question is nil")
		return ""
	}

	switch question.Type {
	case "select":
		if index, ok := answer.(int); ok && index >= 0 && index < len(question.Options) {
			// 返回选项对应的数字
			return fmt.Sprintf("%d", index+1)
		}
		p.logger.Warn("invalid select answer: not a valid index", "type", fmt.Sprintf("%T", answer))
		return ""

	case "input":
		if text, ok := answer.(string); ok {
			return text
		}
		p.logger.Warn("invalid input answer: not a string", "type", fmt.Sprintf("%T", answer))
		return ""

	case "confirm":
		if text, ok := answer.(string); ok {
			if strings.ToLower(text) == "yes" {
				return "y"
			} else if strings.ToLower(text) == "no" {
				return "n"
			}
		}
		p.logger.Warn("invalid confirm answer: not yes/no, defaulting to no", "answer", answer)
		return "n" // 默认否

	default:
		p.logger.Warn("unknown question type", "type", question.Type)
		return ""
	}
}

// DetectQuestionType 检测输出是否包含交互式问题
func (p *ClaudeInteractiveParser) DetectQuestionType(output string) (string, bool) {
	if p.patterns["select"].MatchString(output) {
		return "select", true
	}
	if p.patterns["input"].MatchString(output) {
		return "input", true
	}
	if p.patterns["confirm"].MatchString(output) {
		return "confirm", true
	}
	return "", false
}
