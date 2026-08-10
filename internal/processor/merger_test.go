package processor

import (
	"testing"

	"github.com/nisrulz/prlogue/internal/types"
)

func TestMerge_Deduplicates(t *testing.T) {
	classifications := []FileClassification{
		{FilePath: "main.go", ChangeType: "feat"},
	}
	m := NewMerger(classifications)
	summaries := []types.ChunkSummary{
		{ChunkID: "main.go/hunk0", OneLiner: "added login handler"},
		{ChunkID: "main.go/hunk1", OneLiner: "added login handler"},
	}
	merged := m.Merge(summaries)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged summary after dedup, got %d", len(merged))
	}
}

func TestMerge_GroupByType(t *testing.T) {
	classifications := []FileClassification{
		{FilePath: "feat.go", ChangeType: "feat"},
		{FilePath: "fix.go", ChangeType: "fix"},
		{FilePath: "chore.go", ChangeType: "chore"},
	}
	m := NewMerger(classifications)
	summaries := []types.ChunkSummary{
		{ChunkID: "feat.go/hunk0", OneLiner: "new feature"},
		{ChunkID: "fix.go/hunk0", OneLiner: "bug fix"},
		{ChunkID: "chore.go/hunk0", OneLiner: "bump deps"},
	}
	merged := m.Merge(summaries)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged, got %d", len(merged))
	}
	for i, want := range []string{"feat.go", "fix.go", "chore.go"} {
		if merged[i].FilePath != want {
			t.Fatalf("merged[%d].FilePath = %q, want %q", i, merged[i].FilePath, want)
		}
	}
}

func TestChunkFilepath_RootLevel(t *testing.T) {
	got := chunkFilepath("main.go/hunk0")
	if got != "main.go" {
		t.Errorf("chunkFilepath(%q) = %q, want %q", "main.go/hunk0", got, "main.go")
	}
}

func TestChunkFilepath_Nested(t *testing.T) {
	got := chunkFilepath("path/to/file.go/hunk0")
	if got != "path/to/file.go" {
		t.Errorf("chunkFilepath(%q) = %q, want %q", "path/to/file.go/hunk0", got, "path/to/file.go")
	}
}

func TestChunkFilepath_Piece(t *testing.T) {
	got := chunkFilepath("path/to/file.go/hunk0/piece1")
	if got != "path/to/file.go" {
		t.Errorf("chunkFilepath(%q) = %q, want %q", "path/to/file.go/hunk0/piece1", got, "path/to/file.go")
	}
}

func TestChunkFilepath_FileLevel(t *testing.T) {
	got := chunkFilepath("path/to/file.go/file")
	if got != "path/to/file.go" {
		t.Errorf("chunkFilepath(%q) = %q, want %q", "path/to/file.go/file", got, "path/to/file.go")
	}
}

func TestMerge_FileLevelPreservesClassification(t *testing.T) {
	m := NewMerger([]FileClassification{{FilePath: "main.go", ChangeType: "feat"}})
	merged := m.Merge([]types.ChunkSummary{{ChunkID: "main.go/file", OneLiner: "changed code"}})
	if len(merged) != 1 || merged[0].FilePath != "main.go" || merged[0].ChangeType != "feat" {
		t.Fatalf("file-level merge lost metadata: %+v", merged)
	}
}

func TestMerge_PieceIDs_GroupByType(t *testing.T) {
	classifications := []FileClassification{
		{FilePath: "internal/foo.go", ChangeType: "feat"},
	}
	m := NewMerger(classifications)
	summaries := []types.ChunkSummary{
		{ChunkID: "internal/foo.go/hunk0/piece0", OneLiner: "added login"},
		{ChunkID: "internal/foo.go/hunk0/piece1", OneLiner: "added login"},
	}
	merged := m.Merge(summaries)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged summary, got %d", len(merged))
	}
	if merged[0].ChangeType != "feat" {
		t.Errorf("ChangeType = %q, want feat (classification lost)", merged[0].ChangeType)
	}
	if merged[0].FilePath != "internal/foo.go" {
		t.Errorf("FilePath = %q, want %q", merged[0].FilePath, "internal/foo.go")
	}
}
