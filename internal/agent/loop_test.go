package agent

import (
	"testing"

	"github.com/konglong87/wukongbot/internal/config"
)

func TestToolLoopDetection(t *testing.T) {
	// Create a simple agent loop for testing
	cfg := Config{
		Workspace:         "/tmp/test",
		Model:             "test-model",
		MaxTokens:         1000,
		MaxToolIterations: 10,
		Identity:          &config.IdentityConfig{},
	}

	// Create minimal agent loop for testing
	loop := &AgentLoop{
		cfg: cfg,
	}

	// Test case: Tool loop detection is temporarily disabled
	args := map[string]interface{}{"path": "/tmp"}
	result := "files: a, b, c"

	isLoop, loopType := loop.checkToolLoop("list_dir", args, result)
	if isLoop {
		t.Errorf("Test failed: Expected no loop (feature disabled), got %s", loopType)
	}

	if loopType != "" {
		t.Errorf("Test failed: Expected empty loop type, got '%s'", loopType)
	}
}
