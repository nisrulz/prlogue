package cmd

import (
	"fmt"
	"os"

	"github.com/nisrulz/prlogue/internal/config"
	"github.com/spf13/cobra"
)

var resetConfigCmd = &cobra.Command{
	Use:   "reset-config",
	Short: "Reset config to defaults",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		target, backup, err := config.Reset(cfgFile)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Created default config: %s\n", displayPath(target))
		if backup != "" {
			fmt.Fprintf(os.Stdout, "Backed up previous config: %s\n", displayPath(backup))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resetConfigCmd)
}
