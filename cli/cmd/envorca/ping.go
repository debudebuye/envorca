package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/cli/internal/client"
)

func init() {
	rootCmd.AddCommand(pingCmd)
}

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Check daemon liveness and version",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		endpoint := commandEndpoint()
		if !client.Probe(ctx, endpoint) {
			return errors.New("Envorca daemon is not running.\nRun 'envorca start' to launch it.")
		}
		conn, err := client.Dial(ctx, endpoint)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer conn.Close()
		resp, err := client.Ping(ctx, conn, 5*time.Second)
		if err != nil {
			return fmt.Errorf("ping: %w", err)
		}
		renderPing(cmd.OutOrStdout(), resp)
		return nil
	},
}

func renderPing(w io.Writer, resp *envorcav1.PingResponse) {
	f := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	f("Pong from Envorca daemon.")
	f("  Version:  %s", resp.Version)
	f("  State:    %s", resp.DaemonState)
	f("  Uptime:   %s", formatUptime(resp.UptimeSeconds))
	f("  Socket:   %s", resp.Socket)
}