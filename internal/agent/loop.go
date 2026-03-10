package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/konglong87/wukongbot/internal/bus"
	"github.com/konglong87/wukongbot/internal/config"
	"github.com/konglong87/wukongbot/internal/feishu/progress"
	h "github.com/konglong87/wukongbot/internal/hooks"
	"github.com/konglong87/wukongbot/internal/providers"
	"github.com/konglong87/wukongbot/internal/toolcontext"
	"github.com/konglong87/wukongbot/internal/tracer"
	"github.com/konglong87/wukongbot/tools"
)

type Tracer interface {
	NewTrace() *tracer.TraceContext
	GetTrace(traceId string) (*tracer.TraceContext, bool)
	EndTrace(traceId string)
	RegisterTrace(traceId string, trace *tracer.TraceContext)
	StartSpan(traceId, name string) *tracer.Span
	Info(traceId string, msg string, keyvals ...interface{})
	Debug(traceId string, msg string, keyvals ...interface{})
	Error(traceId string, msg string, err error, keyvals ...interface{})
	AddTag(traceId, key, value string) bool
	Fail(span *tracer.Span, err error)
	End(span *tracer.Span)
}

type TracerWrapper struct {
	*tracer.Tracer
}

func (w *TracerWrapper) Fail(span *tracer.Span, err error) {
	span.Fail(err.Error())
}

func (w *TracerWrapper) End(span *tracer.Span) {
	span.End()
}

// CronSchedule represents a cron schedule
type CronSchedule struct {
	Kind     string
	EveryMs  int64
	CronExpr string
}

// JobEntry represents a scheduled job entry (imported from tools/cron.go)
type JobEntry tools.CronJob

// LLMMessage represents a message with role, content, and timestamp
type LLMMessage struct {
	Role      string
	Content   string
	Timestamp time.Time
}

// cronServiceAdapter wraps the CronService interface implementation to work with tools.CronTool
type cronServiceAdapter struct {
	service tools.CronService
}

// NewCronServiceAdapter creates an adapter
func NewCronServiceAdapter(service tools.CronService) *cronServiceAdapter {
	return &cronServiceAdapter{service: service}
}

func (a *cronServiceAdapter) AddJob(name, scheduleKind string, everyMs int64, cronExpr, message, channel, to string, oneTime, directSend bool) *tools.JobEntry {
	return a.service.AddJob(name, scheduleKind, everyMs, cronExpr, message, channel, to, oneTime, directSend)
}

func (a *cronServiceAdapter) UpdateJob(id string, scheduleKind string, everyMs int64, cronExpr, message string, oneTime, directSend bool) *tools.JobEntry {
	return a.service.UpdateJob(id, scheduleKind, everyMs, cronExpr, message, oneTime, directSend)
}

func (a *cronServiceAdapter) ListJobs() []*tools.JobEntry {
	return a.service.ListJobs()
}

func (a *cronServiceAdapter) RemoveJob(id string) bool {
	return a.service.RemoveJob(id)
}

func (a *cronServiceAdapter) EnableJob(id string) bool {
	return a.service.EnableJob(id)
}

func (a *cronServiceAdapter) DisableJob(id string) bool {
	return a.service.DisableJob(id)
}

func (a *cronServiceAdapter) GetExecutionLogs(jobID string, limit int) string {
	return a.service.GetExecutionLogs(jobID, limit)
}

// Config holds agent configuration
type Config struct {
	Workspace             string
	Model                 string
	MaxTokens             int
	Temperature           float64
	MaxToolIterations     int
	SearchProvider        string // "brave" or "duckduckgo"
	BraveAPIKey           string
	ExecTimeout           int
	ExecRestrictToWs      bool
	Identity              *config.IdentityConfig
	MaxHistoryMessages    int                     // Maximum number of history messages to include in context (0 = no history)
	UseHistoryMessages    bool                    // Whether to use history messages at all (false for real-time data queries)
	HistoryTimeoutSeconds int                     // Only use history messages within this time window (seconds, 0 = no limit)
	ErrorResponse         string                  // Default error response when LLM returns empty content
	ImageTools            config.ImageToolsConfig // Image tools configuration
	CodeDevEnabled        bool                    // Whether code dev tools are enabled
	CodeDevTimeout        int                     // Code dev tool timeout
	CodeDevExecutors      map[string]config.CodeDevExecutorConfig
}

// AgentLoop is the core processing engine
type AgentLoop struct {
	cfg             Config
	bus             bus.MessageBus
	provider        providers.LLMProvider
	tools           *tools.ToolRegistry
	context         *ContextBuilder
	mu              sync.RWMutex
	running         bool
	subagentMgr     SubagentManager
	cronService     tools.CronService
	sessionManager  SessionManager
	hooks           *h.HookRegistry
	enhancedHandler *EnhancedHandler
	currentChannel  string
	currentSender   string
	taskStartTime   time.Time
	tracer          Tracer
}

// SubagentManager interface for subagent operations
type SubagentManager interface {
	Spawn(ctx context.Context, task, label, originChannel, originChatID string) (string, error)
}

// SessionManager interface for session history management
type SessionManager interface {
	GetSessionMessagesAsLLMMessages(sessionKey string, limit int) ([]LLMMessage, error)
	AddMessage(sessionKey string, role, content string) error
}

// NewAgentLoop creates a new agent loop
func NewAgentLoop(bus bus.MessageBus, provider providers.LLMProvider, cfg Config) *AgentLoop {
	loop := &AgentLoop{
		cfg:      cfg,
		bus:      bus,
		provider: provider,
		tools:    tools.NewToolRegistry(),
		context:  NewContextBuilder(cfg.Workspace, cfg.Identity),
		tracer:   &TracerWrapper{Tracer: tracer.NewTracer(true)}, // Enable tracing by default
	}
	loop.registerDefaultTools()
	return loop
}

