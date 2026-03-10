package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/konglong87/wukongbot/internal/bus"
	"github.com/konglong87/wukongbot/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockMessageBus struct {
	publishedInbound  bool
	lastInboundMsg    bus.InboundMessage
	publishInboundErr error
	publishedOutbound bool
	lastOutboundMsg   bus.OutboundMessage
}

func (m *MockMessageBus) PublishInbound(ctx context.Context, msg bus.InboundMessage) error {
	m.publishedInbound = true
	m.lastInboundMsg = msg
	return m.publishInboundErr
}

func (m *MockMessageBus) ConsumeInbound(ctx context.Context) (bus.InboundMessage, error) {
	return bus.InboundMessage{}, nil
}

func (m *MockMessageBus) PublishOutbound(ctx context.Context, msg bus.OutboundMessage) error {
	m.publishedOutbound = true
	m.lastOutboundMsg = msg
	return nil
}

func (m *MockMessageBus) ConsumeOutbound(ctx context.Context) (bus.OutboundMessage, error) {
	return bus.OutboundMessage{}, nil
}

func (m *MockMessageBus) Close() error {
	return nil
}

type MockStorage struct {
	savedExecutionLog *session.ExecutionLog
	jobs              []*session.CronJob
}

func (m *MockStorage) Close() error {
	return nil
}

func (m *MockStorage) GetConfig(key string) (string, error) {
	return "", nil
}

func (m *MockStorage) SetConfig(key, value string) error {
	return nil
}

func (m *MockStorage) GetOrCreateSession(sessionKey string) (*session.Session, error) {
	return &session.Session{ID: 1}, nil
}

func (m *MockStorage) AddMessage(sessionID int64, role, content string) error {
	return nil
}

func (m *MockStorage) GetMessages(sessionID int64, limit int) ([]session.Message, error) {
	return nil, nil
}

func (m *MockStorage) GetLastNMessages(sessionID int64, n int) ([]session.Message, error) {
	return nil, nil
}

func (m *MockStorage) DeleteSession(sessionID int64) error {
	return nil
}

func (m *MockStorage) SaveCronJob(job *session.CronJob) error {
	return nil
}

func (m *MockStorage) GetCronJob(jobID string) (*session.CronJob, error) {
	return nil, nil
}

func (m *MockStorage) GetAllCronJobs() ([]*session.CronJob, error) {
	if m.jobs != nil {
		return m.jobs, nil
	}
	return []*session.CronJob{}, nil
}

func (m *MockStorage) DeleteCronJob(id string) error {
	return nil
}

func (m *MockStorage) SaveExecutionLog(execLog *session.ExecutionLog) error {
	m.savedExecutionLog = execLog
	return nil
}

func (m *MockStorage) GetExecutionLogs(jobID string, limit int) ([]*session.ExecutionLog, error) {
	return nil, nil
}

func (m *MockStorage) CreateSubagentRun(run *session.SubagentRun) error {
	return nil
}

func (m *MockStorage) UpdateSubagentRunStatus(runID string, status string, startedAt, endedAt *time.Time) error {
	return nil
}

func (m *MockStorage) RecordSubagentOutcome(runID string, outcome *session.SubagentRunOutcome) error {
	return nil
}

func (m *MockStorage) GetSubagentRun(runID string) (*session.SubagentRun, error) {
	return nil, nil
}

func (m *MockStorage) GetRunningSubagentRuns() ([]*session.SubagentRun, error) {
	return nil, nil
}

func (m *MockStorage) GetSubagentRunHistory(page, limit int) ([]*session.SubagentRun, error) {
	return nil, nil
}

func (m *MockStorage) CleanupExpiredSubagentRuns(olderThan time.Time) (int, error) {
	return 0, nil
}

func TestExecuteJob_PublishesInboundMessage(t *testing.T) {
	mockBus := &MockMessageBus{}
	mockStorage := &MockStorage{}
	service := NewService(mockStorage, mockBus, "feishu", "ou_test123")

	job := &JobEntry{
		ID:             "test123",
		Name:           "Test Job",
		Message:        "Test message",
		Channel:        "feishu",
		To:             "ou_test123",
		DeleteAfterRun: false,
		DirectSend:     false, // LLM processing mode
	}

	service.executeJob(job)

	assert.True(t, mockBus.publishedInbound, "PublishInbound should be called")
	assert.Equal(t, "feishu", mockBus.lastInboundMsg.ChannelID)
	assert.Equal(t, "ou_test123", mockBus.lastInboundMsg.SenderID, "SenderID should match job.To")
	assert.Contains(t, mockBus.lastInboundMsg.Content, "Test message", "Content should contain original message")
	assert.Contains(t, mockBus.lastInboundMsg.Content, "【定时任务提醒】", "Content should have structured prefix")
	assert.NotEmpty(t, mockBus.lastInboundMsg.MessageID, "MessageID should be set")
	assert.NotEmpty(t, mockBus.lastInboundMsg.TraceId, "TraceId should be set")
}

