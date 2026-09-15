package state

import (
	"context"
	"fmt"
)

// Diagnostic is one recorded status snapshot, newest first.
type Diagnostic struct {
	PerformedAt   string
	OverallStatus string
	Payload       string
}

// Repair is one recorded repair outcome, newest first.
type Repair struct {
	PerformedAt  string
	ActionID     string
	ComponentID  string
	Success      bool
	Result       string
	Verification string
}

// Retention bounds for history rows. Records are pruned on insert so the
// database cannot grow without bound.
const (
	keepDiagnostics = 200
	keepRepairs     = 1000
)

// RecordDiagnostic appends a status snapshot to the diagnostics history and
// prunes records beyond keepDiagnostics.
func (d *DB) RecordDiagnostic(ctx context.Context, overallStatus, payload string) error {
	if _, err := d.db.ExecContext(ctx,
		`INSERT INTO diagnostics(overall_status, payload) VALUES (?, ?)`, overallStatus, payload); err != nil {
		return fmt.Errorf("record diagnostic: %w", err)
	}
	if _, err := d.db.ExecContext(ctx,
		`DELETE FROM diagnostics WHERE id NOT IN (SELECT id FROM diagnostics ORDER BY id DESC LIMIT ?)`, keepDiagnostics); err != nil {
		return fmt.Errorf("prune diagnostics: %w", err)
	}
	return nil
}

// RecentDiagnostics returns the most recent diagnostics, newest first.
func (d *DB) RecentDiagnostics(ctx context.Context, limit int) ([]Diagnostic, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := d.db.QueryContext(ctx,
		`SELECT performed_at, overall_status, payload FROM diagnostics ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read diagnostics: %w", err)
	}
	defer rows.Close()
	var out []Diagnostic
	for rows.Next() {
		var r Diagnostic
		if err := rows.Scan(&r.PerformedAt, &r.OverallStatus, &r.Payload); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecordRepair appends a repair outcome to the repair history and prunes
// records beyond keepRepairs.
func (d *DB) RecordRepair(ctx context.Context, actionID, componentID, result, verification string, success bool) error {
	if _, err := d.db.ExecContext(ctx,
		`INSERT INTO repair_history(action_id, component_id, success, result, verification) VALUES (?, ?, ?, ?, ?)`,
		actionID, componentID, success, result, verification); err != nil {
		return fmt.Errorf("record repair: %w", err)
	}
	if _, err := d.db.ExecContext(ctx,
		`DELETE FROM repair_history WHERE id NOT IN (SELECT id FROM repair_history ORDER BY id DESC LIMIT ?)`, keepRepairs); err != nil {
		return fmt.Errorf("prune repair history: %w", err)
	}
	return nil
}

// RecentRepairs returns the most recent repair outcomes, newest first.
func (d *DB) RecentRepairs(ctx context.Context, limit int) ([]Repair, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := d.db.QueryContext(ctx,
		`SELECT performed_at, action_id, component_id, success, result, verification FROM repair_history ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read repair history: %w", err)
	}
	defer rows.Close()
	var out []Repair
	for rows.Next() {
		var r Repair
		var success int
		if err := rows.Scan(&r.PerformedAt, &r.ActionID, &r.ComponentID, &success, &r.Result, &r.Verification); err != nil {
			return nil, err
		}
		r.Success = success != 0
		out = append(out, r)
	}
	return out, rows.Err()
}