package generator

import (
	"fmt"
	"strings"
)

// rejectionReason explains why model output is unusable. reasonNone means the
// output is accepted.
type rejectionReason int

const (
	reasonNone rejectionReason = iota
	reasonUnusable
	reasonAcknowledgment
	reasonRefusal
	reasonNoChanges
	reasonFormat
)

func (r rejectionReason) String() string {
	switch r {
	case reasonUnusable:
		return "output failed PR description validation"
	case reasonAcknowledgment:
		return "model echoed an acknowledgment instead of generating"
	case reasonRefusal:
		return "model refused to summarize the changes"
	case reasonNoChanges:
		return "model reported no changes despite repository data"
	case reasonFormat:
		return "output does not follow the configured output format"
	default:
		return ""
	}
}

// outputRejectionReason classifies why the model output is unusable, or returns
// reasonNone when the output is accepted.
func outputRejectionReason(input *GenerateInput, useStandardStyle bool, s string) rejectionReason {
	switch {
	case !isUsableLLMOutput(s):
		return reasonUnusable
	case isAckOnlyOutput(s):
		return reasonAcknowledgment
	case isRefusalOutput(s):
		return reasonRefusal
	case input.DiffStats.Files > 0 && claimsNoChanges(s):
		return reasonNoChanges
	case !outputFollowsFormat(useStandardStyle, input.DiffStats.Files, s):
		return reasonFormat
	}
	return reasonNone
}

// outputFollowsFormat reports whether the reply carries the headings the
// standard output style requires: a PR description section and, when files
// changed, a key changes section.
func outputFollowsFormat(useStandardStyle bool, files int, s string) bool {
	if !useStandardStyle {
		return true
	}
	if !strings.Contains(s, descriptionHeading) {
		return false
	}
	if files > 0 && !strings.Contains(s, keyChangesHeading) {
		return false
	}
	return true
}

// buildRetryInstruction points the model back at the repository data and the
// collected statistics after a rejected output.
func buildRetryInstruction(input *GenerateInput, useStandardStyle bool, reason rejectionReason) string {
	instruction := fmt.Sprintf(
		"Your previous reply was rejected: %s. The repository data above contains %d changed file(s) with +%d/-%d lines and %d commit(s). Read the diff and the commit list in the repository data above. Generate the PR title and description from that data. Do not claim there are no changes and do not reply with an acknowledgment.",
		reason, input.DiffStats.Files, input.DiffStats.Additions, input.DiffStats.Deletions, len(input.Commits))
	if useStandardStyle {
		instruction += " Follow the output format: put 'Title: <title>' on the first line, then '## PR Description' with the summary, then '### Key Changes' with one bullet per change."
	}
	return instruction
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
