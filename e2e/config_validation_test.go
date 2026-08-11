//go:build e2e

package e2e

import (
	"fmt"
	"testing"
)

// TestConfigValidation covers the rejection paths for invalid user config:
// bad base branches, context lengths, base URLs, output formats, providers,
// and the remote API key requirement.
func TestConfigValidation(t *testing.T) {
	t.Run("nonexistent base branch errors cleanly", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, ".prlogue.yaml", "git:\n  default_branch: nonexistent-branch\n")
		out, _ := runCLI(t, dir)
		assertContains(t, out, "collect diff")
	})

	t.Run("revision expressions in base branch are rejected", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, ".prlogue.yaml", "git:\n  default_branch: HEAD~10\n")
		out, _ := runCLI(t, dir)
		assertContains(t, out, "invalid branch")
	})

	t.Run("context below the floor is rejected", func(t *testing.T) {
		dir := newRepo(t)
		cfg := writeConfig(t, smallContextConfig())
		out, _ := runCLI(t, dir, "--config", cfg)
		assertContains(t, out, "context lengths must be at least 4096")
	})

	t.Run("non-HTTP base URL is rejected", func(t *testing.T) {
		dir := newRepo(t)
		cfg := writeConfig(t, nonHTTPConfig)
		out, _ := runCLI(t, dir, "--config", cfg)
		assertContains(t, out, "http or https")
	})

	t.Run("unsupported output format is rejected", func(t *testing.T) {
		dir := newRepo(t)
		cfg := writeConfig(t, xmlFormatConfig())
		out, _ := runCLI(t, dir, "--config", cfg)
		assertContains(t, out, "output.format must be 'markdown' or 'json'")
	})

	t.Run("unknown provider is rejected", func(t *testing.T) {
		dir := newRepo(t)
		cfg := writeConfig(t, "provider: bogus-provider\n")
		out, _ := runCLI(t, dir, "--config", cfg)
		assertContains(t, out, "unsupported provider")
	})

	t.Run("remote openai_compat requires an API key", func(t *testing.T) {
		t.Setenv("PRLOGUE_OPENAI_COMPAT_API_KEY", "")
		dir := newRepo(t)
		cfg := writeConfig(t, remoteOpenAIConfig)
		out, _ := runCLI(t, dir, "--config", cfg)
		assertContains(t, out, "PRLOGUE_OPENAI_COMPAT_API_KEY is required")
	})

	t.Run("credentials in base URL are rejected", func(t *testing.T) {
		dir := newRepo(t)
		cfg := writeConfig(t, credsConfig)
		out, _ := runCLI(t, dir, "--config", cfg)
		assertContains(t, out, "must not contain credentials")
	})

	t.Run("plain HTTP for a non-loopback host is rejected", func(t *testing.T) {
		dir := newRepo(t)
		cfg := writeConfig(t, httpRemoteConfig)
		out, _ := runCLI(t, dir, "--config", cfg)
		assertContains(t, out, "https for non-loopback")
	})
}

const (
	nonHTTPConfig = `provider: openai_compat
model: lfm2.5:8b
base_url: ftp://example.com/v1
`
	credsConfig = `provider: openai_compat
model: lfm2.5:8b
base_url: http://user:pass@localhost:1234/v1
`
	httpRemoteConfig = `provider: openai_compat
model: lfm2.5:8b
base_url: http://example.com/v1
`
	remoteOpenAIConfig = `provider: openai_compat
model: gpt-5.6-luna
base_url: https://api.openai.com/v1
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
  format: markdown
`
)

// smallContextConfig is a complete config with a context floor violation.
func smallContextConfig() string {
	return fmt.Sprintf(`provider: openai_compat
model: %s
base_url: %s
context:
  mode: auto
  manual: 100
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: markdown
`, model, baseURL)
}

// xmlFormatConfig is a complete config with an unsupported output format.
func xmlFormatConfig() string {
	return fmt.Sprintf(`provider: openai_compat
model: %s
base_url: %s
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
  format: xml
`, model, baseURL)
}
