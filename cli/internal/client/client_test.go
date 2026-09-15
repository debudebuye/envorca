package client

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/api/ipc"
)

var pipeSeq atomic.Int64

// testEndpoint returns a unique transport endpoint enabled by the
// ENVORCA_SOCKET override: a Windows named pipe or a Unix socket path.
func testEndpoint(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return `\\.\pipe\envorca-client-test-` + strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(pipeSeq.Add(1), 10)
	}
	return filepath.Join(t.TempDir(), "envorca.sock")
}

// fakeServer implements envorcav1.DaemonServer with deterministic fixtures.
type fakeServer struct {
	envorcav1.UnimplementedDaemonServer

	status  *envorcav1.EnvironmentStatus
	events  []*envorcav1.Event
	diags   []*envorcav1.DiagnosticRecord
	repairs []*envorcav1.RepairRecord
	pings   atomic.Int64
}

func (s *fakeServer) Ping(ctx context.Context, _ *envorcav1.PingRequest) (*envorcav1.PingResponse, error) {
	s.pings.Add(1)
	return &envorcav1.PingResponse{DaemonState: "running"}, nil
}

func (s *fakeServer) GetStatus(ctx context.Context, _ *envorcav1.GetStatusRequest) (*envorcav1.EnvironmentStatus, error) {
	return s.status, nil
}

func (s *fakeServer) GetDiagnostics(_ context.Context, _ *envorcav1.GetDiagnosticsRequest) (*envorcav1.GetDiagnosticsResponse, error) {
	return &envorcav1.GetDiagnosticsResponse{Diagnostics: s.diags}, nil
}

func (s *fakeServer) GetRepairHistory(_ context.Context, _ *envorcav1.GetRepairHistoryRequest) (*envorcav1.GetRepairHistoryResponse, error) {
	return &envorcav1.GetRepairHistoryResponse{Repairs: s.repairs}, nil
}

func (s *fakeServer) ExecuteRepair(_ context.Context, req *envorcav1.ExecuteRepairRequest) (*envorcav1.RepairOutcome, error) {
	return &envorcav1.RepairOutcome{ActionId: req.ActionId, Success: true, Result: "done"}, nil
}

func (s *fakeServer) StreamEvents(_ *envorcav1.StreamEventsRequest, stream envorcav1.Daemon_StreamEventsServer) error {
	for _, ev := range s.events {
		if err := stream.Send(ev); err != nil {
			return err
		}
	}
	return nil
}

// startFakeServer listens on a fresh endpoint (via the ENVORCA_SOCKET
// override) and returns the endpoint plus a running server.
func startFakeServer(t *testing.T, fake *fakeServer) string {
	t.Helper()
	ep := testEndpoint(t)
	t.Setenv(ipc.EnvOverride, ep)
	lis, err := ipc.Listen(ep)
	if err != nil {
		t.Fatalf("ipc.Listen(%q): %v", ep, err)
	}
	srv := grpc.NewServer()
	envorcav1.RegisterDaemonServer(srv, fake)
	done := make(chan struct{})
	go func() {
		_ = srv.Serve(lis)
		close(done)
	}()
	t.Cleanup(func() {
		srv.Stop()
		<-done
	})
	return ep
}

