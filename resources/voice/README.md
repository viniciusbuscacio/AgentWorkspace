# aw local voice transcription

Agent Workspace mirrors AW2's local voice pipeline:

1. Browser records audio (`MediaRecorder`).
2. Go backend writes the recording to a temp file.
3. `ffmpeg` converts it to 16 kHz mono WAV.
4. `whisper-cli` from whisper.cpp transcribes locally.
5. The transcript is inserted back into the chat composer.

No audio is uploaded to OpenAI/OpenRouter/cloud providers.

## Runtime lookup order

Binaries:

- `AW_WHISPER_CLI_PATH`
- `AW_FFMPEG_PATH`
- `resources/voice/bin/<platform-arch>/whisper-cli`
- `resources/voice/bin/<platform-arch>/ffmpeg`
- `resources/voice/bin/whisper-cli`
- `resources/voice/bin/ffmpeg`
- `/opt/homebrew/bin`, `/usr/local/bin`, then `PATH`

Models:

- `AW_WHISPER_MODEL_PATH`
- `~/Library/Application Support/aw/voice/models/` on macOS
- `resources/voice/models/`
- `~/AgentWorkspace2/resources/voice/models/` as an AW2 migration fallback

Default model: `tiny` (`ggml-tiny.bin`). If the selected model is missing, Agent Workspace downloads the GGML model from the official whisper.cpp HuggingFace repository and verifies SHA-1 when using the default URL.
