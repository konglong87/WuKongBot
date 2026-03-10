package tracer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTracer_NewTrace(t *testing.T) {
	tracer := NewTracer(true)
	trace := tracer.NewTrace()

	assert.NotEmpty(t, trace.TraceId, "TraceId should not be empty")
	assert.NotEmpty(t, trace.RootId, "RootId should not be empty")
	assert.False(t, trace.StartTime.IsZero(), "StartTime should be set")
	assert.NotEqual(t, trace.TraceId, trace.RootId, "TraceId and RootId should be different")
}

func TestTracer_GetTrace(t *testing.T) {
	tracer := NewTracer(true)
	trace := tracer.NewTrace()

	found, exists := tracer.GetTrace(trace.TraceId)
	assert.True(t, exists, "Trace should exist")
	assert.Equal(t, trace.TraceId, found.TraceId, "TraceId should match")
}

func TestTracer_EndTrace(t *testing.T) {
	tracer := NewTracer(true)
	trace := tracer.NewTrace()
	traceId := trace.TraceId

	tracer.EndTrace(traceId)

	_, exists := tracer.GetTrace(traceId)
	assert.False(t, exists, "Trace should be removed after EndTrace")
}

func TestSpan_StartAndEnd(t *testing.T) {
	tracer := NewTracer(true)
	trace := tracer.NewTrace()
	span := tracer.StartSpan(trace.TraceId, "test-operation")

	span.End()

	assert.NotEmpty(t, span.SpanId, "SpanId should not be empty")
	assert.False(t, span.StartTime.IsZero(), "StartTime should be set")
	assert.Less(t, time.Duration(0), span.Duration, "Duration should be positive")
}

func TestSpan_Fail(t *testing.T) {
	tracer := NewTracer(true)
	trace := tracer.NewTrace()
	span := tracer.StartSpan(trace.TraceId, "test-operation")

	span.Fail("test error")

	assert.False(t, span.Success, "Span should be marked as failed")
	assert.Equal(t, "test error", span.ErrorMsg, "Error message should be set")
	assert.Less(t, time.Duration(0), span.Duration, "Duration should be calculated")
}

func TestTracer_SpanChaining(t *testing.T) {
	tracer := NewTracer(true)
	trace := tracer.NewTrace()
	traceId := trace.TraceId

	span1 := tracer.StartSpan(traceId, "operation1")
	assert.Empty(t, span1.ParentId, "First span should have no parent")

	span2 := tracer.StartSpan(traceId, "operation2")
	assert.Equal(t, span1.SpanId, span2.ParentId, "Second span should have first span as parent")
}

func TestEnsureTraceId(t *testing.T) {
	// Empty string should generate new ID
	traceId := EnsureTraceId("")
	assert.NotEmpty(t, traceId, "Should generate new traceId for empty string")

	// Non-empty string should be returned as-is
	customId := "custom-trace-id"
	result := EnsureTraceId(customId)
	assert.Equal(t, customId, result, "Should return custom ID as-is")
}

func TestTracer_Tags(t *testing.T) {
	tracer := NewTracer(true)
	trace := tracer.NewTrace()
	traceId := trace.TraceId

	ok := tracer.AddTag(traceId, "channel", "telegram")
	assert.True(t, ok, "AddTag should succeed")

	ok = tracer.AddTag(traceId, "userid", "12345")
	assert.True(t, ok, "AddTag should succeed")

	tags := tracer.GetTags(traceId)
	assert.NotNil(t, tags, "Tags should not be nil")
	assert.Equal(t, "telegram", tags["channel"], "Channel tag should match")
	assert.Equal(t, "12345", tags["userid"], "Userid tag should match")
}

func TestTracer_Tags_NonExistent(t *testing.T) {
	tracer := NewTracer(true)
	ok := tracer.AddTag("non-existent", "channel", "telegram")
	assert.False(t, ok, "AddTag should fail for non-existent trace")
}

func TestGetTraceIdFromContext(t *testing.T) {
	// Empty context should return empty string
	ctx := context.Background()
	traceId := GetTraceIdFromContext(ctx)
	assert.Empty(t, traceId, "Should return empty string for empty context")

	// Context with traceId should return it
	ctx = context.WithValue(context.Background(), "traceId", "test-trace-id")
	traceId = GetTraceIdFromContext(ctx)
	assert.Equal(t, "test-trace-id", traceId, "Should return traceId from context")
}

func TestSetTraceIdInContext(t *testing.T) {
	ctx := context.Background()
	ctx = SetTraceIdInContext(ctx, "test-trace-id")

	traceId := GetTraceIdFromContext(ctx)
	assert.Equal(t, "test-trace-id", traceId, "TraceId should be retrievable after setting")
}
