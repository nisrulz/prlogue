package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextLength_Manual(t *testing.T) {
	cfg := validConfig()
	cfg.Context.Mode = "manual"
	cfg.Context.Manual = 65536
	if n := cfg.ContextLength(); n != 65536 {
		t.Errorf("ContextLength() = %d, want 65536", n)
	}
}

func TestContextLength_Auto(t *testing.T) {
	cfg := validConfig()
	cfg.Context.calculated = 131072
	if n := cfg.ContextLength(); n != 131072 {
		t.Errorf("ContextLength() = %d, want 131072", n)
	}
}

func TestCalcAutoContext_WithinBounds(t *testing.T) {
	cfg := validConfig()
	cfg.Context.Manual = 999999
	cfg.Context.MaxAuto = 131072
	n := calcAutoContext(cfg)
	if n < cfg.Context.MinAuto || n > cfg.Context.MaxAuto {
		t.Errorf("calcAutoContext = %d, want between %d and %d", n, cfg.Context.MinAuto, cfg.Context.MaxAuto)
	}
}

func TestDefaultConfig_IsValidOpenAICompat(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Name != "Ollama" {
		t.Errorf("name = %q, want Ollama", cfg.Name)
	}
	if cfg.Provider != "openai_compat" {
		t.Errorf("provider = %q, want openai_compat", cfg.Provider)
	}
	if cfg.Model != "lfm2.5:8b" {
		t.Errorf("model = %q, want lfm2.5:8b", cfg.Model)
	}
	if cfg.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("base_url = %q", cfg.BaseURL)
	}
	if cfg.ResponseMaxTokens != DefaultResponseMaxTokens {
		t.Errorf("response_max_tokens = %d, want %d", cfg.ResponseMaxTokens, DefaultResponseMaxTokens)
	}
	if cfg.OutputStylePrompt != DefaultOutputStylePrompt() {
		t.Error("default prompt is not configured")
	}
	if !strings.Contains(cfg.OutputStylePrompt, "Omit `### Key Changes`") {
		t.Error("default prompt must omit Key Changes when the diff has no meaningful changes")
	}
	if !strings.Contains(DefaultPrompt(), "assume a default branch name") {
		t.Error("default task prompt must explain default branch handling")
	}
	if strings.Contains(DefaultPrompt(), "### PR Description") {
		t.Error("default task prompt must not define the output style")
	}
	if strings.Contains(cfg.OutputStylePrompt, "IMMUTABLE") || strings.Contains(cfg.OutputStylePrompt, "security policy") {
		t.Error("default output style prompt must not contain immutable policy rules")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultConfig invalid: %v", err)
	}
}

func TestSave_DefaultConfigWritesFallbackPromptToYAML(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if _, err := Save(DefaultConfig(), p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "output_style_prompt:") {
		t.Fatal("default YAML does not contain output_style_prompt")
	}
	if !strings.Contains(text, "### PR Description") || !strings.Contains(text, "### Key Changes") {
		t.Fatal("default YAML does not contain the fallback format prompt")
	}
	if strings.Contains(text, "IMMUTABLE SECURITY POLICY") || strings.Contains(text, "IMMUTABLE OUTPUT SANITIZATION POLICY") {
		t.Fatal("default YAML must not contain immutable policy prompts")
	}
}

func TestLoad_ExplicitPath(t *testing.T) {
	p := writeConfigT(t, func(c *Config) { c.Model = "test-model" })
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Provider != "openai_compat" || cfg.Model != "test-model" {
		t.Errorf("provider=%q model=%q", cfg.Provider, cfg.Model)
	}
}

func TestLoad_ExplicitPathMissing(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected error for missing explicit config path")
	}
}

func TestLoad_ExplicitRejectsOversizedConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(strings.Repeat("#", maxConfigBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected config size error, got %v", err)
	}
}

func TestLoad_ExplicitBaseURLWins(t *testing.T) {
	p := writeConfigT(t, func(c *Config) { c.BaseURL = "https://example.com/v1" })
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != "https://example.com/v1" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
}

