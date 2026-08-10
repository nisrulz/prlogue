package processor

import (
	"strings"

	"github.com/nisrulz/prlogue/internal/types"
)

type FileClassification struct {
	FilePath   string
	ChangeType types.ChangeType
	Confidence string
}

func Analyze(files []types.FileDiff, branchName string, commitSubjects []string) []FileClassification {
	classifications := make([]FileClassification, 0, len(files))
	branchType := BranchChangeType(branchName)
	commitTypes := types.CommitChangeCounts(commitSubjects)

	for _, f := range files {
		ct := classifyFile(f.Path, branchType, commitTypes)
		classifications = append(classifications, FileClassification{
			FilePath:   f.Path,
			ChangeType: ct,
			Confidence: classifyConfidence(f.Path, branchType, commitTypes),
		})
	}
	return classifications
}

func BranchChangeType(branch string) types.ChangeType {
	return types.ChangeTypeFromBranch(branch)
}

func classifyFile(path string, branchType types.ChangeType, commitTypes map[string]int) types.ChangeType {
	if ct := pathChangeType(path); ct != "" {
		return ct
	}
	if branchType != "" {
		return branchType
	}
	if len(commitTypes) > 0 {
		best := ""
		max := 0
		for _, ct := range types.ChangeOrder {
			count := commitTypes[ct]
			if count > max {
				max = count
				best = ct
			}
		}
		if best != "" {
			return types.ChangeType(best)
		}
	}
	return types.ChangeChore
}

func classifyConfidence(path string, branchType types.ChangeType, commitTypes map[string]int) string {
	if ct := pathChangeType(path); ct != "" {
		return "high"
	}
	if branchType != "" {
		return "medium"
	}
	if len(commitTypes) > 0 {
		return "low"
	}
	return "low"
}

func pathChangeType(path string) types.ChangeType {
	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]
	dir := ""
	if len(parts) > 1 {
		dir = parts[0]
	}

	if strings.HasSuffix(filename, "_test.go") ||
		strings.HasSuffix(filename, "_test.py") ||
		strings.HasSuffix(filename, "_test.rs") ||
		strings.HasSuffix(filename, "_spec.go") ||
		strings.HasSuffix(filename, "_spec.rb") ||
		strings.Contains(path, "__tests__") ||
		strings.Contains(path, "test/") {
		return types.ChangeTest
	}

	if strings.HasPrefix(dir, "docs") ||
		strings.HasPrefix(dir, "documentation") ||
		strings.HasSuffix(filename, ".md") {
		return types.ChangeDocs
	}

	if filename == "go.mod" || filename == "go.sum" ||
		filename == "Cargo.toml" || filename == "Cargo.lock" ||
		filename == "package.json" || filename == "package-lock.json" ||
		filename == "yarn.lock" || filename == "Gemfile" ||
		filename == "Gemfile.lock" || filename == "Pipfile" ||
		filename == "Pipfile.lock" || filename == "poetry.lock" ||
		filename == "requirements.txt" ||
		strings.HasSuffix(filename, ".tf") ||
		strings.HasSuffix(filename, ".yml") ||
		strings.HasSuffix(filename, ".yaml") ||
		strings.HasPrefix(dir, ".github") {
		return types.ChangeChore
	}

	return ""
}
