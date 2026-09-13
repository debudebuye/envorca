package runner

// Package runner locates and spawns the daemon process on behalf of the CLI.
// Spawning is the only process-management logic the CLI owns; everything
// else is delegated to the daemon API.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"runorka.dev/runorka/api/ipc"
)

func daemonName() string {
	if runtime.GOOS == "windows" {
		return "runorkad.exe"
	}
	return "runorkad"
}

// FindDaemonBinary returns the path to the daemon binary: the RUNORKA_DAEMON_BIN
// override, the CLI's own directory, or PATH.
func FindDaemonBinary() (string, error) {
	if p := os.Getenv("RUNORKA_DAEMON_BIN"); p != "" {
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), daemonName())
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, nil
		}
	}
	if p, err := exec.LookPath(daemonName()); err == nil {
		return p, nil
	}
	return "", errors.New("runorkad binary not found; set RUNORKA_DAEMON_BIN or put runorkad on PATH")
}

// OpenSpawnLog opens the file that captures daemon output at startup.
func OpenSpawnLog() (*os.File, error) {
	dir := filepath.Join(ipc.DefaultStateDir(), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, "daemon.out.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}
