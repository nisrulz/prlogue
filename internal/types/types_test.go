package types_test

import (
	"testing"

	"github.com/nisrulz/prlogue/internal/types"
)

func TestChangeTypeFromBranch(t *testing.T) {
	tests := []struct {
		branch string
		want   types.ChangeType
	}{
		{"feat/add-login", types.ChangeFeat},
		{"feature/new-ui", types.ChangeFeat},
		{"fix/crash-on-null", types.ChangeFix},
		{"bugfix/issue-42", types.ChangeFix},
		{"hotfix/security", types.ChangeFix},
		{"chore/update-deps", types.ChangeChore},
		{"docs/api-readme", types.ChangeDocs},
		{"refactor/cleanup", types.ChangeRefactor},
		{"refact/simplify", types.ChangeRefactor},
		{"test/add-unit-tests", types.ChangeTest},
		{"main", types.ChangeType("")},
		{"random/thing", types.ChangeType("")},
	}
	for _, tt := range tests {
		got := types.ChangeTypeFromBranch(tt.branch)
		if got != tt.want {
			t.Errorf("ChangeTypeFromBranch(%q) = %q, want %q", tt.branch, got, tt.want)
		}
	}
}

func TestCommitChangeCounts(t *testing.T) {
	subjects := []string{
		"feat: add login",
		"fix: null pointer",
		"feat: add logout",
		"docs: update readme",
		"chore: bump version",
	}
	counts := types.CommitChangeCounts(subjects)
	if counts["feat"] != 2 {
		t.Errorf("feat count = %d, want 2", counts["feat"])
	}
	if counts["fix"] != 1 {
		t.Errorf("fix count = %d, want 1", counts["fix"])
	}
	if counts["docs"] != 1 {
		t.Errorf("docs count = %d, want 1", counts["docs"])
	}
}

func TestMergeGroupByType(t *testing.T) {
	merged := []types.MergedSummary{
		{FilePath: "a.go", ChangeType: "feat", Summary: "add"},
		{FilePath: "b.go", ChangeType: "fix", Summary: "patch"},
		{FilePath: "c.go", ChangeType: "feat", Summary: "another"},
	}
	groups := types.MergeGroupByType(merged)
	if len(groups["feat"]) != 2 {
		t.Errorf("feat group size = %d, want 2", len(groups["feat"]))
	}
	if len(groups["fix"]) != 1 {
		t.Errorf("fix group size = %d, want 1", len(groups["fix"]))
	}
}
