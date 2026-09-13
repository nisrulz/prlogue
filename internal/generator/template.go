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
		fmt.Fprintf(&body, "## PR Description\n\n")
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

	if len(input.CommitSummaries) > 0 {
		writeCommitSummaryChanges(&body, input.CommitSummaries)
	} else if len(input.Merged) > 0 {
		writeMergedChanges(&body, input.Merged)
	}

	r.Body = body.String()
	r.CommitDesc = commitList(input.Commits)
	return r
}

// writeCommitSummaryChanges lists model-generated key changes when the final
// generation call failed but the per-commit summaries succeeded.
func writeCommitSummaryChanges(b *strings.Builder, summaries []types.CommitSummary) {
	bullets := commitSummaryBullets(summaries)
	if len(bullets) == 0 {
		return
	}
	fmt.Fprint(b, "### Key Changes\n\n")
	for _, bullet := range bullets {
		fmt.Fprintf(b, "- %s\n", bullet)
	}
	fmt.Fprintln(b)
}

func commitSummaryBullets(summaries []types.CommitSummary) []string {
	var bullets []string
	seen := make(map[string]bool)
	for _, s := range summaries {
		changes := s.KeyChanges
		if len(changes) == 0 && strings.TrimSpace(s.Summary) != "" {
			changes = []string{s.Summary}
		}
		for _, change := range changes {
			change = strings.TrimSpace(change)
			key := strings.ToLower(change)
			if change == "" || seen[key] {
				continue
			}
			seen[key] = true
			bullets = append(bullets, change)
		}
	}
	return bullets
}

func writeMergedChanges(b *strings.Builder, merged []types.MergedSummary) {
	fmt.Fprint(b, "### Key Changes\n\n")
	groups := types.MergeGroupByType(merged)
	for _, ct := range types.ChangeOrder {
		items, ok := groups[ct]
		if !ok {
			continue
		}
		section := changeTypeSection(ct)
		for _, m := range items {
			fmt.Fprintf(b, "- **%s:** %s\n", section, m.Summary)
		}
	}
	fmt.Fprintln(b)
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
		fmt.Fprintf(&b, "- `%s` %s\n", shortHash(c.Hash), c.Subject)
	}
	return b.String()
}
