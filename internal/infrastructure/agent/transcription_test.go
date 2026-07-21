package agent

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTranscribeLocalVoiceUsesFfmpegAndWhisperCli(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake executables are unix-only")
	}
	tempDir := t.TempDir()
	ffmpegPath := filepath.Join(tempDir, "ffmpeg")
	whisperPath := filepath.Join(tempDir, "whisper-cli")
	modelPath := filepath.Join(tempDir, "ggml-tiny.bin")
	if err := os.WriteFile(modelPath, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, ffmpegPath, `#!/bin/sh
out=""
for arg in "$@"; do out="$arg"; done
printf wav > "$out"
`)
	writeExecutable(t, whisperPath, `#!/bin/sh
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-of" ]; then
    shift
    out="$1"
  fi
  shift || true
done
printf ' [00:00:00.000 --> 00:00:01.000] olá mundo  ' > "${out}.txt"
`)
	t.Setenv("AW_FFMPEG_PATH", ffmpegPath)
	t.Setenv("AW_WHISPER_CLI_PATH", whisperPath)
	t.Setenv("AW_WHISPER_MODEL_PATH", modelPath)

	result, err := TranscribeLocalVoice(context.Background(), LocalVoiceTranscriptionInput{
		Data:     []byte("audio"),
		MIMEType: "audio/webm",
		Model:    "tiny",
	})
	if err != nil {
		t.Fatalf("TranscribeLocalVoice() error = %v", err)
	}
	if result.Text != "olá mundo" || result.RawText != "olá mundo" || result.Model != "tiny" || result.Corrected {
		t.Fatalf("result = %+v", result)
	}
}

func TestTranscribeLocalVoiceRejectsEmptyAudio(t *testing.T) {
	_, err := TranscribeLocalVoice(context.Background(), LocalVoiceTranscriptionInput{})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error = %v, want empty audio error", err)
	}
}

func TestNormalizeWhisperOutputRemovesTimestamps(t *testing.T) {
	got := normalizeWhisperOutput("[00:00:00.000 --> 00:00:01.000] oi\n[00:00:01.000 --> 00:00:02.000] mundo")
	if got != "oi mundo" {
		t.Fatalf("normalizeWhisperOutput() = %q", got)
	}
}

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
