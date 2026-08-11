//go:build e2e

package e2e

import "testing"

// TestDoctor verifies the health check against the active provider (mock or a
// live model server).
func TestDoctor(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "x.txt", "x\n")
	commitAll(t, dir, "feat: add x")
	out, _ := runBinary(t, dir, "doctor")
	assertContains(t, out, "All checks passed.")
}
