package channels

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// FeishuChannel implements BaseChannel for Feishu/Lark
type FeishuChannel struct {
	name     string
	cfg      FeishuConfig
	client   *lark.Client
	wsClient *larkws.Client
	mu       sync.RWMutex
	running  bool
	messages chan Message
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	enhanced interface{} // FeishuAdapter if enabled (using interface to avoid import cycle)
}

// NewFeishuChannel creates a new Feishu channel
func NewFeishuChannel(cfg FeishuConfig) *FeishuChannel {
	client := lark.NewClient(cfg.AppID, cfg.AppSecret)
	return &FeishuChannel{
		name:     "feishu",
		cfg:      cfg,
		client:   client,
		messages: make(chan Message, 100),
	}
}

// Name returns the channel name
func (c *FeishuChannel) Name() string {
	return c.name
}

// Type returns the channel type
func (c *FeishuChannel) Type() string {
	return "feishu"
}

// Start initializes and starts the Feishu channel
func (c *FeishuChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.running = true

	// Default to websocket mode if not specified
	if c.cfg.ConnectionMode == "" {
		c.cfg.ConnectionMode = "websocket"
		log.Info("Feishu connection mode not specified, using default: websocket")
	}

	log.Info("[FEISHU] Starting Feishu channel", "app_id", c.cfg.AppID, "mode", c.cfg.ConnectionMode)

	// Start WebSocket connection if configured
	if c.cfg.ConnectionMode == "websocket" {
		c.wg.Add(1)
		go c.startWebSocket(c.ctx)
		log.Info("[FEISHU] WebSocket connection starting in background...")
	} else {
		log.Info("[FEISHU] Feishu channel started (Webhook mode)")
	}

	return nil
}

// Stop gracefully stops the Feishu channel
func (c *FeishuChannel) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	log.Info("[FEISHU] Stopping Feishu channel...")

	c.running = false
	c.cancel()

	// Create a timeout context for waiting the WebSocket to stop
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Use a separate goroutine to wait and report timeout
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("[FEISHU] WebSocket stopped successfully")
	case <-shutdownCtx.Done():
		log.Warn("[FEISHU] WebSocket shutdown timeout, continuing...")
	}

	// Mark wsClient as nil to prevent further use
	c.wsClient = nil

	close(c.messages)
	log.Info("[FEISHU] Feishu channel stopped")
	return nil
}

// Send sends a message through Feishu
func (c *FeishuChannel) Send(msg OutboundMessage) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.running {
		return fmt.Errorf("channel is not running")
	}

	log.Info("[FEISHU] Sending message",
		"recipient_id", msg.RecipientID,
		"content_length", len(msg.Content),
		"has_media", len(msg.Media) > 0)

	ctx := context.Background()

	// Handle messages with media (images)
	if len(msg.Media) > 0 {
		for _, media := range msg.Media {
			if media.Type == "image" && media.URL != "" {
				// Step 1: Upload image to Feishu to get image_key
				imageKey, err := c.uploadImage(ctx, media.URL)
				if err != nil {
					log.Warn("[FEISHU] Failed to upload image, falling back to text message", "error", err)
					// Fallback to text message with the image URL
					return c.sendText(ctx, OutboundMessage{
						RecipientID: msg.RecipientID,
						Content:     msg.Content + "\n\n图片链接: " + media.URL,
					})
				}

				// Step 2: Send image message with the image_key
				imageContent := map[string]string{"image_key": imageKey}
				contentJSON, _ := json.Marshal(imageContent)

				req := larkim.NewCreateMessageReqBuilder().
					ReceiveIdType("open_id").
					Body(larkim.NewCreateMessageReqBodyBuilder().
						ReceiveId(msg.RecipientID).
						MsgType("image").
						Content(string(contentJSON)).
						Build()).
					Build()

				resp, err := c.client.Im.Message.Create(ctx, req)
				if err != nil {
					log.Error("[FEISHU] Failed to send image message", "error", err)
					// Fallback to text message with the image URL
					return c.sendText(ctx, OutboundMessage{
						RecipientID: msg.RecipientID,
						Content:     msg.Content + "\n\n图片链接: " + media.URL,
					})
				}
				if !resp.Success() {
					log.Error("[FEISHU] Image API returned error", "msg", resp.Msg)
					// Fallback to text message with the image URL
					return c.sendText(ctx, OutboundMessage{
						RecipientID: msg.RecipientID,
						Content:     msg.Content + "\n\n图片链接: " + media.URL,
					})
				}

				log.Info("[FEISHU] Image message sent successfully", "message_id", resp.Data.MessageId)

				// If there's also text content, send it as a follow-up message
				if msg.Content != "" {
					_ = c.sendText(ctx, OutboundMessage{
						RecipientID: msg.RecipientID,
						Content:     msg.Content,
					})
				}

				return nil
			}
		}
		// If media couldn't be sent, fall back to text
		return c.sendText(ctx, msg)
	}

	return c.sendText(ctx, msg)
}

