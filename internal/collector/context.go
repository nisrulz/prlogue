package collector

import (
	"regexp"
	"strings"

	"github.com/nisrulz/prlogue/internal/git"
	"github.com/nisrulz/prlogue/internal/types"
)

type BranchContext struct {
	CurrentBranch string
	DefaultBranch string
	BranchType    string
	IssueRefs     []string
}

func CollectContext(defaultBranch string, commits []types.Commit) (*BranchContext, error) {
	current, err := git.CurrentBranch()
	if err != nil {
		current = "unknown"
	}
	commitSubjects := make([]string, len(commits))
	for i, commit := range commits {
		commitSubjects[i] = commit.Subject
	}

	ctx := &BranchContext{
		CurrentBranch: current,
		DefaultBranch: defaultBranch,
		BranchType:    string(types.ChangeTypeFromBranch(current)),
		IssueRefs:     extractIssueRefs(current, commitSubjects),
	}
	return ctx, nil
}

var issueRefRe = regexp.MustCompile(`[A-Z]+-\d+|#\d+`)

func extractIssueRefs(branch string, commitSubjects []string) []string {
	seen := map[string]bool{}
	var refs []string

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			refs = append(refs, s)
		}
	}

	for _, match := range issueRefRe.FindAllString(branch, -1) {
		add(match)
	}

	for _, subj := range commitSubjects {
		for _, match := range issueRefRe.FindAllString(subj, -1) {
			add(match)
		}
	}

	return refs
}
