package processor

import (
	"fmt"
	"strings"

	"github.com/nisrulz/prlogue/internal/config"
	"github.com/nisrulz/prlogue/internal/types"
)

func ChunkDiff(diffs []types.FileDiff, cfg config.ChunkConf) []types.Chunk {
	var chunks []types.Chunk

	for _, file := range diffs {
		totalLines := file.Additions + file.Deletions

		switch cfg.Strategy {
		case "two-tier":
			if totalLines <= cfg.FileSummaryThreshold {
				chunks = append(chunks, fileLevelChunk(file))
			} else {
				chunks = append(chunks, perHunkChunks(file, cfg.HunkSplitThreshold)...)
			}
		case "file":
			chunks = append(chunks, fileLevelChunk(file))
		case "hunk":
			chunks = append(chunks, perHunkChunks(file, cfg.HunkSplitThreshold)...)
		default:
			chunks = append(chunks, fileLevelChunk(file))
		}
	}

	return chunks
}

func fileLevelChunk(file types.FileDiff) types.Chunk {
	var lines []string
	for _, h := range file.Hunks {
		lines = append(lines, strings.Split(h.Content, "\n")...)
	}

	return types.Chunk{
		ID:       fmt.Sprintf("%s/file", file.Path),
		FilePath: file.Path,
		Lines:    lines,
		TokenEst: estimateTokens(lines),
	}
}

func perHunkChunks(file types.FileDiff, splitThreshold int) []types.Chunk {
	var chunks []types.Chunk

	for i, hunk := range file.Hunks {
		hunkLines := strings.Split(hunk.Content, "\n")
		threshold := splitThreshold
		if threshold <= 0 {
			threshold = len(hunkLines)
		}

		if len(hunkLines) > threshold {
			for j := 0; j < len(hunkLines); j += threshold {
				end := j + threshold
				if end > len(hunkLines) {
					end = len(hunkLines)
				}
				piece := hunkLines[j:end]
				chunks = append(chunks, types.Chunk{
					ID:       fmt.Sprintf("%s/hunk%d/piece%d", file.Path, i, j/threshold),
					FilePath: file.Path,
					Hunk:     hunk,
					Lines:    piece,
					TokenEst: estimateTokens(piece),
				})
			}
		} else {
			chunks = append(chunks, types.Chunk{
				ID:       fmt.Sprintf("%s/hunk%d", file.Path, i),
				FilePath: file.Path,
				Hunk:     hunk,
				Lines:    hunkLines,
				TokenEst: estimateTokens(hunkLines),
			})
		}
	}

	return chunks
}

func estimateTokens(lines []string) int {
	count := 0
	for _, l := range lines {
		count += len(strings.Fields(l)) + 1
	}
	return count * 2 // rough heuristic: ~2 tokens per word
}