// SetTracer sets the tracer (for testing or configuration)
func (l *AgentLoop) SetTracer(tracer Tracer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tracer = tracer
}

// SetSubagentManager sets the subagent manager
func (l *AgentLoop) SetSubagentManager(mgr SubagentManager) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.subagentMgr = mgr
}

// SetCronService sets the cron service
func (l *AgentLoop) SetCronService(service tools.CronService) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cronService = service
	// Register cron tool now that we have the service
	if service != nil {
		adapter := &cronServiceAdapter{service: service}
		l.tools.Register(tools.NewCronTool(adapter))
		log.Info("Cron tool registered successfully")
	}
}

// SetSessionManager sets the session manager
func (l *AgentLoop) SetSessionManager(mgr SessionManager) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sessionManager = mgr
}

// SetHooksRegistry sets the hooks registry
func (l *AgentLoop) SetHooksRegistry(registry *h.HookRegistry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = registry
}

// SetEnhancedHandler sets the enhanced handler for Feishu integration
func (l *AgentLoop) SetEnhancedHandler(handler *EnhancedHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enhancedHandler = handler
}

// GetToolRegistry returns the tool registry for external use
func (l *AgentLoop) GetToolRegistry() *tools.ToolRegistry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tools
}

// Start starts the agent loop
func (l *AgentLoop) Start(ctx context.Context) error {
	l.mu.Lock()
	l.running = true
	l.mu.Unlock()
	fmt.Println("Agent loop started fmt")
	log.Info("Agent loop started")

	for {
		select {
		case <-ctx.Done():
			l.Stop()
			return ctx.Err()
		default:
			msg, err := l.bus.ConsumeInbound(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				continue
			}

			// Ensure traceId exists
			if msg.TraceId == "" {
				msg.TraceId = l.tracer.NewTrace().TraceId
			}

			go func() {
				response, err := l.processMessage(context.Background(), msg)
				if err != nil {
					l.tracer.Error(msg.TraceId, "Error processing message", err)
					return
				}
				if response != nil {
					// Propagate traceId to outbound message
					response.TraceId = msg.TraceId
					if err := l.bus.PublishOutbound(context.Background(), *response); err != nil {
						l.tracer.Error(msg.TraceId, "Error publishing response", err)
					}
				}
			}()
		}
	}
}

// Stop stops the agent loop
func (l *AgentLoop) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.running = false
	log.Info("Agent loop stopping")
}

// IsRunning returns whether the agent is running
func (l *AgentLoop) IsRunning() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.running
}

// ProcessDirect processes a message directly (for CLI usage)
func (l *AgentLoop) ProcessDirect(ctx context.Context, content, sessionKey, channel, chatID string) (string, error) {
	msg := bus.InboundMessage{
		ChannelID: channel,
		SenderID:  "user",
		Content:   content,
		Timestamp: time.Now(),
	}

	response, err := l.processMessage(ctx, msg)
	if err != nil {
		return "", err
	}
	if response != nil {
		return response.Content, nil
	}
	return "", nil
}

