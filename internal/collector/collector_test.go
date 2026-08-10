package collector

import (
	"slices"
	"strings"
	"testing"
)

func TestDiffArgs_DisablesExternalDrivers(t *testing.T) {
	args := diffArgs("main", false)
	for _, required := range []string{"--no-ext-diff", "--no-textconv", "main..HEAD", "--"} {
		if !slices.Contains(args, required) {
			t.Errorf("diff args missing %q: %v", required, args)
		}
	}
}

func TestCommandOutput_EnforcesLimit(t *testing.T) {
	_, err := commandOutput([]string{"--version"}, 1)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected output limit error, got %v", err)
	}
}

func TestParseDiff(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
new file mode 100644
--- /dev/null
+++ b/main.go
@@ -0,0 +1,3 @@
+package main
+
+func main() {
+}
`
	files := parseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Path != "main.go" {
		t.Errorf("path = %q, want %q", f.Path, "main.go")
	}
	if f.Status != "added" {
		t.Errorf("status = %q, want %q", f.Status, "added")
	}
}

func TestParseDiff_DeletedFile(t *testing.T) {
	diff := `diff --git a/old.go b/old.go
deleted file mode 100644
--- a/old.go
+++ /dev/null
@@ -1,2 +0,0 @@
-func old() {
-}
`
	files := parseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Status != "deleted" {
		t.Errorf("status = %q, want %q", f.Status, "deleted")
	}
	if f.Deletions != 2 {
		t.Errorf("deletions = %d, want 2", f.Deletions)
	}
}

func TestParseDiff_ModifiedFile(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
index abc..def 100644
--- a/a.go
+++ b/a.go
@@ -1,3 +1,4 @@
 line1
-line2
+line2 modified
+line3
 line4
`
	files := parseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Status != "modified" {
		t.Errorf("status = %q, want %q", f.Status, "modified")
	}
}

func TestParseDiff_RenamedFile(t *testing.T) {
	diff := `diff --git a/old.go b/new.go
rename from old.go
rename to new.go
`
	files := parseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Status != "renamed" {
		t.Errorf("status = %q, want %q", files[0].Status, "renamed")
	}
}

func TestParseDiff_BinaryFile(t *testing.T) {
	diff := `diff --git a/image.png b/image.png
new file mode 100644
index 0000000..abcdef
Binary files /dev/null and b/image.png differ
`
	files := parseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Status != "binary" {
		t.Errorf("status = %q, want %q", files[0].Status, "binary")
	}
}

func TestParseDiff_Empty(t *testing.T) {
	files := parseDiff("")
	if len(files) != 0 {
		t.Errorf("expected 0 files for empty input, got %d", len(files))
	}
}

func TestParseCommits(t *testing.T) {
	output := "abc123|John Doe|feat: add login\ndef456|Jane Doe|fix: crash"
	commits := parseCommits(output)
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].Hash != "abc123" {
		t.Errorf("hash = %q", commits[0].Hash)
	}
	if commits[0].Author != "John Doe" {
		t.Errorf("author = %q", commits[0].Author)
	}
	if commits[0].Subject != "feat: add login" {
		t.Errorf("subject = %q", commits[0].Subject)
	}
}

func TestParseCommits_Empty(t *testing.T) {
	commits := parseCommits("")
	if len(commits) != 0 {
		t.Errorf("expected 0 commits, got %d", len(commits))
	}
}

func TestExtractIssueRefs(t *testing.T) {
	branch := "feat/PROJ-123-add-login"
	subjects := []string{"fixes #456", "related to ABC-789"}
	refs := extractIssueRefs(branch, subjects)
	expected := []string{"PROJ-123", "#456", "ABC-789"}
	if len(refs) != len(expected) {
		t.Fatalf("expected %d refs, got %d: %v", len(expected), len(refs), refs)
	}
	for i, ref := range expected {
		if refs[i] != ref {
			t.Errorf("refs[%d] = %q, want %q", i, refs[i], ref)
		}
	}
}

func TestExtractIssueRefs_Deduplicates(t *testing.T) {
	subjects := []string{"fixes #123", "see also #123"}
	refs := extractIssueRefs("", subjects)
	if len(refs) != 1 {
		t.Errorf("expected 1 unique ref, got %d: %v", len(refs), refs)
	}
}

func TestParseHunkHeader(t *testing.T) {
	header := parseHunkHeader("@@ -10,6 +12,8 @@ func foo()")
	if header == nil {
		t.Fatal("expected non-nil hunk")
	}
	if header.OldStart != 10 || header.OldLines != 6 {
		t.Errorf("old: start=%d, lines=%d", header.OldStart, header.OldLines)
	}
	if header.NewStart != 12 || header.NewLines != 8 {
		t.Errorf("new: start=%d, lines=%d", header.NewStart, header.NewLines)
	}
}

func TestParseHunkHeader_DefaultCount(t *testing.T) {
	header := parseHunkHeader("@@ -1 +1 @@")
	if header == nil {
		t.Fatal("expected non-nil hunk")
	}
	if header.OldLines != 1 || header.NewLines != 1 {
		t.Errorf("expected default 1 lines, got old=%d new=%d", header.OldLines, header.NewLines)
	}
}