// sendText sends a text message using V4 markdown format
func (c *FeishuChannel) sendText(ctx context.Context, msg OutboundMessage) error {
	if msg.Content == "" || strings.TrimSpace(msg.Content) == "" {
		return nil
	}
	return c.sendTextV4(ctx, msg)
}

// sendTextV2 sends rich text post message using correct format
func (c *FeishuChannel) sendTextV2(ctx context.Context, msg OutboundMessage) error {
	log.Info("[FEISHU] Sending rich text post message V2")

	// Build post content using correct structure
	postContent := c.convertToPostContentV2(msg.Content)

	// The content should be the entire post object serialized
	contentJSON, _ := json.Marshal(postContent)
	log.Debug("[FEISHU] Post content JSON", "content", string(contentJSON))

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("open_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(msg.RecipientID).
			MsgType("post").
			Content(string(contentJSON)).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		log.Error("[FEISHU] API call failed", "error", err)
		return err
	}
	if !resp.Success() {
		log.Error("[发送消息报错][FEISHU] API returned error", "msg", resp.Msg, "resp", resp)
		// Fallback to plain text if post format fails
		log.Warn("[FEISHU] Falling back to plain text message")
		return c.sendPlainText(ctx, msg)
	}

	log.Info("[FEISHU] Message sent successfully", "message_id", resp.Data.MessageId)
	return nil
}

// convertToPostContentV2 converts text to Feishu post format
// Based on: https://open.feishu.cn/document/server-docs/im-v1/message/create
func (c *FeishuChannel) convertToPostContentV2(text string) map[string]interface{} {
	log.Debug("[FEISHU] Converting text to post format V2")

	// Parse bold formatting
	elements := c.parseBoldFormattingV2(text)

	// Build proper nested content structure: [[paragraph]]
	post := map[string]interface{}{
		"post": map[string]interface{}{
			"zh_cn": map[string]interface{}{
				"title":   "",
				"content": [][]map[string]interface{}{elements},
			},
		},
	}

	log.Debug("[FEISHU] Built post structure", "elements_count", len(elements))
	return post
}

// parseBoldFormattingV2 parses bold (**text**) markers
func (c *FeishuChannel) parseBoldFormattingV2(text string) []map[string]interface{} {
	elements := []map[string]interface{}{}
	remaining := text

	for len(remaining) > 0 {
		boldStart := strings.Index(remaining, "**")
		if boldStart >= 0 {
			boldEnd := strings.Index(remaining[boldStart+2:], "**")
			if boldEnd >= 0 {
				// Text before bold
				if boldStart > 0 {
					elements = append(elements, map[string]interface{}{
						"tag":  "text",
						"text": remaining[:boldStart],
					})
				}
				// Bold text with style as string array
				elements = append(elements, map[string]interface{}{
					"tag":   "text",
					"text":  remaining[boldStart+2 : boldStart+2+boldEnd],
					"style": []string{"bold"},
				})
				remaining = remaining[boldStart+2+boldEnd+2:]
				continue
			}
		}

		// Remaining text
		elements = append(elements, map[string]interface{}{
			"tag":  "text",
			"text": remaining,
		})
		break
	}

	return elements
}

