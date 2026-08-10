package processor

import (
	"testing"

	"github.com/nisrulz/prlogue/internal/config"
	"github.com/nisrulz/prlogue/internal/types"
)

func TestChunkDiff_TwoTier_FileLevel(t *testing.T) {
	diffs := []types.FileDiff{
		{
			Path:      "main.go",
			Additions: 10,
			Deletions: 5,
			Hunks: []types.Hunk{
				{Content: "+fmt.Println(\"hello\")\n-fmt.Println(\"old\")\n"},
			},
		},
	}
	cfg := config.ChunkConf{
		Strategy:             "two-tier",
		FileSummaryThreshold: 200,
		HunkSplitThreshold:   500,
	}
	chunks := ChunkDiff(diffs, cfg)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].FilePath != "main.go" {
		t.Errorf("file path = %q, want %q", chunks[0].FilePath, "main.go")
	}
}

func TestChunkDiff_TwoTier_HunkSplit(t *testing.T) {
	var lines string
	for i := 0; i < 100; i++ {
		lines += "+line\n"
	}
	diffs := []types.FileDiff{
		{
			Path:      "large.go",
			Additions: 300,
			Deletions: 0,
			Hunks: []types.Hunk{
				{Content: lines},
			},
		},
	}
	cfg := config.ChunkConf{
		Strategy:             "two-tier",
		FileSummaryThreshold: 50,
		HunkSplitThreshold:   30,
	}
	chunks := ChunkDiff(diffs, cfg)
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunk pieces, got %d", len(chunks))
	}
}

func TestChunkDiff_EmptyDiffs(t *testing.T) {
	chunks := ChunkDiff(nil, config.ChunkConf{})
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for nil diffs, got %d", len(chunks))
	}
}

func TestChunkDiff_ZeroSplitThresholdDoesNotPanic(t *testing.T) {
	diffs := []types.FileDiff{{
		Path:      "main.go",
		Additions: 2,
		Hunks:     []types.Hunk{{Content: "+one\n+two\n"}},
	}}
	chunks := ChunkDiff(diffs, config.ChunkConf{Strategy: "hunk", HunkSplitThreshold: 0})
	if len(chunks) != 1 {
		t.Fatalf("expected one safe hunk chunk, got %d", len(chunks))
	}
}

func TestEstimateTokens(t *testing.T) {
	lines := []string{"func main() {", "    return 0", "}"}
	tokens := estimateTokens(lines)
	if tokens <= 0 {
		t.Errorf("expected positive token estimate, got %d", tokens)
	}
}
