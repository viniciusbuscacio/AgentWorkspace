//go:build windows

package subprocess

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// HideConsoleWindow keeps console-subsystem child processes from flashing a
// terminal window when aw is running as a GUI app on Windows.
func HideConsoleWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
