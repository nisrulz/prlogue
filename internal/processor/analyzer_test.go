package processor

import (
	"testing"

	"github.com/nisrulz/prlogue/internal/types"
)

func TestAnalyze_BranchPriority(t *testing.T) {
	files := []types.FileDiff{{Path: "internal/app.go"}}
	subjects := []string{"fix: crash"}
	results := Analyze(files, "feat/add-login", subjects)
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	if results[0].ChangeType != types.ChangeFeat {
		t.Errorf("expected feat, got %q", results[0].ChangeType)
	}
}

func TestAnalyze_FilePathPriority(t *testing.T) {
	files := []types.FileDiff{{Path: "internal/app_test.go"}}
	results := Analyze(files, "feat/add-login", nil)
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	if results[0].ChangeType != types.ChangeTest {
		t.Errorf("expected test, got %q", results[0].ChangeType)
	}
}

func TestAnalyze_CommitTieUsesChangeOrder(t *testing.T) {
	files := []types.FileDiff{{Path: "internal/app.go"}}
	results := Analyze(files, "topic", []string{"fix: crash", "feat: login"})
	if results[0].ChangeType != types.ChangeFeat {
		t.Fatalf("tie classification = %q, want feat", results[0].ChangeType)
	}
}

func TestPathChangeType(t *testing.T) {
	tests := []struct {
		path string
		want types.ChangeType
	}{
		{"pkg/file_test.go", types.ChangeTest},
		{"pkg/file_spec.go", types.ChangeTest},
		{"__tests__/test.js", types.ChangeTest},
		{"test/util_test.py", types.ChangeTest},
		{"docs/readme.md", types.ChangeDocs},
		{"README.md", types.ChangeDocs},
		{"go.mod", types.ChangeChore},
		{".github/workflows/ci.yml", types.ChangeChore},
		{"deploy/main.tf", types.ChangeChore},
		{"src/main.go", ""},
	}
	for _, tt := range tests {
		got := pathChangeType(tt.path)
		if got != tt.want {
			t.Errorf("pathChangeType(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
