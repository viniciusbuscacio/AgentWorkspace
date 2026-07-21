//go:build linux || darwin

package diagnostics

import (
	"context"
	"os/exec"
	"runtime"
)

func numCPU() int         { return runtime.NumCPU() }
func runtimeArch() string { return runtime.GOARCH }

// runCommandIfPresent runs a fixed command only when its binary is on PATH,
// returning ("", nil) when it is absent so callers degrade to honest partial
// results instead of erroring.
func runCommandIfPresent(ctx context.Context, name string, args ...string) (string, error) {
	if _, err := exec.LookPath(name); err != nil {
		return "", nil
	}
	return runCommand(ctx, name, args...)
}
