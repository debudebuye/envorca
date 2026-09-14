//go:build windows

package ipc

import (
	"os"
	"path/filepath"
)

func defaultStateDir() string {
	if app := os.Getenv("LOCALAPPDATA"); app != "" {
		return filepath.Join(app, "Envorka")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "Envorka")
	}
	return filepath.Join(home, "AppData", "Local", "Envorka")
}

func platformEndpoint(string) string {
	return `\\.\pipe\envorka`
}