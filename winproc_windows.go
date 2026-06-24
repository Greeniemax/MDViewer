//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideProcessWindow prevents a child process from flashing a console window.
func hideProcessWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
