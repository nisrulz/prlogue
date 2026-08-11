//go:build e2e

package e2e

import "testing"

// TestRepoState covers how the CLI detects repository state: outside a git
// repo, a branch with no changes, nothing staged, an empty repo, and
// untracked-only work.
func TestRepoState(t *testing.T) {
	t.Run("outside a git repository", func(t *testing.T) {
		dir := t.TempDir()
		out, _ := runCLI(t, dir)
		assertAnyContains(t, out, "could not detect default branch", "not a git repository")
	})

	t.Run("no changes on the branch", func(t *testing.T) {
		dir := newRepo(t)
		out, _ := runCLI(t, dir)
		assertContains(t, out, "no changes found")
	})

	t.Run("--staged with nothing staged", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, "README.md", "edited\n")
		out, _ := runCLI(t, dir, "--staged")
		assertContains(t, out, "no staged changes found")
	})

	t.Run("empty repo with no commits", func(t *testing.T) {
		dir := t.TempDir()
		git(t, dir, "init", "-q")
		git(t, dir, "config", "user.email", "test@test")
		git(t, dir, "config", "user.name", "Test")
		out, _ := runCLI(t, dir)
		assertContains(t, out, "could not detect base branch")
	})

	t.Run("untracked-only changes", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, "untracked.txt", "new\n")
		out, _ := runCLI(t, dir)
		assertContains(t, out, "no changes found")
	})
}
