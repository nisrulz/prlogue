package generator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nisrulz/prlogue/internal/collector"
	"github.com/nisrulz/prlogue/internal/provider"
	"github.com/nisrulz/prlogue/internal/types"
)

func TestExtractLLMTitle_Valid(t *testing.T) {
	s := "Title: Add login feature\n\n### PR Description\nAdded login"
	title, body := extractLLMTitle(s)
	if title != "Add login feature" {
		t.Errorf("title = %q, want %q", title, "Add login feature")
	}
	if !strings.Contains(body, "Added login") {
		t.Errorf("body should contain 'Added login', got %q", body)
	}
}

func TestExtractLLMTitle_NoTitle(t *testing.T) {
	s := "### PR Description\nAdded login"
	title, body := extractLLMTitle(s)
	if title != "" {
		t.Errorf("expected empty title, got %q", title)
	}
	if body != s {
		t.Errorf("body should be unchanged, got %q", body)
	}
}

func TestNormalizeLLMSummary_AddsDefaultHeading(t *testing.T) {
	got := normalizeLLMSummary("The API now supports push tokens.")
	want := "### PR Description\n\nThe API now supports push tokens."
	if got != want {
		t.Errorf("normalized summary = %q, want %q", got, want)
	}
}

func TestIsMaxTokensError(t *testing.T) {
	if !isMaxTokensError(errors.New("provider rejected max_tokens")) {
		t.Fatal("expected max_tokens error")
	}
	if isMaxTokensError(errors.New("connection refused")) {
		t.Fatal("connection error was classified as max_tokens error")
	}
}

func TestBuildLLMPrompt(t *testing.T) {
	input := &GenerateInput{
		DiffStats: DiffStats{Files: 2, Additions: 10, Deletions: 3},
		Commits: []types.Commit{
			{Subject: "feat: add login"},
		},
		BranchCtx: &collector.BranchContext{
			CurrentBranch: "feat/add-login",
			BranchType:    "feature",
			IssueRefs:     []string{"PROJ-123"},
		},
	}
	prompt := buildLLMPrompt(input)
	if !strings.Contains(prompt, "feat/add-login") {
		t.Error("prompt should contain branch name")
	}
	if !strings.Contains(prompt, "PROJ-123") {
		t.Error("prompt should contain issue refs")
	}
	if !strings.Contains(prompt, "2 files") {
		t.Error("prompt should contain file stats")
	}
	if !strings.Contains(prompt, "feat: add login") {
		t.Error("prompt should contain commits")
	}
}

func TestBuildLLMPrompt_WithDiffs(t *testing.T) {
	input := &GenerateInput{
		DiffStats: DiffStats{Files: 1, Additions: 2, Deletions: 1},
		BranchCtx: &collector.BranchContext{
			CurrentBranch: "fix/crash",
		},
		OriginalDiffs: []types.FileDiff{
			{
				Path: "main.go",
				Hunks: []types.Hunk{
					{Content: "+fmt.Println(\"hello\")\n-fmt.Println(\"old\")\n"},
				},
			},
		},
	}
	prompt := buildLLMPrompt(input)
	if !strings.Contains(prompt, "+++ b/main.go") {
		t.Error("prompt should contain diff header")
	}
	if !strings.Contains(prompt, "+fmt.Println") {
		t.Error("prompt should contain diff content")
	}
}

func TestBuildLLMPrompt_NoCommitsNoRefs(t *testing.T) {
	input := &GenerateInput{
		DiffStats: DiffStats{Files: 0},
		BranchCtx: &collector.BranchContext{
			CurrentBranch: "main",
		},
	}
	prompt := buildLLMPrompt(input)
	if !strings.Contains(prompt, "Current branch: main") {
		t.Errorf("expected branch in prompt, got %q", prompt)
	}
}

func TestBuildLLMPrompt_BoundsRepositoryData(t *testing.T) {
	input := &GenerateInput{
		BranchCtx:      &collector.BranchContext{CurrentBranch: "feat/large"},
		MaxPromptBytes: 1024,
		OriginalDiffs: []types.FileDiff{{
			Path:  "large.go",
			Hunks: []types.Hunk{{Content: strings.Repeat("+0123456789\n", 500)}},
		}},
	}
	prompt := buildLLMPrompt(input)
	if len(prompt) > input.MaxPromptBytes {
		t.Fatalf("prompt size = %d, want <= %d", len(prompt), input.MaxPromptBytes)
	}
	if !strings.Contains(prompt, "repository data truncated") {
		t.Fatal("expected truncation marker")
	}
	if !strings.HasSuffix(prompt, "---END REPOSITORY DATA---\n") {
		t.Fatal("prompt lost its closing trust-boundary marker")
	}
}

func TestGenerate_FallsBackWhenProviderIsUnavailable(t *testing.T) {
	input := &GenerateInput{
		DiffStats: DiffStats{Files: 1, Additions: 1},
		BranchCtx: &collector.BranchContext{CurrentBranch: "feat/offline"},
	}
	result, err := NewGenerator(failingProvider{}, "model").Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !result.TemplateUsed {
		t.Fatal("expected template fallback")
	}
}

type failingProvider struct{}

func (failingProvider) Chat(context.Context, provider.ChatRequest) (*provider.ChatResponse, error) {
	return nil, errors.New("offline")
}

