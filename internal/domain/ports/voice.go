package ports

import (
	"context"

	"aw/internal/domain"
)

// VoiceCapture records microphone audio natively (outside the WebView) and
// returns the captured WAV bytes on stop. Needed because the Wails WKWebView
// on macOS exposes no getUserMedia (wails:// is not a secure context).
type VoiceCapture interface {
	Start() error
	Stop() ([]byte, error)
	Cancel() error
}

// VoiceTranscriber turns recorded audio into text. The implementation drives
// external binaries (ffmpeg + whisper), so the interface layer must reach it
// through this port instead of calling the infrastructure helper directly.
type VoiceTranscriber interface {
	Transcribe(ctx context.Context, input domain.VoiceTranscriptionInput) (domain.VoiceTranscriptionResult, error)
}