// dialDefault dials using the default endpoint derivation (override-aware).
func dialDefault(t *testing.T) *grpc.ClientConn {
	t.Helper()
	ep := ipc.DefaultEndpoint(ipc.DefaultStateDir())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := Dial(ctx, ep)
	if err != nil {
		t.Fatalf("Dial(%q): %v", ep, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestProbeReachableAndUnreachable(t *testing.T) {
	fake := &fakeServer{}
	ep := startFakeServer(t, fake)

	if !Probe(context.Background(), ipc.DefaultEndpoint(ipc.DefaultStateDir())) {
		t.Error("Probe on live server = false, want true")
	}
	t.Cleanup(func() {})
	_ = ep

	// Point the override at a dead endpoint and confirm Probe clears.
	lost := testEndpoint(t)
	t.Setenv(ipc.EnvOverride, lost)
	if Probe(context.Background(), lost) {
		t.Error("Probe on dead endpoint = true, want false")
	}
	if fake.pings.Load() != 1 {
		t.Errorf("pings = %d, want 1", fake.pings.Load())
	}
}

func TestGetStatusRoundTrip(t *testing.T) {
	startFakeServer(t, &fakeServer{
		status: &envorcav1.EnvironmentStatus{
			DaemonState:   "stopping",
			Version:       "0.1.0-test",
			Socket:        "inmemory",
			UptimeSeconds: 120,
			OverallStatus: envorcav1.Status_WARNING,
			Components: []*envorcav1.ComponentStatus{
				{Id: "wsl", Status: envorcav1.Status_WARNING, Summary: "slow startup"},
			},
		},
	})
	conn := dialDefault(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := GetStatus(ctx, conn, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got.GetVersion() != "0.1.0-test" || got.GetDaemonState() != "stopping" {
		t.Errorf("status = %v, want version/dstate", got)
	}
	if got.GetOverallStatus() != envorcav1.Status_WARNING {
		t.Errorf("overall = %v, want WARNING", got.GetOverallStatus())
	}
	if len(got.GetComponents()) != 1 || got.GetComponents()[0].GetId() != "wsl" {
		t.Errorf("components = %v", got.GetComponents())
	}
}

func TestStreamEventsRoundTrip(t *testing.T) {
	startFakeServer(t, &fakeServer{
		events: []*envorcav1.Event{
			{Seq: 1, Timestamp: "2026-09-15T00:00:00Z", Level: "info", Component: "api", Name: "started", Fields: map[string]string{"version": "0.1.0"}},
			{Seq: 2, Timestamp: "2026-09-15T00:00:01Z", Level: "warn", Component: "recovery", Name: "repair_failed", Fields: map[string]string{"action": "wsl.restart"}},
		},
	})
	conn := dialDefault(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := StreamEvents(ctx, conn, true)
	if err != nil {
		t.Fatal(err)
	}
	var got []*envorcav1.Event
	for {
		ev, err := stream.Recv()
		if err != nil {
			break // EOF
		}
		got = append(got, ev)
	}
	if len(got) != 2 {
		t.Fatalf("events = %d, want 2", len(got))
	}
	if got[0].GetName() != "started" || got[1].GetName() != "repair_failed" {
		t.Errorf("event names = %q, %q", got[0].GetName(), got[1].GetName())
	}
	if got[0].GetFields()["version"] != "0.1.0" {
		t.Errorf("fields = %v", got[0].GetFields())
	}
}

func TestGetDiagnosticsAndRepairHistoryRoundTrip(t *testing.T) {
	diag := &envorcav1.DiagnosticRecord{PerformedAt: "2026-09-15T00:00:00Z", OverallStatus: "WARNING", Payload: `{"overall_status":"WARNING"}`}
	repair := &envorcav1.RepairRecord{PerformedAt: "2026-09-15T00:00:01Z", ActionId: "wsl.restart", Success: true, Result: "ok"} 
	startFakeServer(t, &fakeServer{
		diags:   []*envorcav1.DiagnosticRecord{diag},
		repairs: []*envorcav1.RepairRecord{repair},
	})
	conn := dialDefault(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	diags, err := GetDiagnostics(ctx, conn, 5, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || diags[0].GetPerformedAt() == "" || diags[0].GetOverallStatus() != "WARNING" || diags[0].GetPayload() == "" {
		t.Errorf("diags = %v", diags)
	}

	repairs, err := GetRepairHistory(ctx, conn, 5, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(repairs) != 1 || repairs[0].GetActionId() != "wsl.restart" || !repairs[0].GetSuccess() {
		t.Errorf("repairs = %v", repairs)
	}
}

func TestExecuteRepairRoundTrip(t *testing.T) {
	startFakeServer(t, &fakeServer{})
	conn := dialDefault(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	outcome, err := ExecuteRepair(ctx, conn, "wsl.start", true, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.GetActionId() != "wsl.start" || !outcome.GetSuccess() {
		t.Errorf("outcome = %v", outcome)
	}
	if outcome.GetResult() != "done" {
		t.Errorf("result = %q", outcome.GetResult())
	}
}