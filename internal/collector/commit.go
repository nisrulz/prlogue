package collector

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/nisrulz/prlogue/internal/types"
)

func CollectCommits(baseBranch string, maxCount int) ([]types.Commit, error) {
	if maxCount <= 0 {
		maxCount = 50
	}

	args := []string{"log", fmt.Sprintf("%s..HEAD", baseBranch),
		"--format=%H|%an|%s",
		fmt.Sprintf("--max-count=%d", maxCount),
	}

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		// If base branch doesn't exist, try with origin/
		args[1] = fmt.Sprintf("origin/%s..HEAD", baseBranch)
		out, err = exec.Command("git", args...).Output()
		if err != nil {
			return nil, fmt.Errorf("git log: %w", err)
		}
	}

	return parseCommits(string(out)), nil
}

func parseCommits(output string) []types.Commit {
	var commits []types.Commit
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return commits
	}

	for _, line := range lines {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		commits = append(commits, types.Commit{
			Hash:    strings.TrimSpace(parts[0]),
			Author:  strings.TrimSpace(parts[1]),
			Subject: strings.TrimSpace(parts[2]),
		})
	}
	return commits
}
