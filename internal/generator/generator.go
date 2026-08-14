package generator

import (
	"context"
	"errors"
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
	CommitSummaries   []types.CommitSummary
	NoThink           bool
	ResponseMaxTokens int
	OutputStylePrompt string
	ExtraBody         map[string]any
	MaxPromptBytes    int
	// StagedContext delivers each prompt block in its own API call. When
	// false, all blocks are sent in one call.
	StagedContext bool
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

var errInvalidLLMOutput = errors.New("invalid PR description from model")

const (
	maxLLMOutputBytes = 16 << 10
	maxLLMOutputLines = 160

	// ackMaxTokens caps each intermediate context-injection call. Those calls
	// only ask the model to acknowledge a block so it is absorbed into its
	// context; the final call does the generation.
	ackMaxTokens = 256
)

const (
	// contextAckInstruction tells the model to hold the final output until
	// every context block has been supplied. It must not discourage reading
	// the block: the model needs to absorb the content now, not defer it.
	contextAckInstruction = "Read and store the context above for later use. Do not write the PR title or description yet. Reply with exactly: ACK"
	// contextAckResponse is the fixed assistant reply stored after each
	// context block. It is fixed so untrusted model output never becomes part
	// of the following conversation.
	contextAckResponse = "ACK"
	// generateInstruction releases the collected context for the final call.
	// It names the delivered blocks so the model reads the repository data and
	// explicitly forbids the ack token so it does not echo the hold response.
	generateInstruction = "FINAL REQUEST: use the security policy, the output sanitization policy, the output style, and the repository data supplied above to generate the PR title and description now. Do not reply with an acknowledgment such as ACK or OK."
)

func NewGenerator(p provider.Provider, model string) *Generator {
	return &Generator{p: p, model: model}
}

func (g *Generator) Generate(ctx context.Context, input *GenerateInput) (*GenerateResult, error) {
	result, err := g.generateLLM(ctx, input)
	if err == nil {
		return result, nil
	}

	if errors.Is(err, errInvalidLLMOutput) {
		fmt.Fprintf(os.Stderr, "⚠ Model returned an invalid PR description, using template fallback: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "⚠ Model unavailable, using template fallback: %v\n", err)
	}
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
		style = config.LoadOutputStylePrompt()
	}
	useStandardStyle := strings.Contains(style, "Title:") && strings.Contains(style, "### PR Description")

	cleanOutput, err := g.generateWithDefense(ctx, input, prompt, style)
	if err != nil {
		return nil, err
	}
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

// generateWithDefense delivers the prompt context to the model, validates the
// output, and retries once with a data-pointing instruction before failing.
func (g *Generator) generateWithDefense(ctx context.Context, input *GenerateInput, prompt, style string) (string, error) {
	messages, err := g.deliverContext(ctx, input, prompt, style)
	if err != nil {
		return "", err
	}
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := g.p.Chat(ctx, provider.ChatRequest{
			Model:       g.model,
			Messages:    messages,
			MaxTokens:   responseMaxTokensForInput(input),
			Temperature: 0,
			NoThink:     input.NoThink,
			ExtraBody:   input.ExtraBody,
		})
		if err != nil {
			return "", fmt.Errorf("generate LLM: %w", err)
		}
		cleanOutput := sanitizeLLMOutput(resp.Content)
		reason := outputRejectionReason(input, cleanOutput)
		if reason == "" {
			return cleanOutput, nil
		}
		if attempt == 0 {
			messages = append(messages, provider.ChatMessage{Role: "user", Content: buildRetryInstruction(input, reason)})
			continue
		}
		return "", fmt.Errorf("%w: %s", errInvalidLLMOutput, reason)
	}
	return "", fmt.Errorf("%w: output failed PR description validation", errInvalidLLMOutput)
}

// deliverContext prepares the conversation. In staged mode every context block
// is delivered in its own API call and the model holds its output until the
// final request. Otherwise the blocks are sent in one call.
func (g *Generator) deliverContext(ctx context.Context, input *GenerateInput, prompt, style string) ([]provider.ChatMessage, error) {
	if !input.StagedContext {
		return []provider.ChatMessage{
			{Role: "system", Content: config.DefaultPrompt()},
			{Role: "system", Content: style},
			{Role: "system", Content: config.SecurityPrompt()},
			{Role: "system", Content: config.SanitizationPrompt()},
			{Role: "user", Content: prompt},
		}, nil
	}

	messages := make([]provider.ChatMessage, 0, 16)
	var err error
	if messages, err = g.injectContext(ctx, input, messages, "system", config.SecurityPrompt()); err != nil {
		return nil, err
	}
	if messages, err = g.injectContext(ctx, input, messages, "system", config.SanitizationPrompt()); err != nil {
		return nil, err
	}
	if messages, err = g.injectContext(ctx, input, messages, "system", style); err != nil {
		return nil, err
	}
	if messages, err = g.injectContext(ctx, input, messages, "user", prompt); err != nil {
		return nil, err
	}
	return append(messages, provider.ChatMessage{Role: "user", Content: generateInstruction + "\n\n" + config.DefaultPrompt()}), nil
}

// injectContext appends one context block, tells the model to hold its output,
// and records a short acknowledgement. Each injection is one API call so the
// model only processes one block per call and accumulates them in context.
func (g *Generator) injectContext(ctx context.Context, input *GenerateInput, messages []provider.ChatMessage, role, content string) ([]provider.ChatMessage, error) {
	messages = append(messages, provider.ChatMessage{Role: role, Content: content})
	messages = append(messages, provider.ChatMessage{Role: "user", Content: contextAckInstruction})
	if _, err := g.p.Chat(ctx, provider.ChatRequest{
		Model:       g.model,
		Messages:    messages,
		MaxTokens:   ackMaxTokens,
		Temperature: 0,
		NoThink:     input.NoThink,
		ExtraBody:   input.ExtraBody,
	}); err != nil {
		return nil, fmt.Errorf("inject context: %w", err)
	}
	return append(messages, provider.ChatMessage{Role: "assistant", Content: contextAckResponse}), nil
}

// outputRejectionReason explains why the model output is unusable, or returns
// an empty string when the output is accepted.
func outputRejectionReason(input *GenerateInput, s string) string {
	if !isUsableLLMOutput(s) {
		return "output failed PR description validation"
	}
	if isAckOnlyOutput(s) {
		return "model echoed an acknowledgment instead of generating"
	}
	if isRefusalOutput(s) {
		return "model refused to summarize the changes"
	}
	if input.DiffStats.Files > 0 && claimsNoChanges(s) {
		return "model reported no changes despite repository data"
	}
	return ""
}

// buildRetryInstruction points the model back at the repository data and the
// collected statistics after a rejected output.
func buildRetryInstruction(input *GenerateInput, reason string) string {
	return fmt.Sprintf(
		"Your previous reply was rejected: %s. The repository data above contains %d changed file(s) with +%d/-%d lines and %d commit(s). Read the diff and the commit list in the repository data above. Generate the PR title and description from that data. Do not claim there are no changes and do not reply with an acknowledgment.",
		reason, input.DiffStats.Files, input.DiffStats.Additions, input.DiffStats.Deletions, len(input.Commits))
}

// stripTitleAndHeadings returns the body of a model reply with title lines and
// Markdown headings removed, so rejection checks test only the content.
func stripTitleAndHeadings(s string) string {
	lines := strings.Split(s, "\n")
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(strings.ToLower(trimmed), "title:") {
			continue
		}
		body = append(body, trimmed)
	}
	return strings.Join(body, " ")
}

