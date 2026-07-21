package application

import (
	"context"
	"errors"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

var errVoiceTranscriberUnavailable = errors.New("voice transcriber is not available")

// VoiceTranscriptionDeps bundles the ports a voice transcription needs: the
// transcriber (ffmpeg + whisper), the secret store (to resolve the local model
// and the active provider used for cleanup) and the runtime (LLM cleanup pass).
type VoiceTranscriptionDeps struct {
	Transcriber ports.VoiceTranscriber
	Store       ports.ProviderSecretStore
	Runtime     ports.ChatTitleRuntime
	// Memory (optional) supplies the user memory doc so the cleanup pass
	// knows the speaker's preferred language; nil falls back to English (US).
	Memory ports.UserMemoryDocStore
}

// TranscribeAndCleanVoice resolves the local whisper model, transcribes the
// recorded audio through the transcriber port and runs the best-effort LLM
// cleanup pass over the raw text. Cleanup failures leave the transcript
// unchanged.
func TranscribeAndCleanVoice(ctx context.Context, deps VoiceTranscriptionDeps, data []byte, mimeType string, workspaceRoot string) (domain.VoiceTranscriptionResult, error) {
	if deps.Transcriber == nil {
		return domain.VoiceTranscriptionResult{}, errVoiceTranscriberUnavailable
	}
	model := ResolveLocalVoiceModel(deps.Store)
	result, err := deps.Transcriber.Transcribe(ctx, domain.VoiceTranscriptionInput{
		Data:          data,
		MIMEType:      mimeType,
		Model:         model,
		WorkspaceRoot: workspaceRoot,
	})
	if err != nil {
		return domain.VoiceTranscriptionResult{}, err
	}
	result.Text = cleanVoiceTranscriptBestEffort(ctx, deps, result.Text)
	return result, nil
}

// cleanVoiceTranscriptBestEffort runs the AW2-style LLM cleanup over a raw
// whisper transcript using the active provider. Without a ready runtime/store
// or provider (or on any failure) the raw text is returned unchanged.
func cleanVoiceTranscriptBestEffort(ctx context.Context, deps VoiceTranscriptionDeps, rawText string) string {
	if deps.Runtime == nil || deps.Store == nil {
		return rawText
	}
	cfg, err := ResolveProviderRuntimeConfig(deps.Store)
	if err != nil {
		return rawText
	}
	language := DefaultVoiceLanguage
	if deps.Memory != nil {
		if doc, err := GetUserMemoryDoc(deps.Memory); err == nil {
			language = PreferredLanguageFromMemory(doc.Content)
		}
	}
	if cleaned, ok := CleanVoiceTranscript(ctx, deps.Runtime, ModelConfigFromProviderRuntimeConfig(cfg), rawText, language); ok {
		return cleaned
	}
	return rawText
}
