//go:build windows

package ipc

import (
	"os"
	"path/filepath"
)

func defaultStateDir() string {
	if app := os.Getenv("LOCALAPPDATA"); app != "" {
		return filepath.Join(app, "Envorca")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "Envorca")
	}
	return filepath.Join(home, "AppData", "Local", "Envorca")
}

func platformEndpoint(string) string {
	return `\\.\pipe\envorca`
}