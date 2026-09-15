package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/cli/internal/client"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon and environment status",
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
		st, err := client.GetStatus(ctx, conn, 5*time.Second)
		if err != nil {
			return fmt.Errorf("get status: %w", err)
		}
		renderStatus(cmd.OutOrStdout(), st)
		return nil
	},
}

func renderStatus(w io.Writer, st *envorcav1.EnvironmentStatus) {
	f := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	f("ENVORCA DAEMON")
	f("  State:    %s", st.DaemonState)
	f("  Version:  %s", st.Version)
	f("  Socket:   %s", st.Socket)
	f("  Uptime:   %s", formatUptime(st.UptimeSeconds))
	f("  Overall:  %s", decorateStatus(st.OverallStatus, plainStatus(st.OverallStatus)))
	f("")
	f("  COMPONENTS")
	for _, c := range st.Components {
		f("  %-22s%s", c.Id, decorateStatus(c.Status, plainStatus(c.Status)))
	}
}

func plainStatus(s envorcav1.Status) string {
	switch s {
	case envorcav1.Status_HEALTHY:
		return "HEALTHY"
	case envorcav1.Status_WARNING:
		return "WARNING"
	case envorcav1.Status_CRITICAL:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

func decorateStatus(s envorcav1.Status, text string) string {
	if !isTerminal() {
		return text
	}
	var color string
	switch s {
	case envorcav1.Status_HEALTHY:
		color = "\x1b[32m"
	case envorcav1.Status_WARNING:
		color = "\x1b[33m"
	case envorcav1.Status_CRITICAL:
		color = "\x1b[31m"
	default:
		color = "\x1b[36m"
	}
	return color + text + "\x1b[0m"
}

func formatUptime(secs int64) string {
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm %ds", secs/60, secs%60)
	}
	return fmt.Sprintf("%dh %dm", secs/3600, (secs%3600)/60)
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
