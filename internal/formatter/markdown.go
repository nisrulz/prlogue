package formatter

import (
	"fmt"
	"strings"

	"github.com/nisrulz/prlogue/internal/generator"
)

func FormatMarkdown(result *generator.GenerateResult, input *generator.GenerateInput) string {
	var b strings.Builder

	appendSection := func(s string) {
		s = strings.TrimRight(s, "\n")
		if s == "" {
			return
		}
		if b.Len() > 0 {
			fmt.Fprint(&b, "\n\n")
		}
		b.WriteString(s)
	}

	if result.Title != "" {
		b.WriteString("# " + result.Title)
	}

	if !result.TemplateUsed && result.Summary != "" {
		appendSection(result.Summary)
	} else {
		appendSection(result.Body)
	}

	return b.String() + "\n"
}
