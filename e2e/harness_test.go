//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// configTpl is a complete valid config for the active provider. format and
// extra are injected so tests can change the output format or add keys.
const configTpl = `provider: openai_compat
model: %s
base_url: %s
no_think: true
context:
  mode: auto
  manual: 131072
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: %s
system:
  model_size_gb: 5.2
%s`

// config returns a complete valid config for the active provider with extra
// keys appended verbatim.
func config(extra string) string {
	return fmt.Sprintf(configTpl, model, baseURL, "markdown", extra)
}

// configJSON returns a complete valid config that emits JSON output.
func configJSON() string {
	return fmt.Sprintf(configTpl, model, baseURL, "json", "")
}

// runCLI executes "prlogue generate" with the given args inside dir and
// returns the combined output and the command error.
func runCLI(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	return runBinary(t, dir, append([]string{"generate", "--quiet"}, args...)...)
}

// runBinary executes the prlogue binary with the given args inside dir.
func runBinary(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// writeConfig writes a trusted config file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func writeOutputStylePrompt(t *testing.T, content string) {
	t.Helper()
	base := os.Getenv("PRLOGUE_CONFIG_DIR")
	dir := filepath.Join(base, "prlogue")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir output style prompt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "output_style_prompt.txt"), []byte(content), 0o600); err != nil {
		t.Fatalf("write output style prompt: %v", err)
	}
}

// writeFile writes content into name inside dir, creating parent directories.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// newRepo creates a throwaway git repository with an empty init commit and a
// feature branch checked out.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.email", "test@test")
	git(t, dir, "config", "user.name", "Test")
	git(t, dir, "commit", "--allow-empty", "-m", "init", "-q")
	git(t, dir, "checkout", "-q", "-b", "feature")
	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := gitOutput(dir, args...); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// commitAll stages every change in dir and commits it.
func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", message)
}

func assertContains(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Fatalf("output = %q, want substring %q", out, want)
	}
}

func assertAnyContains(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if strings.Contains(out, w) {
			return
		}
	}
	t.Fatalf("output = %q, want any of %q", out, wants)
}

// assertGeneratedBody verifies that a PR body was produced. In mock mode the
// output is deterministic, so we require the exact "PR Description" heading.
// Against a live model the wording varies and -v mode prepends a verbose
// preamble, so we require a title heading ("# ...") anywhere in the output.
func assertGeneratedBody(t *testing.T, out string) {
	t.Helper()
	if mockMode {
		assertContains(t, out, "PR Description")
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.HasPrefix(line, "# ") {
			return
		}
	}
	t.Fatalf("output = %q, want a generated PR body with a title heading", out)
}
