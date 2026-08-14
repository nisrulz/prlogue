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
	ResponseMaxTokens int
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
	if isMaxTokensError(err) {
		fmt.Fprintf(os.Stderr, "  The provider rejected max_tokens=%d. Lower response_max_tokens in the config file used by PRlogue and retry.\n", responseMaxTokensForInput(input))
		fmt.Fprintln(os.Stderr, "  Default config: $PRLOGUE_CONFIG_DIR/prlogue/config.yaml")
		fmt.Fprintln(os.Stderr, "  Example: response_max_tokens: 8192")
	}

	tmpl := &TemplateGenerator{}
	return tmpl.Generate(input), nil
}

func (g *Generator) generateLLM(ctx context.Context, input *GenerateInput) (*GenerateResult, error) {
	prompt := buildLLMPrompt(input)

	style := input.OutputStylePrompt
	if strings.TrimSpace(style) == "" {
		style = config.DefaultOutputStylePrompt()
	}
	useStandardStyle := strings.Contains(style, "Title:") && strings.Contains(style, "### PR Description")

	resp, err := g.p.Chat(ctx, provider.ChatRequest{
		Model: g.model,
		Messages: []provider.ChatMessage{
			{
				Role:    "system",
				Content: config.DefaultPrompt(),
			},
			{
				Role:    "system",
				Content: style,
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
		MaxTokens:   responseMaxTokensForInput(input),
		Temperature: 0.3,
		NoThink:     input.NoThink,
		ExtraBody:   input.ExtraBody,
	})
	if err != nil {
		return nil, fmt.Errorf("generate LLM: %w", err)
	}

	cleanOutput := sanitizeLLMOutput(resp.Content)
	title, summary := extractLLMTitle(cleanOutput)
	if useStandardStyle {
		summary = normalizeLLMSummary(summary)
	}
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

func responseMaxTokensForInput(input *GenerateInput) int {
	if input.ResponseMaxTokens > 0 {
		return input.ResponseMaxTokens
	}
	return config.DefaultResponseMaxTokens
}

func isMaxTokensError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"max_tokens", "max tokens", "token limit", "maximum context", "context length"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func extractLLMTitle(s string) (title, body string) {
	lines := strings.SplitN(s, "\n", 2)
	if len(lines) == 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "Title:") {
		title = strings.TrimSpace(strings.TrimPrefix(lines[0], "Title:"))
		body = strings.TrimSpace(lines[1])
		return title, body
	}
	if len(lines) == 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "# ") {
		title = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[0]), "# "))
		body = strings.TrimSpace(lines[1])
		return title, body
	}
	return "", s
}

func normalizeLLMSummary(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "### PR Description") {
		return s
	}
	return "### PR Description\n\n" + s
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

	w.printf("Current branch: %s (%s)\n", input.BranchCtx.CurrentBranch, input.BranchCtx.BranchType)
	w.printf("Default branch: %s\n", input.BranchCtx.DefaultBranch)
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
