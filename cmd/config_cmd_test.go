package cmd

import (
	"testing"

	"github.com/nisrulz/prlogue/internal/config"
)

func TestConfigValue(t *testing.T) {
	cfg := &config.Config{Name: "Ollama", Provider: "openai_compat", ResponseMaxTokens: 8192, APIKey: "secret"}
	if got, ok := configValue(cfg, "name"); !ok || got != "Ollama" {
		t.Fatalf("name = %q, %v", got, ok)
	}
	if got, ok := configValue(cfg, "provider"); !ok || got != "openai_compat" {
		t.Fatalf("provider = %q, %v", got, ok)
	}
	if got, ok := configValue(cfg, "api_key"); !ok || got != "*** (PRLOGUE_OPENAI_COMPAT_API_KEY)" {
		t.Fatalf("api_key was not redacted: %q, %v", got, ok)
	}
	if got, ok := configValue(cfg, "response_max_tokens"); !ok || got != "8192" {
		t.Fatalf("response_max_tokens = %q, %v", got, ok)
	}
	if got, ok := configValue(cfg, "output_style_prompt"); !ok || got != "" {
		t.Fatalf("output_style_prompt = %q, %v", got, ok)
	}
	if _, ok := configValue(cfg, "missing"); ok {
		t.Fatal("unknown config key was accepted")
	}
}
