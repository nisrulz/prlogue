package provider

import "context"

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string         `json:"model"`
	Messages    []ChatMessage  `json:"messages"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	NoThink     bool           `json:"-"`
	ExtraBody   map[string]any `json:"-"`
}

type ChatResponse struct {
	Content string
	Model   string
	Usage   TokenUsage
}

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
