package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nisrulz/prlogue/internal/config"
)

func TestRunDoctor_AllChecksPass(t *testing.T) {
	srv := newDoctorServer(t, "test-model")
	defer srv.Close()

	cfgFile = writeDoctorConfig(t, srv.URL+"/v1", "test-model")
	defer func() { cfgFile = "" }()

	output := captureStdout(t, func() {
		if err := runDoctor(); err != nil {
			t.Fatalf("runDoctor: %v", err)
		}
	})
	for _, want := range []string{"All checks passed.", "provider", "llm connection", "operational"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestRunDoctor_ProviderNameShown(t *testing.T) {
	srv := newDoctorServer(t, "test-model")
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Name = "TestServer"
	cfg.Model = "test-model"
	cfg.BaseURL = srv.URL + "/v1"
	p := filepath.Join(t.TempDir(), "config.yaml")
	if _, err := config.Save(cfg, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cfgFile = p
	defer func() { cfgFile = "" }()

	output := captureStdout(t, func() {
		if err := runDoctor(); err != nil {
			t.Fatalf("runDoctor: %v", err)
		}
	})
	if !strings.Contains(output, "TestServer") {
		t.Fatalf("output does not show the configured provider name:\n%s", output)
	}
}

func TestRunDoctor_ModelNotLoadedFails(t *testing.T) {
	srv := newDoctorServer(t, "other-model")
	defer srv.Close()

	cfgFile = writeDoctorConfig(t, srv.URL+"/v1", "test-model")
	defer func() { cfgFile = "" }()

	output := captureStdout(t, func() {
		if err := runDoctor(); err == nil {
			t.Fatal("expected runDoctor to fail")
		}
	})
	if !strings.Contains(output, "not found in list") {
		t.Fatalf("expected model-not-found diagnosis:\n%s", output)
	}
}

func TestRunDoctor_EndpointDownFails(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	deadURL := srv.URL + "/v1"
	srv.Close()

	cfgFile = writeDoctorConfig(t, deadURL, "test-model")
	defer func() { cfgFile = "" }()

	output := captureStdout(t, func() {
		if err := runDoctor(); err == nil {
			t.Fatal("expected runDoctor to fail")
		}
	})
	if !strings.Contains(output, "not reachable") {
		t.Fatalf("expected unreachable-endpoint diagnosis:\n%s", output)
	}
}

func TestRunDoctor_ProjectConfigRejected(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".prlogue.yaml", []byte("chunking:\n  strategy: two-tier\n"), 0600); err != nil {
		t.Fatal(err)
	}

	srv := newDoctorServer(t, "test-model")
	defer srv.Close()

	cfgFile = writeDoctorConfig(t, srv.URL+"/v1", "test-model")
	defer func() { cfgFile = "" }()

	output := captureStdout(t, func() {
		if err := runDoctor(); err == nil {
			t.Fatal("expected runDoctor to fail")
		}
	})
	if !strings.Contains(output, `project config key "chunking.strategy" is not allowed`) {
		t.Fatalf("expected project config diagnosis:\n%s", output)
	}
	if !strings.Contains(output, "llm") {
		t.Fatalf("expected checks to continue past the project config problem:\n%s", output)
	}
}

// newDoctorServer serves /v1/models and /v1/chat/completions for doctor tests.
func newDoctorServer(t *testing.T, modelID string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			w.Write([]byte(`{"data":[{"id":"` + modelID + `"}]}`))
		case "/v1/chat/completions":
			w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"model":"` + modelID + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func writeDoctorConfig(t *testing.T, baseURL, model string) string {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Model = model
	p := filepath.Join(t.TempDir(), "config.yaml")
	if _, err := config.Save(cfg, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return p
}
