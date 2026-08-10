package formatter_test

import (
	"strings"
	"testing"

	"github.com/nisrulz/prlogue/internal/collector"
	"github.com/nisrulz/prlogue/internal/formatter"
	"github.com/nisrulz/prlogue/internal/generator"
	"github.com/nisrulz/prlogue/internal/types"
)

func TestFormatJSON_LLMMode(t *testing.T) {
	result := &generator.GenerateResult{
		Title:   "feat/add-login",
		Summary: "Added login feature",
		Body:    "Added login feature",
		Raw:     "full raw output",
	}
	input := &generator.GenerateInput{
		DiffStats: generator.DiffStats{Files: 1, Additions: 10, Deletions: 0},
		Commits: []types.Commit{
			{Hash: "abc123def", Subject: "feat: add login", Author: "John"},
		},
		BranchCtx: &collector.BranchContext{},
	}
	output, err := formatter.FormatJSON(result, input)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	if !strings.Contains(output, "abc123d") {
		t.Errorf("expected short hash in json output: %s", output)
	}
	if !strings.Contains(output, "John") {
		t.Errorf("expected author in json output: %s", output)
	}
	if !strings.Contains(output, "Added login feature") {
		t.Errorf("expected summary in json output: %s", output)
	}
}

func TestFormatJSON_TemplateMode(t *testing.T) {
	result := &generator.GenerateResult{
		Title:        "chore/update",
		TemplateUsed: true,
		Body:         "Updated deps",
	}
	input := &generator.GenerateInput{
		DiffStats: generator.DiffStats{Files: 2, Additions: 5, Deletions: 5},
		BranchCtx: &collector.BranchContext{
			IssueRefs: []string{"#42"},
		},
		Merged: []types.MergedSummary{
			{FilePath: "go.mod", ChangeType: "chore", Summary: "bump"},
		},
	}
	output, err := formatter.FormatJSON(result, input)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	if !strings.Contains(output, "#42") {
		t.Errorf("expected issues in json: %s", output)
	}
	if !strings.Contains(output, `"type": "chore"`) {
		t.Errorf("expected change type in json: %s", output)
	}
}
