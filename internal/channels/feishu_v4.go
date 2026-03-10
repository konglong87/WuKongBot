package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// convertToMarkdownContentV4 converts text to Feishu V4 post format with md tags only
// Uses pure Markdown format including Markdown tables
// Returns the postContent structure directly (no "post" wrapper)
// Smart format: uses block format for tables, single block for plain text
func (c *FeishuChannel) convertToMarkdownContentV4(text string) map[string]interface{} {
	log.Debug("[FEISHU] convertToMarkdownContentV4: Starting conversion")

	var content [][]map[string]interface{}

	// 检测是否包含表格
	if c.containsMarkdownTable(text) {
		log.Debug("[FEISHU] convertToMarkdownContentV4: Table detected, using block format")
		// 包含表格，使用现有的分块逻辑
		content = c.parseMixedContent(text)
	} else {
		log.Debug("[FEISHU] convertToMarkdownContentV4: No table, using single block")
		// 不包含表格，直接整体发送，不分块
		content = [][]map[string]interface{}{
			{{
				"tag":  "md",
				"text": text,
			}},
		}
	}

	// Build postContent structure directly (no "post" wrapper)
	postContent := map[string]interface{}{
		"zh_cn": map[string]interface{}{
			"title":   "", // Optional title
			"content": content,
		},
	}

	log.Debug("[FEISHU] convertToMarkdownContentV4: Built V4 structure with md tags")
	return postContent
}

// containsMarkdownTable checks if the text contains Markdown table format
// Markdown table format requires at least 2 lines starting and ending with |
func (c *FeishuChannel) containsMarkdownTable(text string) bool {
	lines := strings.Split(text, "\n")
	pipeLineCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 检测以 | 开头和结尾的行（标准 Markdown 表格格式）
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			pipeLineCount++
		}
	}

	// 至少2行才算表格（表头 + 分隔线或数据行）
	return pipeLineCount >= 2
}

// parseMixedContent parses text and returns content with md tags only
// Feishu API does not support table tag, so we use pure Markdown format
// Splits by blank lines (\n\n) to preserve paragraph structure with internal newlines
func (c *FeishuChannel) parseMixedContent(text string) [][]map[string]interface{} {
	log.Debug("[FEISHU] parseMixedContent: Parsing content")

	// Split text into blocks by blank lines
	blocks := strings.Split(text, "\n\n")
	var content [][]map[string]interface{}

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		// Convert single newline to double newline for better rendering
		block = strings.ReplaceAll(block, "\n", "\n\n")

		// Each block uses md tag (Markdown format)
		// This includes tables, which will be rendered as Markdown tables
		content = append(content, []map[string]interface{}{
			{
				"tag":  "md",
				"text": block,
			},
			{
				"tag": "hr",
			},
		})
	}

	log.Debug("[FEISHU] parseMixedContent: Parsed", "blocks", len(content))
	return content
}

// sendTextV4 sends a message using Feishu V4 format with mixed md and table tags
func (c *FeishuChannel) sendTextV4(ctx context.Context, msg OutboundMessage) error {
	log.Info("[FEISHU] sendTextV4: Sending message with mixed tags")

	// Build post content using mixed md and table tags
	postContent := c.convertToMarkdownContentV4(msg.Content)
	postContentWithWrapper := postContent

	// Serialize to JSON (same as project's other places)
	contentJSON, err := json.Marshal(postContentWithWrapper)
	if err != nil {
		log.Error("[FEISHU] sendTextV4: Failed to marshal content", "error", err)
		return err
	}

	log.Debug("[FEISHU] sendTextV4: Content JSON", "content", string(contentJSON))

	// Create message request with post type
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeOpenId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(msg.RecipientID).
			MsgType(larkim.MsgTypePost).
			Content(string(contentJSON)).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		log.Error("[FEISHU] sendTextV4: API call failed", "error", err)
		return err
	}

	if !resp.Success() {
		log.Error("[FEISHU] sendTextV4: API returned error", "msg", resp.Msg, "resp", resp)
		return fmt.Errorf("feishu API error: %s", resp.Msg)
	}

	log.Info("[FEISHU] sendTextV4: Message sent successfully", "message_id", resp.Data.MessageId)
	return nil
}

// sendTextV4WithTitle sends a markdown message with an optional title
func (c *FeishuChannel) sendTextV4WithTitle(ctx context.Context, title, text string, recipientID string) error {
	log.Info("[FEISHU] sendTextV4WithTitle: Sending message with title")

	// Build post content using smart format detection
	postContent := c.convertToMarkdownContentV4(text)

	// Set the title
	if zhCn, ok := postContent["zh_cn"].(map[string]interface{}); ok {
		zhCn["title"] = title
	}

	// Serialize to JSON (same as project's other places)
	contentJSON, err := json.Marshal(postContent)
	if err != nil {
		log.Error("[FEISHU] sendTextV4WithTitle: Failed to marshal content", "error", err)
		return err
	}

	log.Debug("[FEISHU] sendTextV4WithTitle: Content JSON", "content", string(contentJSON))

	// Create message request with post type
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeOpenId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(recipientID).
			MsgType(larkim.MsgTypePost).
			Content(string(contentJSON)).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		log.Error("[FEISHU] sendTextV4WithTitle: API call failed", "error", err)
		return err
	}

	if !resp.Success() {
		log.Error("[FEISHU] sendTextV4WithTitle: API returned error", "msg", resp.Msg, "resp", resp)
		return fmt.Errorf("feishu API error: %s", resp.Msg)
	}

	log.Info("[FEISHU] sendTextV4WithTitle: Message sent successfully", "message_id", resp.Data.MessageId)
	return nil
}
