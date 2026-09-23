// envorca is the thin Envorca CLI. It dials the daemon over local IPC and
// renders responses; it must never reimplement daemon business logic.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// cliVersion is the CLI version. Defaults to a dev build; release builds
// override it via -ldflags, e.g.
//
//	go build -ldflags "-X envorca.dev/envorca/cli/cmd/envorca.cliVersion=0.2.0" ./cmd/envorca
var cliVersion = "0.1.0-dev"

// cliCommit is the git commit the CLI was built from. Make and release builds
// override it via -ldflags; every other build falls back to the VCS revision
// the Go toolchain embeds automatically when compiling from a git checkout.
var cliCommit string

// fullCliVersion returns cliVersion annotated with the build commit, e.g.
// "0.1.0-dev (7307870)". Compare it against `git rev-parse --short HEAD` to
// check whether the running binary matches the checkout.
func fullCliVersion() string {
	c := cliCommit
	dirty := false
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if c == "" {
					c = s.Value
				}
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
	}
	if len(c) > 7 {
		c = c[:7]
	}
	if c == "" {
		return cliVersion
	}
	if dirty {
		c += "-dirty"
	}
	return cliVersion + " (" + c + ")"
}

var rootCmd = &cobra.Command{
	Use:           "envorca",
	Short:         "Envorca makes Linux development on Windows just work",
	Version:       fullCliVersion(),
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
		fmt.Fprintln(cmd.OutOrStdout(), fullCliVersion())
	},
}
