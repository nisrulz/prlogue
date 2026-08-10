package git_test

import (
	"testing"

	"github.com/nisrulz/prlogue/internal/git"
)

func TestValidateBranch(t *testing.T) {
	for _, branch := range []string{"main", "origin/main", "feature/PROJ-123"} {
		if err := git.ValidateBranch(branch); err != nil {
			t.Errorf("ValidateBranch(%q): %v", branch, err)
		}
	}
}

func TestValidateBranch_RejectsRevisionExpressions(t *testing.T) {
	for _, branch := range []string{"", "--output=/tmp/x", "HEAD~10", "main..other"} {
		if err := git.ValidateBranch(branch); err == nil {
			t.Errorf("ValidateBranch(%q) succeeded", branch)
		}
	}
}
