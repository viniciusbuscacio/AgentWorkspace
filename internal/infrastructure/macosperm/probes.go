// Package macosperm — probes run a cheap best-effort check for each TCC
// permission. They fire ONLY on explicit user request (the Test button); they
// never run on page load and never trigger new TCC prompts themselves.
package macosperm

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ProbeStatus is the result of a single-permission status check.
type ProbeStatus string

const (
	ProbeGranted ProbeStatus = "granted"
	ProbeDenied  ProbeStatus = "denied"
	ProbeUnknown ProbeStatus = "unknown"
)

// ProbeResult is the outcome of ProbeMacosPermission.
type ProbeResult struct {
	ID     string      `json:"id"`
	Status ProbeStatus `json:"status"`
	// Detail is a short human-readable message (empty on "unknown").
	Detail string `json:"detail,omitempty"`
}

// Probe runs the cheap status check for the given permission id. Returns
// ProbeUnknown on non-darwin platforms or for ids with no available probe.
// Each probe fires only when explicitly called — never on page load.
func Probe(id string) ProbeResult {
	if runtime.GOOS != "darwin" {
		return ProbeResult{ID: id, Status: ProbeUnknown}
	}
	switch id {
	case "microphone":
		return probeMicrophone(id)
	case "automation":
		return probeAutomation(id)
	case "files":
		return probeFiles(id)
	default:
		return ProbeResult{ID: id, Status: ProbeUnknown}
	}
}

// probeMicrophone tries to open the default audio capture device via ffmpeg
// for a very short duration. A TCC denial appears as an avfoundation error in
// stderr; a successful start (even if aborted immediately) indicates access.
func probeMicrophone(id string) ProbeResult {
	ffmpeg, err := resolveFfmpegPath()
	if err != nil {
		return ProbeResult{ID: id, Status: ProbeUnknown, Detail: "ffmpeg not found"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	// -t 0.1: record 0.1 s then stop; this is enough to trigger the TCC check.
	cmd := exec.CommandContext(ctx, ffmpeg,
		"-loglevel", "error",
		"-f", "avfoundation",
		"-i", ":default",
		"-t", "0.1",
		"-f", "null",
		os.DevNull,
	)
	cmd.Stderr = &stderr
	_ = cmd.Run() // non-zero exit is expected; we look at stderr content

	errOut := stderr.String()
	lower := strings.ToLower(errOut)
	if strings.Contains(lower, "unable to get capture device") ||
		strings.Contains(lower, "not permitted") ||
		strings.Contains(lower, "failed to create capture session") {
		return ProbeResult{ID: id, Status: ProbeDenied, Detail: "microphone access denied by macOS"}
	}
	if ctx.Err() != nil {
		// ffmpeg hung past the deadline — commonly a pending TCC prompt or a
		// busy/absent default input device. Say so instead of a silent unknown.
		return ProbeResult{ID: id, Status: ProbeUnknown, Detail: "audio capture did not respond within 3s — grant the permission in System Settings and test again"}
	}
	// ffmpeg ran without a device-access error → granted (or an
	// unrelated error; err is fine to return as unknown in that case).
	if errOut == "" || strings.Contains(lower, "silence") || strings.Contains(lower, "finalize") {
		return ProbeResult{ID: id, Status: ProbeGranted}
	}
	return ProbeResult{ID: id, Status: ProbeUnknown, Detail: strings.SplitN(strings.TrimSpace(errOut), "\n", 2)[0]}
}

// probeAutomation sends a harmless Apple Events query to System Events.
// Error code -1743 means "not allowed to send keystrokes" — Automation denied.
func probeAutomation(id string) ProbeResult {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "osascript", "-e",
		`tell application "System Events" to get count of every process`)
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return ProbeResult{ID: id, Status: ProbeGranted}
	}
	errOut := stderr.String()
	lower := strings.ToLower(errOut)
	if strings.Contains(errOut, "-1743") ||
		strings.Contains(lower, "not allowed to send keystrokes") ||
		strings.Contains(lower, "not allowed to interact with") {
		return ProbeResult{ID: id, Status: ProbeDenied, Detail: "Automation (Apple Events) denied by macOS"}
	}
	return ProbeResult{ID: id, Status: ProbeUnknown}
}

// probeFiles tries os.ReadDir on ~/Desktop. EPERM means the TCC Files and
// Folders (or Full Disk Access) permission is missing.
func probeFiles(id string) ProbeResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return ProbeResult{ID: id, Status: ProbeUnknown}
	}
	desktop := filepath.Join(home, "Desktop")
	_, readErr := os.ReadDir(desktop)
	if readErr == nil {
		return ProbeResult{ID: id, Status: ProbeGranted}
	}
	lower := strings.ToLower(readErr.Error())
	if strings.Contains(lower, "operation not permitted") ||
		strings.Contains(lower, "permission denied") {
		return ProbeResult{ID: id, Status: ProbeDenied, Detail: "Files and Folders access denied by macOS"}
	}
	return ProbeResult{ID: id, Status: ProbeUnknown}
}

// resolveFfmpegPath finds the ffmpeg binary using common installation paths.
func resolveFfmpegPath() (string, error) {
	if env := strings.TrimSpace(os.Getenv("AW_FFMPEG_PATH")); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env, nil
		}
	}
	candidates := []string{
		"/opt/homebrew/bin/ffmpeg",
		"/usr/local/bin/ffmpeg",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path, nil
	}
	return "", os.ErrNotExist
}
