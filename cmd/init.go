package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/nisrulz/prlogue/internal/config"
	"github.com/nisrulz/prlogue/internal/sysinfo"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create user config and print setup instructions",
	Long: `Creates the trusted user config and prints provider setup steps.
Does NOT modify your system or start servers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func runInit() error {
	ram, err := sysinfo.DetectRAM(0)
	if err != nil {
		return fmt.Errorf("detect RAM: %w", err)
	}

	cfg, err := config.LoadUser(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	autoCtx := cfg.ContextLengthWithRAM(ram)

	target, err := config.Save(cfg, cfgFile)
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if _, err := config.EnsureOutputStylePromptFile(); err != nil {
		return fmt.Errorf("create output style prompt: %w", err)
	}
	stylePath, err := config.OutputStylePromptPath()
	if err != nil {
		return fmt.Errorf("resolve output style prompt: %w", err)
	}
	fmt.Printf("✔ Created %s\n", displayPath(target))
	fmt.Printf("  Edit output style: %s\n", displayPath(stylePath))
	fmt.Println()
	printSetup(ram, cfg, autoCtx, displayPath(target))
	return nil
}

// displayPath renders an absolute path for CLI output, replacing the user's
// home directory with "~" so real usernames never leak into the output.
func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}

func printSetup(ram *sysinfo.RAMInfo, cfg *config.Config, autoCtx int, cfgPath string) {
	fmt.Println("───────────────────────────────────────────────────")
	fmt.Println("  PRlogue Setup")
	fmt.Println("───────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("  System RAM:   %.0f GB\n", ram.TotalRAMGB)
	fmt.Printf("  OS Reserve:   %.0f GB\n", ram.OSReserveGB)
	fmt.Printf("  Available:    %.0f GB\n", ram.AvailableRAMGB)
	fmt.Printf("  Auto Context: %d tokens\n", autoCtx)
	fmt.Printf("  Provider:     %s\n", cfg.Provider)
	fmt.Println()

	printOpenAICompatSetup()

	fmt.Println("───────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("Config (%s):\n", cfgPath)
	fmt.Printf("  name:          %s\n", cfg.Name)
	fmt.Printf("  provider:      %s\n", cfg.Provider)
	fmt.Printf("  model:         %s\n", cfg.Model)
	fmt.Printf("  context.mode:  %s\n", cfg.Context.Mode)
	fmt.Printf("  context.auto:  %d tokens\n", autoCtx)
	fmt.Printf("  chunking:      %s (file≤%d → hunk≤%d)\n",
		cfg.Chunking.Strategy, cfg.Chunking.FileSummaryThreshold, cfg.Chunking.HunkSplitThreshold)

	if runtime.GOOS == "darwin" {
		fmt.Println()
		fmt.Println("Tip: On Apple Silicon, use MLX format models for best performance.")
		fmt.Printf("     Current: %s\n", cfg.Model)
	}
}

func printOpenAICompatSetup() {
	fmt.Println("  Step 1: Point the config at your OpenAI-compatible server:")
	fmt.Println("          set name, base_url, and model in $PRLOGUE_CONFIG_DIR/prlogue/config.yaml")
	fmt.Println("          (see docs/reference.md for setup examples)")
	fmt.Println()
	fmt.Println("  Step 2: For remote servers that need auth:")
	fmt.Println("          export PRLOGUE_OPENAI_COMPAT_API_KEY=<token>")
	fmt.Println()
	fmt.Println("  Step 3: Generate a PR description:")
	fmt.Println("          prlogue generate")
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(initCmd)
}
