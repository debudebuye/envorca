package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"envorca.dev/envorca/api/ipc"
	"envorca.dev/envorca/cli/internal/client"
	"envorca.dev/envorca/cli/internal/runner"
)

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Envorca daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		out := cmd.OutOrStdout()
		endpoint := ipc.DefaultEndpoint(ipc.DefaultStateDir())
		if client.Probe(ctx, endpoint) {
			fmt.Fprintln(out, "Envorca daemon is already running.")
			return nil
		}
		bin, err := runner.FindDaemonBinary()
		if err != nil {
			return err
		}
		logFile, err := runner.OpenSpawnLog()
		if err != nil {
			return fmt.Errorf("open spawn log: %w", err)
		}
		defer logFile.Close()
		if err := runner.Spawn(bin, logFile); err != nil {
			return fmt.Errorf("start daemon: %w", err)
		}

		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if client.Probe(ctx, endpoint) {
				fmt.Fprintf(out, "Envorca daemon started.\nSocket: %s\n", endpoint)
				return nil
			}
			time.Sleep(150 * time.Millisecond)
		}
		return fmt.Errorf("daemon did not become ready within 15s; see %s for startup logs", logFile.Name())
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Gracefully stop the Envorca daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		out := cmd.OutOrStdout()
		endpoint := ipc.DefaultEndpoint(ipc.DefaultStateDir())
		if !client.Probe(ctx, endpoint) {
			fmt.Fprintln(out, "Envorca daemon is not running.")
			return nil
		}
		conn, err := client.Dial(ctx, endpoint)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer conn.Close()
		if err := client.Shutdown(ctx, conn, 5*time.Second); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	},
}
