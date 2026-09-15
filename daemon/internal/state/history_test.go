package state

import (
	"context"
	"path/filepath"
	"testing"
)

func TestHistoryMigration(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	v, err := db.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Errorf("schema version = %d, want 2", v)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal("re-migrate must be idempotent:", err)
	}
}

func TestDiagnosticsRoundTrip(t *testing.T) {
	db := newMigrated(t)
	ctx := context.Background()
	if err := db.RecordDiagnostic(ctx, "HEALTHY", `{"overall_status":"HEALTHY"}`); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordDiagnostic(ctx, "WARNING", `{"overall_status":"WARNING"}`); err != nil {
		t.Fatal(err)
	}
	recs, err := db.RecentDiagnostics(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d diagnostics, want 2", len(recs))
	}
	if recs[0].OverallStatus != "WARNING" {
		t.Errorf("newest record = %q, want WARNING (newest first)", recs[0].OverallStatus)
	}
	if recs[0].Payload == "" {
		t.Error("payload should be stored")
	}
	if recs[1].OverallStatus != "HEALTHY" {
		t.Errorf("oldest record = %q, want HEALTHY", recs[1].OverallStatus)
	}
}

func TestDiagnosticsPrunesToRetention(t *testing.T) {
	db := newMigrated(t)
	ctx := context.Background()
	for i := 0; i < keepDiagnostics+25; i++ {
		if err := db.RecordDiagnostic(ctx, "HEALTHY", "{}"); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := db.RecentDiagnostics(ctx, keepDiagnostics+50)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != keepDiagnostics {
		t.Errorf("retained %d diagnostics, want %d", len(recs), keepDiagnostics)
	}
}

func TestRepairHistoryRoundTrip(t *testing.T) {
	db := newMigrated(t)
	ctx := context.Background()
	if err := db.RecordRepair(ctx, "wsl.start", "wsl", "ok", "verified", true); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordRepair(ctx, "wsl.restart", "wsl", "execute failed", "", false); err != nil {
		t.Fatal(err)
	}
	recs, err := db.RecentRepairs(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d repairs, want 2", len(recs))
	}
	first, second := recs[0], recs[1]
	if first.ActionID != "wsl.restart" || first.Success || first.Result != "execute failed" {
		t.Errorf("newest repair wrong: %+v", first)
	}
	if second.ActionID != "wsl.start" || !second.Success || second.Verification != "verified" {
		t.Errorf("oldest repair wrong: %+v", second)
	}
}

func TestHistoryEmptyWhenFresh(t *testing.T) {
	db := newMigrated(t)
	ctx := context.Background()
	if recs, err := db.RecentDiagnostics(ctx, 10); err != nil || len(recs) != 0 {
		t.Errorf("fresh diagnostics = %v, %v", len(recs), err)
	}
	if recs, err := db.RecentRepairs(ctx, 10); err != nil || len(recs) != 0 {
		t.Errorf("fresh repairs = %v, %v", len(recs), err)
	}
	if recs, _ := db.RecentDiagnostics(ctx, 0); recs != nil {
		t.Error("limit 0 should return nil slice")
	}
}

func newMigrated(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db
}