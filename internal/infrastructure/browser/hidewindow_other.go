//go:build !windows

package browser

import "os/exec"

// hideConsole is a no-op off Windows: there is no stray console window to hide.
func hideConsole(*exec.Cmd) {}
