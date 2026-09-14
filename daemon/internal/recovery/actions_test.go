package recovery

import (
	"context"
	"strings"
	"testing"

	envorkav1 "envorka.dev/envorka/api/gen/go/envorka/v1"
)

func components(st *envorkav1.ComponentStatus) []*envorkav1.ComponentStatus {
	return []*envorkav1.ComponentStatus{st}
}

func wslStatus(s envorkav1.Status, summary string) *envorkav1.ComponentStatus {
	return &envorkav1.ComponentStatus{Id: "wsl", Status: s, Summary: summary}
}

// scriptedRunner emulates wsl.exe: --list --verbose reports the default
// distro, --shutdown succeeds, and boots fail for boot numbers >= failBootsFrom
// (0 = never fail).
type scriptedRunner struct {
	defaultDistro string
	failBootsFrom int
	boots         int
	shutdowns     int
}

func (s *scriptedRunner) run(_ context.Context, args ...string) ([]byte, error) {
	switch strings.Join(args, " ") {
	case "--list --verbose":
		return []byte("  NAME            STATE     VERSION\n* " + s.defaultDistro + "  Running   2\n"), nil
	case "--shutdown":
		s.shutdowns++
		return nil, nil
	default:
		if strings.HasPrefix(strings.Join(args, " "), "--distribution ") {
			s.boots++
			if s.failBootsFrom > 0 && s.boots >= s.failBootsFrom {
				return nil, context.DeadlineExceeded
			}
			return []byte("envorka:boot"), nil
		}
		return nil, nil
	}
}

func TestPlanEmptyWhenHealthy(t *testing.T) {
	r := New(nil)
	plan := r.Plan(components(wslStatus(envorkav1.Status_HEALTHY, "WSL2 ready")))
	if len(plan) != 0 {
		t.Fatalf("healthy components produced %d actions: %+v", len(plan), plan)
	}
}

func TestPlanOffersStartAndRestartOnBootFailure(t *testing.T) {
	r := New(nil)
	st := wslStatus(envorkav1.Status_CRITICAL, "Linux environment failed to start")
	plan := r.Plan(components(st))
	if len(plan) != 2 {
		t.Fatalf("got %d actions, want 2: %+v", len(plan), plan)
	}
	start, restart := plan[0], plan[1]
	if start.ActionId != "wsl.start" || start.RequiresConfirmation {
		t.Errorf("wsl.start should be first and non-confirming, got %+v", start)
	}
	if restart.ActionId != "wsl.restart" || !restart.RequiresConfirmation {
		t.Errorf("wsl.restart should be second and confirming, got %+v", restart)
	}
	if !start.Safe {
		t.Error("wsl.start must be marked safe")
	}
}

func TestPlanOffersOnlyRestartOnQueryFailure(t *testing.T) {
	r := New(nil)
	st := wslStatus(envorkav1.Status_CRITICAL, "could not query WSL status")
	plan := r.Plan(components(st))
	if len(plan) != 1 || plan[0].ActionId != "wsl.restart" {
		t.Fatalf("want only wsl.restart, got %+v", plan)
	}
}

func TestPlanEmptyForWarnings(t *testing.T) {
	r := New(nil)
	plan := r.Plan(components(wslStatus(envorkav1.Status_WARNING, "no default Linux distribution")))
	if len(plan) != 0 {
		t.Fatalf("warnings must not auto-offer repairs, got %+v", plan)
	}
}

func TestExecuteStartVerifies(t *testing.T) {
	s := &scriptedRunner{defaultDistro: "Ubuntu"}
	r := New(s.run)
	outcome, err := r.Execute(context.Background(), "wsl.start", false,
		components(wslStatus(envorkav1.Status_CRITICAL, "Linux environment failed to start")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !outcome.Success {
		t.Fatalf("wsl.start failed: %+v", outcome)
	}
	if s.boots < 2 {
		t.Errorf("expected execute + verify boots, got %d", s.boots)
	}
}

func TestExecuteStartReportsFailure(t *testing.T) {
	s := &scriptedRunner{defaultDistro: "Ubuntu", failBootsFrom: 1}
	r := New(s.run)
	outcome, err := r.Execute(context.Background(), "wsl.start", false,
		components(wslStatus(envorkav1.Status_CRITICAL, "Linux environment failed to start")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if outcome.Success {
		t.Fatal("wsl.start should report failure when boot keeps failing")
	}
	if outcome.Result == "" {
		t.Error("failed outcome must explain the result")
	}
}

func TestExecuteRequiresConfirmationForRestart(t *testing.T) {
	s := &scriptedRunner{defaultDistro: "Ubuntu"}
	r := New(s.run)
	_, err := r.Execute(context.Background(), "wsl.restart", false,
		components(wslStatus(envorkav1.Status_CRITICAL, "Linux environment failed to start")))
	if err == nil {
		t.Fatal("wsl.restart without confirmation must error")
	}
	if !strings.Contains(err.Error(), "confirmation required") {
		t.Errorf("unexpected error: %v", err)
	}
	if s.shutdowns != 0 {
		t.Error("no shutdown should run without confirmation")
	}
}

func TestExecuteRestartConfirmed(t *testing.T) {
	s := &scriptedRunner{defaultDistro: "Ubuntu"}
	r := New(s.run)
	outcome, err := r.Execute(context.Background(), "wsl.restart", true,
		components(wslStatus(envorkav1.Status_CRITICAL, "could not query WSL status")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !outcome.Success {
		t.Fatalf("wsl.restart failed: %+v", outcome)
	}
	if s.shutdowns != 1 {
		t.Errorf("expected one shutdown, got %d", s.shutdowns)
	}
	if s.boots < 2 {
		t.Errorf("expected execute + verify boots, got %d", s.boots)
	}
}

func TestExecuteVerifyFailure(t *testing.T) {
	s := &scriptedRunner{defaultDistro: "Ubuntu", failBootsFrom: 2}
	r := New(s.run)
	outcome, err := r.Execute(context.Background(), "wsl.start", false,
		components(wslStatus(envorkav1.Status_CRITICAL, "Linux environment failed to start")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if outcome.Success {
		t.Fatal("action must report failure when verification fails")
	}
	if !strings.Contains(outcome.Result, "verification failed") {
		t.Errorf("result %q should mention verification failure", outcome.Result)
	}
}

func TestExecuteUnknownAction(t *testing.T) {
	r := New(nil)
	if _, err := r.Execute(context.Background(), "nope", false, nil); err == nil {
		t.Fatal("unknown action must error")
	}
}

func TestExecuteRejectsNoLongerApplicable(t *testing.T) {
	s := &scriptedRunner{defaultDistro: "Ubuntu"}
	r := New(s.run)
	outcome, err := r.Execute(context.Background(), "wsl.start", false,
		components(wslStatus(envorkav1.Status_HEALTHY, "WSL2 ready")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if outcome.Success {
		t.Fatal("action on a healthy component must not succeed")
	}
}