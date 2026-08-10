package cmd

import (
	"strings"
	"testing"

	"github.com/nisrulz/prlogue/internal/collector"
	"github.com/nisrulz/prlogue/internal/config"
	"github.com/nisrulz/prlogue/internal/generator"
)

func TestResolveProviderSettings_UsesConfiguredValues(t *testing.T) {
	cfg := &config.Config{
		Provider: "openai_compat",
		BaseURL:  "http://localhost:11434/v1",
		Model:    "qwen2.5-coder:7b",
	}
	got, err := resolveProviderSettings(cfg)
	if err != nil {
		t.Fatalf("resolveProviderSettings: %v", err)
	}
	if got.name != "openai_compat" || got.baseURL != cfg.BaseURL || got.model != cfg.Model {
		t.Fatalf("unexpected provider settings: %+v", got)
	}
}

func TestResolveProviderSettings_RejectsInsecureRemoteURL(t *testing.T) {
	cfg := &config.Config{Provider: "openai_compat", BaseURL: "http://example.com/v1", Model: "model"}
	if _, err := resolveProviderSettings(cfg); err == nil {
		t.Fatal("expected insecure remote URL error")
	}
}

func TestPromptByteLimit(t *testing.T) {
	if got := promptByteLimit(4096); got != 16<<10 {
		t.Fatalf("small context limit = %d", got)
	}
	if got := promptByteLimit(1000000); got != 1<<20 {
		t.Fatalf("large context limit = %d", got)
	}
}

func TestSanitizeTitle(t *testing.T) {
	got := sanitizeTitle("  title\nwith\tcontrols  ")
	if got != "title with controls" {
		t.Fatalf("sanitizeTitle = %q", got)
	}
	if got := sanitizeTitle(strings.Repeat("a", 300)); len([]rune(got)) != 256 {
		t.Fatalf("title length = %d", len([]rune(got)))
	}
}

func TestFormatOutput_RejectsUnknownFormat(t *testing.T) {
	_, err := formatOutput(
		&generator.GenerateResult{},
		&generator.GenerateInput{BranchCtx: &collector.BranchContext{}},
		"xml",
	)
	if err == nil {
		t.Fatal("expected unknown format error")
	}
}

func TestGenerateCommandRejectsArguments(t *testing.T) {
	if err := generateCmd.Args(generateCmd, []string{"unexpected"}); err == nil {
		t.Fatal("expected positional argument error")
	}
}