// sendPlainText sends a plain text message (fallback)
func (c *FeishuChannel) sendPlainText(ctx context.Context, msg OutboundMessage) error {
	log.Info("[FEISHU] Sending plain text message (fallback)")

	contentJSON, _ := json.Marshal(map[string]string{"text": msg.Content})

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("open_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(msg.RecipientID).
			MsgType("text").
			Content(string(contentJSON)).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		log.Error("[FEISHU] Plain text API call failed", "error", err)
		return err
	}
	if !resp.Success() {
		log.Error("[FEISHU] Plain text API returned error", "msg", resp.Msg)
		return fmt.Errorf("feishu API error: %s", resp.Msg)
	}

	log.Info("[FEISHU] Plain text message sent successfully", "message_id", resp.Data.MessageId)
	return nil
}

// convertToPostContent converts text to Feishu post format
func (c *FeishuChannel) convertToPostContent(text string) map[string]interface{} {
	log.Debug("[FEISHU] Converting text to post format", "text_length", len(text))

	// Remove markdown syntax that's not supported in post messages
	//cleaned := c.cleanUnsupportedMarkdown(text)

	// Parse bold formatting
	//elements := c.parseBoldFormatting(text)
	element := map[string]interface{}{}
	element["tag"] = "md"
	element["text"] = text

	elements := []map[string]interface{}{}
	elements = append(elements, element)

	// DEBUG: Log elements before building final structure
	log.Debug("[FEISHU] Parsed elements count", "count", len(elements))

	// Build post with proper structure: content is array of arrays (paragraphs)
	post := map[string]interface{}{
		"post": map[string]interface{}{
			"zh_cn": map[string]interface{}{
				"title":   "",
				"content": [][]map[string]interface{}{elements},
			},
		},
	}

	log.Debug("[FEISHU] Final post structure", "paragraphs", 1, "total_elements", len(elements), "elements", elements)
	return post
}

// cleanUnsupportedMarkdown removes markdown syntax not supported in post messages
func (c *FeishuChannel) cleanUnsupportedMarkdown(text string) string {
	result := text
	// Remove markdown headings
	result = strings.ReplaceAll(result, "### ", "")
	result = strings.ReplaceAll(result, "## ", "")
	result = strings.ReplaceAll(result, "# ", "")
	// Remove blockquotes
	result = strings.ReplaceAll(result, "> ", "")
	result = strings.ReplaceAll(result, ">", "")
	// Remove list markers
	result = strings.ReplaceAll(result, "- ", "")
	result = strings.ReplaceAll(result, "* ", "")
	// Remove markdown emphasis
	result = strings.ReplaceAll(result, "*italic*", "")
	result = strings.ReplaceAll(result, "~underline~", "")
	// Keep bold markers for processing
	return result
}

// removeMarkdownSyntax removes markdown syntax from text
func (c *FeishuChannel) removeMarkdownSyntax(text string) string {
	// Replace headings with prefix
	result := text

	// Remove ### headings
	result = strings.ReplaceAll(result, "### ", "")
	// Remove ## headings
	result = strings.ReplaceAll(result, "## ", "")
	// Remove # headings
	result = strings.ReplaceAll(result, "# ", "")

	// Remove blockquote prefix
	result = strings.ReplaceAll(result, "> \n", "")
	result = strings.ReplaceAll(result, "> ", "")
	result = strings.ReplaceAll(result, ">", "")

	return result
}

// parseBoldFormatting parses bold (**text**) only
func (c *FeishuChannel) parseBoldFormatting(text string) []map[string]interface{} {
	elements := []map[string]interface{}{}
	remaining := text

	for len(remaining) > 0 {
		boldStart := strings.Index(remaining, "**")
		if boldStart >= 0 {
			boldEnd := strings.Index(remaining[boldStart+2:], "**")
			if boldEnd >= 0 {
				// Add text before bold
				if boldStart > 0 {
					elements = append(elements, map[string]interface{}{
						"tag":  "text",
						"text": remaining[:boldStart],
					})
				}
				// Add bold text
				elements = append(elements, map[string]interface{}{
					"tag":  "text",
					"text": remaining[boldStart+2 : boldStart+2+boldEnd],
				})
				remaining = remaining[boldStart+2+boldEnd+2:]
				continue
			}
		}

		// Add remaining text
		elements = append(elements, map[string]interface{}{
			"tag":  "text",
			"text": remaining,
		})
		break
	}

	return elements
}

