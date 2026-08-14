package formatter

import (
	"fmt"
	"strings"

	"github.com/nisrulz/prlogue/internal/generator"
)

func FormatMarkdown(result *generator.GenerateResult, input *generator.GenerateInput) string {
	var b strings.Builder

	appendSection := func(s string) {
		s = strings.TrimRight(s, "\n")
		if s == "" {
			return
		}
		if b.Len() > 0 {
			fmt.Fprint(&b, "\n\n")
		}
		b.WriteString(s)
	}

	if result.Title != "" {
		b.WriteString("# " + result.Title)
	}

	if !result.TemplateUsed && result.Summary != "" {
		appendSection(result.Summary)
	} else {
		appendSection(result.Body)
	}

	if len(input.BranchCtx.IssueRefs) > 0 {
		var sb strings.Builder
		sb.WriteString("## Related Issues\n")
		for _, ref := range input.BranchCtx.IssueRefs {
			fmt.Fprintf(&sb, "- %s\n", ref)
		}
		appendSection(sb.String())
	}

	if result.TemplateUsed {
		if result.CommitDesc != "" {
			appendSection(result.CommitDesc)
		} else if len(input.Commits) > 0 {
			var sb strings.Builder
			sb.WriteString("## Commits\n")
			for _, c := range input.Commits {
				fmt.Fprintf(&sb, "- `%s` %s\n", c.Hash[:7], c.Subject)
			}
			appendSection(sb.String())
		}
	}

	return b.String() + "\n"
}
