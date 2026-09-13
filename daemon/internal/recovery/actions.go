// Package recovery proposes and executes safe, deterministic repairs. Every
// action explains itself, and actions that disrupt or touch anything beyond
// a simple restart require explicit confirmation. V1 ships a small action
// set; the framework is what matters (recovery flow: diagnose -> propose ->
// confirm -> execute -> verify).
package recovery

import (
	"context"
	"errors"
	"fmt"
	"strings"

	runorkav1 "runorka.dev/runorka/api/gen/go/runorka/v1"
	"runorka.dev/runorka/daemon/internal/wsl"
)

// Action is one repair operation tied to a health component.
type Action interface {
	ID() string
	ComponentID() string
	Summary() string
	Description() string
	RequiresConfirmation() bool
	// CanExecute reports whether the action is applicable to the given
	// component states. The daemon only proposes actions that CanExecute.
	CanExecute(components []*runorkav1.ComponentStatus) bool
	Execute(ctx context.Context, runner wsl.Runner) error
	Verify(ctx context.Context, runner wsl.Runner) error
}

// Recovery owns the known action set and executes actions.
type Recovery struct {
	runner  wsl.Runner
	actions []Action
}

// New creates a Recovery that runs actions through the given wsl runner.
func New(runner wsl.Runner) *Recovery {
	return &Recovery{
		runner: runner,
		actions: []Action{
			WSLStartAction{},
			WSLRestartAction{},
		},
	}
}

// Plan returns the repair actions applicable to the given component states,
// in a stable order.
func (r *Recovery) Plan(components []*runorkav1.ComponentStatus) []*runorkav1.RepairAction {
	var out []*runorkav1.RepairAction
	for _, a := range r.actions {
		if !a.CanExecute(components) {
			continue
		}
		out = append(out, &runorkav1.RepairAction{
			ComponentId:          a.ComponentID(),
			ActionId:             a.ID(),
			Summary:              a.Summary(),
			Description:          a.Description(),
			Safe:                 true,
			RequiresConfirmation: a.RequiresConfirmation(),
		})
	}
	return out
}

// Execute runs one action by ID and verifies the result. It returns a
// non-error only for transport/validation-level failures; action results are
// reported inside the outcome.
func (r *Recovery) Execute(ctx context.Context, actionID string, confirmed bool, components []*runorkav1.ComponentStatus) (*runorkav1.RepairOutcome, error) {
	for _, a := range r.actions {
		if a.ID() != actionID {
			continue
		}
		if !a.CanExecute(components) {
			return &runorkav1.RepairOutcome{
				ActionId:    actionID,
				ComponentId: a.ComponentID(),
				Success:     false,
				Result:      "action no longer applicable; re-run doctor",
			}, nil
		}
		if a.RequiresConfirmation() && !confirmed {
			return nil, errors.New("confirmation required for " + actionID)
		}
		result := "ok"
		outcome := &runorkav1.RepairOutcome{ActionId: actionID, ComponentId: a.ComponentID()}
		if err := a.Execute(ctx, r.runner); err != nil {
			result = "execute failed: " + err.Error()
			outcome.Success = false
			outcome.Result = result
			return outcome, nil
		}
		if err := a.Verify(ctx, r.runner); err != nil {
			result = "executed but verification failed: " + err.Error()
			outcome.Success = false
			outcome.Result = result
			return outcome, nil
		}
		outcome.Success = true
		outcome.Result = result
		outcome.Verification = "component now passes verification"
		return outcome, nil
	}
	return nil, fmt.Errorf("unknown repair action %q", actionID)
}

func wslCritical(components []*runorkav1.ComponentStatus, predicate func(*runorkav1.ComponentStatus) bool) bool {
	for _, c := range components {
		if c.Id == "wsl" && c.Status == runorkav1.Status_CRITICAL && predicate(c) {
			return true
		}
	}
	return false
}

func summaryHas(c *runorkav1.ComponentStatus, keywords ...string) bool {
	s := strings.ToLower(c.Summary)
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// WSLStartAction boots the default distribution. Safe and non-destructive;
// no confirmation required.
type WSLStartAction struct{}

func (WSLStartAction) ID() string          { return "wsl.start" }
func (WSLStartAction) ComponentID() string { return "wsl" }
func (WSLStartAction) Summary() string     { return "Start the WSL environment" }
func (WSLStartAction) Description() string {
	return "Boots the default Linux distribution. Does not affect user data."
}
func (WSLStartAction) RequiresConfirmation() bool { return false }
func (WSLStartAction) CanExecute(c []*runorkav1.ComponentStatus) bool {
	return wslCritical(c, func(s *runorkav1.ComponentStatus) bool {
		return summaryHas(s, "failed to start")
	})
}
func (WSLStartAction) Execute(ctx context.Context, runner wsl.Runner) error {
	distro, err := wsl.DefaultDistro(ctx, runner)
	if err != nil {
		return err
	}
	return wsl.Boot(ctx, runner, distro)
}
func (WSLStartAction) Verify(ctx context.Context, runner wsl.Runner) error {
	distro, err := wsl.DefaultDistro(ctx, runner)
	if err != nil {
		return err
	}
	return wsl.Boot(ctx, runner, distro)
}

// WSLRestartAction stops the WSL virtual machine and boots the default
// distribution again. Interrupts all running distributions, so it always
// requires explicit confirmation.
type WSLRestartAction struct{}

func (WSLRestartAction) ID() string          { return "wsl.restart" }
func (WSLRestartAction) ComponentID() string { return "wsl" }
func (WSLRestartAction) Summary() string     { return "Restart the WSL environment" }
func (WSLRestartAction) Description() string {
	return "Stops and restarts the WSL virtual machine. Running distributions are interrupted; no user data is removed."
}
func (WSLRestartAction) RequiresConfirmation() bool { return true }
func (WSLRestartAction) CanExecute(c []*runorkav1.ComponentStatus) bool {
	return wslCritical(c, func(s *runorkav1.ComponentStatus) bool {
		return summaryHas(s, "failed to start", "could not query", "broken")
	})
}
func (WSLRestartAction) Execute(ctx context.Context, runner wsl.Runner) error {
	if err := wsl.Shutdown(ctx, runner); err != nil {
		return err
	}
	distro, err := wsl.DefaultDistro(ctx, runner)
	if err != nil {
		return err
	}
	return wsl.Boot(ctx, runner, distro)
}
func (WSLRestartAction) Verify(ctx context.Context, runner wsl.Runner) error {
	distro, err := wsl.DefaultDistro(ctx, runner)
	if err != nil {
		return err
	}
	return wsl.Boot(ctx, runner, distro)
}
