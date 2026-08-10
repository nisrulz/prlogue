package types

import "strings"

type Hunk struct {
	FilePath string
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Content  string
}

type FileDiff struct {
	Path      string
	Status    string
	Additions int
	Deletions int
	Hunks     []Hunk
}

type Commit struct {
	Hash    string
	Subject string
	Author  string
}

type Chunk struct {
	ID       string
	FilePath string
	Hunk     Hunk
	Lines    []string
	TokenEst int
}

type ChunkSummary struct {
	ChunkID    string
	OneLiner   string
	KeyChanges string
	Risk       string
}

type MergedSummary struct {
	FilePath   string
	ChangeType string
	Summary    string
}

type ChangeType string

const (
	ChangeFeat     ChangeType = "feat"
	ChangeFix      ChangeType = "fix"
	ChangeRefactor ChangeType = "refactor"
	ChangeDocs     ChangeType = "docs"
	ChangeTest     ChangeType = "test"
	ChangeChore    ChangeType = "chore"
)

var ChangeOrder = []string{"feat", "fix", "refactor", "docs", "test", "chore"}

func MergeGroupByType(merged []MergedSummary) map[string][]MergedSummary {
	groups := make(map[string][]MergedSummary)
	for _, m := range merged {
		groups[m.ChangeType] = append(groups[m.ChangeType], m)
	}
	return groups
}

func ChangeTypeFromBranch(branch string) ChangeType {
	lower := strings.ToLower(branch)
	switch {
	case strings.HasPrefix(lower, "feat/"), strings.HasPrefix(lower, "feature/"):
		return ChangeFeat
	case strings.HasPrefix(lower, "fix/"), strings.HasPrefix(lower, "bugfix/"), strings.HasPrefix(lower, "hotfix/"):
		return ChangeFix
	case strings.HasPrefix(lower, "chore/"):
		return ChangeChore
	case strings.HasPrefix(lower, "docs/"):
		return ChangeDocs
	case strings.HasPrefix(lower, "refactor/"), strings.HasPrefix(lower, "refact/"):
		return ChangeRefactor
	case strings.HasPrefix(lower, "test/"):
		return ChangeTest
	default:
		return ""
	}
}

func CommitChangeCounts(subjects []string) map[string]int {
	counts := make(map[string]int)
	for _, s := range subjects {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "feat"), strings.HasPrefix(s, "feature"):
			counts["feat"]++
		case strings.HasPrefix(s, "fix"):
			counts["fix"]++
		case strings.HasPrefix(s, "refactor"), strings.HasPrefix(s, "refact"):
			counts["refactor"]++
		case strings.HasPrefix(s, "docs"):
			counts["docs"]++
		case strings.HasPrefix(s, "test"):
			counts["test"]++
		case strings.HasPrefix(s, "chore"):
			counts["chore"]++
		}
	}
	return counts
}
