// envorka is the thin Envorka CLI. It dials the daemon over local IPC and
// renders responses; it must never reimplement daemon business logic.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const cliVersion = "0.1.0-dev"

var rootCmd = &cobra.Command{
	Use:           "envorka",
	Short:         "Envorka makes Linux development on Windows just work",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "envorka:", err)
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
