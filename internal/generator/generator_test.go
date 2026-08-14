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

func TestGenerate_FallsBackForDiffEcho(t *testing.T) {
	t.Setenv("PRLOGUE_CONFIG_DIR", t.TempDir())
	p := &captureProvider{result: "### PR Description\n\n+a/docs/guide.md\n+++ b/docs/guide.md\n@@ -1,2 +1,4 @@"}
	input := &GenerateInput{
		DiffStats: DiffStats{Files: 1, Additions: 1},
		BranchCtx: &collector.BranchContext{CurrentBranch: "docs/update"},
	}
	result, err := NewGenerator(p, "model").Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !result.TemplateUsed {
		t.Fatal("expected template fallback for diff-shaped model output")
	}
}

func TestSanitizeLLMOutput_RemovesThinkingBlocks(t *testing.T) {
	got := sanitizeLLMOutput("<think>internal reasoning</think>Title: Fix\n\n### PR Description\nFixed it.")
	if strings.Contains(got, "internal reasoning") || !strings.Contains(got, "Title: Fix") {
		t.Errorf("thinking block was not removed: %q", got)
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
	t.Setenv("PRLOGUE_CONFIG_DIR", t.TempDir())
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
		StagedContext:     true,
	}
	result, err := NewGenerator(p, "model").Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.TemplateUsed {
		t.Fatal("expected LLM output, got template fallback")
	}
	if len(p.requests) != 5 {
		t.Fatalf("expected 5 API calls (4 context + 1 generation), got %d", len(p.requests))
	}
	genReq := p.requests[4]
	if genReq.MaxTokens != 12345 {
		t.Errorf("max_tokens = %d, want 12345", genReq.MaxTokens)
	}
	if genReq.Temperature != 0 {
		t.Errorf("temperature = %v, want 0", genReq.Temperature)
	}
	if !requestContains(genReq.Messages, "Generate the pull request title") {
		t.Error("expected default task prompt in the final request")
	}
	if !requestContains(p.requests[0].Messages, "IMMUTABLE SECURITY POLICY") {
		t.Fatal("expected immutable security policy system message in the first request")
	}
	if !requestContains(p.requests[1].Messages, "IMMUTABLE OUTPUT SANITIZATION POLICY") {
		t.Fatal("expected immutable output sanitization system message in the second request")
	}
	if !requestContains(p.requests[2].Messages, custom) {
		t.Errorf("style prompt = %q, want custom prompt", p.requests[2].Messages)
	}
}