// processMessage processes a single inbound message
func (l *AgentLoop) processMessage(ctx context.Context, msg bus.InboundMessage) (*bus.OutboundMessage, error) {
	if msg.ChannelID == "system" {
		return l.processSystemMessage(ctx, msg)
	}

	// Try enhanced handler routing first - before LLM processing
	if l.enhancedHandler != nil {
		log.Info("[LOOP] Enhanced handler exists, attempting to route message",
			"channel_id", msg.ChannelID,
			"sender_id", msg.SenderID,
			"content", msg.Content,
			"content_length", len(msg.Content))

		handled, err := l.enhancedHandler.RouteMessage(msg.ChannelID, msg.SenderID, msg.Content)
		if err != nil {
			log.Error("[LOOP] Enhanced handler error", "error", err)
			return nil, fmt.Errorf("enhanced handler error: %w", err)
		}

		log.Info("[LOOP] RouteMessage returned",
			"handled", handled,
			"channel_id", msg.ChannelID,
			"sender_id", msg.SenderID,
			"content", msg.Content)

		if handled {
			// Message was handled by enhanced handler (e.g., Claude Code session)
			// Return empty message to indicate message was processed
			log.Info("[LOOP] Message handled by enhanced handler (PTY mode), returning empty message",
				"channel_id", msg.ChannelID,
				"sender_id", msg.SenderID)
			return &bus.OutboundMessage{
				ChannelID:   msg.ChannelID,
				RecipientID: msg.SenderID,
				Content:     "",
				Timestamp:   time.Now(),
			}, nil
		}
		log.Debug("[LOOP] Message not handled, continuing to LLM processing")
	}

	// Log with traceId
	l.tracer.Info(msg.TraceId, "Processing message", "channel", msg.ChannelID, "sender", msg.SenderID)

	// Add tags for easier filtering
	l.tracer.AddTag(msg.TraceId, "channel", msg.ChannelID)
	l.tracer.AddTag(msg.TraceId, "sender", msg.SenderID)

	// Store current channel info for notifications
	l.currentChannel = msg.ChannelID
	l.currentSender = msg.SenderID

	// Enhanced feature: Set context for enhanced handler
	if l.enhancedHandler != nil && l.enhancedHandler.IsEnabled() {
		l.enhancedHandler.SetContext(msg.ChannelID, msg.SenderID)
	}

	// Execute UserPromptSubmit hooks before processing the message
	if l.hooks != nil {
		hookOutput, hookErrors := l.hooks.ExecuteUserPromptSubmit(ctx, map[string]interface{}{
			"prompt": msg.Content,
		})
		if len(hookErrors) > 0 {
			log.Warn("UserPromptSubmit hook errors", "errors", hookErrors)
		}

		// Check if hook wants to intercept the message
		if hookOutput.Decision == h.HookDeny && hookOutput.Message != "" {
			// Hook handled the message, return the hook's message as response
			log.Info("processMessage Message intercepted by UserPromptSubmit hook", "hook_message", hookOutput.Message)
			return &bus.OutboundMessage{
				ChannelID:   msg.ChannelID,
				RecipientID: msg.SenderID,
				Content:     hookOutput.Message,
				Type:        "text",
				Timestamp:   time.Now(),
			}, nil
		}
	}

	l.updateToolContexts(msg.ChannelID, msg.SenderID)

	// Save user message to session history if enabled
	sessionKey := msg.ChannelID + ":" + msg.SenderID
	if l.sessionManager != nil && l.cfg.UseHistoryMessages && l.cfg.MaxHistoryMessages > 0 {
		if err := l.sessionManager.AddMessage(sessionKey, "user", msg.Content); err != nil {
			log.Warn("processMessage Failed to save user message to history", "session_key", sessionKey, "error", err)
		} else {
			log.Info("processMessage Saved user message to history", "session_key", sessionKey, "content", msg.Content)
		}
	}

	// Fetch history messages if enabled and filter by timeout
	historyMessages := []LLMMessage{}
	if l.sessionManager != nil && l.cfg.UseHistoryMessages && l.cfg.MaxHistoryMessages > 0 {
		allHistory, err := l.sessionManager.GetSessionMessagesAsLLMMessages(sessionKey, l.cfg.MaxHistoryMessages*3) // Get 3x for filtering
		if err != nil {
			log.Warn("processMessage Failed to load history messages", "session_key", sessionKey, "error", err)
		} else {
			log.Info("processMessage Loaded history messages", "session_key", sessionKey, "total_loaded", len(allHistory))
			now := time.Now()
			timeoutThreshold := time.Duration(l.cfg.HistoryTimeoutSeconds) * time.Second

			validHistory := []LLMMessage{}
			for _, msg := range allHistory {
				msgAge := now.Sub(msg.Timestamp)
				ageSeconds := int(msgAge.Seconds())
				threshold := l.cfg.HistoryTimeoutSeconds
				log.Info("processMessage Checking history message age", "age_seconds", ageSeconds, "threshold", threshold, "msg.Timestamp", msg.Timestamp, "now", now, "msgAge", msgAge)
				if l.cfg.HistoryTimeoutSeconds > 0 && msgAge > timeoutThreshold {
					log.Debug("processMessage Skipping old history", "age_seconds", ageSeconds, "threshold", threshold, "content_preview", msg.Content[:min(100, len(msg.Content))])
					continue
				}
				validHistory = append(validHistory, msg)
			}
			historyMessages = validHistory
			if len(validHistory) > 0 {
				log.Info("processMessage Loaded valid history messages", "session_key", sessionKey, "total_loaded", len(allHistory), "valid_count", len(validHistory), "timeout_seconds", l.cfg.HistoryTimeoutSeconds)
			} else {
				log.Debug("processMessage No valid history messages found", "session_key", sessionKey, "reason", "all messages filtered out")
			}
		}
	}

	// Convert to providers.Message format
	llmHistory := make([]providers.Message, len(historyMessages))
	for i, m := range historyMessages {
		llmHistory[i] = providers.Message{Role: m.Role, Content: m.Content}
	}

	// Convert media from InboundMessage to providers.Media
	media := make([]providers.Media, len(msg.Media))
	for i, m := range msg.Media {
		media[i] = providers.Media{
			Type:     m.Type,
			URL:      m.URL,
			Data:     m.Data,
			MimeType: m.MimeType,
		}
	}

	messages := l.context.BuildMessages(llmHistory, msg.Content, nil, nil, msg.ChannelID, msg.SenderID)

	// Add media to the last message (user message)
	if len(media) > 0 && len(messages) > 0 {
		messages[len(messages)-1].Media = media
	}
	log.Info("Built messages for LLM", "total_messages", len(messages), "history_count", len(llmHistory), "use_history", l.cfg.UseHistoryMessages, "has_history", len(messages) > 1)

	// Print full prompt for debugging
	log.Info("===== FULL PROMPT TO LLM =====")
	for i, m := range messages {
		role := m.Role
		content := m.Content
		if len(content) > 2000 {
			content = content[:2000] + "... [truncated]"
		}
		log.Info(fmt.Sprintf("[Message %d] Role: %s", i, role))
		log.Info(fmt.Sprintf("[Message %d] Content: %s", i, content))
		if len(m.Media) > 0 {
			log.Info(fmt.Sprintf("[Message %d] Media: %d items", i, len(m.Media)))
			for j, mediaItem := range m.Media {
				log.Info(fmt.Sprintf("[Message %d] Media[%d]: Type=%s, MimeType=%s, URL=%s, DataLength=%d",
					i, j, mediaItem.Type, mediaItem.MimeType, mediaItem.URL, len(mediaItem.Data)))
			}
		}
	}
	log.Info("===== END FULL PROMPT =====")

	iteration := 0
	finalContent := ""
	lastContent := "" // Track the last non-empty content

	for iteration < l.cfg.MaxToolIterations {
		iteration++

		toolDefs := l.getToolDefinitions()

		req := providers.ChatRequest{
			Messages:    messages,
			Tools:       toolDefs,
			Model:       l.cfg.Model,
			MaxTokens:   l.cfg.MaxTokens,
			Temperature: l.cfg.Temperature,
		}

		response, err := l.provider.Chat(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("LLM error: %w", err)
		}

		log.Info("LLM response received", "content_length", len(response.Content), "tool_calls", len(response.ToolCalls), "finish_reason", response.FinishReason)
		if len(response.Content) > 0 {
			log.Debug("LLM content", "content", response.Content[:min(2048, len(response.Content))])
			// Save the last non-empty content for cases where LLM returns empty content after tool calls
			lastContent = response.Content
		}

		if len(response.ToolCalls) > 0 {
			messages = l.addAssistantMessage(messages, response.Content, response.ToolCalls)

			// Clear lastContent after executing tools - intermediate thoughts should not be used as final response
			lastContent = ""

			// Split tool calls into concurrent-safe and non-concurrent-safe
			var concurrentCalls, sequentialCalls []providers.ToolCall
			for _, tc := range response.ToolCalls {
				if tool, exists := l.tools.Get(tc.Name); exists && tool.ConcurrentSafe() {
					concurrentCalls = append(concurrentCalls, tc)
				} else {
					sequentialCalls = append(sequentialCalls, tc)
				}
			}

			// Execute concurrent-safe tools in parallel
			var wg sync.WaitGroup
			results := make(map[string]string)
			resultsMu := sync.Mutex{}

			for _, tc := range concurrentCalls {
				wg.Add(1)
				go func(tc providers.ToolCall) {
					defer wg.Done()
					log.Info("Executing tool (concurrent)", "tool", tc.Name, "args", tc.Arguments)
					result, err := l.executeTool(ctx, tc.Name, tc.Arguments)
					if err != nil {
						result = fmt.Sprintf("Error executing tool: %v", err)
						log.Error("Tool execution failed", "tool", tc.Name, "error", err)
					}
					log.Info("Tool result", "tool", tc.Name, "result_length", len(result), "result_preview", result[:min(2048, len(result))])
					resultsMu.Lock()
					results[tc.ID] = result
					resultsMu.Unlock()
				}(tc)
			}
			wg.Wait()

			// Execute non-concurrent-safe tools sequentially
			for _, tc := range sequentialCalls {
				log.Info("Executing tool (sequential)", "tool", tc.Name, "args", tc.Arguments)
				result, err := l.executeTool(ctx, tc.Name, tc.Arguments)
				if err != nil {
					result = fmt.Sprintf("Error executing tool: %v", err)
					log.Error("Tool execution failed", "tool", tc.Name, "error", err)
				}
				log.Info("Tool result", "tool", tc.Name, "result_length", len(result), "result_preview", result[:min(2048, len(result))])
				results[tc.ID] = result
			}

			// Add all tool results to messages in original order
			for _, tc := range response.ToolCalls {
				messages = l.addToolResult(messages, tc.ID, tc.Name, results[tc.ID])
			}
		} else {
			// Use content if available, otherwise use lastContent
			if len(response.Content) > 0 {
				finalContent = response.Content
			} else if lastContent != "" {
				finalContent = lastContent
			}
			break
		}
	}

	// If we have no final content but have a last content (from a tool call response), use it
	if finalContent == "" && lastContent != "" {
		finalContent = lastContent
		log.Info("Using last content from tool response", "content_length", len(finalContent))
	}

	// Fallback: if we have no final content but executed tools with errors, ask LLM to respond
	if finalContent == "" && len(messages) > 0 {
		// Check if there are tool results in the messages
		hasToolResults := false
		for _, m := range messages {
			if m.Role == "tool" {
				hasToolResults = true
				break
			}
		}

		if hasToolResults {
			log.Info("LLM returned empty content after tool execution, requesting fallback response")
			// Send a final request to LLM asking it to respond based on the tool results
			fallbackReq := providers.ChatRequest{
				Messages:    messages,
				Tools:       nil, // No more tools
				Model:       l.cfg.Model,
				MaxTokens:   l.cfg.MaxTokens,
				Temperature: l.cfg.Temperature,
			}
			fallbackReq.Messages = append(fallbackReq.Messages, providers.Message{
				Role:    "user",
				Content: "Please provide a response based on the tool execution results above. If there were errors, explain them to the user and suggest alternatives if possible.",
			})

			response, err := l.provider.Chat(ctx, fallbackReq)
			if err == nil && len(response.Content) > 0 {
				finalContent = response.Content
				log.Info("Generated fallback response", "content_length", len(finalContent))
			}
		}
	}

	if finalContent == "" {
		// Use configured error response or default
		if l.cfg.ErrorResponse != "" {
			finalContent = l.cfg.ErrorResponse
		} else {
			finalContent = "大模型出错了！？？？"
		}
		log.Warn("LLM returned empty content, using fallback", "iteration", iteration, "response", finalContent)
	}

	// Parse image URLs from content for channel display
	imageURLs, cleanContent := l.parseImageURLs(finalContent)
	responseMedia := make([]bus.Media, 0)

	// Also scan tool results in messages for image URLs (LLM might have converted the [IMAGE] tags)
	for _, m := range messages {
		if m.Role == "tool" {
			toolImageURLs, _ := l.parseImageURLs(m.Content)
			for _, url := range toolImageURLs {
				// Check if URL is already in the list
				found := false
				for _, existingURL := range imageURLs {
					if existingURL == url {
						found = true
						break
					}
				}
				if !found {
					imageURLs = append(imageURLs, url)
				}
			}
		}
	}

	// Construct media for each image URL found
	for _, url := range imageURLs {
		responseMedia = append(responseMedia, bus.Media{
			Type: "image",
			URL:  url,
		})
	}

	// Log if images were found
	if len(imageURLs) > 0 {
		log.Info("Parsed image URLs from response/tool results", "count", len(imageURLs), "channel", msg.ChannelID)
	}

	// Use clean content (without [IMAGE] tags) for display
	if cleanContent != "" {
		finalContent = cleanContent
	}

	// Enhance response with Feishu features (if enabled)
	if l.enhancedHandler != nil && l.enhancedHandler.IsEnabled() {
		l.enhancedHandler.SetContext(msg.ChannelID, msg.SenderID)
		enhancedContent := l.enhancedHandler.EnhanceResponse(finalContent)
		if enhancedContent == "" {
			// Enhanced handler already sent the message (e.g., interactive card)
			// Return empty message to indicate message was sent by enhanced handler
			return &bus.OutboundMessage{
				ChannelID:   msg.ChannelID,
				RecipientID: msg.SenderID,
				Content:     "",
				Timestamp:   time.Now(),
			}, nil
		}
		finalContent = enhancedContent
	}

	// Save assistant response to session history if enabled
	if l.sessionManager != nil && l.cfg.UseHistoryMessages && l.cfg.MaxHistoryMessages > 0 {
		if err := l.sessionManager.AddMessage(sessionKey, "assistant", finalContent); err != nil {
			log.Warn("Failed to save assistant message to history", "session_key", sessionKey, "error", err)
		}
	}

	return &bus.OutboundMessage{
		ChannelID:   msg.ChannelID,
		RecipientID: msg.SenderID,
		Content:     finalContent,
		Timestamp:   time.Now(),
		Media:       responseMedia,
	}, nil
}

