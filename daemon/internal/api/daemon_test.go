package api_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	envorkav1 "envorka.dev/envorka/api/gen/go/envorka/v1"
	"envorka.dev/envorka/api/ipc"
	daemonapi "envorka.dev/envorka/daemon/internal/api"
	"envorka.dev/envorka/daemon/internal/events"
	"envorka.dev/envorka/daemon/internal/health"
	"envorka.dev/envorka/daemon/internal/recovery"
	"envorka.dev/envorka/daemon/internal/wsl"
)

type harness struct {
	client       envorkav1.DaemonClient
	setWSLCritical func(bool)
}

func startTestDaemon(t *testing.T) *harness {
	t.Helper()
	ep := filepath.Join(t.TempDir(), "envorka.sock")
	ln, err := ipc.Listen(ep)
	if err != nil {
		t.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	ctx, cancel := context.WithCancel(context.Background())

	wslCritical := false
	runner := fakeRunner()
	reg := health.NewRegistry()
	if err := reg.Register("daemon", func(context.Context) envorkav1.ComponentStatus {
		return envorkav1.ComponentStatus{Status: envorkav1.Status_HEALTHY, Summary: "ok"}
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("wsl", func(context.Context) envorkav1.ComponentStatus {
		if wslCritical {
			return envorkav1.ComponentStatus{
				Status:         envorkav1.Status_CRITICAL,
				Summary:        "Linux environment failed to start",
				Recommendation: "Restart the WSL environment.",
				SafeToFix:      true,
			}
		}
		return envorkav1.ComponentStatus{Status: envorkav1.Status_HEALTHY, Summary: "WSL2 ready"}
	}); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := daemonapi.New(logger, events.New(16), reg, recovery.New(runner), ep, cancel)
	envorkav1.RegisterDaemonServer(grpcServer, d)

	go func() { grpcServer.Serve(ln) }()
	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.NewClient("passthrough:///envorka",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return ipc.DialContext(ctx, ep)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	client := envorkav1.NewDaemonClient(conn)
	for i := 0; i < 20; i++ {
		if _, err = client.GetStatus(ctx, &envorkav1.GetStatusRequest{}); err == nil {
			return &harness{client: client, setWSLCritical: func(b bool) { wslCritical = b }}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("daemon not reachable: %v", err)
	return nil
}

func fakeRunner() wsl.Runner {
	return func(_ context.Context, args ...string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "--list --verbose":
			return []byte("  NAME            STATE     VERSION\n* Ubuntu-24.04  Running   2\n"), nil
		default:
			return []byte("envorka:boot"), nil
		}
	}
}

func TestPingAndGetStatus(t *testing.T) {
	h := startTestDaemon(t)
	ctx := context.Background()

	resp, err := h.client.Ping(ctx, &envorkav1.PingRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Version == "" {
		t.Error("empty version")
	}

	st, err := h.client.GetStatus(ctx, &envorkav1.GetStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if st.DaemonState != "running" {
		t.Errorf("daemon state = %q, want running", st.DaemonState)
	}
	if st.OverallStatus != envorkav1.Status_HEALTHY {
		t.Errorf("overall = %s, want HEALTHY", st.OverallStatus)
	}
	if len(st.Components) != 2 {
		t.Fatalf("components = %d, want 2", len(st.Components))
	}
	if st.Components[0].Id != "daemon" || st.Components[0].Status != envorkav1.Status_HEALTHY {
		t.Errorf("daemon component = %+v", st.Components[0])
	}
	if st.UptimeSeconds < 0 {
		t.Error("negative uptime")
	}
}

func TestRepairPlanRoundTrip(t *testing.T) {
	h := startTestDaemon(t)
	ctx := context.Background()

	h.setWSLCritical(true)
	plan, err := h.client.GetRepairPlan(ctx, &envorkav1.GetRepairPlanRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 2 {
		t.Fatalf("plan actions = %d, want 2: %+v", len(plan.Actions), plan.Actions)
	}
	if plan.Actions[0].ActionId != "wsl.start" || plan.Actions[0].RequiresConfirmation {
		t.Errorf("first action = %+v, want non-confirming wsl.start", plan.Actions[0])
	}
	if plan.Actions[1].ActionId != "wsl.restart" || !plan.Actions[1].RequiresConfirmation {
		t.Errorf("second action = %+v, want confirming wsl.restart", plan.Actions[1])
	}
	if plan.EvaluatedAt == "" {
		t.Error("plan should carry an RFC3339 evaluation timestamp")
	}

	outcome, err := h.client.ExecuteRepair(ctx, &envorkav1.ExecuteRepairRequest{ActionId: "wsl.start"})
	if err != nil {
		t.Fatalf("ExecuteRepair wsl.start: %v", err)
	}
	if !outcome.Success {
		t.Fatalf("wsl.start outcome = %+v, want success", outcome)
	}

	outcome, err = h.client.ExecuteRepair(ctx, &envorkav1.ExecuteRepairRequest{ActionId: "wsl.restart"})
	if err == nil {
		t.Fatalf("wsl.restart without confirmation should fail, got %+v", outcome)
	}
	if codes.Code(status.Code(err)) != codes.FailedPrecondition {
		t.Errorf("wsl.restart err code = %v, want FailedPrecondition", status.Code(err))
	}

	outcome, err = h.client.ExecuteRepair(ctx, &envorkav1.ExecuteRepairRequest{ActionId: "wsl.restart", Confirmed: true})
	if err != nil {
		t.Fatalf("ExecuteRepair wsl.restart confirmed: %v", err)
	}
	if !outcome.Success || outcome.Verification == "" {
		t.Fatalf("confirmed restart outcome = %+v", outcome)
	}
}

func TestRepairPlanEmptyWhenHealthy(t *testing.T) {
	h := startTestDaemon(t)
	plan, err := h.client.GetRepairPlan(context.Background(), &envorkav1.GetRepairPlanRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 0 {
		t.Fatalf("healthy plan = %+v, want empty", plan.Actions)
	}
}

func TestGetStatusReflectsCriticalOverall(t *testing.T) {
	h := startTestDaemon(t)
	h.setWSLCritical(true)
	st, err := h.client.GetStatus(context.Background(), &envorkav1.GetStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if st.OverallStatus != envorkav1.Status_CRITICAL {
		t.Errorf("overall = %s, want CRITICAL", st.OverallStatus)
	}
	if st.Components[1].Id != "wsl" || st.Components[1].SafeToFix != true {
		t.Errorf("wsl component = %+v", st.Components[1])
	}
}

func TestShutdownStops(t *testing.T) {
	startTestDaemon(t)
}