// groupIntoParagraphs groups elements into paragraphs by double newlines
func (c *FeishuChannel) groupIntoParagraphs(elements []map[string]interface{}) [][]map[string]interface{} {
	paragraphs := [][]map[string]interface{}{}
	current := []map[string]interface{}{}

	for _, elem := range elements {
		text, ok := elem["text"].(string)
		if !ok {
			text = ""
		}

		// Check for double newline
		if strings.Contains(text, "\n\n") {
			// Split by double newline
			parts := strings.Split(text, "\n\n")
			for i, part := range parts {
				if i > 0 {
					// End of current paragraph
					if len(current) > 0 {
						paragraphs = append(paragraphs, current)
						current = []map[string]interface{}{}
					}
				}
				if part != "" {
					current = append(current, map[string]interface{}{
						"tag":  "text",
						"text": part,
					})
				}
			}
		} else if strings.Contains(text, "\\n") {
			// Escaped newline (from JSON), treat as real newline
			parts := strings.Split(text, "\\n")
			for i, part := range parts {
				if i > 0 {
					// Newline within same paragraph, add separator
					current = append(current, map[string]interface{}{
						"tag":  "text",
						"text": "\n",
					})
				}
				if part != "" {
					if elem["tag"] == "text" && elem["style"] != nil {
						// Preserve style
						current = append(current, map[string]interface{}{
							"tag":   "text",
							"text":  part,
							"style": elem["style"],
						})
					} else {
						current = append(current, map[string]interface{}{
							"tag":  "text",
							"text": part,
						})
					}
				}
			}
		} else {
			current = append(current, elem)
		}
	}

	if len(current) > 0 {
		paragraphs = append(paragraphs, current)
	}

	return paragraphs
}

// parseInlineFormattingInline parses inline markdown formatting (bold, code, links)
// Returns a flat array of elements
func (c *FeishuChannel) parseInlineFormattingInline(text string) []map[string]interface{} {
	elements := []map[string]interface{}{}
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		remaining := line
		lineElements := []map[string]interface{}{}

		for len(remaining) > 0 {
			// Check for bold **text**
			boldStart := strings.Index(remaining, "**")
			if boldStart >= 0 {
				boldEnd := strings.Index(remaining[boldStart+2:], "**")
				if boldEnd >= 0 {
					// Add text before bold
					if boldStart > 0 {
						lineElements = append(lineElements, map[string]interface{}{
							"tag":  "text",
							"text": remaining[:boldStart],
						})
					}
					// Add bold element with style as string array
					lineElements = append(lineElements, map[string]interface{}{
						"tag":   "text",
						"text":  remaining[boldStart+2 : boldStart+2+boldEnd],
						"style": []string{"bold"},
					})
					remaining = remaining[boldStart+2+boldEnd+2:]
					continue
				}
			}

			// Check for inline code `code`
			codeStart := strings.Index(remaining, "`")
			if codeStart >= 0 {
				codeEnd := strings.Index(remaining[codeStart+1:], "`")
				if codeEnd >= 0 {
					// Note: Feishu doesn't have inline_code tag in post format,
					// use text with style or ignore
					if codeStart > 0 {
						lineElements = append(lineElements, map[string]interface{}{
							"tag":  "text",
							"text": remaining[:codeStart],
						})
					}
					lineElements = append(lineElements, map[string]interface{}{
						"tag":  "text",
						"text": remaining[codeStart+1 : codeStart+1+codeEnd],
					})
					remaining = remaining[codeStart+codeEnd+2:]
					continue
				}
			}

			// No more formatting
			lineElements = append(lineElements, map[string]interface{}{
				"tag":  "text",
				"text": remaining,
			})
			break
		}

		elements = append(elements, lineElements...)

		// Add newline marker between lines (except last line)
		if line != lines[len(lines)-1] {
			elements = append(elements, map[string]interface{}{
				"tag": "newline",
			})
		}
	}

	return elements
}

// parseInlineFormatting is used for other purposes (deprecated)
func (c *FeishuChannel) parseInlineFormatting(text string) []map[string]interface{} {
	// This is deprecated but kept for compatibility
	return []map[string]interface{}{
		{"tag": "text", "text": text},
	}
}