// isAckOnlyOutput reports whether the model echoed an acknowledgment token
// instead of producing a PR title and description.
func isAckOnlyOutput(s string) bool {
	normalized := strings.ToLower(stripTitleAndHeadings(s))
	if normalized == "" {
		return false
	}
	for _, ack := range []string{"ok", "ack", "received", "done", "understood", "got it", "acknowledged", "yes", "y"} {
		if normalized == ack {
			return true
		}
	}
	return false
}

// claimsNoChanges reports whether the model claimed the repository has no
// changes or that it lacks the data to summarize any.
func claimsNoChanges(s string) bool {
	body := strings.ToLower(stripTitleAndHeadings(s))
	for _, phrase := range []string{
		"no changes were identified",
		"no changes identified",
		"no changes found",
		"no changes to summarize",
		"unable to determine changes",
		"cannot determine changes",
		"nothing to report",
		"nothing to summarize",
		"no commit history",
		"cannot find any changes",
		"could not find any changes",
	} {
		if strings.Contains(body, phrase) {
			return true
		}
	}
	if strings.Contains(body, "does not contain sufficient") && (strings.Contains(body, "commit") || strings.Contains(body, "diff")) {
		return true
	}
	if strings.Contains(body, "insufficient") && (strings.Contains(body, "commit") || strings.Contains(body, "diff") || strings.Contains(body, "history") || strings.Contains(body, "information")) {
		return true
	}
	return false
}

// isRefusalOutput reports whether the model declined to summarize the changes.
func isRefusalOutput(s string) bool {
	body := strings.ToLower(stripTitleAndHeadings(s))
	for _, phrase := range []string{
		"as an ai",
		"as an llm",
		"as a language model",
		"i cannot",
		"i can't",
		"i am unable",
		"i'm unable",
		"i am not able",
		"i'm not able",
		"cannot help",
		"cannot assist",
		"unable to assist",
		"cannot provide",
		"cannot generate",
	} {
		if strings.Contains(body, phrase) {
			return true
		}
	}
	return false
}

func isUsableLLMOutput(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > maxLLMOutputBytes || strings.Count(s, "\n")+1 > maxLLMOutputLines {
		return false
	}
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "diff --git ") ||
			strings.HasPrefix(trimmed, "--- a/") ||
			strings.HasPrefix(trimmed, "+++ b/") ||
			strings.HasPrefix(trimmed, "@@ ") {
			return false
		}
	}
	return true
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
