package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/nisrulz/prlogue/internal/config"
	"github.com/nisrulz/prlogue/internal/git"
	"github.com/nisrulz/prlogue/internal/provider"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that the CLI and provider are ready to run",
	Long: `Runs pre-flight checks before you generate: config in place,
endpoint reachability, model availability, a live test request through
the provider, and API key requirements.

Exit code is non-zero when a check fails.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoctor()
	},
}

type doctorReport struct {
	failures int
	problems []doctorProblem
}

type doctorProblem struct {
	label string
	msg   string
}

func runDoctor() error {
	fmt.Println()
	report := &doctorReport{}

	cfg, err := config.LoadUser(cfgFile)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	cfgPath := cfgFile
	if cfgPath == "" {
		if p, err := config.UserConfigPath(); err == nil {
			cfgPath = p
		}
	}
	report.pass("config", displayPath(cfgPath))

	name := cfg.Name
	if name == "" {
		name = cfg.Provider
	}
	report.pass("provider", fmt.Sprintf("%s · %s", name, cfg.Provider))

	u, err := url.Parse(cfg.BaseURL)
	if err != nil {
		report.fail("endpoint", fmt.Sprintf("invalid base URL: %v", err))
		return report.finish()
	}
	loopback := config.IsLoopbackHost(u.Hostname())
	if loopback {
		report.pass("api key", "not required")
	} else if cfg.APIKey == "" {
		report.fail("api key", "PRLOGUE_OPENAI_COMPAT_API_KEY missing")
	} else {
		report.pass("api key", "set")
	}

	if report.checkEndpoint(cfg) {
		report.checkLLM(cfg)
	}

	if err := config.ProjectConfigErr(); err != nil {
		report.fail("repo config", err.Error())
	}

	if git.InsideWorkTree() {
		report.pass("git", "ready")
	} else {
		report.warn("git", "not a work tree (generate needs one)")
	}

	return report.finish()
}

// checkEndpoint verifies the models endpoint responds. It returns true when
// the endpoint is reachable, so the caller can run a live test request.
func (r *doctorReport) checkEndpoint(cfg *config.Config) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	p := provider.NewOpenAICompatible(cfg.BaseURL, cfg.APIKey, cfg.Model)
	models, err := p.ListModels(ctx)
	switch {
	case err == nil:
		r.pass("endpoint", cfg.BaseURL)
		switch {
		case len(models) == 0:
			r.warn("model", fmt.Sprintf("%s unverified (endpoint lists no models)", cfg.Model))
		default:
			found := false
			for _, id := range models {
				if id == cfg.Model {
					found = true
					break
				}
			}
			if found {
				r.pass("model", cfg.Model+" loaded")
			} else {
				r.fail("model", fmt.Sprintf("%s not found in list (listed: %s)", cfg.Model, strings.Join(models, ", ")))
			}
		}
		return true
	case isModelsUnsupported(err):
		r.warn("endpoint", cfg.BaseURL+" (no /v1/models)")
		r.warn("model", fmt.Sprintf("%s unverified (endpoint does not list models)", cfg.Model))
		return true
	default:
		r.fail("endpoint", fmt.Sprintf("%s not reachable: %v", cfg.BaseURL, err))
		return false
	}
}

// checkLLM sends a minimal chat completion through the provider to verify the
// model actually responds. Thinking models can spend a lot of tokens on a
// <think> block, so the request gets the full configured context as its
// max_tokens budget and the response is checked before think-block stripping.
func (r *doctorReport) checkLLM(cfg *config.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p := provider.NewOpenAICompatible(cfg.BaseURL, cfg.APIKey, cfg.Model)
	resp, err := p.Chat(ctx, provider.ChatRequest{
		Messages:  []provider.ChatMessage{{Role: "user", Content: "Hi"}},
		MaxTokens: cfg.ContextLength(),
		NoThink:   false,
	})
	if err != nil {
		r.fail("llm", fmt.Sprintf("test request failed: %v", err))
		return
	}
	if strings.TrimSpace(resp.Content) == "" {
		r.fail("llm connection", "test request returned an empty response")
		return
	}
	r.pass("llm connection", "operational")
}

func isModelsUnsupported(err error) bool {
	var apiErr *openai.APIError
	return errors.As(err, &apiErr) && apiErr.HTTPStatusCode == http.StatusNotFound
}

func (r *doctorReport) pass(label, msg string) {
	fmt.Printf("  ✔ %-17s %s\n", label, msg)
}

func (r *doctorReport) warn(label, msg string) {
	fmt.Printf("  ⚠ %-17s %s\n", label, msg)
}

func (r *doctorReport) fail(label, msg string) {
	r.failures++
	r.problems = append(r.problems, doctorProblem{label: label, msg: msg})
	fmt.Printf("  ✗ %-17s %s\n", label, msg)
}

func (r *doctorReport) finish() error {
	fmt.Println()
	if r.failures == 0 {
		fmt.Println("All checks passed.")
		return nil
	}
	fmt.Println("Errors found:")
	for i, p := range r.problems {
		fmt.Printf("  %d. %s: %s\n", i+1, p.label, p.msg)
	}
	fmt.Println()
	return fmt.Errorf("doctor found %d problem(s)", r.failures)
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
