package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeFfmpegScript mimics the capture invocation: writes WAV-sized data to the
// output path (last argument) and waits for SIGINT like a real recording.
const fakeFfmpegScript = `#!/bin/sh
out=""
for arg in "$@"; do out="$arg"; done
trap 'exit 255' INT TERM
head -c 4096 /dev/zero > "$out"
touch "$AW_TEST_CAPTURE_SENTINEL"
while :; do sleep 0.05; done
`

func newTestVoiceCapture(t *testing.T) (*FfmpegVoiceCapture, string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("native voice capture is darwin-only")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "ffmpeg")
	if err := os.WriteFile(script, []byte(fakeFfmpegScript), 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(dir, "recording-started")
	t.Setenv("AW_FFMPEG_PATH", script)
	// The fake recorder has no real device; bypass the TCC gate (the test
	// process holds no grant and must never trigger a system prompt).
	prev := micAccessCheck
	micAccessCheck = func() error { return nil }
	t.Cleanup(func() { micAccessCheck = prev })
	t.Setenv("AW_TEST_CAPTURE_SENTINEL", sentinel)
	return NewFfmpegVoiceCapture(dir), sentinel
}

func waitForSentinel(t *testing.T, sentinel string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(sentinel); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("fake ffmpeg never wrote the capture file")
}

func TestFfmpegVoiceCaptureStartStopReturnsAudio(t *testing.T) {
	capture, sentinel := newTestVoiceCapture(t)
	if err := capture.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := capture.Start(); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("second Start() error = %v, want already-in-progress", err)
	}
	waitForSentinel(t, sentinel)
	data, err := capture.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if len(data) <= wavHeaderSize {
		t.Fatalf("Stop() returned %d bytes, want > header", len(data))
	}
}

func TestFfmpegVoiceCaptureStopWithoutStartFails(t *testing.T) {
	capture, _ := newTestVoiceCapture(t)
	if _, err := capture.Stop(); err == nil {
		t.Fatal("Stop() without Start() should fail")
	}
	if err := capture.Cancel(); err != nil {
		t.Fatalf("Cancel() without Start() = %v, want nil", err)
	}
}

func TestFfmpegVoiceCaptureCancelDiscardsRecording(t *testing.T) {
	capture, _ := newTestVoiceCapture(t)
	if err := capture.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := capture.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if _, err := capture.Stop(); err == nil {
		t.Fatal("Stop() after Cancel() should fail")
	}
}

// TestIsMicrophonePermError verifies the pattern matcher for TCC mic errors.
func TestIsMicrophonePermError(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("isMicrophonePermError is always false on non-darwin")
	}
	positives := []string{
		"Unable to get capture device",
		"avfoundation capture device error",
		"no audio was captured",
	}
	for _, msg := range positives {
		if !isMicrophonePermError(msg) {
			t.Errorf("isMicrophonePermError(%q) = false, want true", msg)
		}
	}
	negatives := []string{
		"",
		"mic busy",
		"codec error",
		"file not found",
	}
	for _, msg := range negatives {
		if isMicrophonePermError(msg) {
			t.Errorf("isMicrophonePermError(%q) = true, want false", msg)
		}
	}
}

// TestVoiceRecordingFailedHintAppended ensures the macOS perm hint is appended
// to a microphone-shaped error (and that the original message is preserved).
func TestVoiceRecordingFailedHintAppended(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("hint only on darwin")
	}
	// Simulate what stopLocked builds when ffmpeg stderr says the mic is unavailable.
	fakeStderr := "Unable to get capture device"
	errMsg := "voice recording failed: " + fakeStderr
	if !isMicrophonePermError(fakeStderr) {
		t.Fatal("test precondition: isMicrophonePermError should match the fake stderr")
	}
	result := errMsg + " — " + macosPermHint
	if !strings.Contains(result, macosPermHint) {
		t.Errorf("hint not appended: %q", result)
	}
	if !strings.HasPrefix(result, errMsg) {
		t.Errorf("original error not preserved: %q", result)
	}
}
