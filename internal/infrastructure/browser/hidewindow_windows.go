//go:build windows

package browser

import (
	"os/exec"
	"syscall"
)

// createNoWindow is the Windows CREATE_NO_WINDOW process creation flag. It
// keeps console helpers (tasklist, taskkill) from flashing an ugly terminal
// window when the GUI app shells out to them.
const createNoWindow = 0x08000000

// hideConsole makes a child process run without popping a console window. On
// Windows a GUI (windowsgui) app still spawns a visible console for every
// console-subsystem child unless this is set.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
