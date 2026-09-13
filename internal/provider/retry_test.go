package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func fastRetry() retryPolicy {
	return retryPolicy{maxAttempts: 3, baseDelay: time.Millisecond, maxDelay: 2 * time.Millisecond}
}

func TestChat_RetriesTransientServerError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":{"message":"busy"}}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"model":"m"}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatible(srv.URL, "", "model")
	p.retry = fastRetry()
	resp, err := p.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("Content = %q, want ok", resp.Content)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2", got)
	}
}

func TestChat_DoesNotRetryClientError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatible(srv.URL, "", "model")
	p.retry = fastRetry()
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}}); err == nil {
		t.Fatal("expected client error")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 400)", got)
	}
}

func TestChat_RetriesExtraBodyServerError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("slow down"))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"model":"m"}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatible(srv.URL, "", "model")
	p.retry = fastRetry()
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages:  []ChatMessage{{Role: "user", Content: "hi"}},
		ExtraBody: map[string]any{"reasoning_effort": "high"},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("Content = %q, want ok", resp.Content)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2", got)
	}
}

func TestRetryableStatus(t *testing.T) {
	retryable := []int{408, 409, 425, 429, 500, 502, 503, 504}
	for _, code := range retryable {
		if !retryableStatus(code) {
			t.Errorf("status %d should be retryable", code)
		}
	}
	for _, code := range []int{400, 401, 403, 404, 422} {
		if retryableStatus(code) {
			t.Errorf("status %d should not be retryable", code)
		}
	}
}
