package processor

import (
	"fmt"

	"github.com/nisrulz/prlogue/internal/types"
)

func FallbackSummarize(chunks []types.Chunk) []types.ChunkSummary {
	out := make([]types.ChunkSummary, len(chunks))
	for i, c := range chunks {
		adds := 0
		dels := 0
		for _, line := range c.Lines {
			if len(line) > 0 {
				switch line[0] {
				case '+':
					adds++
				case '-':
					dels++
				}
			}
		}
		summary := fmt.Sprintf("changed %d lines (+%d/-%d)", adds+dels, adds, dels)
		if c.Hunk.NewLines > 0 {
			summary = fmt.Sprintf("modified %d lines (+%d/-%d)", c.Hunk.NewLines, adds, dels)
		}
		out[i] = types.ChunkSummary{
			ChunkID:    c.ID,
			OneLiner:   summary,
			KeyChanges: fmt.Sprintf("+%d/-%d", adds, dels),
			Risk:       "low",
		}
	}
	return out
}
