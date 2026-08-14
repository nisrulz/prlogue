package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/nisrulz/prlogue/internal/collector"
	"github.com/nisrulz/prlogue/internal/config"
	"github.com/nisrulz/prlogue/internal/formatter"
	"github.com/nisrulz/prlogue/internal/generator"
	"github.com/nisrulz/prlogue/internal/git"
	"github.com/nisrulz/prlogue/internal/processor"
	"github.com/nisrulz/prlogue/internal/provider"
	"github.com/nisrulz/prlogue/internal/spinner"
	"github.com/nisrulz/prlogue/internal/types"
	"github.com/spf13/cobra"
)

type generateFlags struct {
	staged     bool
	outputFile string
	publish    bool
}

var genFlags generateFlags

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a PR description",
	Long: `Analyzes git diff and commits to generate a PR description.
By default compares current branch against the default branch (auto-detected).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenerate()
	},
}

func runGenerate() error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	settings, err := resolveProviderSettings(cfg)
	if err != nil {
		return err
	}
	if settings.remote && cfg.APIKey == "" {
		return fmt.Errorf("PRLOGUE_OPENAI_COMPAT_API_KEY is required for remote OpenAI-compatible endpoints")
	}
	noThink := cfg.NoThink

	baseBranch := cfg.Git.DefaultBranch
	if baseBranch == "" {
		branch, err := git.DetectDefaultBranch()
		if err != nil {
			return fmt.Errorf("could not detect base branch: %w\n  Set git.default_branch in your config", err)
		}
		baseBranch = branch
	}
	if err := git.ValidateBranch(baseBranch); err != nil {
		return err
	}
	currentBranch, err := git.CurrentBranch()
	if err != nil {
		return fmt.Errorf("could not determine current branch: %w", err)
	}
	if err := git.ValidateBranch(currentBranch); err != nil {
		return err
	}

	commits, err := collector.CollectCommits(baseBranch, currentBranch, 50)
	if err != nil && verbose {
		fmt.Fprintf(os.Stderr, "Commit warning: %v\n", err)
	}
	if err != nil {
		commits = nil
	}
	branchCtx, err := collector.CollectContext(baseBranch, currentBranch, commits)
	if err != nil {
		return fmt.Errorf("collect context: %w", err)
	}

	diffs, err := collector.CollectDiff(baseBranch, currentBranch, genFlags.staged)
	if err != nil {
		return fmt.Errorf("collect diff: %w", err)
	}
	if len(diffs) == 0 {
		if genFlags.staged {
			return fmt.Errorf("no staged changes found")
		}
		return fmt.Errorf("no changes found between %s and %s", baseBranch, currentBranch)
	}

	contextLen := cfg.ContextLength()

	diffStats := computeDiffStats(diffs)
	if verbose {
		printVerboseInfo(settings.name, settings.baseURL, settings.model, baseBranch, contextLen, branchCtx, diffStats, len(commits))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	p := newProvider(settings.baseURL, cfg.APIKey, settings.model)

	progress := spinner.NewProgressList(os.Stderr, commitLabels(commits))
	progress.Start()
	summarizer := generator.NewCommitSummarizer(p, settings.model, noThink, cfg.ExtraBody, contextLen)
	commitSummaries, summariesPath, err := summarizer.Summarize(ctx, commits, progress.Advance)
	progress.Finish()
	if err != nil {
		return err
	}
	if verbose && summariesPath != "" {
		fmt.Fprintf(os.Stderr, "Commit summaries: %s\n", summariesPath)
	}

	genInput := buildGenerateInput(diffs, commits, branchCtx, cfg, noThink, promptByteLimit(contextLen), commitSummaries)

	gen := generator.NewGenerator(p, settings.model)

	spin := spinner.New("Generating PR description")
	spin.Start()
	genResult, err := gen.Generate(ctx, genInput)
	spin.Stop()
	if err != nil {
		return err
	}

	format := cfg.Output.Format

	output, err := formatOutput(genResult, genInput, format)
	if err != nil {
		return err
	}

	if genFlags.outputFile != "" {
		if err := os.WriteFile(genFlags.outputFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "Written to %s\n", genFlags.outputFile)
		}
	} else {
		fmt.Println(output)
	}

	if genFlags.publish {
		if err := publishPR(genResult, output, baseBranch); err != nil {
			return fmt.Errorf("publish PR: %w", err)
		}
	}

	return nil
}

type providerSettings struct {
	name    string
	baseURL string
	model   string
	remote  bool
}

func resolveProviderSettings(cfg *config.Config) (providerSettings, error) {
	settings := providerSettings{name: cfg.Provider, baseURL: cfg.BaseURL, model: cfg.Model}
	if err := config.ValidateBaseURL(settings.baseURL); err != nil {
		return providerSettings{}, err
	}
	if u, err := url.Parse(settings.baseURL); err == nil {
		settings.remote = !config.IsLoopbackHost(u.Hostname())
	}
	if strings.TrimSpace(settings.model) == "" {
		return providerSettings{}, fmt.Errorf("model must not be empty")
	}
	return settings, nil
}

func newProvider(baseURL, apiKey, model string) provider.Provider {
	return provider.NewOpenAICompatible(baseURL, apiKey, model)
}

func computeDiffStats(diffs []types.FileDiff) generator.DiffStats {
	var s generator.DiffStats
	s.Files = len(diffs)
	for _, f := range diffs {
		s.Additions += f.Additions
		s.Deletions += f.Deletions
		s.Hunks += len(f.Hunks)
	}
	return s
}

// commitLabels builds short, fixed-width labels for the commit progress list.
// The lines must never wrap, so each subject is truncated.
func commitLabels(commits []types.Commit) []string {
	labels := make([]string, 0, len(commits))
	for _, c := range commits {
		label := c.Subject
		if len(c.Hash) >= 7 {
			label = c.Hash[:7] + " " + label
		}
		if r := []rune(label); len(r) > 72 {
			label = string(r[:72]) + "..."
		}
		labels = append(labels, label)
	}
	return labels
}

func buildGenerateInput(diffs []types.FileDiff, commits []types.Commit, branchCtx *collector.BranchContext, cfg *config.Config, noThink bool, maxPromptBytes int, commitSummaries []types.CommitSummary) *generator.GenerateInput {
	diffStats := computeDiffStats(diffs)
	commitSubjects := make([]string, len(commits))
	for i, c := range commits {
		commitSubjects[i] = c.Subject
	}

	classifications := processor.Analyze(diffs, branchCtx.CurrentBranch, commitSubjects)
	chunks := processor.ChunkDiff(diffs, cfg.Chunking)
	if verbose {
		fmt.Fprintf(os.Stderr, "Chunks:    %d\n", len(chunks))
	}

	summaries := processor.FallbackSummarize(chunks)
	merger := processor.NewMerger(classifications)
	merged := merger.Merge(summaries)

	return &generator.GenerateInput{
		DiffStats:         diffStats,
		Commits:           commits,
		Merged:            merged,
		BranchCtx:         branchCtx,
		OriginalDiffs:     diffs,
		CommitSummaries:   commitSummaries,
		NoThink:           noThink,
		ResponseMaxTokens: cfg.ResponseMaxTokens,
		ExtraBody:         cfg.ExtraBody,
		MaxPromptBytes:    maxPromptBytes,
		StagedContext:     cfg.StagedContext,
	}
}

func formatOutput(result *generator.GenerateResult, input *generator.GenerateInput, format string) (string, error) {
	switch format {
	case "json":
		s, err := formatter.FormatJSON(result, input)
		if err != nil {
			return "", err
		}
		return s, nil
	case "markdown":
		return formatter.FormatMarkdown(result, input), nil
	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}
}

func publishPR(genResult *generator.GenerateResult, body string, baseBranch string) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found: install from https://cli.github.com")
	}
	title := sanitizeTitle(genResult.Title)
	if title == "" {
		b, _ := git.CurrentBranch()
		title = "PR from " + b
	}
	cmd := exec.Command("gh", "pr", "create",
		"--base", baseBranch,
		"--title", title,
		"--body", "-",
	)
	cmd.Stdin = strings.NewReader(body)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func sanitizeTitle(title string) string {
	title = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, title)
	title = strings.Join(strings.Fields(title), " ")
	runes := []rune(title)
	if len(runes) > 256 {
		title = string(runes[:256])
	}
	return title
}

func promptByteLimit(contextTokens int) int {
	const (
		minBytes         = 16 << 10
		maxBytes         = 1 << 20
		completionTokens = 2048
	)
	availableTokens := contextTokens - completionTokens
	if availableTokens < minBytes/4 {
		return minBytes
	}
	limit := availableTokens * 4
	if limit > maxBytes {
		return maxBytes
	}
	return limit
}

func printVerboseInfo(providerName, baseURL, modelName, baseBranch string, contextLen int, branchCtx *collector.BranchContext, diffStats generator.DiffStats, commitCount int) {
	fmt.Fprintf(os.Stderr, "Provider:  %s\n", providerName)
	fmt.Fprintf(os.Stderr, "Endpoint:  %s\n", baseURL)
	fmt.Fprintf(os.Stderr, "Model:     %s\n", modelName)
	fmt.Fprintf(os.Stderr, "Base:      %s\n", baseBranch)
	fmt.Fprintf(os.Stderr, "Branch:    %s (%s)\n", branchCtx.CurrentBranch, branchCtx.BranchType)
	fmt.Fprintf(os.Stderr, "Context:   %d tokens\n", contextLen)
	fmt.Fprintf(os.Stderr, "Files:     %d\n", diffStats.Files)
	fmt.Fprintf(os.Stderr, "Hunks:     %d\n", diffStats.Hunks)
	fmt.Fprintf(os.Stderr, "+%d/-%d\n", diffStats.Additions, diffStats.Deletions)
	fmt.Fprintf(os.Stderr, "Commits:   %d\n", commitCount)
	if len(branchCtx.IssueRefs) > 0 {
		fmt.Fprintf(os.Stderr, "Issues:    %v\n", branchCtx.IssueRefs)
	}
	fmt.Fprintln(os.Stderr)
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().BoolVarP(&genFlags.staged, "staged", "s", false, "Only staged changes")
	generateCmd.Flags().StringVarP(&genFlags.outputFile, "output", "o", "", "Write to file")
	generateCmd.Flags().BoolVarP(&genFlags.publish, "publish", "", false, "Create PR via gh CLI")
}
