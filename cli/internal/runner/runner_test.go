package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindDaemonBinaryEnvOverride(t *testing.T) {
	p := filepath.Join(t.TempDir(), "envorcad.exe")
	if err := os.WriteFile(p, []byte("."), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENVORCA_DAEMON_BIN", p)
	got, err := FindDaemonBinary()
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Errorf("FindDaemonBinary = %q, want %q", got, p)
	}
}

func TestFindDaemonBinaryOnPATH(t *testing.T) {
	dir := t.TempDir()
	name := daemonName()
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, []byte("."), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENVORCA_DAEMON_BIN", "")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := FindDaemonBinary()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(full) {
		t.Errorf("FindDaemonBinary = %q, want %q", got, full)
	}
}

func TestFindDaemonBinaryMissing(t *testing.T) {
	// Clear override and PATH to guarantee no candidate exists.
	t.Setenv("ENVORCA_DAEMON_BIN", "")
	t.Setenv("PATH", t.TempDir())
	if _, err := FindDaemonBinary(); err == nil {
		t.Fatal("FindDaemonBinary should fail when no candidate exists")
	}
}

func TestOpenSpawnLogCreatesDir(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	t.Setenv("ENVORCA_STATE_DIR", stateDir)
	f, err := OpenSpawnLog()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if !strings.HasSuffix(f.Name(), "daemon.out.log") {
		t.Errorf("log path = %q, want ...daemon.out.log", f.Name())
	}
	if !strings.Contains(f.Name(), stateDir) {
		t.Errorf("log path = %q, want inside state dir %q", f.Name(), stateDir)
	}
	if fi, err := f.Stat(); err != nil || fi.Size() != 0 {
		t.Errorf("log stat = %v, %v", fi, err)
	}
}