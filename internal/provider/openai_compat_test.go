package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStripThinkBlocks(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello <think>deep thought</think> world", "Hello  world"},
		{"<think>nested</think>only after", "only after"},
		{"only before <think>nested</think>", "only before"},
		{"no think blocks", "no think blocks"},
		{"<think>unclosed", ""},
		{"<think>one</think>middle<think>two</think>end", "middleend"},
		{"</think><think>out of order</think>tail", "</think>tail"},
		{"</think><think>unclosed", "</think>"},
	}
	for _, tt := range tests {
		got := stripThinkBlocks(tt.input)
		if got != tt.want {
			t.Errorf("stripThinkBlocks(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestChat_ExtraBodyRejectsProtectedFields(t *testing.T) {
	p := NewOpenAICompatible("http://localhost:1234/v1", "", "test-model")
	_, err := p.Chat(context.Background(), ChatRequest{
		Messages:  []ChatMessage{{Role: "user", Content: "hi"}},
		ExtraBody: map[string]any{"max_tokens": 100000},
	})
	if err == nil {
		t.Fatal("expected protected extra_body field to be rejected")
	}
}

func TestNewOpenAICompatible_DisablesRedirects(t *testing.T) {
	p := NewOpenAICompatible("https://example.com/v1", "", "model")
	if p.httpClient.CheckRedirect == nil {
		t.Fatal("redirects are enabled")
	}
	if err := p.httpClient.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Fatalf("CheckRedirect error = %v", err)
	}
}

func TestBoundedTransport_LimitsResponseBody(t *testing.T) {
	transport := boundedTransport{base: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", maxResponseBytes+100))),
		}, nil
	})}
	resp, err := transport.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != maxResponseBytes+1 {
		t.Fatalf("bounded body size = %d", len(body))
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestChat_ExtraBody_SendsAPIKeyAndExtras(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"done"}}],"model":"m","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatible(srv.URL, "secret-key-123", "test-model")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
		MaxTokens: 10,
		ExtraBody: map[string]any{"reasoning_effort": "high"},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if gotAuth != "Bearer secret-key-123" {
		t.Errorf("Authorization = %q, want Bearer secret-key-123", gotAuth)
	}
	if gotBody["reasoning_effort"] != "high" {
		t.Errorf("extra_body missing from request: %v", gotBody)
	}
	if gotBody["model"] != "test-model" {
		t.Errorf("model = %v, want test-model", gotBody["model"])
	}
	if resp.Content != "done" {
		t.Errorf("Content = %q, want done", resp.Content)
	}
}
