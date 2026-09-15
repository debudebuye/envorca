package main

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/cli/internal/client"
)

func init() {
	eventsCmd.Flags().Bool("replay", true, "replay recent daemon events before live ones")
	rootCmd.AddCommand(eventsCmd)
}

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Stream daemon events",
	Long: "Streams Envorca daemon events (startup, shutdown, repairs). By " +
		"default the recent event buffer is replayed first; --replay=false " +
		"shows only new events.",
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

		replay, _ := cmd.Flags().GetBool("replay")
		stream, err := client.StreamEvents(ctx, conn, replay)
		if err != nil {
			return fmt.Errorf("open event stream: %w", err)
		}
		out := cmd.OutOrStdout()
		for {
			ev, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) || ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("event stream: %w", err)
			}
			fmt.Fprintln(out, renderEvent(ev))
		}
	},
}

// renderEvent formats a single event as a log-style line. Fields are sorted
// for stable output.
func renderEvent(ev *envorcav1.Event) string {
	var b strings.Builder
	if ts := timeOnly(ev.Timestamp); ts != "" {
		fmt.Fprintf(&b, "%s ", ts)
	}
	fmt.Fprintf(&b, "%-5s %-12s %s", ev.Level, ev.Component, ev.Name)
	if ev.Project != "" {
		fmt.Fprintf(&b, " project=%s", ev.Project)
	}
	if len(ev.Fields) > 0 {
		keys := make([]string, 0, len(ev.Fields))
		for k := range ev.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, " %s=%q", k, ev.Fields[k])
		}
	}
	return b.String()
}

// timeOnly returns the HH:MM:SS portion of an RFC3339 timestamp, or "" when
// the timestamp is malformed.
func timeOnly(ts string) string {
	if ts == "" {
		return ""
	}
	// RFC3339[ Nano ]: time is the second colon-delimited field group.
	if i := strings.Index(ts, "T"); i >= 0 && i+9 <= len(ts) {
		return ts[i+1 : i+9]
	}
	return ""
}