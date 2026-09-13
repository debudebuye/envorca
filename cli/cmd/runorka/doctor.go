package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	runorkav1 "runorka.dev/runorka/api/gen/go/runorka/v1"
	"runorka.dev/runorka/api/ipc"
	"runorka.dev/runorka/cli/internal/client"
)

func init() {
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose the development environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		endpoint := ipc.DefaultEndpoint(ipc.DefaultStateDir())
		if !client.Probe(ctx, endpoint) {
			return errors.New("Runorka daemon is not running.\nRun 'runorka start' to launch it.")
		}
		conn, err := client.Dial(ctx, endpoint)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer conn.Close()
		st, err := client.GetStatus(ctx, conn, 8*time.Second)
		if err != nil {
			return fmt.Errorf("get status: %w", err)
		}
		renderDoctor(cmd.OutOrStdout(), st)
		return nil
	},
}

func renderDoctor(w io.Writer, st *runorkav1.EnvironmentStatus) {
	f := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	f("RUNORKA DOCTOR")
	f("")
	f("  %-22s%-10s%s", "COMPONENT", "STATUS", "NOTES")

	var critical, warning, unknown int
	for _, c := range st.Components {
		status := plainStatus(c.Status)
		switch c.Status {
		case runorkav1.Status_CRITICAL:
			critical++
		case runorkav1.Status_WARNING:
			warning++
		case runorkav1.Status_UNKNOWN:
			unknown++
		}
		f("  %-22s%-10s%s", c.Id, decorateStatus(c.Status, status), c.Summary)
		for _, detail := range doctorDetails(c) {
			f("        %s", detail)
		}
	}

	f("")
	f("  Overall: %s", decorateStatus(st.OverallStatus, plainStatus(st.OverallStatus)))

	counts := []string{}
	if critical > 0 {
		counts = append(counts, fmt.Sprintf("%d critical", critical))
	}
	if warning > 0 {
		counts = append(counts, fmt.Sprintf("%d warnings", warning))
	}
	if unknown > 0 {
		counts = append(counts, fmt.Sprintf("%d unknown", unknown))
	}
	if len(counts) == 0 {
		f("  Summary: 0 problems detected — the environment is healthy.")
		return
	}
	joined := counts[0]
	for _, c := range counts[1:] {
		joined += ", " + c
	}
	f("  Summary: %s detected.", joined)
	f("")
	for _, c := range st.Components {
		if c.SafeToFix {
			f("  %s: %s", c.Id, c.Recommendation)
			f("  Runorka can fix this. Run 'runorka repair' to apply the repair.")
		} else if c.Status == runorkav1.Status_WARNING || c.Status == runorkav1.Status_CRITICAL {
			if c.Recommendation != "" {
				f("  %s: %s", c.Id, c.Recommendation)
			}
		}
	}
}

func doctorDetails(c *runorkav1.ComponentStatus) []string {
	if c.Status != runorkav1.Status_WARNING && c.Status != runorkav1.Status_CRITICAL {
		return nil
	}
	var out []string
	if c.Reason != "" {
		out = append(out, "Why: "+c.Reason)
	}
	if c.Recommendation != "" {
		out = append(out, "Recommend: "+c.Recommendation)
	}
	return out
}
