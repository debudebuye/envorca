package api

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/daemon/internal/events"
	"envorca.dev/envorca/daemon/internal/health"
	"envorca.dev/envorca/daemon/internal/recovery"
	"envorca.dev/envorca/daemon/internal/version"
)

// Daemon implements the envorca.v1.Daemon gRPC service. It is stateless with
// respect to infrastructure; it reads live probes from the health registry
// and delegates mutation to the services wired in cmd/envorcad.
type Daemon struct {
	envorcav1.UnimplementedDaemonServer

	log         *slog.Logger
	bus         *events.Bus
	health      *health.Registry
	recovery    *recovery.Recovery
	endpoint    string
	started     time.Time
	requestStop context.CancelFunc

	mu    sync.Mutex
	state string
}

// New creates the Daemon service handler.
func New(log *slog.Logger, bus *events.Bus, reg *health.Registry, rec *recovery.Recovery, endpoint string, requestStop context.CancelFunc) *Daemon {
	return &Daemon{
		log:         log,
		bus:         bus,
		health:      reg,
		recovery:    rec,
		endpoint:    endpoint,
		started:     time.Now(),
		requestStop: requestStop,
		state:       "running",
	}
}

// SetState updates the daemon lifecycle state exposed to clients.
func (d *Daemon) SetState(state string) {
	d.mu.Lock()
	d.state = state
	d.mu.Unlock()
}

// State returns the current daemon lifecycle state.
func (d *Daemon) State() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

// Ping reports daemon identity and liveness.
func (d *Daemon) Ping(ctx context.Context, in *envorcav1.PingRequest) (*envorcav1.PingResponse, error) {
	return &envorcav1.PingResponse{
		Version:       version.Version,
		DaemonState:   d.State(),
		UptimeSeconds: int64(time.Since(d.started) / time.Second),
		Socket:        d.endpoint,
	}, nil
}

// GetStatus returns the live environment status snapshot.
func (d *Daemon) GetStatus(ctx context.Context, in *envorcav1.GetStatusRequest) (*envorcav1.EnvironmentStatus, error) {
	components := d.health.Snapshot(ctx)
	return &envorcav1.EnvironmentStatus{
		Version:       version.Version,
		DaemonState:   d.State(),
		UptimeSeconds: int64(time.Since(d.started) / time.Second),
		Socket:        d.endpoint,
		OverallStatus: health.OverallStatus(components),
		Components:    components,
	}, nil
}

// GetRepairPlan evaluates the current component states and returns every
// applicable, safe repair action.
func (d *Daemon) GetRepairPlan(ctx context.Context, in *envorcav1.GetRepairPlanRequest) (*envorcav1.RepairPlan, error) {
	components := d.health.Snapshot(ctx)
	return &envorcav1.RepairPlan{
		Actions:     d.recovery.Plan(components),
		EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// ExecuteRepair runs one planned action. Actions that require confirmation
// are rejected unless the request carries confirmed=true.
func (d *Daemon) ExecuteRepair(ctx context.Context, in *envorcav1.ExecuteRepairRequest) (*envorcav1.RepairOutcome, error) {
	if d.recovery == nil {
		return nil, status.Error(codes.FailedPrecondition, "repair subsystem not available")
	}
	if in.ActionId == "" {
		return nil, status.Error(codes.InvalidArgument, "action_id is required")
	}
	components := d.health.Snapshot(ctx)
	outcome, err := d.recovery.Execute(ctx, in.ActionId, in.Confirmed, components)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	d.log.Info("repair executed", "action", in.ActionId, "success", outcome.Success, "result", outcome.Result)
	d.bus.Publish(events.Event{Level: "info", Component: "recovery", Name: "repair_executed", Fields: map[string]any{"result": outcome.Result}})
	return outcome, nil
}

// Shutdown requests a graceful daemon stop.
func (d *Daemon) Shutdown(ctx context.Context, in *envorcav1.ShutdownRequest) (*envorcav1.ShutdownResponse, error) {
	d.log.Info("shutdown requested")
	d.bus.Publish(events.Event{Level: "info", Component: "daemon", Name: "shutdown_requested"})
	d.SetState("stopping")
	if d.requestStop != nil {
		d.requestStop()
	}
	return &envorcav1.ShutdownResponse{}, nil
}