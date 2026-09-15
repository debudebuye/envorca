//go:build windows

package wsl

import (
	"os/exec"
	"syscall"
)

// hideConsole prevents a console window from flashing when the detached
// daemon spawns console applications like wsl.exe.
func hideConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}