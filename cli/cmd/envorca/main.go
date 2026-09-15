// envorca is the thin Envorca CLI. It dials the daemon over local IPC and
// renders responses; it must never reimplement daemon business logic.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// cliVersion is the CLI version. Defaults to a dev build; release builds
// override it via -ldflags, e.g.
//
//	go build -ldflags "-X envorca.dev/envorca/cli/cmd/envorca.cliVersion=0.2.0" ./cmd/envorca
var cliVersion = "0.1.0-dev"

var rootCmd = &cobra.Command{
	Use:           "envorca",
	Short:         "Envorca makes Linux development on Windows just work",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "path to daemon config.yaml (default: ENVORCA_CONFIG)")
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "envorca:", err)
		os.Exit(1)
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), cliVersion)
	},
}
