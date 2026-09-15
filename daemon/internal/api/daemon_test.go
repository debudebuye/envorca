package api_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/api/ipc"
	daemonapi "envorca.dev/envorca/daemon/internal/api"
	"envorca.dev/envorca/daemon/internal/events"
	"envorca.dev/envorca/daemon/internal/health"
	"envorca.dev/envorca/daemon/internal/recovery"
	"envorca.dev/envorca/daemon/internal/state"
	"envorca.dev/envorca/daemon/internal/wsl"
)

type harness struct {
	client         envorcav1.DaemonClient
	setWSLCritical func(bool)
}

// testEndpoint returns a unique endpoint for the platform: a Windows named
// pipe on Windows, a Unix socket path elsewhere.
func testEndpoint(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return `\\.\pipe\envorca-test-` + strconv.Itoa(os.Getpid()) + "-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	}
	return filepath.Join(t.TempDir(), "envorca.sock")
}

func startTestDaemon(t *testing.T) *harness {
	t.Helper()
	ep := testEndpoint(t)
	ln, err := ipc.Listen(ep)
	if err != nil {
		t.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	ctx, cancel := context.WithCancel(context.Background())

	wslCritical := false
	runner := fakeRunner()
	reg := health.NewRegistry()
	if err := reg.Register("daemon", func(context.Context) envorcav1.ComponentStatus {
		return envorcav1.ComponentStatus{Status: envorcav1.Status_HEALTHY, Summary: "ok"}
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("wsl", func(context.Context) envorcav1.ComponentStatus {
		if wslCritical {
			return envorcav1.ComponentStatus{
				Status:         envorcav1.Status_CRITICAL,
				Summary:        "Linux environment failed to start",
				Recommendation: "Restart the WSL environment.",
				SafeToFix:      true,
			}
		}
		return envorcav1.ComponentStatus{Status: envorcav1.Status_HEALTHY, Summary: "WSL2 ready"}
	}); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := state.Open(filepath.Join(t.TempDir(), "db", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	d := daemonapi.New(logger, events.New(16), reg, recovery.New(runner), db, ep, cancel)
	envorcav1.RegisterDaemonServer(grpcServer, d)

	go func() { grpcServer.Serve(ln) }()
	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.NewClient("passthrough:///envorca",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return ipc.DialContext(ctx, ep)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	client := envorcav1.NewDaemonClient(conn)
	for i := 0; i < 20; i++ {
		if _, err = client.GetStatus(ctx, &envorcav1.GetStatusRequest{}); err == nil {
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
			return []byte("envorca:boot"), nil
		}
	}
}

func TestPingAndGetStatus(t *testing.T) {
	h := startTestDaemon(t)
	ctx := context.Background()

	resp, err := h.client.Ping(ctx, &envorcav1.PingRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Version == "" {
		t.Error("empty version")
	}

	st, err := h.client.GetStatus(ctx, &envorcav1.GetStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if st.DaemonState != "running" {
		t.Errorf("daemon state = %q, want running", st.DaemonState)
	}
	if st.OverallStatus != envorcav1.Status_HEALTHY {
		t.Errorf("overall = %s, want HEALTHY", st.OverallStatus)
	}
	if len(st.Components) != 2 {
		t.Fatalf("components = %d, want 2", len(st.Components))
	}
	if st.Components[0].Id != "daemon" || st.Components[0].Status != envorcav1.Status_HEALTHY {
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
	plan, err := h.client.GetRepairPlan(ctx, &envorcav1.GetRepairPlanRequest{})
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

	outcome, err := h.client.ExecuteRepair(ctx, &envorcav1.ExecuteRepairRequest{ActionId: "wsl.start"})
	if err != nil {
		t.Fatalf("ExecuteRepair wsl.start: %v", err)
	}
	if !outcome.Success {
		t.Fatalf("wsl.start outcome = %+v, want success", outcome)
	}

	outcome, err = h.client.ExecuteRepair(ctx, &envorcav1.ExecuteRepairRequest{ActionId: "wsl.restart"})
	if err == nil {
		t.Fatalf("wsl.restart without confirmation should fail, got %+v", outcome)
	}
	if codes.Code(status.Code(err)) != codes.FailedPrecondition {
		t.Errorf("wsl.restart err code = %v, want FailedPrecondition", status.Code(err))
	}

	outcome, err = h.client.ExecuteRepair(ctx, &envorcav1.ExecuteRepairRequest{ActionId: "wsl.restart", Confirmed: true})
	if err != nil {
		t.Fatalf("ExecuteRepair wsl.restart confirmed: %v", err)
	}
	if !outcome.Success || outcome.Verification == "" {
		t.Fatalf("confirmed restart outcome = %+v", outcome)
	}
}

func TestRepairPlanEmptyWhenHealthy(t *testing.T) {
	h := startTestDaemon(t)
	plan, err := h.client.GetRepairPlan(context.Background(), &envorcav1.GetRepairPlanRequest{})
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
	st, err := h.client.GetStatus(context.Background(), &envorcav1.GetStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if st.OverallStatus != envorcav1.Status_CRITICAL {
		t.Errorf("overall = %s, want CRITICAL", st.OverallStatus)
	}
	if st.Components[1].Id != "wsl" || st.Components[1].SafeToFix != true {
		t.Errorf("wsl component = %+v", st.Components[1])
	}
}

func TestShutdownStops(t *testing.T) {
	startTestDaemon(t)
}

func TestDiagnosticsAndRepairHistory(t *testing.T) {
	h := startTestDaemon(t)
	ctx := context.Background()

	dg, err := h.client.GetDiagnostics(ctx, &envorcav1.GetDiagnosticsRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(dg.Diagnostics) == 0 {
		t.Fatal("GetStatus readiness loop should have recorded diagnostics")
	}
	if dg.Diagnostics[0].OverallStatus == "" {
		t.Error("diagnostic missing overall status")
	}
	if dg.Diagnostics[0].Payload == "" {
		t.Error("diagnostic payload not persisted")
	}

	h.setWSLCritical(true)
	outcome, err := h.client.ExecuteRepair(ctx, &envorcav1.ExecuteRepairRequest{ActionId: "wsl.start"})
	if err != nil {
		t.Fatalf("ExecuteRepair: %v", err)
	}
	if !outcome.Success {
		t.Fatalf("repair outcome not success: %+v", outcome)
	}

	rh, err := h.client.GetRepairHistory(ctx, &envorcav1.GetRepairHistoryRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rh.Repairs) == 0 {
		t.Fatal("ExecuteRepair should have recorded repair history")
	}
	rec := rh.Repairs[0]
	if rec.ActionId != "wsl.start" || !rec.Success {
		t.Errorf("newest repair = %+v, want successful wsl.start", rec)
	}
	if rec.PerformedAt == "" {
		t.Error("repair record missing timestamp")
	}
}

func TestDiagnosticsLimitClamped(t *testing.T) {
	h := startTestDaemon(t)
	ctx := context.Background()
	for i := 0; i < 30; i++ {
		if _, err := h.client.GetStatus(ctx, &envorcav1.GetStatusRequest{}); err != nil {
			t.Fatal(err)
		}
	}
	dg, err := h.client.GetDiagnostics(ctx, &envorcav1.GetDiagnosticsRequest{Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(dg.Diagnostics) > 5 {
		t.Errorf("got %d diagnostics, want <= 5", len(dg.Diagnostics))
	}
	// Newest first: previous value should not be after the newer ones.
	if len(dg.Diagnostics) >= 2 && dg.Diagnostics[0].PerformedAt < dg.Diagnostics[1].PerformedAt {
		t.Error("diagnostics are not newest-first")
	}
}

func TestStreamEventsReplay(t *testing.T) {
	h := startTestDaemon(t)
	ctx := context.Background()

	h.setWSLCritical(true)
	if _, err := h.client.ExecuteRepair(ctx, &envorcav1.ExecuteRepairRequest{ActionId: "wsl.start"}); err != nil {
		t.Fatal(err)
	}

	stream, err := h.client.StreamEvents(ctx, &envorcav1.StreamEventsRequest{Replay: true})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Name != "repair_executed" {
		t.Errorf("replayed event = %q, want repair_executed", ev.Name)
	}
	if ev.Seq == 0 {
		t.Error("replayed event missing seq")
	}
	if ev.Timestamp == "" {
		t.Error("replayed event missing timestamp")
	}
	if ev.Component != "recovery" {
		t.Errorf("component = %q, want recovery", ev.Component)
	}
}

func TestStreamEventsLive(t *testing.T) {
	h := startTestDaemon(t)
	ctx := context.Background()

	stream, err := h.client.StreamEvents(ctx, &envorcav1.StreamEventsRequest{Replay: false})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond) // let the server subscription land

	h.setWSLCritical(true)
	if _, err := h.client.ExecuteRepair(ctx, &envorcav1.ExecuteRepairRequest{ActionId: "wsl.start"}); err != nil {
		t.Fatal(err)
	}

	type recv struct {
		ev  *envorcav1.Event
		err error
	}
	ch := make(chan recv, 1)
	go func() {
		ev, err := stream.Recv()
		ch <- recv{ev, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("stream recv: %v", r.err)
		}
		if r.ev.Name != "repair_executed" {
			t.Errorf("live event = %q, want repair_executed", r.ev.Name)
		}
		if r.ev.Seq == 0 {
			t.Error("live event missing seq")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no live event received")
	}
}