// processSystemMessage processes a system message
func (l *AgentLoop) processSystemMessage(ctx context.Context, msg bus.InboundMessage) (*bus.OutboundMessage, error) {
	log.Info("Processing system message", "sender", msg.SenderID)

	messages := l.context.BuildMessages([]providers.Message{}, msg.Content, nil, nil, "cli", "direct")

	// Print full prompt for debugging
	log.Info("===== FULL PROMPT TO LLM (SYSTEM MESSAGE) =====")
	for i, m := range messages {
		role := m.Role
		content := m.Content
		if len(content) > 2000 {
			content = content[:2000] + "... [truncated]"
		}
		log.Info(fmt.Sprintf("[Message %d] Role: %s", i, role))
		log.Info(fmt.Sprintf("[Message %d] Content: %s", i, content))
	}
	log.Info("===== END FULL PROMPT =====")

	iteration := 0
	finalContent := ""
	lastContent := "" // Track the last non-empty content

	for iteration < l.cfg.MaxToolIterations {
		iteration++

		toolDefs := l.getToolDefinitions()

		req := providers.ChatRequest{
			Messages:    messages,
			Tools:       toolDefs,
			Model:       l.cfg.Model,
			MaxTokens:   l.cfg.MaxTokens,
			Temperature: l.cfg.Temperature,
		}

		response, err := l.provider.Chat(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("LLM error: %w", err)
		}

		log.Info("LLM response received", "content_length", len(response.Content), "tool_calls", len(response.ToolCalls), "finish_reason", response.FinishReason)
		if len(response.Content) > 0 {
			log.Debug("LLM content", "content", response.Content[:min(2048, len(response.Content))])
			// Save the last non-empty content for cases where LLM returns empty content after tool calls
			lastContent = response.Content
		}

		if len(response.ToolCalls) > 0 {
			messages = l.addAssistantMessage(messages, response.Content, response.ToolCalls)

			// Clear lastContent after executing tools - intermediate thoughts should not be used as final response
			lastContent = ""

			// Split tool calls into concurrent-safe and non-concurrent-safe
			var concurrentCalls, sequentialCalls []providers.ToolCall
			for _, tc := range response.ToolCalls {
				if tool, exists := l.tools.Get(tc.Name); exists && tool.ConcurrentSafe() {
					concurrentCalls = append(concurrentCalls, tc)
				} else {
					sequentialCalls = append(sequentialCalls, tc)
				}
			}

			// Execute concurrent-safe tools in parallel
			var wg sync.WaitGroup
			results := make(map[string]string)
			resultsMu := sync.Mutex{}

			for _, tc := range concurrentCalls {
				wg.Add(1)
				go func(tc providers.ToolCall) {
					defer wg.Done()
					result, err := l.executeTool(ctx, tc.Name, tc.Arguments)
					if err != nil {
						result = fmt.Sprintf("Error executing tool: %v", err)
					}
					resultsMu.Lock()
					results[tc.ID] = result
					resultsMu.Unlock()
				}(tc)
			}
			wg.Wait()

			// Execute non-concurrent-safe tools sequentially
			for _, tc := range sequentialCalls {
				result, err := l.executeTool(ctx, tc.Name, tc.Arguments)
				if err != nil {
					result = fmt.Sprintf("Error executing tool: %v", err)
				}
				results[tc.ID] = result
			}

			// Add all tool results to messages in original order
			for _, tc := range response.ToolCalls {
				messages = l.addToolResult(messages, tc.ID, tc.Name, results[tc.ID])
			}
		} else {
			// Use content if available, otherwise use lastContent
			if len(response.Content) > 0 {
				finalContent = response.Content
			} else if lastContent != "" {
				finalContent = lastContent
			}
			break
		}
	}

	// If we have no final content but have a last content (from a tool call response), use it
	if finalContent == "" && lastContent != "" {
		finalContent = lastContent
		log.Info("Using last content from tool response", "content_length", len(finalContent))
	}

	// Fallback: if we have no final content but executed tools with errors, ask LLM to respond
	if finalContent == "" && len(messages) > 0 {
		// Check if there are tool results in the messages
		hasToolResults := false
		for _, m := range messages {
			if m.Role == "tool" {
				hasToolResults = true
				break
			}
		}

		if hasToolResults {
			log.Info("LLM returned empty content after tool execution in system message, requesting fallback")
			// Send a final request to LLM asking it to respond based on the tool results
			fallbackReq := providers.ChatRequest{
				Messages:    messages,
				Tools:       nil, // No more tools
				Model:       l.cfg.Model,
				MaxTokens:   l.cfg.MaxTokens,
				Temperature: l.cfg.Temperature,
			}
			fallbackReq.Messages = append(fallbackReq.Messages, providers.Message{
				Role:    "user",
				Content: "Please provide a response based on the tool execution results above.",
			})

			response, err := l.provider.Chat(ctx, fallbackReq)
			if err == nil && len(response.Content) > 0 {
				finalContent = response.Content
				log.Info("Generated fallback response for system message", "content_length", len(finalContent))
			}
		}
	}

	if finalContent == "" {
		finalContent = "Background task completed."
	}

	return &bus.OutboundMessage{
		ChannelID:   "cli",
		RecipientID: "direct",
		Content:     finalContent,
		Timestamp:   time.Now(),
	}, nil
}

