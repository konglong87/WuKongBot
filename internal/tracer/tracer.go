package tracer

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
)

// TraceContext represents a trace context with all necessary information
type TraceContext struct {
	TraceId    string            // UUID v4 - global unique trace ID
	ParentSpan string            // Parent span ID (optional for nested operations)
	RootId     string            // Root trace ID (for cross-process tracking)
	StartTime  time.Time         // Trace start time
	Metadata   map[string]string // Custom metadata
	Tags       map[string]string // Tags (channel, sender, tool, etc.)
}

// Span represents a single operation within a trace
type Span struct {
	SpanId    string        // Span ID
	ParentId  string        // Parent span ID (empty for root span)
	Name      string        // Operation name (e.g., "executeTool", "llm_call")
	StartTime time.Time     // Span start time
	Duration  time.Duration // Span duration (calculated on End())
	Success   bool          // Whether operation succeeded
	ErrorMsg  string        // Error message if failed
}

// Tracer manages active traces and spans
type Tracer struct {
	mu           sync.RWMutex
	activeTraces map[string]*TraceContext // traceId -> TraceContext
	logger       *log.Logger
	enabled      bool
}

// NewTracer creates a new tracer instance
func NewTracer(enabled bool) *Tracer {
	return &Tracer{
		activeTraces: make(map[string]*TraceContext),
		logger:       log.New(os.Stdout),
		enabled:      enabled,
	}
}

// NewTrace creates a new trace context with a unique ID
func (t *Tracer) NewTrace() *TraceContext {
	trace := &TraceContext{
		TraceId:   uuid.New().String(),
		RootId:    uuid.New().String(),
		StartTime: time.Now(),
		Metadata:  make(map[string]string),
		Tags:      make(map[string]string),
	}

	t.mu.Lock()
	t.activeTraces[trace.TraceId] = trace
	t.mu.Unlock()

	return trace
}

// GetTrace retrieves a trace context by traceId (returns a copy to avoid race conditions)
func (t *Tracer) GetTrace(traceId string) (*TraceContext, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	trace, exists := t.activeTraces[traceId]
	if !exists {
		return nil, false
	}
	// Return a copy
	return &TraceContext{
		TraceId:    trace.TraceId,
		ParentSpan: trace.ParentSpan,
		RootId:     trace.RootId,
		StartTime:  trace.StartTime,
		Metadata:   copyMap(trace.Metadata),
		Tags:       copyMap(trace.Tags),
	}, true
}

// copyMap creates a copy of a string map
func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	copy := make(map[string]string)
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

// GetTraceInternal returns the actual trace context pointer (internal use only)
// This is NOT thread-safe and should only be called when holding the mutex
func (t *Tracer) GetTraceInternal(traceId string) *TraceContext {
	return t.activeTraces[traceId]
}

// RegisterTrace registers a trace context manually
func (t *Tracer) RegisterTrace(traceId string, trace *TraceContext) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.activeTraces[traceId] = trace
}

// EndTrace removes a trace from active traces
func (t *Tracer) EndTrace(traceId string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.activeTraces, traceId)
}

// StartSpan creates and returns a new span
func (t *Tracer) StartSpan(traceId, name string) *Span {
	span := &Span{
		SpanId:    uuid.New().String(),
		Name:      name,
		StartTime: time.Now(),
		Success:   true,
	}

	// Lock to safely access trace context
	t.mu.Lock()
	defer t.mu.Unlock()

	if trace, exists := t.activeTraces[traceId]; exists {
		// Child span receives current parent as its parent ID
		span.ParentId = trace.ParentSpan
		// Update parent to this new span for next operation
		trace.ParentSpan = span.SpanId
	}

	return span
}

// End marks the span as completed
func (s *Span) End() {
	s.Duration = time.Since(s.StartTime)
}

// Fail marks the span as failed with an error message
func (s *Span) Fail(errMsg string) {
	s.Success = false
	s.ErrorMsg = errMsg
	s.End()
}

// IsEnabled returns whether tracing is enabled
func (t *Tracer) IsEnabled() bool {
	return t.enabled
}

// Enable enables tracing
func (t *Tracer) Enable() {
	t.enabled = true
}

// Disable disables tracing
func (t *Tracer) Disable() {
	t.enabled = false
}

// Info logs an info message with trace context
func (t *Tracer) Info(traceId string, msg string, keyvals ...interface{}) {
	t.log("INFO", traceId, msg, nil, keyvals...)
}

// Debug logs a debug message with trace context
func (t *Tracer) Debug(traceId string, msg string, keyvals ...interface{}) {
	t.log("DEBUG", traceId, msg, nil, keyvals...)
}

// Error logs an error message with trace context
func (t *Tracer) Error(traceId string, msg string, err error, keyvals ...interface{}) {
	t.log("ERROR", traceId, msg, err, keyvals...)
}

// log is the internal logging method
func (t *Tracer) log(level, traceId string, msg string, err error, keyvals ...interface{}) {
	// Add trace_id to keyvals (even when disabled, for log aggregation)
	kv := make([]interface{}, 0, len(keyvals)+2)
	kv = append(kv, "trace_id", traceId)
	kv = append(kv, keyvals...)

	if !t.enabled {
		// Fall back to standard logging (but still include trace_id)
		log.Info(msg, kv...)
		return
	}

	// Format prefix with traceId
	prefix := fmt.Sprintf("[trace_id: %s]", traceId)
	fullMsg := fmt.Sprintf("%s %s", prefix, msg)

	// Use charmbracelet/log with structured fields
	switch level {
	case "DEBUG":
		log.Debug(fullMsg, kv...)
	case "ERROR":
		log.Error(fullMsg, kv...)
	default:
		log.Info(fullMsg, kv...)
	}
}

// AddTag adds a tag to the trace context
func (t *Tracer) AddTag(traceId, key, value string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if trace, exists := t.activeTraces[traceId]; exists {
		trace.Tags[key] = value
		return true
	}
	return false
}

// GetTags returns all tags for a trace
func (t *Tracer) GetTags(traceId string) map[string]string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if trace, exists := t.activeTraces[traceId]; exists {
		// Return copy
		tags := make(map[string]string)
		for k, v := range trace.Tags {
			tags[k] = v
		}
		return tags
	}
	return nil
}

// EnsureTraceId ensures a message has a valid traceId
func EnsureTraceId(traceId string) string {
	if traceId == "" {
		return uuid.New().String()
	}
	return traceId
}

// GetTraceIdFromContext extracts traceId from context if available
func GetTraceIdFromContext(ctx context.Context) string {
	if traceId, ok := ctx.Value("traceId").(string); ok && traceId != "" {
		return traceId
	}
	return ""
}

// SetTraceIdInContext sets traceId in context
func SetTraceIdInContext(ctx context.Context, traceId string) context.Context {
	return context.WithValue(ctx, "traceId", traceId)
}