// splitLines splits text into lines while preserving empty lines for paragraph separation
func splitLines(text string) []string {
	return strings.Split(text, "\n")
}

// uploadImage uploads an image to Feishu and returns the image_key
// The image is downloaded from the given URL and then uploaded to Feishu
func (c *FeishuChannel) uploadImage(ctx context.Context, imageURL string) (string, error) {
	log.Info("[FEISHU] Uploading image", "url", imageURL)

	// Step 1: Download the image from URL
	resp, err := http.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载图片失败，HTTP状态码: %d", resp.StatusCode)
	}

	// Read the image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取图片数据失败: %w", err)
	}

	// Check image size (Feishu limit: 10MB)
	const maxImageSize = 10 * 1024 * 1024 // 10MB
	if len(imageData) > maxImageSize {
		return "", fmt.Errorf("图片大小超过限制 (%dMB)", maxImageSize/1024/1024)
	}

	log.Debug("[FEISHU] Image downloaded", "size_bytes", len(imageData))

	// Step 2: Upload to Feishu Create
	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType("message").
			Image(bytes.NewReader(imageData)).
			Build()).
		Build()

	uploadResp, err := c.client.Im.Image.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("上传到飞书失败: %w", err)
	}

	if !uploadResp.Success() {
		return "", fmt.Errorf("飞书API返回错误: %s", uploadResp.Msg)
	}

	imageKey := uploadResp.Data.ImageKey
	if imageKey == nil {
		return "", fmt.Errorf("上传成功但未返回image_key")
	}
	log.Info("[FEISHU] Image uploaded successfully", "image_key", *imageKey)

	return *imageKey, nil
}

// getImageAsBase64 downloads an image from Feishu by message_id and image_key and returns it as base64
func (c *FeishuChannel) getImageAsBase64(ctx context.Context, messageID, imageKey string) (string, error) {
	log.Info("[FEISHU] Getting image as base64", "message_id", messageID, "image_key", imageKey)

	// Use the GetMessageResource API to download the image
	// API: GET /open-apis/im/v1/messages/{message_id}/resources/{file_key}?type=image
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(imageKey).
		Type("image").
		Build()

	resp, err := c.client.Im.MessageResource.Get(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get image resource: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("Feishu API returned error: %s", resp.Msg)
	}

	// Read the image data from the response (File field is io.Reader)
	imageData, err := io.ReadAll(resp.File)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}

	if len(imageData) == 0 {
		return "", fmt.Errorf("downloaded image is empty")
	}

	// Check image size limit (LLMs typically have size limits)
	const maxLlmImageSize = 5 * 1024 * 1024 // 5MB
	if len(imageData) > maxLlmImageSize {
		return "", fmt.Errorf("image too large for LLM processing (%dMB)", len(imageData)/1024/1024)
	}

	// Encode to base64
	base64String := base64.StdEncoding.EncodeToString(imageData)
	log.Info("[FEISHU] Image downloaded and encoded", "size_bytes", len(imageData), "base64_size", len(base64String))

	return base64String, nil
}

// getImageAsURL returns a Feishu API URL for accessing the image
// This URL can be used by LLM to retrieve the image with proper authentication
func (c *FeishuChannel) getImageAsURL(messageID, imageKey string) (string, error) {
	log.Info("[FEISHU] Getting image URL", "message_id", messageID, "image_key", imageKey)

	// Construct Feishu API URL for image resource access
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages/%s/resources/%s?type=image", messageID, imageKey)

	log.Info("[FEISHU] Image URL generated successfully", "url", url)
	return url, nil
}

// Receive returns a channel for incoming messages
func (c *FeishuChannel) Receive() <-chan Message {
	return c.messages
}

// IsRunning returns whether the channel is running
func (c *FeishuChannel) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// IsAllowed checks if a sender is authorized
func (c *FeishuChannel) IsAllowed(senderID string) bool {
	if len(c.cfg.AllowFrom) == 0 {
		return true
	}
	for _, id := range c.cfg.AllowFrom {
		if id == senderID {
			return true
		}
	}
	return false
}

