//go:build e2e

package e2e

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	ollamaURL   = "http://localhost:11434/v1"
	ollamaModel = "lfm2.5:8b"
)

var (
	binaryPath string
	baseURL    string
	model      string
	mockMode   bool
)

func TestMain(m *testing.M) {
	if err := buildBinary(); err != nil {
		log.Fatalf("build prlogue: %v", err)
	}

	// Isolate from the user's real config so the suite never reads or writes it.
	if err := isolateConfigDir(); err != nil {
		log.Fatalf("isolate config dir: %v", err)
	}

	if os.Getenv("PRLOGUE_LIVE_TEST_PROVIDER") == "mock" {
		srv := startMockServer()
		defer srv.Close()
		baseURL = srv.URL + "/v1"
		model = "ci-mock"
		mockMode = true
		if err := writeIsolatedConfig(); err != nil {
			log.Fatalf("write mock config: %v", err)
		}
	} else {
		if err := probeOllama(); err != nil {
			log.Fatalf("Ollama is not reachable: %v\nStart Ollama and pull %s before running make test-live", err, ollamaModel)
		}
		baseURL = ollamaURL
		model = ollamaModel
		mockMode = false
	}

	os.Exit(m.Run())
}

func buildBinary() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "prlogue-e2e-bin-*")
	if err != nil {
		return err
	}
	binaryPath = filepath.Join(dir, "prlogue")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(wd, ".."))
}

func isolateConfigDir() error {
	dir, err := os.MkdirTemp("", "prlogue-e2e-config-*")
	if err != nil {
		return err
	}
	return os.Setenv("PRLOGUE_CONFIG_DIR", dir)
}

func writeIsolatedConfig() error {
	base := os.Getenv("PRLOGUE_CONFIG_DIR")
	dir := filepath.Join(base, "prlogue")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(config("")), 0o600)
}

func probeOllama() error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(ollamaURL + "/models")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
