//go:build !windows

package wsl

import "os/exec"

// hideConsole is a no-op off Windows; only wsl.exe running under Windows
// can open an extra console window.
func hideConsole(*exec.Cmd) {}