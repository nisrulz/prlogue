package collector

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nisrulz/prlogue/internal/types"
)

const maxDiffBytes = 8 << 20

func CollectDiff(baseBranch string, staged bool) ([]types.FileDiff, error) {
	args := diffArgs(baseBranch, staged)
	out, err := commandOutput(args, maxDiffBytes)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	return parseDiff(out), nil
}

func diffArgs(baseBranch string, staged bool) []string {
	args := []string{"diff", "--no-ext-diff", "--no-textconv"}
	if staged {
		args = append(args, "--staged")
	} else {
		args = append(args, baseBranch+"..HEAD")
	}
	args = append(args, "--unified=3", "--")
	return args
}

func commandOutput(args []string, limit int64) (string, error) {
	cmd := exec.Command("git", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	out, readErr := io.ReadAll(io.LimitReader(stdout, limit+1))
	if int64(len(out)) > limit {
		if killErr := cmd.Process.Kill(); killErr != nil {
			// Process may have already exited — only log if kill actually failed.
			if waitErr := cmd.Wait(); waitErr == nil {
				return "", fmt.Errorf("output exceeds %d bytes (kill failed: %v)", limit, killErr)
			}
		} else {
			_ = cmd.Wait()
		}
		return "", fmt.Errorf("output exceeds %d bytes", limit)
	}
	waitErr := cmd.Wait()
	if readErr != nil {
		return "", readErr
	}
	if waitErr != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%w: %s", waitErr, strings.TrimSpace(stderr.String()))
		}
		return "", waitErr
	}
	return string(out), nil
}

func parseDiff(output string) []types.FileDiff {
	var files []types.FileDiff
	var current *types.FileDiff

	lines := strings.Split(output, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				files = append(files, *current)
			}
			current = &types.FileDiff{
				Status: "modified",
			}
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "--- ") {
			if line == "--- /dev/null" {
				current.Status = "added"
			}
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			if line == "+++ /dev/null" {
				current.Status = "deleted"
			}
			// Extract file path from "+++ b/path"
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) == 2 {
				current.Path = strings.TrimSpace(parts[1])
			}
			continue
		}

		if strings.HasPrefix(line, "new file mode") {
			current.Status = "added"
			continue
		}
		if strings.HasPrefix(line, "deleted file mode") {
			current.Status = "deleted"
			continue
		}
		if strings.HasPrefix(line, "rename from ") {
			current.Status = "renamed"
			continue
		}
		if strings.HasPrefix(line, "Binary files ") {
			current.Status = "binary"
			continue
		}

		if strings.HasPrefix(line, "@@") {
			hunk := parseHunkHeader(line)
			if hunk != nil {
				hunk.FilePath = current.Path
				// Collect hunk content
				contentStart := i + 1
				for j := contentStart; j < len(lines); j++ {
					if strings.HasPrefix(lines[j], "@@") || strings.HasPrefix(lines[j], "diff --git ") {
						break
					}
					if len(lines[j]) > 0 {
						ch := lines[j][0]
						if ch == ' ' || ch == '+' || ch == '-' || ch == '\\' {
							hunk.Content += lines[j] + "\n"
							if ch == '+' {
								current.Additions++
							} else if ch == '-' {
								current.Deletions++
							}
						}
					}
				}
				current.Hunks = append(current.Hunks, *hunk)
			}
		}
	}

	if current != nil {
		files = append(files, *current)
	}

	return files
}

func parseHunkHeader(line string) *types.Hunk {
	// @@ -oldStart,oldCount +newStart,newCount @@ optional context
	line = strings.TrimPrefix(line, "@@")
	idx := strings.Index(line, "@@")
	if idx < 0 {
		return nil
	}
	header := strings.TrimSpace(line[:idx])

	parts := strings.Split(header, " ")
	if len(parts) < 2 {
		return nil
	}

	oldPart := strings.TrimPrefix(parts[0], "-")
	newPart := strings.TrimPrefix(parts[1], "+")

	hunk := &types.Hunk{}
	hunk.OldStart, _ = parseHunkPos(oldPart)
	hunk.OldLines, _ = parseHunkCount(oldPart)
	hunk.NewStart, _ = parseHunkPos(newPart)
	hunk.NewLines, _ = parseHunkCount(newPart)

	return hunk
}

func parseHunkPos(s string) (int, error) {
	parts := strings.Split(s, ",")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid hunk pos: %s", s)
	}
	return strconv.Atoi(parts[0])
}

func parseHunkCount(s string) (int, error) {
	parts := strings.Split(s, ",")
	if len(parts) < 2 {
		return 1, nil // default 1 line if count omitted
	}
	return strconv.Atoi(parts[1])
}
