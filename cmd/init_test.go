package cmd

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/nisrulz/prlogue/internal/config"
	"github.com/nisrulz/prlogue/internal/sysinfo"
)

func TestPrintSetup_UsesOpenAICompatInstructions(t *testing.T) {
	cfg := &config.Config{Provider: "openai_compat", Model: "model"}
	output := captureStdout(t, func() {
		printSetup(&sysinfo.RAMInfo{}, cfg, 4096, "~/.config/prlogue/config.yaml")
	})
	if !strings.Contains(output, "PRLOGUE_OPENAI_COMPAT_API_KEY") {
		t.Fatalf("setup output does not contain OpenAI-compatible instructions:\n%s", output)
	}
	if !strings.Contains(output, "Config (~/.config/prlogue/config.yaml):") {
		t.Fatalf("setup output does not show the config path:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = original }()

	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output)
}
