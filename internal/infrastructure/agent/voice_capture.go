package agent

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"aw/internal/infrastructure/subprocess"
)

// macosPermHint is appended to microphone-shaped errors so the user knows
// where to look. Must stay in sync with macosperm.PermHint.
const macosPermHint = "This looks like a macOS permission — see Settings › Security › macOS permissions."

// isMicrophonePermError reports whether an ffmpeg stderr message looks like
// an avfoundation device access failure (macOS Microphone TCC).
func isMicrophonePermError(msg string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "unable to get capture device") ||
		(strings.Contains(lower, "avfoundation") && strings.Contains(lower, "capture device")) ||
		strings.Contains(lower, "no audio was captured")
}

// wavHeaderSize is the byte length of a PCM WAV header with no samples.
const wavHeaderSize = 44

// FfmpegVoiceCapture records the default microphone with ffmpeg into a
// 16 kHz mono WAV ready for whisper-cli.
type FfmpegVoiceCapture struct {
	mu            sync.Mutex
	workspaceRoot string
	cmd           *exec.Cmd
	stderr        *bytes.Buffer
	tempDir       string
	wavPath       string
}

func NewFfmpegVoiceCapture(workspaceRoot string) *FfmpegVoiceCapture {
	return &FfmpegVoiceCapture{workspaceRoot: workspaceRoot}
}

func nativeCaptureInputArgs() ([]string, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("native voice capture is only implemented on macOS")
	}
	// ":default" selects the system default microphone; numeric indexes are
	// unstable (virtual devices like Teams Audio can occupy index 0).
	return []string{"-f", "avfoundation", "-i", ":default"}, nil
}

// micAccessCheck is a seam: tests run under a process with no TCC grant and a
// fake ffmpeg, so they stub this out.
var micAccessCheck = ensureMicAccess

func (c *FfmpegVoiceCapture) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil {
		return fmt.Errorf("a voice recording is already in progress")
	}
	inputArgs, err := nativeCaptureInputArgs()
	if err != nil {
		return err
	}
	// The app must hold the macOS microphone grant itself; ffmpeg alone gets
	// silently denied by TCC (no prompt). May block on the system prompt.
	if err := micAccessCheck(); err != nil {
		return err
	}
	ffmpegPath, err := resolveFfmpeg(c.workspaceRoot)
	if err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp("", "aw-voice-capture-")
	if err != nil {
		return err
	}
	wavPath := filepath.Join(tempDir, "capture.wav")
	args := append([]string{"-hide_banner", "-loglevel", "error", "-y"}, inputArgs...)
	args = append(args, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wavPath)
	cmd := exec.Command(ffmpegPath, args...)
	subprocess.HideConsoleWindow(cmd)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(tempDir)
		return fmt.Errorf("could not start microphone recording: %w", err)
	}
	c.cmd = cmd
	c.stderr = stderr
	c.tempDir = tempDir
	c.wavPath = wavPath
	return nil
}

func (c *FfmpegVoiceCapture) Stop() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopLocked(true)
}

func (c *FfmpegVoiceCapture) Cancel() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd == nil {
		return nil
	}
	_, err := c.stopLocked(false)
	return err
}

func (c *FfmpegVoiceCapture) stopLocked(keep bool) ([]byte, error) {
	if c.cmd == nil {
		return nil, fmt.Errorf("no voice recording is in progress")
	}
	cmd, stderr, tempDir, wavPath := c.cmd, c.stderr, c.tempDir, c.wavPath
	c.cmd, c.stderr, c.tempDir, c.wavPath = nil, nil, "", ""
	defer func() { _ = os.RemoveAll(tempDir) }()

	// SIGINT lets ffmpeg finalize the WAV header; exiting non-zero is normal.
	_ = cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
	if !keep {
		return nil, nil
	}
	data, err := os.ReadFile(wavPath)
	if err != nil || len(data) <= wavHeaderSize {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = "no audio was captured"
		}
		errMsg := fmt.Sprintf("voice recording failed: %s", message)
		if isMicrophonePermError(message) {
			errMsg = errMsg + " — " + macosPermHint
		}
		return nil, fmt.Errorf("%s", errMsg) //nolint:goerr113
	}
	return data, nil
}