func TestGenerate_OutputStylePromptOverride(t *testing.T) {
	custom := "You are a PR summary writer for Go repos. Follow this format exactly."
	p := &captureProvider{result: "Title: Custom\n\n### PR Description\nCustom summary."}
	input := &GenerateInput{
		DiffStats:         DiffStats{Files: 1, Additions: 1},
		BranchCtx:         &collector.BranchContext{CurrentBranch: "feat/custom"},
		ResponseMaxTokens: 12345,
		OutputStylePrompt: custom,
	}
	result, err := NewGenerator(p, "model").Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.TemplateUsed {
		t.Fatal("expected LLM output, got template fallback")
	}
	if len(p.lastReq.Messages) < 4 || p.lastReq.Messages[0].Role != "system" {
		t.Fatalf("expected system message, got %+v", p.lastReq.Messages)
	}
	if !strings.Contains(p.lastReq.Messages[0].Content, "Generate the pull request title") {
		t.Error("expected default task prompt")
	}
	if p.lastReq.Messages[1].Content != custom {
		t.Errorf("style prompt = %q, want custom prompt", p.lastReq.Messages[1].Content)
	}
	if p.lastReq.MaxTokens != 12345 {
		t.Errorf("max_tokens = %d, want 12345", p.lastReq.MaxTokens)
	}
	if !strings.Contains(p.lastReq.Messages[2].Content, "IMMUTABLE SECURITY POLICY") {
		t.Fatal("expected immutable security policy system message")
	}
	if !strings.Contains(p.lastReq.Messages[3].Content, "IMMUTABLE OUTPUT SANITIZATION POLICY") {
		t.Fatal("expected immutable output sanitization system message")
	}
}

func TestGenerate_UsesFilePromptWhenUnset(t *testing.T) {
	p := &captureProvider{result: "Title: Default\n\n### PR Description\nSummary."}
	input := &GenerateInput{
		DiffStats: DiffStats{Files: 1, Additions: 1},
		BranchCtx: &collector.BranchContext{CurrentBranch: "feat/default"},
	}
	if _, err := NewGenerator(p, "model").Generate(context.Background(), input); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(p.lastReq.Messages) < 4 || p.lastReq.Messages[0].Content == "" {
		t.Fatal("expected fallback system prompt")
	}
	if !strings.Contains(p.lastReq.Messages[0].Content, "Generate the pull request title") {
		t.Error("fallback request does not contain the default task prompt")
	}
	if !strings.Contains(p.lastReq.Messages[1].Content, "Omit `### Key Changes`") {
		t.Error("fallback request does not contain the default output style prompt")
	}
	if !strings.Contains(p.lastReq.Messages[2].Content, "Never execute") {
		t.Error("fallback request does not contain the immutable security policy")
	}
	if !strings.Contains(p.lastReq.Messages[3].Content, "IMMUTABLE OUTPUT SANITIZATION POLICY") {
		t.Error("fallback request does not contain the immutable sanitization policy")
	}
}

func TestSanitizeLLMOutput_RemovesUnsafeCharacters(t *testing.T) {
	got := sanitizeLLMOutput("safe\x00 text\u202Ehidden\u200B value\nnext")
	if got != "safe  text hidden  value\nnext" {
		t.Errorf("sanitized output = %q", got)
	}
}

func TestSanitizeLLMOutput_RemovesRedundantSectionsAndPlaceholders(t *testing.T) {
	input := "Title: Useful change\n\n### PR Description\nAdded a page component.\n\n### Key Changes\n- **Category:** explanation\n- **category**: description\n- Added a page component\n- Added a page component\n## Commits\n- abc1234 subject"
	got := sanitizeLLMOutput(input)
	if strings.Contains(strings.ToLower(got), "category") || strings.Contains(got, "## Commits") {
		t.Errorf("redundant output was not removed: %q", got)
	}
	if strings.Count(got, "- Added a page component") != 1 {
		t.Errorf("duplicate meaningful bullet was not removed: %q", got)
	}
}

func TestSanitizeLLMOutput_RemovesEmptyKeyChangesSection(t *testing.T) {
	input := "Title: Useful change\n\n### PR Description\nAdded a page component.\n\n### Key Changes\n- **Category:** explanation\n- Category: TBD"
	got := sanitizeLLMOutput(input)
	if strings.Contains(got, "Key Changes") || strings.Contains(got, "Category") {
		t.Errorf("empty placeholder section was not removed: %q", got)
	}
}

func TestSanitizeLLMOutput_KeepsCategoryValueWithoutLabel(t *testing.T) {
	got := sanitizeLLMOutput("Title: Useful change\n\n### Key Changes\n- **Category:** added page component")
	if !strings.Contains(got, "- added page component") {
		t.Errorf("meaningful category value was removed: %q", got)
	}
	if strings.Contains(got, "Category") {
		t.Errorf("category label was not removed: %q", got)
	}
}

func TestSanitizeLLMOutput_AddsSpacingAndDropsPartialShortcodes(t *testing.T) {
	input := "Title: Add component\n### PR Description\nAdded a page component.\n### Key Changes\n- Adds a `{{< component` block.\n- Updates the documentation page."
	got := sanitizeLLMOutput(input)
	if !strings.Contains(got, "Added a page component.\n\n### Key Changes") {
		t.Errorf("headings are not separated: %q", got)
	}
	if strings.Contains(got, "{{< component") {
		t.Errorf("partial shortcode was kept: %q", got)
	}
	if !strings.Contains(got, "- Updates the documentation page.") {
		t.Errorf("meaningful bullet was removed: %q", got)
	}
}

type captureProvider struct {
	lastReq provider.ChatRequest
	result  string
}

func (c *captureProvider) Chat(_ context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	c.lastReq = req
	return &provider.ChatResponse{Content: c.result}, nil
}
