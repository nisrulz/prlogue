//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOutputFormat covers JSON output, writing the body to a file with
// --output, and the failure path for an unwritable target.
func TestOutputFormat(t *testing.T) {
	t.Run("JSON output", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, "data.txt", "data\n")
		commitAll(t, dir, "feat: add data")
		writeFile(t, dir, ".prlogue.yaml", "output:\n  format: json\n")
		out, _ := runCLI(t, dir)
		if !strings.Contains(out, `"title"`) || !strings.Contains(out, `"stats"`) {
			t.Fatalf("output = %q, want JSON title and stats", out)
		}
	})

	t.Run("--output writes the body to a file", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, "x.txt", "x\n")
		commitAll(t, dir, "feat: add x")
		outFile := filepath.Join(t.TempDir(), "pr.md")
		if _, err := runCLI(t, dir, "--output", outFile); err != nil {
			t.Fatalf("generate: %v", err)
		}
		body, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatalf("read output file: %v", err)
		}
		assertGeneratedBody(t, string(body))
	})

	t.Run("--output to an unwritable path", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, "x.txt", "x\n")
		commitAll(t, dir, "feat: add x")
		out, _ := runCLI(t, dir, "--output", "/nonexistent-prlogue-dir/x.md")
		assertContains(t, out, "write output")
	})
}
