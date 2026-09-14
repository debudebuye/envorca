package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"envorka.dev/envorka/api/ipc"
	"envorka.dev/envorka/cli/internal/client"
)

func init() {
	repairCmd.PersistentFlags().BoolP("yes", "y", false, "authorize all listed repairs without prompting")
	rootCmd.AddCommand(repairCmd)
}

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Propose and apply safe repairs",
	Long: "Asks the daemon to evaluate the environment and present safe, deterministic " +
		"repair actions. Actions that require confirmation are only run when the user " +
		"authorizes them (or --yes is passed).",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		endpoint := ipc.DefaultEndpoint(ipc.DefaultStateDir())
		if !client.Probe(ctx, endpoint) {
			return errors.New("Envorka daemon is not running.\nRun 'envorka start' to launch it.")
		}
		conn, err := client.Dial(ctx, endpoint)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer conn.Close()

		plan, err := client.GetRepairPlan(ctx, conn, 8*time.Second)
		if err != nil {
			return fmt.Errorf("get repair plan: %w", err)
		}
		if len(plan.Actions) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing to repair.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Envorka can fix %d issue(s):\n\n", len(plan.Actions))
		for _, a := range plan.Actions {
			flag := ""
			if a.RequiresConfirmation {
				flag = " requires confirmation"
			}
			if !a.Safe {
				flag += " UNSAFE"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s%s\n", a.ActionId, a.Summary, flag)
			if a.Description != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "        %s\n", a.Description)
			}
		}

		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			ok, err := promptYesNo(cmd.InOrStdin(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted. No changes were made.")
				return nil
			}
		}

		failures := 0
		for _, a := range plan.Actions {
			outcome, err := client.ExecuteRepair(ctx, conn, a.ActionId, yes, 30*time.Second)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: failed — %v\n", a.ActionId, err)
				failures++
				continue
			}
			if outcome.Success {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: ok%s\n", a.ActionId, outcomeLabel(outcome.Verification))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: failed — %s\n", a.ActionId, outcome.Result)
				failures++
			}
		}
		if failures == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "\nAll repairs applied. Run 'envorka doctor' to confirm.")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d repair(s) failed. Run 'envorka doctor' for details.\n", failures)
		}
		return nil
	},
}

func outcomeLabel(verification string) string {
	if verification == "" {
		return ""
	}
	return " (verified)"
}

func promptYesNo(r io.Reader, w io.Writer) (bool, error) {
	fmt.Fprint(w, "\nApply these repairs now? [y/N]: ")
	var answer string
	if _, err := fmt.Fscanln(r, &answer); err != nil {
		// No input (EOF) or blank line means no, unless it's a real read error.
		if err.Error() == "unexpected newline" || err.Error() == "EOF" {
			return false, nil
		}
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
