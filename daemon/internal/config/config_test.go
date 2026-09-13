package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := Defaults()
	if cfg.StateDir == "" {
		t.Error("default state dir is empty")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("default log level = %q, want info", cfg.LogLevel)
	}
	if cfg.Endpoint() == "" {
		t.Error("default endpoint is empty")
	}
}

func TestLoadDefaultsCreatesDirs(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{cfg.StateDir, cfg.RuntimeDir(), cfg.LogDir()} {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			t.Errorf("dir %s not created: %v", d, err)
		}
	}
	if cfg.DBPath() == "" {
		t.Error("empty DBPath")
	}
}

func TestLoadFileOverrides(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "state")
	file := filepath.Join(dir, "config.yaml")
	content := "state_dir: " + explicit + "\nlog_level: debug\n"
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StateDir != explicit {
		t.Errorf("StateDir = %q, want %q", cfg.StateDir, explicit)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}

func TestLoadMissingFileError(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml")); err == nil {
		t.Error("expected error for missing config file")
	}
}