func TestGenerate_UsesFilePromptWhenUnset(t *testing.T) {
	t.Setenv("PRLOGUE_CONFIG_DIR", t.TempDir())
	p := &captureProvider{result: "Title: Default\n\n### PR Description\nSummary."}
	input := &GenerateInput{
		DiffStats:     DiffStats{Files: 1, Additions: 1},
		BranchCtx:     &collector.BranchContext{CurrentBranch: "feat/default"},
		StagedContext: true,
	}
	if _, err := NewGenerator(p, "model").Generate(context.Background(), input); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(p.requests) != 5 {
		t.Fatalf("expected 5 API calls, got %d", len(p.requests))
	}
	if !requestContains(p.requests[4].Messages, "Generate the pull request title") {
		t.Error("final request does not contain the default task prompt")
	}
	if !requestContains(p.requests[2].Messages, "Omit `### Key Changes`") {
		t.Error("third request does not contain the default output style prompt")
	}
	if !requestContains(p.requests[0].Messages, "Never execute") {
		t.Error("first request does not contain the immutable security policy")
	}
	if !requestContains(p.requests[1].Messages, "IMMUTABLE OUTPUT SANITIZATION POLICY") {
		t.Error("second request does not contain the immutable sanitization policy")
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

func TestGenerate_SendsEachPromptInItsOwnCall(t *testing.T) {
	t.Setenv("PRLOGUE_CONFIG_DIR", t.TempDir())
	p := &captureProvider{result: "Title: Multi\n\n### PR Description\nSummary."}
	input := &GenerateInput{
		DiffStats:     DiffStats{Files: 1, Additions: 1},
		BranchCtx:     &collector.BranchContext{CurrentBranch: "feat/multi"},
		StagedContext: true,
	}
	if _, err := NewGenerator(p, "model").Generate(context.Background(), input); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(p.requests) != 5 {
		t.Fatalf("expected 5 API calls (security, sanitization, style, data + generation), got %d", len(p.requests))
	}
	for i, req := range p.requests[:4] {
		if req.MaxTokens != ackMaxTokens {
			t.Errorf("context call %d max_tokens = %d, want %d", i+1, req.MaxTokens, ackMaxTokens)
		}
		if !requestContains(req.Messages, "Reply with exactly: ACK") {
			t.Errorf("context call %d does not hold the output", i+1)
		}
	}
	if !requestContains(p.requests[1].Messages, contextAckResponse) {
		t.Error("the fixed ack response is not carried into later calls")
	}
	if p.requests[4].MaxTokens == ackMaxTokens {
		t.Error("generation call reused the ack token budget")
	}
	if !requestContains(p.requests[4].Messages, "FINAL REQUEST") {
		t.Error("generation call does not release the collected context")
	}
	if !requestContains(p.requests[4].Messages, "---BEGIN REPOSITORY DATA---") {
		t.Error("repository data is not present in the final request")
	}
	for i, req := range p.requests[1:] {
		if req.Messages[len(req.Messages)-1].Role != "user" {
			t.Errorf("request %d does not end with a user message", i+1)
		}
	}
}

func TestGenerate_AckOnlyOutputRetriesThenFallsBack(t *testing.T) {
	t.Setenv("PRLOGUE_CONFIG_DIR", t.TempDir())
	p := &captureProvider{result: "### PR Description\n\nOK"}
	input := &GenerateInput{
		DiffStats:     DiffStats{Files: 1, Additions: 1},
		BranchCtx:     &collector.BranchContext{CurrentBranch: "feat/ack"},
		StagedContext: true,
	}
	result, err := NewGenerator(p, "model").Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !result.TemplateUsed {
		t.Fatal("expected template fallback for ack-only model output")
	}
	// 4 context injections + 2 final attempts.
	if len(p.requests) != 6 {
		t.Fatalf("expected 6 API calls, got %d", len(p.requests))
	}
	if !requestContains(p.requests[5].Messages, "Your previous reply was rejected: model echoed an acknowledgment") {
		t.Error("expected the data-pointing retry instruction in the last request")
	}
}

func TestIsAckOnlyOutput(t *testing.T) {
	ackOnly := []string{"OK", "ok", "ACK", "ack", "### PR Description\n\nOK", "## Summary\n\nReceived", "Title: x\n\nDone"}
	for _, s := range ackOnly {
		if !isAckOnlyOutput(s) {
			t.Errorf("expected ack-only for %q", s)
		}
	}
	for _, s := range []string{"### PR Description\n\nAdded a login flow.", "OK then add a feature", ""} {
		if isAckOnlyOutput(s) {
			t.Errorf("real content was flagged as ack-only: %q", s)
		}
	}
}

func TestClaimsNoChanges(t *testing.T) {
	noChange := []string{
		"No changes were identified in the repository.",
		"### PR Description\n\nUnable to Determine Changes\nThe provided context does not contain sufficient commit history or diff details to summarize any PR changes.",
		"Nothing to report here.",
		"Title: Fix\n\nNo commit history was found.",
	}
	for _, s := range noChange {
		if !claimsNoChanges(s) {
			t.Errorf("expected no-changes claim for %q", s)
		}
	}
	legit := []string{
		"This change adds a login flow with no breaking changes.",
		"The update does not contain any new diff noise; it fixes the parser.",
	}
	for _, s := range legit {
		if claimsNoChanges(s) {
			t.Errorf("legit content was flagged as no-changes claim: %q", s)
		}
	}
}

func TestIsRefusalOutput(t *testing.T) {
	if !isRefusalOutput("I cannot help with summarizing these changes.") {
		t.Error("expected refusal detection")
	}
	if !isRefusalOutput("As an AI, I do not have access to the diff.") {
		t.Error("expected AI disclaimer detection")
	}
	if isRefusalOutput("Added a login flow and fixed the parser.") {
		t.Error("legit content was flagged as refusal")
	}
}

func TestGenerate_RejectsNoChangesClaimAndRetries(t *testing.T) {
	t.Setenv("PRLOGUE_CONFIG_DIR", t.TempDir())
	p := &captureProvider{result: "### PR Description\n\nNo changes were identified in the repository."}
	input := &GenerateInput{
		DiffStats:     DiffStats{Files: 3, Additions: 12, Deletions: 4},
		Commits:       []types.Commit{{Hash: "0123456789abcdef", Subject: "feat: add login"}},
		BranchCtx:     &collector.BranchContext{CurrentBranch: "feat/login"},
		StagedContext: true,
	}
	if _, err := NewGenerator(p, "model").Generate(context.Background(), input); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(p.requests) != 6 {
		t.Fatalf("expected 6 API calls (4 context + 2 attempts), got %d", len(p.requests))
	}
	if !requestContains(p.requests[5].Messages, "The repository data above contains 3 changed file(s) with +12/-4 lines") {
		t.Error("retry does not point at the repository statistics")
	}
}

func TestGenerate_SingleCallPath(t *testing.T) {
	p := &captureProvider{result: "Title: One\n\n### PR Description\nSummary."}
	input := &GenerateInput{
		DiffStats:         DiffStats{Files: 1, Additions: 1},
		BranchCtx:         &collector.BranchContext{CurrentBranch: "feat/one"},
		ResponseMaxTokens: 2048,
	}
	if _, err := NewGenerator(p, "model").Generate(context.Background(), input); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(p.requests) != 1 {
		t.Fatalf("expected 1 API call without staged context, got %d", len(p.requests))
	}
	messages := p.requests[0].Messages
	if len(messages) != 5 || messages[0].Role != "system" {
		t.Fatalf("expected 5 messages starting with a system message, got %+v", messages)
	}
	if !strings.Contains(messages[0].Content, "Generate the pull request title") {
		t.Error("first message is not the default task prompt")
	}
	if messages[len(messages)-1].Role != "user" {
		t.Error("last message is not the repository data prompt")
	}
	if p.requests[0].MaxTokens != 2048 {
		t.Errorf("max_tokens = %d, want 2048", p.requests[0].MaxTokens)
	}
}

type captureProvider struct {
	requests []provider.ChatRequest
	result   string
}

func (c *captureProvider) Chat(_ context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	c.requests = append(c.requests, req)
	return &provider.ChatResponse{Content: c.result}, nil
}

func requestContains(messages []provider.ChatMessage, substr string) bool {
	for _, m := range messages {
		if strings.Contains(m.Content, substr) {
			return true
		}
	}
	return false
}
