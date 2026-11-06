//go:build !windows

package mdviewer

import "os/exec"

// hideProcessWindow is a no-op on platforms without Windows console windows.
func hideProcessWindow(cmd *exec.Cmd) {}
