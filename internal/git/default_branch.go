package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func DetectDefaultBranch() (string, error) {
	branch, err := remoteHEAD()
	if err == nil && branch != "" {
		return branch, nil
	}
	for _, name := range []string{"main", "master", "develop"} {
		if localBranchExists(name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("could not detect default branch, use --from")
}

func remoteHEAD() (string, error) {
	out, err := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD").Output()
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(out))
	ref = strings.TrimPrefix(ref, "refs/remotes/origin/")
	if ref == "" {
		return "", fmt.Errorf("empty remote HEAD ref")
	}
	return ref, nil
}

func localBranchExists(name string) bool {
	err := exec.Command("git", "rev-parse", "--verify", name).Run()
	return err == nil
}

// InsideWorkTree reports whether the current directory is inside a git work
// tree, which is what generate requires.
func InsideWorkTree() bool {
	out, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

func CurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("not on a branch (detached HEAD)")
	}
	return branch, nil
}

func ValidateBranch(branch string) error {
	if strings.TrimSpace(branch) == "" {
		return fmt.Errorf("empty branch name")
	}
	if err := exec.Command("git", "check-ref-format", "--branch", branch).Run(); err != nil {
		return fmt.Errorf("invalid branch %q", branch)
	}
	return nil
}
