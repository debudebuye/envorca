package logging

import (
	"log/slog"
	"testing"
)

func TestLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	for name, want := range cases {
		got, err := Level(name)
		if err != nil {
			t.Errorf("Level(%q): %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("Level(%q) = %v, want %v", name, got, want)
		}
	}
	if _, err := Level("bogus"); err == nil {
		t.Error("expected error for unknown level")
	}
}