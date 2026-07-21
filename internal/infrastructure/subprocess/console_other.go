//go:build !windows

package subprocess

import "os/exec"

// HideConsoleWindow is a no-op outside Windows.
func HideConsoleWindow(*exec.Cmd) {}