// Handler returns the HTTP handler for Feishu webhook events
func (c *FeishuChannel) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.cfg.ConnectionMode == "websocket" {
			http.Error(w, "Webhook handler not available in WebSocket mode", http.StatusMethodNotAllowed)
			return
		}

		// Verify request method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read request body
		var reqBody FeishuWebhookRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			log.Error("Failed to decode feishu webhook request", "error", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Handle URL verification (challenge)
		if reqBody.Type == "url_verification" {
			if reqBody.Token != c.cfg.VerificationToken {
				log.Warn("Feishu verification token mismatch")
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"challenge": reqBody.Challenge,
			})
			return
		}

		// Handle events
		if reqBody.Type == "event_callback" {
			for _, event := range reqBody.Event {
				c.handleEvent(event)
			}
		}

		// Always return 200 OK
		w.WriteHeader(http.StatusOK)
	})
}

// startWebSocket establishes a WebSocket connection using lark SDK
func (c *FeishuChannel) startWebSocket(ctx context.Context) {
	defer c.wg.Done()

	log.Info("========================================")
	log.Info("Feishu WebSocket Process Starting")
	log.Info("========================================")
	log.Info("Step 1: Creating event dispatcher...")
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			// Log available event fields
			log.Info("[WS] 收到 Received P2MessageReceiveV1 event from SDK", "eventInfo", event.Event, "sender", event.Event.Sender, "message", event.Event.Message)
			c.handleP2MessageReceive(event)
			return nil
		})

	log.Info("Step 2: Creating larkws Client with app_id", c.cfg.AppID)

	// Create larkws client using SDK
	wsClient := larkws.NewClient(c.cfg.AppID, c.cfg.AppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelDebug),
		larkws.WithAutoReconnect(true),
	)

	c.wsClient = wsClient

	log.Info("Step 3: Starting larkws client...")
	log.Info("========================================")
	log.Info("Note: larkws client will handle all WebSocket connections automatically")
	log.Info("The following log types will be visible from lark SDK:")
	log.Info("  - DEBUG: Verbose connection details")
	log.Info("  - INFO: Connection status, login events")
	log.Info("  - Data messages (your chat messages)")
	log.Info("========================================")

	// Start larkws client (this is blocking until context canceled)
	err := wsClient.Start(ctx)
	if err != nil {
		log.Error("========================================")
		log.Error("[ERROR] larkws client failed to start", "error", err)
		log.Error("========================================")
		return
	}

	log.Info("Feishu WebSocket client stopped")
}

// handleP2MessageReceive handles P2 message receive events from WebSocket
func (c *FeishuChannel) handleP2MessageReceive(event *larkim.P2MessageReceiveV1) {
	if event.Event == nil || event.Event.Message == nil {
		return
	}

	// Dereference pointers - these are all pointer types in the SDK
	content := ""
	if event.Event.Message.Content != nil {
		content = *event.Event.Message.Content
	}

	var senderID string
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		senderID = *event.Event.Sender.SenderId.OpenId
	}

	msgType := ""
	if event.Event.Message.MessageType != nil {
		msgType = *event.Event.Message.MessageType
	}

	messageID := ""
	if event.Event.Message.MessageId != nil {
		messageID = *event.Event.Message.MessageId
	}

	log.Info("[WS] Processing P2 message", "sender", senderID, "msg_type", msgType, "content", content)

	// Handle image messages
	var msgMedia []Media
	if msgType == "image" {
		var imageContent struct {
			ImageKey string `json:"image_key"`
		}
		if err := json.Unmarshal([]byte(content), &imageContent); err == nil && imageContent.ImageKey != "" {
			log.Info("[WS] Detected image message", "image_key", imageContent.ImageKey, "message_id", messageID)
			// Get image base64 data directly
			imageData, err := c.getImageAsBase64(context.Background(), messageID, imageContent.ImageKey)
			if err != nil {
				log.Error("[WS] Failed to get image base64 data", "error", err)
				content = "[发送了一张图片，但处理失败]"
			} else {
				// Add base64 image data to Media array - this will be sent directly to LLM
				msgMedia = []Media{
					{
						Type:     "image",
						Data:     imageData,
						MimeType: "image/jpeg",
					},
				}
				content = "[发送了一张图片，请描述]"
				log.Info("[WS] Image base64 prepared for multimodal LLM", "image_data_size", len(imageData))
			}
		}
	} else {
		// Handle text messages
		var parsedContent struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(content), &parsedContent); err != nil {
			log.Warn("[WS] Failed to parse message content as JSON, using raw content", "error", err)
		} else if parsedContent.Text != "" {
			content = parsedContent.Text
			log.Info("[WS] Extracted text content", "text", content)
		}
	}

	// Send to message channel
	c.mu.RLock()
	running := c.running
	c.mu.RUnlock()

	if !running {
		return
	}

	select {
	case c.messages <- Message{
		ChannelID: "feishu", // Set ChannelID for session management
		SenderID:  senderID,
		Content:   content,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"message_id":   event.Event.Message.MessageId,
			"chat_id":      event.Event.Message.ChatId,
			"message_type": event.Event.Message.MessageType,
			"source":       "websocket",
		},
		Media: msgMedia,
	}:
		log.Info("[WS] Message forwarded to message channel", "sender", senderID, "has_image", len(msgMedia) > 0, "content", content, "media_count", len(msgMedia))
	default:
		log.Warn("[WS] Message channel full, dropping message")
	}
}

