package processor

import (
	"fmt"
	"strings"

	"github.com/nisrulz/prlogue/internal/types"
)

type Merger struct {
	classifications []FileClassification
}

func NewMerger(classifications []FileClassification) *Merger {
	return &Merger{classifications: classifications}
}

func (m *Merger) Merge(summaries []types.ChunkSummary) []types.MergedSummary {
	grouped := make(map[string]*mergeGroup)
	var order []string

	for _, s := range summaries {
		filePath := chunkFilepath(s.ChunkID)
		ct := m.changeTypeForFile(filePath)

		key := fmt.Sprintf("%s|%s", filePath, ct)
		if _, ok := grouped[key]; !ok {
			order = append(order, key)
			grouped[key] = &mergeGroup{
				filePath:   filePath,
				changeType: string(ct),
				summaries:  make([]string, 0),
			}
		}
		grouped[key].summaries = append(grouped[key].summaries, s.OneLiner)
	}

	merged := make([]types.MergedSummary, 0, len(grouped))
	for _, key := range order {
		g := grouped[key]
		merged = append(merged, types.MergedSummary{
			FilePath:   g.filePath,
			ChangeType: g.changeType,
			Summary:    g.combine(),
		})
	}

	return merged
}

type mergeGroup struct {
	filePath   string
	changeType string
	summaries  []string
}

func (g *mergeGroup) combine() string {
	if len(g.summaries) == 0 {
		return ""
	}
	if len(g.summaries) == 1 {
		return g.summaries[0]
	}

	seen := make(map[string]bool)
	unique := make([]string, 0, len(g.summaries))
	for _, s := range g.summaries {
		normalized := strings.TrimSpace(strings.ToLower(s))
		if !seen[normalized] {
			seen[normalized] = true
			unique = append(unique, s)
		}
	}

	if len(unique) == 1 {
		return unique[0]
	}

	return unique[0] + "; " + strings.Join(unique[1:], "; ")
}

func (m *Merger) changeTypeForFile(filePath string) types.ChangeType {
	for _, c := range m.classifications {
		if c.FilePath == filePath {
			return c.ChangeType
		}
	}
	return types.ChangeChore
}

func chunkFilepath(chunkID string) string {
	segs := strings.Split(chunkID, "/")
	// Chunk IDs are "<filepath>/hunk<N>" or "<filepath>/hunk<N>/piece<M>";
	// strip those trailing segments to recover the file path.
	if len(segs) > 0 && strings.HasPrefix(segs[len(segs)-1], "piece") {
		segs = segs[:len(segs)-1]
	}
	if len(segs) > 0 && strings.HasPrefix(segs[len(segs)-1], "hunk") {
		segs = segs[:len(segs)-1]
	}
	if len(segs) > 0 && segs[len(segs)-1] == "file" {
		segs = segs[:len(segs)-1]
	}
	if len(segs) == 0 {
		return chunkID
	}
	return strings.Join(segs, "/")
}
