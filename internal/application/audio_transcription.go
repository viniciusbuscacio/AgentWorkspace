package application

import (
	"strings"

	"aw/internal/domain/ports"
)

const defaultLocalVoiceModel = "tiny"
const voiceTranscriptionModelSecret = "_voice_transcription_model"

// ResolveLocalVoiceModel mirrors AW2: the optional vault secret
// _voice_transcription_model selects the local whisper.cpp GGML model, defaulting
// to tiny for fast desktop dictation.
func ResolveLocalVoiceModel(store ports.ProviderSecretStore) string {
	if store == nil {
		return defaultLocalVoiceModel
	}
	value, exists, err := store.GetSecret(voiceTranscriptionModelSecret)
	if err != nil || !exists {
		return defaultLocalVoiceModel
	}
	model := strings.TrimSpace(value)
	if model == "" {
		return defaultLocalVoiceModel
	}
	return model
}
