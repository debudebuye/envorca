//go:build !windows

package ipc

import (
	"os"
	"path/filepath"
)

func defaultStateDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "runorka")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "runorka")
	}
	return filepath.Join(home, ".local", "share", "runorka")
}

func platformEndpoint(stateDir string) string {
	return filepath.Join(stateDir, "run", "runorka.sock")
}