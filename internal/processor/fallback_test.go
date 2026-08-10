package processor_test

import (
	"testing"

	"github.com/nisrulz/prlogue/internal/processor"
	"github.com/nisrulz/prlogue/internal/types"
)

func TestFallbackSummarize(t *testing.T) {
	chunks := []types.Chunk{
		{
			ID:       "main.go/file",
			FilePath: "main.go",
			Lines:    []string{"+new line", "-old line", " context"},
		},
	}
	summaries := processor.FallbackSummarize(chunks)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].ChunkID != "main.go/file" {
		t.Errorf("chunk id = %q", summaries[0].ChunkID)
	}
	if summaries[0].OneLiner == "" {
		t.Error("expected non-empty summary")
	}
}