func (l *AgentLoop) updateToolContexts(channel, chatID string) {
	if msgTool, ok := l.tools.Get("message"); ok {
		if mt, ok := msgTool.(*tools.MessageTool); ok {
			mt.SetContext(channel, chatID)
		}
	}
	if spawnTool, ok := l.tools.Get("spawn"); ok {
		if st, ok := spawnTool.(*tools.SpawnTool); ok {
			st.SetContext(channel, chatID)
		}
	}
	if cronTool, ok := l.tools.Get("cron"); ok {
		if ct, ok := cronTool.(*tools.CronTool); ok {
			ct.SetContext(channel, chatID)
		}
	}
}

func (l *AgentLoop) getToolDefinitions() []providers.ToolDefinition {
	defs := []providers.ToolDefinition{}
	for _, tool := range l.tools.List() {
		defs = append(defs, providers.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		})
	}
	return defs
}

func (l *AgentLoop) executeTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	tool, ok := l.tools.Get(name)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}

	// Parse arguments from the "raw" field if present (handle nested raw fields)
	args = l.parseRawArguments(args)

	// Execute PreToolUse hooks
	l.mu.RLock()
	hooks := l.hooks
	l.mu.RUnlock()

	if hooks != nil {
		output, hookErrors := hooks.ExecutePreToolUse(ctx, name, args)
		// Log any hook errors but don't fail
		for _, herr := range hookErrors {
			log.Error("executeTool PreToolUse hook error", "hook", herr.HookName, "error", herr.Error)
		}

		switch output.Decision {
		case h.HookDeny:
			log.Info("executeTool Tool execution denied by hook", "tool", name, "reason", output.Reason)
			return fmt.Sprintf("Tool execution denied: %s", output.Reason), nil
		case h.HookModify:
			log.Info("executeTool Tool execution modified by hook", "tool", name)
			// Apply modified arguments
			for k, v := range output.Modified {
				args[k] = v
			}
		}
	}

	// Check if tool implements ToolWithHooks
	toolWithHooks, hasHooks := tool.(tools.ToolWithHooks)
	var toolCtx *toolcontext.ToolContext

	// Call BeforeExecute hook if implemented
	if hasHooks {
		log.Info("executeTool Calling BeforeExecute hook", "tool", name)

		// Create adapter for MessageBus
		busAdapter := &toolcontext.MessageBusAdapter{
			SendFunc: func(channelID, senderID, content string) error {
				return l.bus.PublishOutbound(context.Background(), bus.OutboundMessage{
					ChannelID:   channelID,
					RecipientID: senderID,
					Content:     content,
					Timestamp:   time.Now(),
				})
			},
		}

		// Create ToolContext with necessary information
		toolCtx = &toolcontext.ToolContext{
			UserID:      l.currentSender,
			ChannelID:   l.currentChannel,
			SessionID:   "", // Will be set if needed
			ToolName:    name,
			ToolCallID:  "", // Will be set if needed
			Params:      args,
			StartTime:   time.Now(),
			Metadata:    make(map[string]string),
			Bus:         busAdapter,
			Adapter:     nil, // Will be set if needed
			Storage:     nil, // Will be set if needed
			CardSession: nil,
		}

		decision, err := toolWithHooks.BeforeExecute(toolCtx)
		if err != nil {
			log.Error("executeTool BeforeExecute hook failed", "tool", name, "error", err)
			// Log but continue execution
		}

		// Handle tool decision
		if decision != nil {
			switch decision.Action {
			case toolcontext.ActionSkip:
				log.Info("executeTool Tool execution skipped by hook", "tool", name)
				return "Tool execution skipped", nil
			case toolcontext.ActionCancel:
				log.Info("executeTool Tool execution cancelled by hook", "tool", name)
				return "Tool execution cancelled", nil
			case toolcontext.ActionWaitCard:
				log.Warn("executeTool Card waiting not yet implemented",
					"tool", name, "session_id", decision.SessionID)
				// Card waiting feature is placeholder for future integration (Tasks 19-20)
				// For now, return an error to indicate this is not ready
				return "", fmt.Errorf("card waiting feature not yet implemented for tool %s (session_id: %s)", name, decision.SessionID)
			case toolcontext.ActionContinue:
				log.Info("executeTool Tool execution continuing", "tool", name)
			}
		}
	}

	// Execute the tool
	var taskStart time.Time
	if name == "external_coding" {
		taskStart = time.Now()
	}

	// Enhanced feature: Send tool start progress
	if l.enhancedHandler != nil && l.enhancedHandler.IsEnabled() {
		l.enhancedHandler.SendProgressMessage(progress.ProgressStart, name, "开始执行")
	}

	result, err := tool.Execute(ctx, args)

	// Send completion notification for external_coding tasks
	if name == "external_coding" && err == nil {
		task, _ := args["task"].(string)
		tool, _ := args["tool"].(string)
		duration := time.Since(taskStart)

		var message string
		if duration.Minutes() > 60 {
			message = fmt.Sprintf("🎉 **开发任务完成**\n\n工具: %s\n任务: %s\n耗时: %.2f 小时\n\n任务已完成！", tool, task, duration.Minutes()/60)
		} else if duration.Minutes() > 1 {
			message = fmt.Sprintf("✅ **开发任务完成**\n\n工具: %s\n任务: %s\n耗时: %.1f 分钟\n\n任务已完成！", tool, task, duration.Minutes())
		} else {
			message = fmt.Sprintf("✅ **开发任务完成**\n\n工具: %s\n任务: %s\n耗时: %.0f 秒\n\n任务已完成！", tool, task, duration.Seconds())
		}

		if l.currentSender != "" && l.currentChannel != "" {
			outboundMsg := bus.OutboundMessage{
				ChannelID:   l.currentChannel,
				RecipientID: l.currentSender,
				Content:     message,
				Timestamp:   time.Now(),
			}
			if sendErr := l.bus.PublishOutbound(ctx, outboundMsg); sendErr != nil {
				log.Error("Failed to send completion notification", "error", sendErr)
			} else {
				log.Info("Completion notification sent", "tool", tool, "duration", duration)
			}
		}
	}

	// Call AfterExecute hook if implemented
	if hasHooks && toolCtx != nil {
		log.Info("executeTool Calling AfterExecute hook", "tool", name)
		if hookErr := toolWithHooks.AfterExecute(toolCtx, result, err); hookErr != nil {
			log.Error("executeTool AfterExecute hook failed", "tool", name, "error", hookErr)
			// Log but don't affect the tool execution result
		}
	}

	// Execute PostToolUse or PostToolUseFailure hooks
	if hooks != nil {
		if err != nil {
			hookErrors := hooks.ExecutePostToolUseFailure(ctx, name, err)
			for _, herr := range hookErrors {
				log.Error("PostToolUseFailure hook error", "hook", herr.HookName, "error", herr.Error)
			}
		} else {
			hookErrors := hooks.ExecutePostToolUse(ctx, name, result)
			for _, herr := range hookErrors {
				log.Error("executeTool PostToolUse hook error", "hook", herr.HookName, "error", herr.Error)
			}
		}
	}

	// Enhanced feature: Send tool result progress
	if l.enhancedHandler != nil && l.enhancedHandler.IsEnabled() {
		status := progress.ProgressCompleted
		message := "执行完成"
		if err != nil {
			status = progress.ProgressFailed
			message = fmt.Sprintf("执行失败: %v", err)
		}
		l.enhancedHandler.SendProgressMessage(status, name, message)
	}

	return result, err
}

