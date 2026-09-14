package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/api/ipc"
	"envorca.dev/envorca/cli/internal/client"
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
			return errors.New("Envorca daemon is not running.\nRun 'envorca start' to launch it.")
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

func renderDoctor(w io.Writer, st *envorcav1.EnvironmentStatus) {
	f := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	f("ENVORCA DOCTOR")
	f("")
	f("  %-22s%-10s%s", "COMPONENT", "STATUS", "NOTES")

	var critical, warning, unknown int
	for _, c := range st.Components {
		status := plainStatus(c.Status)
		switch c.Status {
		case envorcav1.Status_CRITICAL:
			critical++
		case envorcav1.Status_WARNING:
			warning++
		case envorcav1.Status_UNKNOWN:
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
			f("  Envorca can fix this. Run 'envorca repair' to apply the repair.")
		} else if c.Status == envorcav1.Status_WARNING || c.Status == envorcav1.Status_CRITICAL {
			if c.Recommendation != "" {
				f("  %s: %s", c.Id, c.Recommendation)
			}
		}
	}
}

func doctorDetails(c *envorcav1.ComponentStatus) []string {
	if c.Status != envorcav1.Status_WARNING && c.Status != envorcav1.Status_CRITICAL {
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
