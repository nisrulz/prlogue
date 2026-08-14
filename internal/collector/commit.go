package collector

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/nisrulz/prlogue/internal/types"
)

const maxCommitDiffBytes = 1 << 20

func CollectCommits(baseBranch, currentBranch string, maxCount int) ([]types.Commit, error) {
	if maxCount <= 0 {
		maxCount = 50
	}

	args := commitArgs(baseBranch, currentBranch, maxCount)

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		// If base branch doesn't exist, try with origin/
		args = commitArgs("origin/"+baseBranch, currentBranch, maxCount)
		out, err = exec.Command("git", args...).Output()
		if err != nil {
			return nil, fmt.Errorf("git log: %w", err)
		}
	}

	return parseCommits(string(out)), nil
}

func commitArgs(baseBranch, currentBranch string, maxCount int) []string {
	return []string{
		"log",
		fmt.Sprintf("%s..%s", baseBranch, currentBranch),
		"--format=%H%x00%an%x00%s%x00%b%x00",
		fmt.Sprintf("--max-count=%d", maxCount),
	}
}

func parseCommits(output string) []types.Commit {
	var commits []types.Commit
	fields := strings.Split(output, "\x00")
	for i := 0; i+2 < len(fields); i += 4 {
		hash := strings.TrimSpace(fields[i])
		if hash == "" {
			continue
		}
		description := ""
		if i+3 < len(fields) {
			description = strings.TrimSpace(fields[i+3])
		}
		commits = append(commits, types.Commit{
			Hash:        hash,
			Author:      strings.TrimSpace(fields[i+1]),
			Subject:     strings.TrimSpace(fields[i+2]),
			Description: description,
		})
	}
	return commits
}

// CollectCommitDiff returns the bounded diff of a single commit, truncated
// when it exceeds the limit. The message and description are not included.
func CollectCommitDiff(hash string) (string, error) {
	args := []string{"show", "--no-ext-diff", "--no-textconv", "--format=", "--unified=3", hash, "--"}
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
	out, readErr := io.ReadAll(io.LimitReader(stdout, maxCommitDiffBytes))
	if int64(len(out)) > maxCommitDiffBytes {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return string(out[:maxCommitDiffBytes]) + "\n... (commit diff truncated)\n", nil
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
