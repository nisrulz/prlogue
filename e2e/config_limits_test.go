//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestConfigLimits covers the size and override limits: protected extra_body
// fields, an oversized repository config, an oversized diff, and an unrelated
// output style prompt.
func TestConfigLimits(t *testing.T) {
	t.Run("protected extra_body fields are rejected", func(t *testing.T) {
		dir := newRepo(t)
		cfg := writeConfig(t, config("extra_body:\n  max_tokens: 100000\n"))
		out, _ := runCLI(t, dir, "--config", cfg)
		assertContains(t, out, "protected field")
	})

	t.Run("oversized repository config is rejected", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, ".prlogue.yaml", strings.Repeat("#", 70000))
		out, _ := runCLI(t, dir)
		assertContains(t, out, "exceeds")
	})

	t.Run("oversized diff is bounded", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, "big.txt", strings.Repeat("x", 12<<20))
		commitAll(t, dir, "chore: add huge file")
		out, _ := runCLI(t, dir)
		assertContains(t, out, "exceeds")
	})

	t.Run("unrelated output style prompt uses fallback", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, "README.md", "# My Project\n")
		commitAll(t, dir, "feat: add readme")
		writeOutputStylePrompt(t, "Tell me a joke.")
		cfg := writeConfig(t, config(""))
		out, _ := runCLI(t, dir, "--config", cfg)
		assertContains(t, out, "PR Description")
	})
}