// parseRawArguments recursively parses nested "raw" fields in arguments
// maxDepth prevents infinite recursion from malformed data
func (l *AgentLoop) parseRawArguments(args map[string]interface{}) map[string]interface{} {
	return l.parseRawArgumentsWithDepth(args, 0)
}

// parseRawArgumentsWithDepth performs the actual parsing with depth tracking
func (l *AgentLoop) parseRawArgumentsWithDepth(args map[string]interface{}, depth int) map[string]interface{} {
	const maxDepth = 10 // Prevent infinite recursion

	if depth > maxDepth {
		log.Warn("parseRawArguments: Maximum recursion depth reached, stopping",
			"depth", depth, "max_depth", maxDepth)
		return args
	}

	// Check for "raw" field containing JSON string
	if rawArgs, ok := args["raw"].(string); ok && rawArgs != "" {
		log.Debug("parseRawArguments: Found raw field", "raw_length", len(rawArgs), "raw_preview", rawArgs)
		var parsedArgs map[string]interface{}
		if err := json.Unmarshal([]byte(rawArgs), &parsedArgs); err == nil {
			// Remove the raw field from original args before recursing
			delete(args, "raw")
			// Merge the parsed args into the original args
			for k, v := range parsedArgs {
				args[k] = v
			}
			// Recursively parse to handle nested raw fields
			log.Debug("Parsed raw arguments", "parsed", parsedArgs)
			return l.parseRawArgumentsWithDepth(args, depth+1)
		} else {
			log.Error("Failed to parse tool arguments", "raw_length", len(rawArgs), "raw_end", rawArgs, "error", err)
		}
	}

	// Check if any value is a string that looks like JSON
	for key, value := range args {
		if strValue, ok := value.(string); ok {
			// Try to parse as JSON object
			var jsonObj interface{}
			if err := json.Unmarshal([]byte(strValue), &jsonObj); err == nil {
				if subMap, ok := jsonObj.(map[string]interface{}); ok {
					log.Debug("Recursively parsing JSON value for key", "key", key)
					args[key] = l.parseRawArgumentsWithDepth(subMap, depth+1)
				}
			}
		}
	}

	return args
}

