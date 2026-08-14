package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/nisrulz/prlogue/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config [get <key>]",
	Short: "View configuration",
	Long: `View current configuration or read one value.

Settings live in the config file and are edited there directly.

Examples:
  prlogue config
  prlogue config get provider`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return showConfig()
		}
		if args[0] == "get" && len(args) == 2 {
			return getConfig(args[1])
		}
		return fmt.Errorf("usage: prlogue config [get <key>]")
	},
}

func showConfig() error {
	cfg, err := config.LoadUser(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Fprintf(os.Stdout, "name:                       %s\n", cfg.Name)
	fmt.Fprintf(os.Stdout, "provider:                    %s\n", cfg.Provider)
	fmt.Fprintf(os.Stdout, "model:                       %s\n", cfg.Model)
	fmt.Fprintf(os.Stdout, "base_url:                    %s\n", cfg.BaseURL)
	fmt.Fprintf(os.Stdout, "response_max_tokens:         %d\n", cfg.ResponseMaxTokens)
	fmt.Fprintf(os.Stdout, "api_key:                     %s\n", secretStatus(cfg.APIKey))
	fmt.Fprintf(os.Stdout, "no_think:                    %v\n", cfg.NoThink)
	fmt.Fprintf(os.Stdout, "output_style_prompt:         %s\n", promptStatus(cfg.OutputStylePrompt))
	fmt.Fprintf(os.Stdout, "context.mode:                %s\n", cfg.Context.Mode)
	fmt.Fprintf(os.Stdout, "context.manual:              %d\n", cfg.Context.Manual)
	fmt.Fprintf(os.Stdout, "context.max_auto:            %d\n", cfg.Context.MaxAuto)
	fmt.Fprintf(os.Stdout, "context.min_auto:            %d\n", cfg.Context.MinAuto)
	if cfg.Context.Mode == "auto" {
		fmt.Fprintf(os.Stdout, "context.calculated:           %d\n", cfg.ContextLength())
	}
	fmt.Fprintf(os.Stdout, "chunking.strategy:           %s\n", cfg.Chunking.Strategy)
	fmt.Fprintf(os.Stdout, "chunking.file_summary_threshold: %d\n", cfg.Chunking.FileSummaryThreshold)
	fmt.Fprintf(os.Stdout, "chunking.hunk_split_threshold:   %d\n", cfg.Chunking.HunkSplitThreshold)
	fmt.Fprintf(os.Stdout, "git.default_branch:          %s\n", cfg.Git.DefaultBranch)
	fmt.Fprintf(os.Stdout, "output.format:               %s\n", cfg.Output.Format)
	fmt.Fprintf(os.Stdout, "system.os_reservation_gb:    %.1f\n", cfg.System.OSReservationGB)
	fmt.Fprintf(os.Stdout, "system.model_size_gb:        %.2f\n", cfg.System.ModelSizeGB)

	return nil
}

func getConfig(key string) error {
	cfg, err := config.LoadUser(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	value, ok := configValue(cfg, key)
	if !ok {
		return fmt.Errorf("unknown key: %s", key)
	}
	fmt.Fprintln(os.Stdout, value)
	return nil
}

func configValue(cfg *config.Config, key string) (string, bool) {
	switch key {
	case "name":
		return cfg.Name, true
	case "provider":
		return cfg.Provider, true
	case "model":
		return cfg.Model, true
	case "base_url":
		return cfg.BaseURL, true
	case "response_max_tokens":
		return strconv.Itoa(cfg.ResponseMaxTokens), true
	case "api_key":
		return secretStatus(cfg.APIKey), true
	case "no_think":
		return strconv.FormatBool(cfg.NoThink), true
	case "output_style_prompt":
		return cfg.OutputStylePrompt, true
	case "context.mode":
		return cfg.Context.Mode, true
	case "context.manual":
		return strconv.Itoa(cfg.Context.Manual), true
	case "context.max_auto":
		return strconv.Itoa(cfg.Context.MaxAuto), true
	case "context.min_auto":
		return strconv.Itoa(cfg.Context.MinAuto), true
	case "chunking.strategy":
		return cfg.Chunking.Strategy, true
	case "chunking.file_summary_threshold":
		return strconv.Itoa(cfg.Chunking.FileSummaryThreshold), true
	case "chunking.hunk_split_threshold":
		return strconv.Itoa(cfg.Chunking.HunkSplitThreshold), true
	case "git.default_branch":
		return cfg.Git.DefaultBranch, true
	case "output.format":
		return cfg.Output.Format, true
	case "system.os_reservation_gb":
		return strconv.FormatFloat(cfg.System.OSReservationGB, 'f', -1, 64), true
	case "system.model_size_gb":
		return strconv.FormatFloat(cfg.System.ModelSizeGB, 'f', -1, 64), true
	default:
		return "", false
	}
}

func secretStatus(value string) string {
	if value == "" {
		return "not set"
	}
	return "*** (PRLOGUE_OPENAI_COMPAT_API_KEY)"
}

func promptStatus(value string) string {
	if value == "" {
		return "not set (bundled prompt)"
	}
	return "set (custom)"
}

func init() {
	rootCmd.AddCommand(configCmd)
}
