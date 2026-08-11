//go:build e2e

package e2e

import (
	"os"
	"testing"
)

// TestRepoConfig covers the repository .prlogue.yaml allowlist and the
// explicit --config path.
func TestRepoConfig(t *testing.T) {
	t.Run("repository config allowlist is honored", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, "data.txt", "data\n")
		commitAll(t, dir, "feat: add data")
		writeFile(t, dir, ".prlogue.yaml", "output:\n  format: json\n")
		out, _ := runCLI(t, dir)
		assertContains(t, out, `"title"`)
	})

	t.Run("repository config rejects disallowed keys", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, ".prlogue.yaml", "provider: openai_compat\n")
		out, _ := runCLI(t, dir)
		assertContains(t, out, "not allowed")
	})

	t.Run("corrupt repository config errors cleanly", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, ".prlogue.yaml", "output: [unclosed\n")
		out, _ := runCLI(t, dir)
		assertContains(t, out, "project config")
	})

	t.Run("repository config must be a regular file", func(t *testing.T) {
		dir := newRepo(t)
		if err := os.MkdirAll(dir+"/.prlogue.yaml", 0o755); err != nil {
			t.Fatalf("mkdir .prlogue.yaml: %v", err)
		}
		out, _ := runCLI(t, dir)
		assertContains(t, out, "regular file")
	})

	t.Run("explicit --config is honored", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, "x.txt", "x\n")
		commitAll(t, dir, "feat: add x")
		cfg := writeConfig(t, configJSON())
		out, _ := runCLI(t, dir, "--config", cfg)
		assertContains(t, out, `"title"`)
	})
}
