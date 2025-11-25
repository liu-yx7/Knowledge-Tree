package ai

import (
	"context"
)

// Provider defines the interface for LLM providers
type Provider interface {
	Name() string
	SendMessage(ctx context.Context, req ChatRequest) (ChatResponse, error)
	StreamMessage(ctx context.Context, req ChatRequest) (<-chan string, <-chan error)
	GetModels() []string
}

// ChatRequest represents a request to send a message to an LLM
type ChatRequest struct {
	Messages     []Message
	Model        string
	SystemPrompt string
	Temperature  float32
	MaxTokens    int32
}

// ChatResponse represents a response from an LLM
type ChatResponse struct {
	Content string
	Tokens  int32
}

// Message represents a single message in a conversation
type Message struct {
	Role    string // "user" or "assistant"
	Content string
}