func TestLoad_FirstRunCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(configDirEnv, dir)
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Provider != "openai_compat" || cfg.Model != "lfm2.5:8b" {
		t.Errorf("first-run config = provider %q model %q", cfg.Provider, cfg.Model)
	}
	if _, err := os.Stat(filepath.Join(dir, "prlogue", "config.yaml")); err != nil {
		t.Errorf("config file was not created on first run: %v", err)
	}
}

func TestLoad_MissingResponseMaxTokensUsesDefault(t *testing.T) {
	p := writeConfigT(t, nil)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	updated := strings.Replace(string(data), fmt.Sprintf("response_max_tokens: %d\n", DefaultResponseMaxTokens), "", 1)
	if updated == string(data) {
		t.Fatal("test config did not contain response_max_tokens")
	}
	if err := os.WriteFile(p, []byte(updated), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ResponseMaxTokens != DefaultResponseMaxTokens {
		t.Errorf("response_max_tokens = %d, want %d", cfg.ResponseMaxTokens, DefaultResponseMaxTokens)
	}
}

func TestLoad_ProjectOverridesSafeFields(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	t.Setenv("HOME", t.TempDir())
	if err := os.WriteFile(projectConfigFile, []byte("git:\n  default_branch: develop\noutput:\n  format: json\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Provider != "openai_compat" || cfg.Git.DefaultBranch != "develop" || cfg.Output.Format != "json" {
		t.Errorf("project overrides not applied safely: %+v", cfg)
	}

	userCfg, err := LoadUser("")
	if err != nil {
		t.Fatalf("LoadUser: %v", err)
	}
	if userCfg.Git.DefaultBranch != "" || userCfg.Output.Format != "markdown" {
		t.Errorf("LoadUser applied project overrides: %+v", userCfg)
	}
}

func TestLoad_ProjectRejectsSensitiveFields(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if err := os.WriteFile(projectConfigFile, []byte("base_url: https://example.com/v1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected sensitive project key error, got %v", err)
	}
}

func TestLoad_ProjectRejectsOversizedConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if err := os.WriteFile(projectConfigFile, []byte(strings.Repeat("#", maxProjectBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected project config size error, got %v", err)
	}
}

func TestLoad_ConfigDirEnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(configDirEnv, dir)
	t.Setenv("HOME", t.TempDir())

	cfg := DefaultConfig()
	cfg.Model = "test-model"
	if _, err := Save(cfg, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Provider != "openai_compat" || loaded.Model != "test-model" {
		t.Errorf("provider=%q model=%q", loaded.Provider, loaded.Model)
	}
}

func TestLoad_APIKeyComesOnlyFromEnvironment(t *testing.T) {
	p := writeConfigT(t, func(c *Config) {
		c.Provider = "openai_compat"
		c.Model = "gpt-5.6-luna"
		c.BaseURL = "https://api.openai.com/v1"
	})
	t.Setenv("PRLOGUE_OPENAI_COMPAT_API_KEY", "env-secret")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey != "env-secret" {
		t.Fatalf("APIKey = %q, want environment value", cfg.APIKey)
	}

	t.Setenv("PRLOGUE_OPENAI_COMPAT_API_KEY", "")
	cfg, err = Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey != "" {
		t.Fatalf("APIKey loaded from file: %q", cfg.APIKey)
	}
}

func TestLoad_OutputStylePromptFromConfigFile(t *testing.T) {
	p := writeConfigT(t, func(c *Config) { c.OutputStylePrompt = "Use short headings." })
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OutputStylePrompt != "Use short headings." {
		t.Errorf("OutputStylePrompt = %q", cfg.OutputStylePrompt)
	}
}

func TestValidate_RejectsOversizedOutputStylePrompt(t *testing.T) {
	cfg := validConfig()
	cfg.OutputStylePrompt = strings.Repeat("p", maxOutputStyleLen+1)
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "output_style_prompt must not exceed") {
		t.Fatalf("expected oversized prompt error, got %v", err)
	}
}

func TestValidate_RejectsInvalidResponseMaxTokens(t *testing.T) {
	for _, value := range []int{0, 1, DefaultResponseMaxTokens - 1, maxResponseTokens + 1} {
		cfg := validConfig()
		cfg.ResponseMaxTokens = value
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "response_max_tokens") {
			t.Errorf("response_max_tokens=%d: expected validation error, got %v", value, err)
		}
	}
}

func TestSave_RoundTripPersistsOutputStylePrompt(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cfg.yaml")
	cfg := validConfig()
	cfg.OutputStylePrompt = "Use short headings."

	target, err := Save(cfg, p)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if target != p {
		t.Fatalf("target = %q, want %q", target, p)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.OutputStylePrompt != "Use short headings." {
		t.Errorf("roundtrip output style prompt = %q", got.OutputStylePrompt)
	}
}

func TestReset_BackupsAndWritesDefaultConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	previous := validConfig()
	previous.OutputStylePrompt = "Use short headings."
	if _, err := Save(previous, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	target, backup, err := Reset(p)
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if target != p || backup == "" || filepath.Ext(backup) != ".bak" {
		t.Fatalf("target=%q backup=%q", target, backup)
	}
	backedUp, err := Load(backup)
	if err != nil {
		t.Fatalf("Load backup: %v", err)
	}
	if backedUp.OutputStylePrompt != "Use short headings." {
		t.Errorf("backup output style prompt = %q", backedUp.OutputStylePrompt)
	}
	reset, err := Load(p)
	if err != nil {
		t.Fatalf("Load reset config: %v", err)
	}
	if reset.OutputStylePrompt != DefaultOutputStylePrompt() {
		t.Error("reset config does not contain the default prompt")
	}
}

func TestSave_RoundTripPersistsName(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cfg.yaml")
	cfg := validConfig()
	cfg.Name = "TestServer"

	target, err := Save(cfg, p)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if target != p {
		t.Fatalf("target = %q, want %q", target, p)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Name != "TestServer" {
		t.Errorf("roundtrip name = %q, want TestServer", got.Name)
	}
}

func TestValidate_RejectsUnsafeValues(t *testing.T) {
	tests := map[string]func(*Config){
		"unsupported provider":     func(c *Config) { c.Provider = "ollama" },
		"zero manual context":      func(c *Config) { c.Context.Manual = 0 },
		"reversed context bounds":  func(c *Config) { c.Context.MinAuto = c.Context.MaxAuto + 1 },
		"zero file threshold":      func(c *Config) { c.Chunking.FileSummaryThreshold = 0 },
		"zero hunk threshold":      func(c *Config) { c.Chunking.HunkSplitThreshold = 0 },
		"unknown chunk strategy":   func(c *Config) { c.Chunking.Strategy = "other" },
		"unknown output format":    func(c *Config) { c.Output.Format = "xml" },
		"insecure remote endpoint": func(c *Config) { c.BaseURL = "http://example.com/v1" },
		"protected extra field": func(c *Config) {
			c.ExtraBody = map[string]any{"messages": []any{}}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := validConfig()
			mutate(cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestSave_RoundTripDoesNotPersistAPIKey(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cfg.yaml")
	cfg := validConfig()
	cfg.Provider = "openai_compat"
	cfg.Model = "gpt-x"
	cfg.BaseURL = "https://api.openai.com/v1"
	cfg.APIKey = "secret-key"
	cfg.Context.Mode = "manual"
	cfg.Context.Manual = 4096

	target, err := Save(cfg, p)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if target != p {
		t.Fatalf("target = %q, want %q", target, p)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("perms = %v, want 0600", fi.Mode().Perm())
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-key") || strings.Contains(string(data), "api_key") {
		t.Fatalf("saved config contains API key: %s", data)
	}

	t.Setenv("PRLOGUE_OPENAI_COMPAT_API_KEY", "")
	got, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Provider != "openai_compat" || got.Model != "gpt-x" || got.APIKey != "" {
		t.Errorf("roundtrip mismatch: provider=%q model=%q apiKey=%q", got.Provider, got.Model, got.APIKey)
	}
}

func validConfig() *Config {
	return DefaultConfig()
}

func writeConfigT(t *testing.T, mutate func(*Config)) string {
	t.Helper()
	cfg := DefaultConfig()
	if mutate != nil {
		mutate(cfg)
	}
	p := filepath.Join(t.TempDir(), "config.yaml")
	if _, err := Save(cfg, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return p
}
