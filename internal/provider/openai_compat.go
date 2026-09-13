package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const (
	maxResponseBytes = 4 << 20
	maxErrorBytes    = 8 << 10
)

type OpenAICompatible struct {
	client     *openai.Client
	model      string
	baseURL    string
	apiKey     string
	httpClient *http.Client
	retry      retryPolicy
}

func NewOpenAICompatible(baseURL, apiKey, model string) *OpenAICompatible {
	return NewOpenAICompatibleWithHTTP(baseURL, apiKey, model, &http.Client{Timeout: 120 * time.Second})
}

func NewOpenAICompatibleWithHTTP(baseURL, apiKey, model string, httpClient *http.Client) *OpenAICompatible {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	clientCopy := *httpClient
	if clientCopy.CheckRedirect == nil {
		clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	transport := clientCopy.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clientCopy.Transport = boundedTransport{base: transport}
	httpClient = &clientCopy
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	config.HTTPClient = httpClient
	client := openai.NewClientWithConfig(config)
	return &OpenAICompatible{
		client:     client,
		model:      model,
		baseURL:    config.BaseURL,
		apiKey:     apiKey,
		httpClient: httpClient,
		retry:      defaultRetryPolicy(),
	}
}

type boundedTransport struct {
	base http.RoundTripper
}

func (t boundedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Body = struct {
		io.Reader
		io.Closer
	}{Reader: io.LimitReader(resp.Body, maxResponseBytes+1), Closer: resp.Body}
	return resp, nil
}

func (p *OpenAICompatible) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if err := validateExtraBody(req.ExtraBody); err != nil {
		return nil, err
	}
	apiReq := buildChatRequest(p.model, req)

	resp, err := p.completeWithRetry(ctx, apiReq, req)
	if err != nil {
		return nil, err
	}
	if req.NoThink {
		resp.Content = stripThinkBlocks(resp.Content)
	}
	return resp, nil
}

func buildChatRequest(fallbackModel string, req ChatRequest) openai.ChatCompletionRequest {
	model := req.Model
	if model == "" {
		model = fallbackModel
	}
	apiMessages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, m := range req.Messages {
		apiMessages[i] = openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return openai.ChatCompletionRequest{
		Model:       model,
		Messages:    apiMessages,
		MaxTokens:   req.MaxTokens,
		Temperature: float32(req.Temperature),
	}
}

// completeWithRetry retries transient provider failures with backoff. It stops
// early when the caller context is done or the failure is permanent.
func (p *OpenAICompatible) completeWithRetry(ctx context.Context, apiReq openai.ChatCompletionRequest, req ChatRequest) (*ChatResponse, error) {
	attempts := p.retry.maxAttempts
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := waitBackoff(ctx, p.retry, attempt); err != nil {
				return nil, lastErr
			}
		}
		resp, err := p.complete(ctx, apiReq, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryable(err) || ctx.Err() != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (p *OpenAICompatible) complete(ctx context.Context, apiReq openai.ChatCompletionRequest, req ChatRequest) (*ChatResponse, error) {
	if len(req.ExtraBody) > 0 {
		return p.chatWithExtraBody(ctx, apiReq, req.ExtraBody)
	}
	resp, err := p.client.CreateChatCompletion(ctx, apiReq)
	if err != nil {
		return nil, fmt.Errorf("chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	return &ChatResponse{
		Content: resp.Choices[0].Message.Content,
		Model:   resp.Model,
		Usage: TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// ListModels returns the model IDs advertised by the endpoint. Some
// OpenAI-compatible servers only list models that are currently loaded.
func (p *OpenAICompatible) ListModels(ctx context.Context) ([]string, error) {
	resp, err := p.client.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(resp.Models))
	for _, m := range resp.Models {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

func (p *OpenAICompatible) chatWithExtraBody(ctx context.Context, baseReq openai.ChatCompletionRequest, extra map[string]any) (*ChatResponse, error) {
	bodyMap := make(map[string]any)

	data, err := json.Marshal(baseReq)
	if err != nil {
		return nil, fmt.Errorf("marshal base request: %w", err)
	}
	if err := json.Unmarshal(data, &bodyMap); err != nil {
		return nil, fmt.Errorf("build request body: %w", err)
	}

	for k, v := range extra {
		bodyMap[k] = v
	}

	bodyJSON, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(p.baseURL, "/") + "/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(respBody) > maxResponseBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
	}

	if httpResp.StatusCode != 200 {
		if len(respBody) > maxErrorBytes {
			respBody = respBody[:maxErrorBytes]
		}
		return nil, &HTTPError{StatusCode: httpResp.StatusCode, Body: string(respBody)}
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &ChatResponse{
		Content: result.Choices[0].Message.Content,
		Model:   result.Model,
		Usage: TokenUsage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}, nil
}

func stripThinkBlocks(s string) string {
	const (
		openTag  = "<think>"
		closeTag = "</think>"
	)
	var b strings.Builder
	remaining := s
	for {
		start := strings.Index(remaining, openTag)
		if start == -1 {
			b.WriteString(remaining)
			break
		}
		b.WriteString(remaining[:start])
		afterOpen := remaining[start+len(openTag):]
		relativeEnd := strings.Index(afterOpen, closeTag)
		if relativeEnd == -1 {
			break
		}
		remaining = afterOpen[relativeEnd+len(closeTag):]
	}
	return strings.TrimSpace(b.String())
}

func validateExtraBody(extra map[string]any) error {
	for key := range extra {
		switch key {
		case "model", "messages", "max_tokens", "temperature", "stream":
			return fmt.Errorf("extra_body must not override protected field %q", key)
		}
	}
	return nil
}
