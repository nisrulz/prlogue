//go:build e2e

package e2e

import (
	"fmt"
	"testing"
)

// TestChunking verifies that a large diff engages the two-tier chunker and
// still completes end to end.
func TestChunking(t *testing.T) {
	t.Run("large diff engages chunking", func(t *testing.T) {
		dir := newRepo(t)
		for i := 1; i <= 10; i++ {
			writeFile(t, dir, fmt.Sprintf("file%d.go", i), baseGoFile(i))
		}
		commitAll(t, dir, "chore: scaffold files")
		for i := 1; i <= 10; i++ {
			writeFile(t, dir, fmt.Sprintf("file%d.go", i), extendedGoFile(i))
		}
		commitAll(t, dir, "feat: extend files")

		out, _ := runCLI(t, dir, "-v")
		assertContains(t, out, "Chunks:")
		assertGeneratedBody(t, out)
	})
}

func baseGoFile(i int) string {
	return fmt.Sprintf("package main\n\nfunc base%d() string {\n  return \"base %d\"\n}\n", i, i)
}

func extendedGoFile(i int) string {
	return baseGoFile(i) + fmt.Sprintf("\nfunc added%d() string {\n  return \"added %d\"\n}\n", i, i)
}
