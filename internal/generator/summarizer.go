package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nisrulz/prlogue/internal/collector"
	"github.com/nisrulz/prlogue/internal/config"
	"github.com/nisrulz/prlogue/internal/provider"
	"github.com/nisrulz/prlogue/internal/types"
)

// CommitSummarizer summarizes each commit from its message, description, and
// diff, then writes the full list to a single JSON file. The final PR
// generation uses that JSON as its context instead of raw git data.
type CommitSummarizer struct {
	p         provider.Provider
	model     string
	noThink   bool
	extraBody map[string]any
	maxTokens int
}

func NewCommitSummarizer(p provider.Provider, model string, noThink bool, extraBody map[string]any, contextLen int) *CommitSummarizer {
	maxTokens := contextLen
	if maxTokens <= 0 {
		maxTokens = config.DefaultResponseMaxTokens
	}
	return &CommitSummarizer{p: p, model: model, noThink: noThink, extraBody: extraBody, maxTokens: maxTokens}
}

// Summarize produces a summary per commit and stores all of them in one JSON
// file in the system temp directory. The returned path is empty when there are
// no commits. A failed summary call falls back to the commit subject and its
// changed paths so nothing is dropped. onProgress runs after every commit and
// is safe to leave nil.
func (s *CommitSummarizer) Summarize(ctx context.Context, commits []types.Commit, onProgress func()) ([]types.CommitSummary, string, error) {
	if len(commits) == 0 {
		return nil, "", nil
	}
	summaries := make([]types.CommitSummary, 0, len(commits))
	for _, c := range commits {
		summaries = append(summaries, s.summarizeOne(ctx, c))
		if onProgress != nil {
			onProgress()
		}
	}
	path, err := writeCommitSummaries(summaries)
	if err != nil {
		return nil, "", err
	}
	return summaries, path, nil
}

func (s *CommitSummarizer) summarizeOne(ctx context.Context, c types.Commit) types.CommitSummary {
	diff, err := collector.CollectCommitDiff(c.Hash)
	if err != nil {
		diff = ""
	}
	resp, chatErr := s.p.Chat(ctx, provider.ChatRequest{
		Model: s.model,
		Messages: []provider.ChatMessage{
			{Role: "system", Content: config.CommitSummaryPrompt()},
			{Role: "user", Content: buildCommitSummaryPrompt(c, diff)},
		},
		MaxTokens:   s.maxTokens,
		Temperature: 0,
		NoThink:     s.noThink,
		ExtraBody:   s.extraBody,
	})
	if chatErr != nil {
		return fallbackCommitSummary(c, diff)
	}
	summary, parseErr := parseCommitSummary(resp.Content)
	if parseErr != nil {
		return fallbackCommitSummary(c, diff)
	}
	summary.Hash = c.Hash
	summary.Subject = c.Subject
	return summary
}

func buildCommitSummaryPrompt(c types.Commit, diff string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Commit: %s\nSubject: %s\n", c.Hash, c.Subject)
	if strings.TrimSpace(c.Description) != "" {
		fmt.Fprintf(&b, "Description:\n%s\n", c.Description)
	}
	if strings.TrimSpace(diff) != "" {
		fmt.Fprintf(&b, "Diff:\n%s\n", diff)
	} else {
		b.WriteString("Diff: none\n")
	}
	return b.String()
}

// parseCommitSummary extracts a single JSON object from the model reply. The
// subject and summary fields are required.
func parseCommitSummary(content string) (types.CommitSummary, error) {
	content = sanitizeRawOutput(content)
	raw := extractJSONObject(content)
	if raw == "" {
		return types.CommitSummary{}, fmt.Errorf("no JSON object in response")
	}
	var s types.CommitSummary
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return types.CommitSummary{}, fmt.Errorf("parse JSON: %w", err)
	}
	if strings.TrimSpace(s.Subject) == "" || strings.TrimSpace(s.Summary) == "" {
		return types.CommitSummary{}, fmt.Errorf("summary missing subject or summary")
	}
	return s, nil
}

func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}

// fallbackCommitSummary builds an entry from the commit subject and the file
// paths it touched, so a failed model call never drops a commit.
func fallbackCommitSummary(c types.Commit, diff string) types.CommitSummary {
	summary := types.CommitSummary{
		Hash:    c.Hash,
		Subject: c.Subject,
		Summary: c.Subject,
	}
	if desc := strings.TrimSpace(c.Description); desc != "" {
		summary.Summary += ". " + desc
	}
	summary.KeyChanges = changedPaths(diff)
	if len(summary.KeyChanges) == 0 {
		summary.KeyChanges = []string{"no diff available for this commit"}
	}
	return summary
}

func changedPaths(diff string) []string {
	var paths []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+++ b/") {
			continue
		}
		path := strings.TrimPrefix(line, "+++ b/")
		if path == "/dev/null" || seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths
}

func writeCommitSummaries(summaries []types.CommitSummary) (string, error) {
	data, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal commit summaries: %w", err)
	}
	file, err := os.CreateTemp("", "prlogue-commit-summaries-*.json")
	if err != nil {
		return "", fmt.Errorf("create commit summaries file: %w", err)
	}
	name := file.Name()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write commit summaries: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close commit summaries: %w", err)
	}
	return name, nil
}

// renderCommitSummaries formats the summaries as a compact context block for
// the final PR generation prompt.
func renderCommitSummaries(summaries []types.CommitSummary) string {
	var b strings.Builder
	b.WriteString("Commit summaries:\n")
	for i, s := range summaries {
		fmt.Fprintf(&b, "\n%d. %s (%s)\n", i+1, s.Subject, shortHash(s.Hash))
		if summary := strings.TrimSpace(s.Summary); summary != "" {
			fmt.Fprintf(&b, "   Summary: %s\n", summary)
		}
		if len(s.KeyChanges) > 0 {
			b.WriteString("   Key changes:\n")
			for _, k := range s.KeyChanges {
				fmt.Fprintf(&b, "   - %s\n", k)
			}
		}
		if impact := strings.TrimSpace(s.Impact); impact != "" {
			fmt.Fprintf(&b, "   Impact: %s\n", impact)
		}
	}
	return b.String()
}

func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
