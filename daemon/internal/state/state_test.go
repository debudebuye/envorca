package state

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrateAndSettings(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "db", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal("re-running migrate must be idempotent:", err)
	}
	v, err := db.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Errorf("schema version = %d, want 2", v)
	}

	if s, err := db.GetSetting(ctx, "nope"); err != nil || s != "" {
		t.Errorf("GetSetting missing = %q, %v", s, err)
	}
	if err := db.SetSetting(ctx, "key", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting(ctx, "key", "v2"); err != nil {
		t.Fatal(err)
	}
	s, err := db.GetSetting(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	if s != "v2" {
		t.Errorf("GetSetting = %q, want v2", s)
	}
}