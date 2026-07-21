package domain

// VoiceTranscriptionInput is the recorded audio plus the parameters needed to
// transcribe it locally. WorkspaceRoot anchors the search for the bundled
// ffmpeg/whisper binaries and models.
type VoiceTranscriptionInput struct {
	Data          []byte
	MIMEType      string
	Model         string
	WorkspaceRoot string
}

// VoiceTranscriptionResult is the transcribed text and the model that produced
// it. Raw/intermediate fields stay in infrastructure; the domain carries only
// what callers need.
type VoiceTranscriptionResult struct {
	Text  string
	Model string
}
