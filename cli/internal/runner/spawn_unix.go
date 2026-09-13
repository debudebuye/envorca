//go:build !windows

package runner

import (
	"os"
	"os/exec"
	"syscall"
)

// Spawn starts the daemon detached from the CLI's process group.
func Spawn(bin string, logFile *os.File) error {
	cmd := exec.Command(bin)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