func TestExecuteJob_SavesExecutionLog(t *testing.T) {
	mockBus := &MockMessageBus{}
	mockStorage := &MockStorage{}
	service := NewService(mockStorage, mockBus, "feishu", "ou_test123")

	job := &JobEntry{
		ID:      "test456",
		Name:    "Test Job",
		Message: "Test message",
		Channel: "feishu",
		To:      "ou_test123",
	}

	service.executeJob(job)

	require.NotNil(t, mockStorage.savedExecutionLog)
	assert.Equal(t, "test456", mockStorage.savedExecutionLog.JobID)
	assert.Equal(t, "Test Job", mockStorage.savedExecutionLog.JobName)
	assert.Equal(t, "success", mockStorage.savedExecutionLog.Status)
	assert.Empty(t, mockStorage.savedExecutionLog.Error)
	assert.WithinDuration(t, time.Now(), mockStorage.savedExecutionLog.ExecutedAt, time.Second)
}

func TestExecuteJob_HandlePublishError(t *testing.T) {
	mockBus := &MockMessageBus{
		publishInboundErr: errors.New("simulated error"),
	}
	mockStorage := &MockStorage{}
	service := NewService(mockStorage, mockBus, "feishu", "ou_test123")

	job := &JobEntry{
		ID:      "error123",
		Name:    "Error Job",
		Message: "Error message",
		Channel: "feishu",
		To:      "ou_test123",
	}

	service.executeJob(job)

	require.NotNil(t, mockStorage.savedExecutionLog)
	assert.Equal(t, "error", mockStorage.savedExecutionLog.Status)
	assert.NotEmpty(t, mockStorage.savedExecutionLog.Error)
	assert.Contains(t, mockStorage.savedExecutionLog.Error, "simulated error")
}

func TestExecuteJob_OneTimeJob(t *testing.T) {
	mockBus := &MockMessageBus{}
	mockStorage := &MockStorage{}
	service := NewService(mockStorage, mockBus, "feishu", "ou_test123")

	job := &JobEntry{
		ID:             "onetime123",
		Name:           "[ONCE] One time job",
		Message:        "One time message",
		Channel:        "feishu",
		To:             "ou_test123",
		DeleteAfterRun: true,
		DirectSend:     false, // LLM processing mode
	}

	service.executeJob(job)

	assert.True(t, mockBus.publishedInbound)
	assert.Contains(t, mockBus.lastInboundMsg.Content, "One time message")
}

func TestExecuteJob_DirectSend(t *testing.T) {
	mockBus := &MockMessageBus{}
	mockStorage := &MockStorage{}
	service := NewService(mockStorage, mockBus, "feishu", "ou_test123")

	job := &JobEntry{
		ID:             "direct123",
		Name:           "Direct Send Job",
		Message:        "⏰ 喝水时间到啦！记得补充水分哦~ 🥤",
		Channel:        "feishu",
		To:             "ou_test123",
		DeleteAfterRun: false,
		DirectSend:     true, // Direct send mode
	}

	service.executeJob(job)

	assert.True(t, mockBus.publishedOutbound, "PublishOutbound should be called")
	assert.False(t, mockBus.publishedInbound, "PublishInbound should NOT be called")
	assert.Equal(t, "feishu", mockBus.lastOutboundMsg.ChannelID)
	assert.Equal(t, "ou_test123", mockBus.lastOutboundMsg.RecipientID)
	assert.Equal(t, "⏰ 喝水时间到啦！记得补充水分哦~ 🥤", mockBus.lastOutboundMsg.Content, "Content should be unchanged")
	assert.NotEmpty(t, mockBus.lastOutboundMsg.MessageID, "MessageID should be set")
}

func TestAddJob_WithDirectSend(t *testing.T) {
	mockBus := &MockMessageBus{}
	mockStorage := &MockStorage{}
	service := NewService(mockStorage, mockBus, "feishu", "ou_test123")

	// Test with DirectSend=true
	job1 := service.AddJob("Direct Job", "every", 3600000, "", "Direct message", "feishu", "ou_test123", false, true)
	require.NotNil(t, job1)
	assert.Equal(t, true, job1.DirectSend, "DirectSend should be true")

	// Test with DirectSend=false
	job2 := service.AddJob("LLM Job", "every", 3600000, "", "LLM message", "feishu", "ou_test123", false, false)
	require.NotNil(t, job2)
	assert.Equal(t, false, job2.DirectSend, "DirectSend should be false")
}

func TestLoadJobs_BackwardCompatibility(t *testing.T) {
	mockBus := &MockMessageBus{}
	mockStorage := &MockStorage{
		jobs: []*session.CronJob{
			{
				ID:      "old_job",
				Name:    "Old Job",
				Enabled: true,
				Schedule: map[string]interface{}{
					"kind": "cron",
					"expr": "0 * * * * *",
				},
				Payload: map[string]interface{}{
					"message": "Old message",
					"channel": "feishu",
					"to":      "ou_test123",
					// Note: NO direct_send field
				},
				DeleteAfterRun: false,
			},
		},
	}

	service := NewService(mockStorage, mockBus, "feishu", "ou_test123")
	err := service.LoadJobs()
	require.NoError(t, err)

	jobs := service.ListJobs()
	require.Len(t, jobs, 1)
	assert.Equal(t, false, jobs[0].DirectSend, "DirectSend should default to false for old jobs")
}