func (l *AgentLoop) addAssistantMessage(messages []providers.Message, content string, toolCalls []providers.ToolCall) []providers.Message {
	tcMap := make([]map[string]interface{}, len(toolCalls))
	for i, tc := range toolCalls {
		argsJSON, _ := json.Marshal(tc.Arguments)
		tcMap[i] = map[string]interface{}{
			"id":   tc.ID,
			"type": "function",
			"function": map[string]interface{}{
				"name":      tc.Name,
				"arguments": string(argsJSON),
			},
		}
	}
	msg := providers.Message{
		Role:    "assistant",
		Content: content,
	}
	if len(tcMap) > 0 {
		msg.Metadata = map[string]interface{}{
			"tool_calls": tcMap,
		}
	}
	messages = append(messages, msg)
	return messages
}

func (l *AgentLoop) addToolResult(messages []providers.Message, toolCallID, toolName, result string) []providers.Message {
	return append(messages, providers.Message{Role: "tool", Content: result, ToolID: toolCallID, ToolResult: result})
}

// parseImageURLs extracts image URLs from content with [IMAGE]...[/IMAGE] tags
// Returns a list of URLs and the content with [IMAGE] tags removed
func (l *AgentLoop) parseImageURLs(content string) ([]string, string) {
	urls := []string{}
	re := regexp.MustCompile(`\[IMAGE\](.+?)\[/IMAGE\]`)

	// Find all matches and extract URLs
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			url := strings.TrimSpace(match[1])
			if url != "" {
				urls = append(urls, url)
			}
		}
	}

	// Remove [IMAGE]...[/IMAGE] tags from the content
	cleanContent := re.ReplaceAllString(content, "")

	return urls, strings.TrimSpace(cleanContent)
}

