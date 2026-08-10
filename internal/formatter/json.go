package formatter

import (
	"encoding/json"
	"fmt"

	"github.com/nisrulz/prlogue/internal/generator"
)

type jsonOutput struct {
	Title   string       `json:"title"`
	Summary string       `json:"summary"`
	Body    string       `json:"body"`
	Raw     string       `json:"raw,omitempty"`
	Stats   jsonStats    `json:"stats"`
	Issues  []string     `json:"issues,omitempty"`
	Changes []jsonChange `json:"changes"`
	Commits []jsonCommit `json:"commits"`
}

type jsonStats struct {
	Files     int `json:"files"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

type jsonChange struct {
	FilePath   string `json:"file"`
	ChangeType string `json:"type"`
	Summary    string `json:"summary"`
}

type jsonCommit struct {
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
	Author  string `json:"author,omitempty"`
}

func FormatJSON(result *generator.GenerateResult, input *generator.GenerateInput) (string, error) {
	out := jsonOutput{
		Title: result.Title,
		Body:  result.Body,
		Stats: jsonStats{
			Files:     input.DiffStats.Files,
			Additions: input.DiffStats.Additions,
			Deletions: input.DiffStats.Deletions,
		},
		Issues:  input.BranchCtx.IssueRefs,
		Changes: make([]jsonChange, len(input.Merged)),
		Commits: make([]jsonCommit, len(input.Commits)),
	}

	if !result.TemplateUsed {
		out.Summary = result.Summary
		out.Body = result.Summary
		out.Raw = result.Raw
	}

	for i, m := range input.Merged {
		out.Changes[i] = jsonChange{
			FilePath:   m.FilePath,
			ChangeType: m.ChangeType,
			Summary:    m.Summary,
		}
	}

	for i, c := range input.Commits {
		out.Commits[i] = jsonCommit{
			Hash:    c.Hash[:7],
			Subject: c.Subject,
			Author:  c.Author,
		}
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}

	return string(b), nil
}
