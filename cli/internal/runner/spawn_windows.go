//go:build windows

package runner

import (
	"os"
	"os/exec"
	"syscall"
)

// Spawn starts the daemon detached from the CLI's console. DETACHED_PROCESS
// keeps the daemon console-free and independent of the CLI session.
func Spawn(bin string, logFile *os.File) error {
	cmd := exec.Command(bin)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008, // DETACHED_PROCESS
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
