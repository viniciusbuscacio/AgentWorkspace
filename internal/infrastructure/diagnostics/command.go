// Package diagnostics is the OS-facing implementation of the native system
// diagnostics port (ports.DiagnosticsProbe). It collects read-only hardware,
// performance, storage, sensor, device/driver, and OS-log data once per call
// and normalizes it into the domain DTOs.
//
// Safety model (spec docs/plans/hardware-diagnostics-tools-spec.md §4/§8.3):
//   - The model never controls a command. Backends run a small set of FIXED
//     internal commands; every argument comes from a validated enum/int, never
//     from free-form model input.
//   - Every command is bounded by the caller's context timeout and a stdout cap.
//   - Output is parsed, then redacted, before leaving this package. Serials, MAC
//     addresses, hostnames, hardware/device IDs, driver paths, command lines, and
//     large raw log payloads are never returned.
package diagnostics

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"aw/internal/infrastructure/subprocess"
)

// maxCommandOutput caps how much stdout a fixed internal command may return
// before truncation. Diagnostics outputs are summaries, not data dumps.
const maxCommandOutput = 4 << 20 // 4 MiB

// runCommand executes a FIXED internal command with a hard output cap, bounded
// by ctx. name and args must be compile-time constants or values derived from
// validated enums/ints — never raw model input. It returns stdout; a non-zero
// exit is reported as an error carrying a short stderr tail.
func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	subprocess.HideConsoleWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &cappedWriter{buf: &stdout, limit: maxCommandOutput}
	cmd.Stderr = &cappedWriter{buf: &stderr, limit: 8 << 10}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return stdout.String(), ctx.Err()
		}
		tail := stderr.String()
		if len(tail) > 300 {
			tail = tail[len(tail)-300:]
		}
		return stdout.String(), fmt.Errorf("%s failed: %w (%s)", name, err, tail)
	}
	return stdout.String(), nil
}

// cappedWriter discards everything past limit so a runaway command cannot
// exhaust memory. It never errors (the command keeps running; we just stop
// keeping bytes), which is the right behavior for a bounded best-effort read.
type cappedWriter struct {
	buf   *bytes.Buffer
	limit int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if remaining := w.limit - w.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			w.buf.Write(p[:remaining])
		} else {
			w.buf.Write(p)
		}
	}
	return len(p), nil
}
