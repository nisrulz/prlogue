package generator

import "strings"

// extractLLMTitle splits an optional title line from the body. It accepts the
// "Title:" label and a leading Markdown "# " heading.
func extractLLMTitle(s string) (title, body string) {
	lines := strings.SplitN(s, "\n", 2)
	first := strings.TrimSpace(lines[0])
	if len(lines) == 2 && strings.HasPrefix(first, titleLabel) {
		title = strings.TrimSpace(strings.TrimPrefix(first, titleLabel))
		body = strings.TrimSpace(lines[1])
		return title, body
	}
	if len(lines) == 2 && strings.HasPrefix(first, "# ") {
		title = strings.TrimSpace(strings.TrimPrefix(first, "# "))
		body = strings.TrimSpace(lines[1])
		return title, body
	}
	return "", s
}

// normalizeLLMSummary adds the PR description heading when the model omitted
// it but produced a body.
func normalizeLLMSummary(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, descriptionHeading) {
		return s
	}
	return descriptionHeading + "\n\n" + s
}
