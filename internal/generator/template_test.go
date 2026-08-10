package generator

import (
	"strings"
	"testing"

	"github.com/nisrulz/prlogue/internal/collector"
	"github.com/nisrulz/prlogue/internal/types"
)

func TestTemplateGenerate_Basic(t *testing.T) {
	tmpl := &TemplateGenerator{}
	input := &GenerateInput{
		DiffStats: DiffStats{Files: 2, Additions: 10, Deletions: 3},
		Commits: []types.Commit{
			{Hash: "abc123def", Subject: "feat: add login"},
			{Hash: "def456ghi", Subject: "fix: crash"},
		},
		Merged: []types.MergedSummary{
			{FilePath: "login.go", ChangeType: "feat", Summary: "added login handler"},
			{FilePath: "crash.go", ChangeType: "fix", Summary: "fixed null pointer"},
		},
		BranchCtx: &collector.BranchContext{
			CurrentBranch: "feat/add-features",
			IssueRefs:     []string{"PROJ-123"},
		},
	}
	result := tmpl.Generate(input)
	if result.Title != "feat/add-features" {
		t.Errorf("title = %q", result.Title)
	}
	if !result.TemplateUsed {
		t.Error("expected TemplateUsed = true")
	}
	if !strings.Contains(result.Body, "2 file(s)") {
		t.Error("body should contain file count")
	}
	if !strings.Contains(result.Body, "added login handler") {
		t.Error("body should contain merged summary")
	}
	if !strings.Contains(result.CommitDesc, "abc123d") {
		t.Error("commit desc should contain short hash")
	}
}

func TestTemplateGenerate_EmptyInput(t *testing.T) {
	tmpl := &TemplateGenerator{}
	input := &GenerateInput{
		BranchCtx: &collector.BranchContext{CurrentBranch: "main"},
	}
	result := tmpl.Generate(input)
	if result.Body != "" {
		t.Errorf("expected empty body, got %q", result.Body)
	}
}

func TestUniqueChangeTypes(t *testing.T) {
	merged := []types.MergedSummary{
		{ChangeType: "feat"},
		{ChangeType: "fix"},
		{ChangeType: "feat"},
	}
	types := uniqueChangeTypes(merged)
	if len(types) != 2 {
		t.Fatalf("expected 2 unique types, got %d", len(types))
	}
}

func TestFormatTypeList(t *testing.T) {
	tests := []struct {
		types []string
		want  string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"feat"}, "feat"},
		{[]string{"feat", "fix"}, "feat and fix"},
		{[]string{"feat", "fix", "chore"}, "feat, fix and chore"},
	}
	for _, tt := range tests {
		got := formatTypeList(tt.types)
		if got != tt.want {
			t.Errorf("formatTypeList(%v) = %q, want %q", tt.types, got, tt.want)
		}
	}
}

func TestJoinSubjects(t *testing.T) {
	tests := []struct {
		commits []types.Commit
		want    string
	}{
		{nil, ""},
		{[]types.Commit{{Subject: "feat: x"}}, "feat: x"},
		{[]types.Commit{{Subject: "feat: x"}, {Subject: "fix: y"}}, "feat: x; fix: y"},
	}
	for _, tt := range tests {
		got := joinSubjects(tt.commits)
		if got != tt.want {
			t.Errorf("joinSubjects(%v) = %q, want %q", tt.commits, got, tt.want)
		}
	}
}

func TestChangeTypeSection(t *testing.T) {
	tests := []struct {
		ct   string
		want string
	}{
		{"feat", "Features"},
		{"fix", "Bug Fixes"},
		{"refactor", "Refactoring"},
		{"docs", "Documentation"},
		{"test", "Tests"},
		{"chore", "Chores"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		got := changeTypeSection(tt.ct)
		if got != tt.want {
			t.Errorf("changeTypeSection(%q) = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestCommitList(t *testing.T) {
	commits := []types.Commit{
		{Hash: "abc123def456", Subject: "feat: x"},
	}
	result := commitList(commits)
	if !strings.Contains(result, "abc123d") {
		t.Error("expected short hash in commit list")
	}
	if !strings.Contains(result, "feat: x") {
		t.Error("expected subject in commit list")
	}
}

func TestCommitList_Empty(t *testing.T) {
	if s := commitList(nil); s != "" {
		t.Errorf("expected empty, got %q", s)
	}
}
