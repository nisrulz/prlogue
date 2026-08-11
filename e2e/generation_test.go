//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestGeneration covers PR body generation across the awkward inputs: staged
// and committed changes, binary files, unicode names, renames, deletes,
// special characters, a detached HEAD, and deletion-only diffs.
func TestGeneration(t *testing.T) {
	cases := []struct {
		name   string
		staged bool
		setup  func(*testing.T, string)
	}{
		{
			name: "committed changes produce a PR body",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "README.md", "# My Project\n")
				commitAll(t, dir, "feat: add readme")
				writeFile(t, dir, "README.md", "# My Project\n## Usage\nRun make build.\n")
				commitAll(t, dir, "docs: add usage section")
			},
		},
		{
			name:   "staged changes generate a body",
			staged: true,
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "handler.go", "new handler\n")
				git(t, dir, "add", "handler.go")
			},
		},
		{
			name: "binary files do not crash the pipeline",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "logo.bin", string([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46}))
				commitAll(t, dir, "chore: add logo")
			},
		},
		{
			name: "unicode filenames",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "café.go", "package main\n")
				commitAll(t, dir, "feat: add café module")
			},
		},
		{
			name: "renamed and deleted files",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "keep.txt", "keep me\n")
				writeFile(t, dir, "gone.txt", "delete me\n")
				commitAll(t, dir, "feat: add files")
				git(t, dir, "mv", "keep.txt", "moved.txt")
				git(t, dir, "rm", "-q", "gone.txt")
				commitAll(t, dir, "chore: rename and remove files")
			},
		},
		{
			name: "special characters in the diff",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "special.txt", "normal line\n")
				commitAll(t, dir, "feat: add file")
				writeFile(t, dir, "special.txt", "normal line\nline with $dollar and `backticks`\nline with \"quotes\" and \\backslash\n")
				commitAll(t, dir, "chore: add special lines")
			},
		},
		{
			name: "generates from a detached HEAD",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "x.txt", "content\n")
				commitAll(t, dir, "feat: add x")
				sha, err := gitOutput(dir, "rev-parse", "HEAD")
				if err != nil {
					t.Fatalf("rev-parse HEAD: %v", err)
				}
				git(t, dir, "checkout", "-q", strings.TrimSpace(sha))
			},
		},
		{
			name:   "generates from a deletion-only diff",
			staged: true,
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "a\nb\nc\n")
				commitAll(t, dir, "chore: add file")
				git(t, dir, "rm", "-q", "f.txt")
			},
		},
		{
			name: "handles filenames with spaces",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "my file.txt", "x\n")
				git(t, dir, "add", "my file.txt")
				commitAll(t, dir, "feat: add spaced file")
			},
		},
		{
			name: "mixed change types split into sections",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "docs/readme.md", "readme\n")
				writeFile(t, dir, "app.go", "code\n")
				commitAll(t, dir, "feat: add login flow")
			},
		},
		{
			name: "issue refs surface in output",
			setup: func(t *testing.T, dir string) {
				git(t, dir, "checkout", "-q", "-b", "feat/PROJ-123-login")
				writeFile(t, dir, "x.txt", "x\n")
				commitAll(t, dir, "feat: wire up login")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := newRepo(t)
			tc.setup(t, dir)
			var args []string
			if tc.staged {
				args = append(args, "--staged")
			}
			out, _ := runCLI(t, dir, args...)
			assertGeneratedBody(t, out)
		})
	}
}
