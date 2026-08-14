package formatter_test

import (
	"strings"
	"testing"

	"github.com/nisrulz/prlogue/internal/collector"
	"github.com/nisrulz/prlogue/internal/formatter"
	"github.com/nisrulz/prlogue/internal/generator"
	"github.com/nisrulz/prlogue/internal/types"
)

func TestFormatMarkdown_LLMMode(t *testing.T) {
	result := &generator.GenerateResult{
		Title:   "feat/add-login",
		Summary: "### PR Description\n\nAdded login feature\n### Key Changes\n- login handler",
		Raw:     "full output",
	}
	input := &generator.GenerateInput{
		DiffStats: generator.DiffStats{Files: 1, Additions: 10, Deletions: 0},
		Commits: []types.Commit{
			{Hash: "abc1234", Subject: "feat: add login"},
		},
		BranchCtx: &collector.BranchContext{
			CurrentBranch: "feat/add-login",
			DefaultBranch: "main",
		},
	}
	output := formatter.FormatMarkdown(result, input)
	if !strings.Contains(output, "# feat/add-login") {
		t.Error("expected title in markdown output")
	}
	if !strings.Contains(output, "Added login feature") {
		t.Error("expected summary in markdown output")
	}
	if strings.Contains(output, "abc1234") {
		t.Error("LLM markdown output must not append commit details")
	}
}

func TestFormatMarkdown_TemplateMode(t *testing.T) {
	result := &generator.GenerateResult{
		Title:        "feat/add-login",
		Body:         "### PR Description\nTemplate body",
		TemplateUsed: true,
		CommitDesc:   "## Commits\n- abc123 feat",
	}
	input := &generator.GenerateInput{
		DiffStats: generator.DiffStats{Files: 1, Additions: 10, Deletions: 5},
		BranchCtx: &collector.BranchContext{
			CurrentBranch: "feat/add-login",
			IssueRefs:     []string{"PROJ-123"},
		},
	}
	output := formatter.FormatMarkdown(result, input)
	if !strings.Contains(output, "Template body") {
		t.Error("expected template body in output")
	}
	if !strings.Contains(output, "PROJ-123") {
		t.Error("expected issue refs in output")
	}
}
