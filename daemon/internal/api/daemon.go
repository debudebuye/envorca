package api

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/daemon/internal/events"
	"envorca.dev/envorca/daemon/internal/health"
	"envorca.dev/envorca/daemon/internal/recovery"
	"envorca.dev/envorca/daemon/internal/state"
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
	state       *state.DB
	endpoint    string
	started     time.Time
	requestStop context.CancelFunc

	mu    sync.Mutex
	dstate string
}

// New creates the Daemon service handler. state may be nil; history records
// are then skipped (used by tests and minimal deployments).
func New(log *slog.Logger, bus *events.Bus, reg *health.Registry, rec *recovery.Recovery, st *state.DB, endpoint string, requestStop context.CancelFunc) *Daemon {
	return &Daemon{
		log:         log,
		bus:         bus,
		health:      reg,
		recovery:    rec,
		state:       st,
		endpoint:    endpoint,
		started:     time.Now(),
		requestStop: requestStop,
		dstate:      "running",
	}
}

// SetState updates the daemon lifecycle state exposed to clients.
func (d *Daemon) SetState(state string) {
	d.mu.Lock()
	d.dstate = state
	d.mu.Unlock()
}

// State returns the current daemon lifecycle state.
func (d *Daemon) State() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dstate
}

// Ping reports daemon identity and liveness.
func (d *Daemon) Ping(ctx context.Context, in *envorcav1.PingRequest) (*envorcav1.PingResponse, error) {
	return &envorcav1.PingResponse{
		Version:       version.Full(),
		DaemonState:   d.State(),
		UptimeSeconds: int64(time.Since(d.started) / time.Second),
		Socket:        d.endpoint,
	}, nil
}

// GetStatus returns the live environment status snapshot and records it in
// the diagnostic history.
func (d *Daemon) GetStatus(ctx context.Context, in *envorcav1.GetStatusRequest) (*envorcav1.EnvironmentStatus, error) {
	components := d.health.Snapshot(ctx)
	overall := health.OverallStatus(components)
	st := &envorcav1.EnvironmentStatus{
		Version:       version.Full(),
		DaemonState:   d.State(),
		UptimeSeconds: int64(time.Since(d.started) / time.Second),
		Socket:        d.endpoint,
		OverallStatus: overall,
		Components:    components,
	}
	if d.state != nil {
		if payload, err := protojson.Marshal(st); err == nil {
			if err := d.state.RecordDiagnostic(ctx, overall.String(), string(payload)); err != nil {
				d.log.Warn("record diagnostic", "error", err)
			}
		}
	}
	return st, nil
}

// GetDiagnostics returns the most recent recorded diagnostic snapshots.
func (d *Daemon) GetDiagnostics(ctx context.Context, in *envorcav1.GetDiagnosticsRequest) (*envorcav1.GetDiagnosticsResponse, error) {
	if d.state == nil {
		return nil, status.Error(codes.Unavailable, "diagnostic history not available")
	}
	limit := clampLimit(in.GetLimit())
	records, err := d.state.RecentDiagnostics(ctx, limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var out []*envorcav1.DiagnosticRecord
	for _, r := range records {
		out = append(out, &envorcav1.DiagnosticRecord{
			PerformedAt:   r.PerformedAt,
			OverallStatus: r.OverallStatus,
			Payload:       r.Payload,
		})
	}
	return &envorcav1.GetDiagnosticsResponse{Diagnostics: out}, nil
}

// GetRepairHistory returns the most recent recorded repair outcomes.
func (d *Daemon) GetRepairHistory(ctx context.Context, in *envorcav1.GetRepairHistoryRequest) (*envorcav1.GetRepairHistoryResponse, error) {
	if d.state == nil {
		return nil, status.Error(codes.Unavailable, "repair history not available")
	}
	limit := clampLimit(in.GetLimit())
	records, err := d.state.RecentRepairs(ctx, limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var out []*envorcav1.RepairRecord
	for _, r := range records {
		out = append(out, &envorcav1.RepairRecord{
			PerformedAt:  r.PerformedAt,
			ActionId:     r.ActionID,
			ComponentId:  r.ComponentID,
			Success:      r.Success,
			Result:       r.Result,
			Verification: r.Verification,
		})
	}
	return &envorcav1.GetRepairHistoryResponse{Repairs: out}, nil
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
// are rejected unless the request carries confirmed=true. The outcome is
// recorded in the repair history.
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
	if d.state != nil {
		if err := d.state.RecordRepair(ctx, in.ActionId, outcome.ComponentId, outcome.Result, outcome.Verification, outcome.Success); err != nil {
			d.log.Warn("record repair", "error", err)
		}
	}
	return outcome, nil
}

// StreamEvents replays the retained event buffer (when requested) and then
// streams live events until the client disconnects.
func (d *Daemon) StreamEvents(in *envorcav1.StreamEventsRequest, stream grpc.ServerStreamingServer[envorcav1.Event]) error {
	ch, unsubscribe := d.bus.Subscribe()
	defer unsubscribe()

	if in.GetReplay() {
		for _, ev := range d.bus.Snapshot() {
			if err := stream.Send(toProtoEvent(ev)); err != nil {
				return err
			}
		}
	}
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case ev := <-ch:
			if err := stream.Send(toProtoEvent(ev)); err != nil {
				return err
			}
		}
	}
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

// toProtoEvent converts an internal event into its gRPC representation.
func toProtoEvent(ev events.Event) *envorcav1.Event {
	out := &envorcav1.Event{
		Seq:       ev.Seq,
		Timestamp: ev.Timestamp.UTC().Format(time.RFC3339Nano),
		Level:     ev.Level,
		Component: ev.Component,
		Name:      ev.Name,
		Project:   ev.Project,
		Fields:    make(map[string]string, len(ev.Fields)),
	}
	for k, v := range ev.Fields {
		out.Fields[k] = fmt.Sprint(v)
	}
	return out
}

func clampLimit(limit int32) int {
	switch {
	case limit <= 0:
		return 20
	case limit > 500:
		return 500
	default:
		return int(limit)
	}
}