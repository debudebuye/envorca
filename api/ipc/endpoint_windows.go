//go:build windows

package ipc

import (
	"os"
	"path/filepath"
)

func defaultStateDir() string {
	if app := os.Getenv("LOCALAPPDATA"); app != "" {
		return filepath.Join(app, "Runorka")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "Runorka")
	}
	return filepath.Join(home, "AppData", "Local", "Runorka")
}

func platformEndpoint(string) string {
	return `\\.\pipe\runorka`
}