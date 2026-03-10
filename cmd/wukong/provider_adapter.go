package main

import (
	"context"

	"github.com/konglong87/wukongbot/internal/agentteam"
	"github.com/konglong87/wukongbot/internal/providers"
)

// agentTeamProviderAdapter adapts providers.LLMProvider to agentteam.LLMProvider
type agentTeamProviderAdapter struct {
	provider providers.LLMProvider
}

func (a *agentTeamProviderAdapter) Chat(ctx context.Context, req agentteam.ChatRequest) (*agentteam.ChatResponse, error) {
	// Convert agentteam request to providers request
	providersReq := providers.ChatRequest{
		Messages:    make([]providers.Message, len(req.Messages)),
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	for i, msg := range req.Messages {
		providersReq.Messages[i] = providers.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Call the actual provider
	resp, err := a.provider.Chat(ctx, providersReq)
	if err != nil {
		return nil, err
	}

	// Convert response back
	return &agentteam.ChatResponse{
		Content:      resp.Content,
		FinishReason: resp.FinishReason,
	}, nil
}