// handleEvent processes a single Feishu event (webhook mode)
func (c *FeishuChannel) handleEvent(event FeishuInnerEvent) {
	// Only handle receive message events
	if event.Type != "im.message.receive_v1" {
		return
	}

	// Get sender info
	senderID := event.Sender.SenderID.OpenID
	senderType := event.Sender.SenderType

	// Only process user messages (not bot or system)
	if senderType != "user" {
		return
	}

	// Parse the message content
	var msgContent struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(event.Message.Content), &msgContent); err != nil {
		log.Error("[WEBHOOK] Failed to parse feishu message content", "error", err)
		return
	}

	// Ignore empty messages
	if msgContent.Text == "" {
		return
	}

	// Send to message channel
	c.mu.RLock()
	running := c.running
	c.mu.RUnlock()

	if !running {
		return
	}

	select {
	case c.messages <- Message{
		ChannelID: "feishu", // Set ChannelID for session management
		SenderID:  senderID,
		Content:   msgContent.Text,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"message_id":   event.Message.MessageID,
			"chat_id":      event.Message.ChatID,
			"message_type": event.Message.MsgType,
			"create_time":  event.Message.CreateTime,
		},
	}:
		log.Info("[WEBHOOK] Received feishu message", "sender", senderID, "content", msgContent.Text)
	default:
		log.Warn("[WEBHOOK] Feishu message channel full, dropping message")
	}
}

// FeishuConfig holds Feishu configuration
type FeishuConfig struct {
	Enabled           bool
	AppID             string
	AppSecret         string
	EncryptKey        string
	VerificationToken string
	AllowFrom         []string
	ConnectionMode    string // "websocket" or "webhook", default is "websocket"
}

// FeishuWebhookRequest represents the incoming webhook request from Feishu
type FeishuWebhookRequest struct {
	Token     string             `json:"token"`
	Challenge string             `json:"challenge"`
	Type      string             `json:"type"`
	Timestamp int64              `json:"ts"`
	Event     []FeishuInnerEvent `json:"event"`
}

// FeishuInnerEvent represents an individual event from Feishu
type FeishuInnerEvent struct {
	Type   string `json:"type"`
	UUID   string `json:"uuid"`
	Sender struct {
		SenderID struct {
			OpenID string `json:"open_id"`
			UserID string `json:"user_id"`
		} `json:"sender_id"`
		SenderType string `json:"sender_type"`
		KeyWord    string `json:"key_word"`
	} `json:"sender"`
	Message struct {
		MessageID  string        `json:"message_id"`
		RootID     string        `json:"root_id"`
		ParentID   string        `json:"parent_id"`
		CreateTime string        `json:"create_time"`
		UpdateTime string        `json:"update_time"`
		ChatID     string        `json:"chat_id"`
		ChatType   string        `json:"chat_type"`
		MsgType    string        `json:"msg_type"`
		Content    string        `json:"content"`
		Mentioned  []interface{} `json:"mentioned"`
		MentionAll bool          `json:"mention_all"`
	} `json:"message"`
}
