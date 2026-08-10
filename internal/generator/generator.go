package generator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/nisrulz/prlogue/internal/collector"
	"github.com/nisrulz/prlogue/internal/config"
	"github.com/nisrulz/prlogue/internal/provider"
	"github.com/nisrulz/prlogue/internal/types"
)

type DiffStats struct {
	Files     int
	Additions int
	Deletions int
	Hunks     int
}

type GenerateInput struct {
	DiffStats         DiffStats
	Commits           []types.Commit
	Merged            []types.MergedSummary
	BranchCtx         *collector.BranchContext
	OriginalDiffs     []types.FileDiff
	NoThink           bool
	OutputStylePrompt string
	ExtraBody         map[string]any
	MaxPromptBytes    int
}

type GenerateResult struct {
	Title        string
	Summary      string
	Body         string
	CommitDesc   string
	Raw          string // full LLM output if available
	TemplateUsed bool
}

type Generator struct {
	p     provider.Provider
	model string
}

func NewGenerator(p provider.Provider, model string) *Generator {
	return &Generator{p: p, model: model}
}

func (g *Generator) Generate(ctx context.Context, input *GenerateInput) (*GenerateResult, error) {
	result, err := g.generateLLM(ctx, input)
	if err == nil {
		return result, nil
	}

	fmt.Fprintf(os.Stderr, "⚠ Model unavailable, using template fallback: %v\n", err)

	tmpl := &TemplateGenerator{}
	return tmpl.Generate(input), nil
}

func (g *Generator) generateLLM(ctx context.Context, input *GenerateInput) (*GenerateResult, error) {
	prompt := buildLLMPrompt(input)

	system := input.OutputStylePrompt
	if strings.TrimSpace(system) == "" {
		system = config.DefaultPrompt()
	}

	resp, err := g.p.Chat(ctx, provider.ChatRequest{
		Model: g.model,
		Messages: []provider.ChatMessage{
			{
				Role:    "system",
				Content: system,
			},
			{
				Role:    "system",
				Content: config.SecurityPrompt(),
			},
			{
				Role:    "system",
				Content: config.SanitizationPrompt(),
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   2048,
		Temperature: 0.3,
		NoThink:     input.NoThink,
		ExtraBody:   input.ExtraBody,
	})
	if err != nil {
		return nil, fmt.Errorf("generate LLM: %w", err)
	}

	cleanOutput := sanitizeLLMOutput(resp.Content)
	title, summary := extractLLMTitle(cleanOutput)
	if isPlaceholderTitle(title) {
		title = ""
	}
	if title == "" {
		title = input.BranchCtx.CurrentBranch
	}
	return &GenerateResult{
		Title:   title,
		Summary: summary,
		Raw:     cleanOutput,
	}, nil
}

func extractLLMTitle(s string) (title, body string) {
	lines := strings.SplitN(s, "\n", 2)
	if len(lines) == 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "Title:") {
		title = strings.TrimSpace(strings.TrimPrefix(lines[0], "Title:"))
		body = strings.TrimSpace(lines[1])
		return title, body
	}
	return "", s
}

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

	w.printf("Branch: %s (%s)\n", input.BranchCtx.CurrentBranch, input.BranchCtx.BranchType)
	if len(input.BranchCtx.IssueRefs) > 0 {
		w.printf("Issues: %v\n", input.BranchCtx.IssueRefs)
	}
	w.printf("Stats: %d files, +%d/-%d\n\n", input.DiffStats.Files, input.DiffStats.Additions, input.DiffStats.Deletions)

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
