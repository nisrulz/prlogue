package generator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

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

const (
	// Output markers required by the built-in style prompt. The generator
	// validates the model reply against them.
	titleLabel         = "Title:"
	descriptionHeading = "## PR Description"
	keyChangesHeading  = "### Key Changes"
)

// isStandardStyle reports whether the style prompt requests the built-in title
// and PR description markers, so the reply can be validated against them.
func isStandardStyle(style string) bool {
	return strings.Contains(style, titleLabel) && strings.Contains(style, descriptionHeading)
}

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
	useStandardStyle := isStandardStyle(style)

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
	useStandardStyle := isStandardStyle(style)
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
		reason := outputRejectionReason(input, useStandardStyle, cleanOutput)
		if reason == reasonNone {
			return cleanOutput, nil
		}
		if attempt == 0 {
			messages = append(messages, provider.ChatMessage{Role: "user", Content: buildRetryInstruction(input, useStandardStyle, reason)})
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

func responseMaxTokensForInput(input *GenerateInput) int {
	if input.ResponseMaxTokens > 0 {
		return input.ResponseMaxTokens
	}
	return config.DefaultResponseMaxTokens
}
