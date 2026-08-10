// Command mock-openai-server serves the small OpenAI-compatible API
// required by live-test.sh.
package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
)

const completionContent = `Title: CI test PR

### PR Description

## Features
- Mock generated description.

## Documentation
- Mock provider response.`

func main() {
	urlFile := os.Args[1]

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
		io.Copy(io.Discard, r.Body)
		writeJSON(w, http.StatusOK, map[string]any{
			"id":    "ci-mock-completion",
			"model": "ci-mock",
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": completionContent}},
			},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	url := "http://127.0.0.1:" + strconv.Itoa(listener.Addr().(*net.TCPAddr).Port) + "/v1\n"
	if err := os.WriteFile(urlFile, []byte(url), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.Serve(listener, mux))
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
