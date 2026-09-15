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
	historyCmd.Flags().Int32("limit", 20, "maximum records to show (1-500)")
	rootCmd.AddCommand(historyCmd)
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show diagnostic and repair history",
	Long: "Shows the most recent daemon diagnostics (status snapshots) and " +
		"repair outcomes recorded by the daemon.",
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

		limit, _ := cmd.Flags().GetInt32("limit")
		diagnostics, err := client.GetDiagnostics(ctx, conn, limit, 5*time.Second)
		if err != nil {
			return fmt.Errorf("get diagnostics: %w", err)
		}
		repairs, err := client.GetRepairHistory(ctx, conn, limit, 5*time.Second)
		if err != nil {
			return fmt.Errorf("get repair history: %w", err)
		}

		out := cmd.OutOrStdout()
		renderDiagnostics(out, diagnostics)
		renderRepairHistory(out, repairs)
		return nil
	},
}

func renderDiagnostics(w io.Writer, records []*envorcav1.DiagnosticRecord) {
	f := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	f("DIAGNOSTICS")
	if len(records) == 0 {
		f("  (none recorded)")
		return
	}
	f("  %-24s%-10s%s", "WHEN", "STATUS", "SNAPSHOT")
	for _, r := range records {
		f("  %-24s%-10s%s", displayTime(r.PerformedAt), decorateStatus(statusFromString(r.OverallStatus), r.OverallStatus), snapshotLine(r.Payload))
	}
	f("")
}

func renderRepairHistory(w io.Writer, records []*envorcav1.RepairRecord) {
	f := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	f("REPAIRS")
	if len(records) == 0 {
		f("  (none recorded)")
		return
	}
	f("  %-24s%-24s%-6s%s", "WHEN", "ACTION", "RESULT", "NOTES")
	for _, r := range records {
		result := "failed"
		if r.Success {
			result = "ok"
		}
		f("  %-24s%-24s%-6s%s", displayTime(r.PerformedAt), r.ActionId, result, r.Result)
	}
	f("")
}

// displayTime relabels an RFC3339 timestamp to a short local form. Falls
// back to the raw value when parsing fails.
func displayTime(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// snapshotLine summarises a diagnostic payload as a one-line preview. The
// payload is a JSON EnvironmentStatus snapshot.
func snapshotLine(payload string) string {
	const preview = 60
	if len(payload) <= preview {
		return payload
	}
	return payload[:preview] + "..."
}

// statusFromString maps a stored status string back to the proto enum.
func statusFromString(s string) envorcav1.Status {
	switch s {
	case "HEALTHY", "HEALTHY_STATUS", "STATUS_HEALTHY", "1":
		return envorcav1.Status_HEALTHY
	case "WARNING", "STATUS_WARNING", "2":
		return envorcav1.Status_WARNING
	case "CRITICAL", "STATUS_CRITICAL", "3":
		return envorcav1.Status_CRITICAL
	case "UNKNOWN", "STATUS_UNKNOWN", "4":
		return envorcav1.Status_UNKNOWN
	default:
		return envorcav1.Status_UNKNOWN
	}
}