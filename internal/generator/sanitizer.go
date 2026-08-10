package generator

import (
	"strings"
	"unicode"
)

func sanitizeLLMOutput(s string) string {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return ' '
		}
		switch r {
		case '\u200B', '\u200C', '\u200D', '\u2060', '\uFEFF':
			return ' '
		case '\u202A', '\u202B', '\u202C', '\u202D', '\u202E', '\u2066', '\u2067', '\u2068', '\u2069':
			return ' '
		default:
			return r
		}
	}, s)
	return sanitizeLLMStructure(clean)
}

func sanitizeLLMStructure(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	seenBullets := make(map[string]bool)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isRedundantSection(trimmed) {
			break
		}
		if hasUnclosedShortcode(trimmed) {
			continue
		}
		if cleaned, keep := sanitizeCategoryBullet(trimmed); !keep {
			continue
		} else if cleaned != "" {
			line = cleaned
			trimmed = cleaned
		}
		if isPlaceholderLine(trimmed) {
			continue
		}
		if isBullet(trimmed) {
			key := normalizeForComparison(trimmed)
			if seenBullets[key] {
				continue
			}
			seenBullets[key] = true
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(addHeadingSpacing(removeEmptySections(out)), "\n"))
}

func addHeadingSpacing(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if isHeading(strings.TrimSpace(line)) && len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out, line)
	}
	return out
}

func sanitizeCategoryBullet(line string) (string, bool) {
	value, ok := bulletValue(line)
	if !ok {
		return line, true
	}
	colon := strings.IndexByte(value, ':')
	if colon < 0 || normalizeForComparison(value[:colon]) != "category" {
		return line, true
	}
	value = strings.TrimSpace(strings.Trim(value[colon+1:], "*` "))
	if isPlaceholderValue(value) {
		return "", false
	}
	return "- " + value, value != ""
}

func removeEmptySections(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		if !isHeading(strings.TrimSpace(lines[i])) {
			out = append(out, lines[i])
			i++
			continue
		}
		start := i
		i++
		content := false
		for i < len(lines) && !isHeading(strings.TrimSpace(lines[i])) {
			if strings.TrimSpace(lines[i]) != "" {
				content = true
			}
			i++
		}
		if content {
			out = append(out, lines[start:i]...)
		}
	}
	return out
}

func isRedundantSection(line string) bool {
	line = strings.TrimSpace(strings.TrimLeft(line, "# "))
	return strings.EqualFold(line, "Commits") || strings.EqualFold(line, "Related Issues")
}

func hasUnclosedShortcode(line string) bool {
	return (strings.Contains(line, "{{<") && !strings.Contains(line, ">}}")) ||
		(strings.Contains(line, "{{%") && !strings.Contains(line, "%}}"))
}

func isHeading(line string) bool {
	return strings.HasPrefix(line, "#")
}

func isBullet(line string) bool {
	_, ok := bulletValue(line)
	return ok
}

func bulletValue(line string) (string, bool) {
	line = strings.TrimSpace(line)
	runes := []rune(line)
	if len(runes) < 2 || (runes[0] != '-' && runes[0] != '*' && runes[0] != '\u2022') {
		return "", false
	}
	value := strings.TrimSpace(string(runes[1:]))
	return value, value != ""
}

func isPlaceholderLine(line string) bool {
	return isPlaceholderValue(line) || normalizeForComparison(line) == "key changes"
}

func isPlaceholderTitle(title string) bool {
	return isPlaceholderValue(title) || normalizeForComparison(title) == "pr title" || normalizeForComparison(title) == "title"
}

func isPlaceholderValue(value string) bool {
	normalized := normalizeForComparison(value)
	switch normalized {
	case "", "explanation", "description", "details", "change", "changes", "placeholder", "n a", "na", "none", "tbd", "todo":
		return true
	}
	return strings.HasPrefix(normalized, "<") && strings.HasSuffix(normalized, ">")
}

func normalizeForComparison(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Trim(s, "*`_:- ")
	return strings.Join(strings.Fields(s), " ")
}
