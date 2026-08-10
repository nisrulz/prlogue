package generator

import (
	"fmt"
	"strings"

	"github.com/nisrulz/prlogue/internal/types"
)

type TemplateGenerator struct{}

func (t *TemplateGenerator) Generate(input *GenerateInput) *GenerateResult {
	r := &GenerateResult{
		Title:        input.BranchCtx.CurrentBranch,
		TemplateUsed: true,
	}

	var body strings.Builder

	if input.DiffStats.Files > 0 {
		fmt.Fprintf(&body, "### PR Description\n\n")
		types := uniqueChangeTypes(input.Merged)
		desc := fmt.Sprintf("This PR makes changes across %d file(s) with %d additions and %d deletions",
			input.DiffStats.Files, input.DiffStats.Additions, input.DiffStats.Deletions)
		if len(types) > 0 {
			desc += fmt.Sprintf(", covering %s", formatTypeList(types))
		}
		desc += "."
		if len(input.Commits) > 0 {
			desc += fmt.Sprintf(" It includes %d commit(s): %s.",
				len(input.Commits), joinSubjects(input.Commits))
		}
		body.WriteString(desc)
		body.WriteString("\n\n")
	}

	if len(input.Merged) > 0 {
		fmt.Fprint(&body, "### Key Changes\n\n")
		groups := types.MergeGroupByType(input.Merged)
		for _, ct := range types.ChangeOrder {
			items, ok := groups[ct]
			if !ok {
				continue
			}
			section := changeTypeSection(ct)
			for _, m := range items {
				fmt.Fprintf(&body, "- **%s:** %s\n", section, m.Summary)
			}
		}
		fmt.Fprintln(&body)
	}

	r.Body = body.String()
	r.CommitDesc = commitList(input.Commits)
	return r
}

func uniqueChangeTypes(merged []types.MergedSummary) []string {
	seen := make(map[string]bool)
	var out []string
	for _, m := range merged {
		if !seen[m.ChangeType] {
			seen[m.ChangeType] = true
			out = append(out, m.ChangeType)
		}
	}
	return out
}

func formatTypeList(types []string) string {
	if len(types) == 0 {
		return ""
	}
	if len(types) == 1 {
		return types[0]
	}
	return strings.Join(types[:len(types)-1], ", ") + " and " + types[len(types)-1]
}

func joinSubjects(commits []types.Commit) string {
	if len(commits) == 0 {
		return ""
	}
	if len(commits) == 1 {
		return commits[0].Subject
	}
	var b strings.Builder
	for i, c := range commits {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(c.Subject)
	}
	return b.String()
}

func changeTypeSection(ct string) string {
	switch ct {
	case "feat":
		return "Features"
	case "fix":
		return "Bug Fixes"
	case "refactor":
		return "Refactoring"
	case "docs":
		return "Documentation"
	case "test":
		return "Tests"
	case "chore":
		return "Chores"
	default:
		return ct
	}
}

func commitList(commits []types.Commit) string {
	if len(commits) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprint(&b, "## Commits\n\n")
	for _, c := range commits {
		fmt.Fprintf(&b, "- `%s` %s\n", c.Hash[:7], c.Subject)
	}
	return b.String()
}