func (l *AgentLoop) registerDefaultTools() {
	l.tools.Register(tools.NewReadFileTool())
	l.tools.Register(tools.NewWriteFileTool(l.cfg.Workspace))
	l.tools.Register(tools.NewEditFileTool())
	l.tools.Register(tools.NewListDirTool())
	l.tools.Register(tools.NewExecTool(l.cfg.ExecTimeout, l.cfg.Workspace, l.cfg.ExecRestrictToWs))
	l.tools.Register(tools.NewWebSearchTool(l.cfg.SearchProvider, l.cfg.BraveAPIKey, 5))
	l.tools.Register(tools.NewWebFetchTool(50000))

	messageTool := tools.NewMessageTool()
	messageTool.SetSendCallback(func(channel, chatID, content string) error {
		msg := bus.OutboundMessage{ChannelID: channel, RecipientID: chatID, Content: content, Timestamp: time.Now()}
		return l.bus.PublishOutbound(context.Background(), msg)
	})
	l.tools.Register(messageTool)

	if l.subagentMgr != nil {
		l.tools.Register(tools.NewSpawnTool(l.subagentMgr))
	}

	if l.cronService != nil {
		adapter := &cronServiceAdapter{service: l.cronService}
		l.tools.Register(tools.NewCronTool(adapter))
	}

	// Register image tools
	l.tools.Register(tools.NewImageGenerationTool(
		l.cfg.ImageTools.Generation.APIKey,
		l.cfg.ImageTools.Generation.APIBase,
		l.cfg.ImageTools.Generation.Model,
		l.cfg.ImageTools.Generation.Models,
	))
	l.tools.Register(tools.NewImageAnalysisTool(
		l.cfg.ImageTools.Analysis.APIKey,
		l.cfg.ImageTools.Analysis.APIBase,
		l.cfg.ImageTools.Analysis.Model,
	))

	// Register tmux interact tool
	l.tools.Register(tools.NewTmuxInteractTool())
	log.Info("TmuxInteractTool registered successfully")

	// Register code development tool
	if l.cfg.CodeDevEnabled {
		executors := l.registerCodeDevExecutors()
		l.tools.Register(tools.NewCodeDevTool(
			l.cfg.Workspace,
			l.cfg.CodeDevTimeout,
			executors,
		))
	}
}

// registerCodeDevExecutors builds the executor map from config
func (l *AgentLoop) registerCodeDevExecutors() map[string]tools.ToolExecutor {
	execMap := make(map[string]tools.ToolExecutor)

	// Configure opencode executor
	if cfg, ok := l.cfg.CodeDevExecutors["opencode"]; ok && cfg.Enabled {
		execMap["opencode"] = tools.NewConfiguredExecutor(
			"opencode",
			cfg.Template,
			cfg.Command,
		)
	} else {
		// Default opencode executor
		execMap["opencode"] = tools.NewopencodeExecutor()
	}

	// Configure Cursor executor
	if cfg, ok := l.cfg.CodeDevExecutors["cursor"]; ok && cfg.Enabled {
		execMap["cursor"] = tools.NewConfiguredExecutor(
			"cursor",
			cfg.Template,
			cfg.Command,
		)
	} else {
		// Default Cursor executor
		execMap["cursor"] = tools.NewCursorExecutor()
	}

	// Configure Claude executor (uses opencode command)
	execMap["claude"] = tools.NewClaudeExecutor()

	return execMap
}

// checkToolLoop detects if tool calls are repeating in a loop
func (l *AgentLoop) checkToolLoop(toolName string, args map[string]interface{}, result string) (bool, string) {
	// Temporarily disabled - toolHistory field needs to be added to AgentLoop struct
	return false, ""
}
