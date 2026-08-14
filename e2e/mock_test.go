//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

// completionContent mirrors the format a real model returns for the default
// output style prompt: a title line followed by a PR description and key
// changes, each under a heading with direct content.
const completionContent = `Title: CI test PR

### PR Description
Mock summary of the PR changes.

### Key Changes
- Mock generated description.`

// commitSummaryContent is a valid per-commit summary for the summarizer.
const commitSummaryContent = `{"subject":"commit","summary":"Mock commit summary.","key_changes":["file.go"],"impact":"Mock impact."}`

func startMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{{"id": "ci-mock"}},
		})
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		content := completionContent
		body, _ := io.ReadAll(r.Body)
		if isCommitSummaryRequest(body) {
			content = commitSummaryContent
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":    "ci-mock-completion",
			"model": "ci-mock",
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": content}},
			},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	})
	return httptest.NewServer(mux)
}

func isCommitSummaryRequest(body []byte) bool {
	var req struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	for _, m := range req.Messages {
		if strings.Contains(m.Content, "Reply with exactly one JSON object") {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(data)
}
