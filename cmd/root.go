package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
	quiet   bool
)

var rootCmd = &cobra.Command{
	Use:           "prlogue",
	Short:         "A prologue for your pull request — generated from git context",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `PRlogue analyzes git diff, commit messages, and related context
to generate pull request descriptions using an OpenAI-compatible LLM.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if !quiet {
			printBanner()
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "trusted config file (default $PRLOGUE_CONFIG_DIR/prlogue/config.yaml, else ~/.config/prlogue/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress the startup banner")
}
