package generator

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/nisrulz/prlogue/internal/collector"
	"github.com/nisrulz/prlogue/internal/types"
)

func TestBuildCommitSummaryPrompt(t *testing.T) {
	c := types.Commit{Hash: "abc123", Subject: "feat: add login", Description: "Adds login flow"}
	prompt := buildCommitSummaryPrompt(c, "+login.go\n-new.go\n")
	for _, want := range []string{"abc123", "feat: add login", "Adds login flow", "+login.go"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q: %q", want, prompt)
		}
	}
}

func TestParseCommitSummary(t *testing.T) {
	content := `Here is the summary:
{"hash":"abc","subject":"feat: x","summary":"Adds x.","key_changes":["added x.go"],"impact":"Improves x."}`
	s, err := parseCommitSummary(content)
	if err != nil {
		t.Fatalf("parseCommitSummary: %v", err)
	}
	if s.Summary != "Adds x." || len(s.KeyChanges) != 1 || s.Impact != "Improves x." {
		t.Errorf("unexpected summary: %+v", s)
	}
}

func TestParseCommitSummary_Fenced(t *testing.T) {
	content := "```json\n{\"subject\":\"feat: x\",\"summary\":\"Adds x.\",\"key_changes\":[],\"impact\":\"\"}\n```"
	s, err := parseCommitSummary(content)
	if err != nil {
		t.Fatalf("parseCommitSummary: %v", err)
	}
	if s.Subject != "feat: x" || s.Summary != "Adds x." {
		t.Errorf("unexpected summary: %+v", s)
	}
}

func TestParseCommitSummary_MissingFields(t *testing.T) {
	if _, err := parseCommitSummary(`{"hash":"a","summary":"only summary"}`); err == nil {
		t.Error("expected error for missing subject")
	}
	if _, err := parseCommitSummary("no json here"); err == nil {
		t.Error("expected error for non-JSON output")
	}
}

func TestFallbackCommitSummary(t *testing.T) {
	c := types.Commit{Hash: "abc123", Subject: "docs: update guide", Description: "See details."}
	s := fallbackCommitSummary(c, "diff --git a/g.md b/g.md\n+++ b/g.md\n")
	if s.Summary != "docs: update guide. See details." {
		t.Errorf("summary = %q", s.Summary)
	}
	if len(s.KeyChanges) != 1 || s.KeyChanges[0] != "g.md" {
		t.Errorf("key changes = %v", s.KeyChanges)
	}
}

func TestChangedPaths(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\n+++ b/a.go\n+++ b/a.go\n+++ b/b.go\n+++ /dev/null\n"
	paths := changedPaths(diff)
	if len(paths) != 2 {
		t.Fatalf("paths = %v, want 2 unique paths", paths)
	}
}

func TestRenderCommitSummaries(t *testing.T) {
	summaries := []types.CommitSummary{
		{Hash: "0123456789abcdef", Subject: "feat: add login", Summary: "Adds login.", KeyChanges: []string{"a.go"}, Impact: "Improves auth."},
	}
	got := renderCommitSummaries(summaries)
	for _, want := range []string{"feat: add login", "0123456", "Adds login.", "a.go", "Improves auth."} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered summary missing %q: %q", want, got)
		}
	}
}

func TestWriteCommitSummaries(t *testing.T) {
	summaries := []types.CommitSummary{{Hash: "a", Subject: "s", Summary: "x"}}
	path, err := writeCommitSummaries(summaries)
	if err != nil {
		t.Fatalf("writeCommitSummaries: %v", err)
	}
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var parsed []types.CommitSummary
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("file is not valid JSON: %v", err)
	}
	if len(parsed) != 1 || parsed[0].Subject != "s" {
		t.Errorf("parsed = %+v", parsed)
	}
}

func TestCommitSummarizer_ValidJSON(t *testing.T) {
	p := &captureProvider{result: `{"subject":"feat: x","summary":"Adds x.","key_changes":["x.go"],"impact":""}`}
	s := NewCommitSummarizer(p, "model", false, nil, 8192)
	commits := []types.Commit{
		{Hash: "abc123", Subject: "feat: x"},
		{Hash: "def456", Subject: "fix: y"},
	}
	summaries, path, err := s.Summarize(context.Background(), commits, nil)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	defer os.Remove(path)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	if summaries[0].Hash != "abc123" || summaries[0].Subject != "feat: x" {
		t.Errorf("summary 0 = %+v", summaries[0])
	}
	if summaries[1].Hash != "def456" {
		t.Errorf("summary 1 hash = %q", summaries[1].Hash)
	}
	if path == "" {
		t.Fatal("expected a JSON file path")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("JSON file missing: %v", err)
	}
}

func TestCommitSummarizer_FallbackOnFailure(t *testing.T) {
	p := &captureProvider{result: "not json at all"}
	s := NewCommitSummarizer(p, "model", false, nil, 8192)
	summaries, _, err := s.Summarize(context.Background(), []types.Commit{{Hash: "abc123", Subject: "feat: x"}}, nil)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Summary != "feat: x" {
		t.Errorf("expected fallback summary, got %+v", summaries)
	}
}

func TestCommitSummarizer_NoCommits(t *testing.T) {
	s := NewCommitSummarizer(&captureProvider{}, "model", false, nil, 8192)
	summaries, path, err := s.Summarize(context.Background(), nil, nil)
	if err != nil || summaries != nil || path != "" {
		t.Errorf("no-commit case: summaries=%v path=%q err=%v", summaries, path, err)
	}
}

func TestCommitSummarizer_ReportsProgress(t *testing.T) {
	p := &captureProvider{result: `{"subject":"feat: x","summary":"Adds x.","key_changes":[],"impact":""}`}
	s := NewCommitSummarizer(p, "model", false, nil, 8192)
	calls := 0
	_, _, err := s.Summarize(context.Background(), []types.Commit{
		{Hash: "abc123", Subject: "feat: x"},
		{Hash: "def456", Subject: "fix: y"},
	}, func() { calls++ })
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 progress calls, got %d", calls)
	}
}

func TestBuildLLMPrompt_UsesCommitSummaries(t *testing.T) {
	input := &GenerateInput{
		DiffStats: DiffStats{Files: 1, Additions: 1},
		BranchCtx: &collector.BranchContext{CurrentBranch: "feat/summaries"},
		OriginalDiffs: []types.FileDiff{{
			Path:  "raw.go",
			Hunks: []types.Hunk{{Content: "+raw\n"}},
		}},
		CommitSummaries: []types.CommitSummary{{Subject: "feat: summarized", Summary: "A summary."}},
	}
	prompt := buildLLMPrompt(input)
	for _, want := range []string{"feat: summarized", "A summary."} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt should contain commit summary %q: %q", want, prompt)
		}
	}
	if strings.Contains(prompt, "raw.go") || strings.Contains(prompt, "Diff:") {
		t.Errorf("prompt should not include raw diff when summaries exist: %q", prompt)
	}
}
