//go:build !windows

package ipc

import (
	"os"
	"path/filepath"
)

func defaultStateDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "envorca")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "envorca")
	}
	return filepath.Join(home, ".local", "share", "envorca")
}

func platformEndpoint(stateDir string) string {
	return filepath.Join(stateDir, "run", "envorca.sock")
}