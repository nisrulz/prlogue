package generator

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func buildLLMPrompt(input *GenerateInput) string {
	const (
		defaultLimit = 1 << 20
		prefix       = "---BEGIN REPOSITORY DATA---\n"
		suffix       = "---END REPOSITORY DATA---\n"
		truncated    = "\n... (repository data truncated)\n"
	)

	limit := input.MaxPromptBytes
	if limit <= len(prefix)+len(suffix)+len(truncated) {
		limit = defaultLimit
	}
	w := newPromptWriter(limit - len(prefix) - len(suffix) - len(truncated))

	w.printf("Current branch: %s (%s)\n", input.BranchCtx.CurrentBranch, input.BranchCtx.BranchType)
	w.printf("Default branch: %s\n", input.BranchCtx.DefaultBranch)
	if len(input.BranchCtx.IssueRefs) > 0 {
		w.printf("Issues: %v\n", input.BranchCtx.IssueRefs)
	}
	w.printf("Stats: %d files, +%d/-%d\n\n", input.DiffStats.Files, input.DiffStats.Additions, input.DiffStats.Deletions)

	if len(input.CommitSummaries) > 0 {
		w.write(renderCommitSummaries(input.CommitSummaries))
		w.write("\n")
	} else {
		if len(input.Commits) > 0 {
			w.write("Commits:\n")
			for _, c := range input.Commits {
				w.printf("- %s\n", c.Subject)
			}
			w.write("\n")
		}

		if len(input.OriginalDiffs) > 0 {
			w.write("Diff:\n")
			for _, f := range input.OriginalDiffs {
				w.printf("--- a/%s\n+++ b/%s\n", f.Path, f.Path)
				for _, h := range f.Hunks {
					lines := strings.Split(h.Content, "\n")
					if len(lines) > 200 {
						lines = append(lines[:200:200], "... (truncated)")
					}
					for _, l := range lines {
						if len(l) > 500 {
							l = truncateUTF8(l, 500) + "..."
						}
						w.write(l + "\n")
					}
				}
				w.write("\n")
			}
		}
	}

	var b strings.Builder
	b.Grow(limit)
	b.WriteString(prefix)
	b.WriteString(w.String())
	if w.truncated {
		b.WriteString(truncated)
	}
	b.WriteString(suffix)
	return b.String()
}

type promptWriter struct {
	b         strings.Builder
	remaining int
	truncated bool
}

func newPromptWriter(limit int) *promptWriter {
	return &promptWriter{remaining: limit}
}

func (w *promptWriter) printf(format string, args ...any) {
	w.write(fmt.Sprintf(format, args...))
}

func (w *promptWriter) write(s string) {
	if w.truncated {
		return
	}
	if len(s) <= w.remaining {
		w.b.WriteString(s)
		w.remaining -= len(s)
		return
	}
	if w.remaining > 0 {
		w.b.WriteString(truncateUTF8(s, w.remaining))
	}
	w.remaining = 0
	w.truncated = true
}

func (w *promptWriter) String() string {
	return w.b.String()
}

func truncateUTF8(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(s) <= limit {
		return s
	}
	for limit > 0 && !utf8.ValidString(s[:limit]) {
		limit--
	}
	return s[:limit]
}
