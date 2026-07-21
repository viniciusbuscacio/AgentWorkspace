package agent

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/infrastructure/subprocess"
)

// VoiceTranscriber is the ports.VoiceTranscriber adapter over the local
// ffmpeg + whisper pipeline. The zero value is ready to use.
type VoiceTranscriber struct{}

// NewVoiceTranscriber returns the local voice transcription adapter.
func NewVoiceTranscriber() VoiceTranscriber { return VoiceTranscriber{} }

// Transcribe implements ports.VoiceTranscriber by delegating to the local
// pipeline and mapping the result onto domain types.
func (VoiceTranscriber) Transcribe(ctx context.Context, input domain.VoiceTranscriptionInput) (domain.VoiceTranscriptionResult, error) {
	result, err := TranscribeLocalVoice(ctx, LocalVoiceTranscriptionInput{
		Data:          input.Data,
		MIMEType:      input.MIMEType,
		Model:         input.Model,
		WorkspaceRoot: input.WorkspaceRoot,
	})
	if err != nil {
		return domain.VoiceTranscriptionResult{}, err
	}
	return domain.VoiceTranscriptionResult{Text: result.Text, Model: result.Model}, nil
}

const maxLocalVoiceAudioBytes = 25 * 1024 * 1024
const whisperCppModelBaseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main"

type LocalVoiceTranscriptionInput struct {
	Data          []byte
	MIMEType      string
	Model         string
	WorkspaceRoot string
}

type LocalVoiceTranscriptionResult struct {
	Text      string
	RawText   string
	Corrected bool
	Model     string
}

type voiceModelFile struct {
	FileName    string
	DownloadURL string
	SHA1        string
}

var voiceModelFiles = map[string]voiceModelFile{
	"tiny": {
		FileName:    "ggml-tiny.bin",
		DownloadURL: whisperCppModelBaseURL + "/ggml-tiny.bin",
		SHA1:        "bd577a113a864445d4c299885e0cb97d4ba92b5f",
	},
	"base": {
		FileName:    "ggml-base.bin",
		DownloadURL: whisperCppModelBaseURL + "/ggml-base.bin",
		SHA1:        "465707469ff3a37a2b9b8d8f89f2f99de7299dac",
	},
	"small": {
		FileName:    "ggml-small.bin",
		DownloadURL: whisperCppModelBaseURL + "/ggml-small.bin",
		SHA1:        "55356645c2b361a969dfd0ef2c5a50d530afd8d5",
	},
	"large-v3-turbo-q5_0": {
		FileName:    "ggml-large-v3-turbo-q5_0.bin",
		DownloadURL: whisperCppModelBaseURL + "/ggml-large-v3-turbo-q5_0.bin",
		SHA1:        "e050f7970618a659205450ad97eb95a18d69c9ee",
	},
}

// TranscribeLocalVoice mirrors AW2's local voice pipeline:
// recorded audio -> ffmpeg WAV 16k mono -> whisper-cli -> normalized text.
func TranscribeLocalVoice(ctx context.Context, input LocalVoiceTranscriptionInput) (LocalVoiceTranscriptionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = "tiny"
	}
	if len(input.Data) == 0 {
		return LocalVoiceTranscriptionResult{}, fmt.Errorf("recorded audio was empty")
	}
	if len(input.Data) > maxLocalVoiceAudioBytes {
		return LocalVoiceTranscriptionResult{}, fmt.Errorf("recorded audio is too large to transcribe")
	}

	whisperCli, err := resolveWhisperCli(input.WorkspaceRoot)
	if err != nil {
		return LocalVoiceTranscriptionResult{}, err
	}
	ffmpegPath, err := resolveFfmpeg(input.WorkspaceRoot)
	if err != nil {
		return LocalVoiceTranscriptionResult{}, err
	}
	modelPath, err := resolveVoiceModelPath(ctx, input.WorkspaceRoot, model)
	if err != nil {
		return LocalVoiceTranscriptionResult{}, err
	}

	tempDir, err := os.MkdirTemp("", "aw-voice-")
	if err != nil {
		return LocalVoiceTranscriptionResult{}, err
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	inputPath := filepath.Join(tempDir, "input."+extensionForAudioMIMEType(input.MIMEType))
	wavPath := filepath.Join(tempDir, "input.wav")
	outputBase := filepath.Join(tempDir, "transcript")
	if err := os.WriteFile(inputPath, input.Data, 0o600); err != nil {
		return LocalVoiceTranscriptionResult{}, err
	}
	if err := convertAudioToWav(ctx, ffmpegPath, inputPath, wavPath); err != nil {
		return LocalVoiceTranscriptionResult{}, err
	}
	stdout, err := runWhisperCli(ctx, whisperCli, modelPath, wavPath, outputBase)
	if err != nil {
		return LocalVoiceTranscriptionResult{}, err
	}
	textBytes, readErr := os.ReadFile(outputBase + ".txt")
	if readErr != nil {
		textBytes = []byte(stdout)
	}
	rawText := normalizeWhisperOutput(string(textBytes))
	if rawText == "" {
		return LocalVoiceTranscriptionResult{}, fmt.Errorf("local voice transcription returned empty text")
	}
	return LocalVoiceTranscriptionResult{Text: rawText, RawText: rawText, Corrected: false, Model: model}, nil
}

func extensionForAudioMIMEType(mimeType string) string {
	mimeType = strings.ToLower(mimeType)
	switch {
	case strings.Contains(mimeType, "mp4"):
		return "mp4"
	case strings.Contains(mimeType, "mpeg"), strings.Contains(mimeType, "mp3"):
		return "mp3"
	case strings.Contains(mimeType, "wav"):
		return "wav"
	case strings.Contains(mimeType, "ogg"):
		return "ogg"
	default:
		return "webm"
	}
}

func voiceRootCandidates(workspaceRoot string) []string {
	var candidates []string
	if env := strings.TrimSpace(os.Getenv("AW_VOICE_RESOURCES_DIR")); env != "" {
		candidates = append(candidates, env)
	}
	if workspaceRoot != "" {
		candidates = append(candidates, filepath.Join(workspaceRoot, "resources", "voice"))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "resources", "voice"))
	}
	// AW2 compatibility/migration fallback: reuse the already-provisioned local
	// whisper.cpp assets when aw is being developed beside AgentWorkspace2.
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, "AgentWorkspace2", "resources", "voice"))
	}
	return uniqueStrings(candidates)
}

func platformBinDir() string {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		return "darwin-arm64"
	}
	return runtime.GOOS + "-" + runtime.GOARCH
}

func resolveWhisperCli(workspaceRoot string) (string, error) {
	if env := strings.TrimSpace(os.Getenv("AW_WHISPER_CLI_PATH")); env != "" && executableExists(env) {
		return env, nil
	}
	var candidates []string
	for _, root := range voiceRootCandidates(workspaceRoot) {
		candidates = append(candidates,
			filepath.Join(root, "bin", platformBinDir(), executableName("whisper-cli")),
			filepath.Join(root, "bin", executableName("whisper-cli")),
		)
	}
	candidates = append(candidates,
		"/opt/homebrew/bin/whisper-cli",
		"/usr/local/bin/whisper-cli",
	)
	if path, err := exec.LookPath("whisper-cli"); err == nil {
		candidates = append(candidates, path)
	}
	for _, candidate := range uniqueStrings(candidates) {
		if executableExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("local voice engine is not installed. Expected whisper-cli under resources/voice/bin or in PATH")
}

func resolveFfmpeg(workspaceRoot string) (string, error) {
	if env := strings.TrimSpace(os.Getenv("AW_FFMPEG_PATH")); env != "" && executableExists(env) {
		return env, nil
	}
	var candidates []string
	for _, root := range voiceRootCandidates(workspaceRoot) {
		candidates = append(candidates,
			filepath.Join(root, "bin", platformBinDir(), executableName("ffmpeg")),
			filepath.Join(root, "bin", executableName("ffmpeg")),
		)
	}
	candidates = append(candidates,
		"/opt/homebrew/bin/ffmpeg",
		"/usr/local/bin/ffmpeg",
	)
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		candidates = append(candidates, path)
	}
	for _, candidate := range uniqueStrings(candidates) {
		if executableExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("ffmpeg is required for local voice transcription")
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func executableExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func userVoiceModelDir() string {
	if base, err := os.UserConfigDir(); err == nil && base != "" {
		return filepath.Join(base, "aw", "voice", "models")
	}
	return filepath.Join(os.TempDir(), "aw-voice", "models")
}

func resolveVoiceModelPath(ctx context.Context, workspaceRoot string, model string) (string, error) {
	manifest, ok := voiceModelFiles[model]
	if !ok {
		manifest = voiceModelFiles["tiny"]
	}
	if env := strings.TrimSpace(os.Getenv("AW_WHISPER_MODEL_PATH")); env != "" && fileExists(env) {
		return env, nil
	}
	var candidates []string
	candidates = append(candidates, filepath.Join(userVoiceModelDir(), manifest.FileName))
	for _, root := range voiceRootCandidates(workspaceRoot) {
		candidates = append(candidates, filepath.Join(root, "models", manifest.FileName))
	}
	for _, candidate := range uniqueStrings(candidates) {
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return downloadVoiceModel(ctx, model, manifest)
}

func downloadVoiceModel(ctx context.Context, model string, manifest voiceModelFile) (string, error) {
	destination := filepath.Join(userVoiceModelDir(), manifest.FileName)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", err
	}
	downloadURL := strings.TrimSpace(os.Getenv("AW_WHISPER_MODEL_URL"))
	if downloadURL == "" {
		downloadURL = manifest.DownloadURL
	}
	tmp := destination + ".download"
	if err := downloadFile(ctx, downloadURL, tmp, 0); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("could not download local voice model %q: %w", model, err)
	}
	if manifest.SHA1 != "" && os.Getenv("AW_WHISPER_MODEL_URL") == "" {
		actual, err := sha1File(tmp)
		if err != nil {
			_ = os.Remove(tmp)
			return "", err
		}
		if actual != manifest.SHA1 {
			_ = os.Remove(tmp)
			return "", fmt.Errorf("downloaded voice model checksum mismatch for %q", model)
		}
	}
	if err := os.Rename(tmp, destination); err != nil {
		return "", err
	}
	return destination, nil
}

func downloadFile(ctx context.Context, url string, destination string, redirects int) error {
	if redirects > 5 {
		return fmt.Errorf("too many redirects while downloading %s", url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 && resp.Header.Get("Location") != "" {
		next := resp.Header.Get("Location")
		if parsed, err := resp.Request.URL.Parse(next); err == nil {
			next = parsed.String()
		}
		return downloadFile(ctx, next, destination, redirects+1)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with HTTP %d", resp.StatusCode)
	}
	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func sha1File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:]), nil
}

func convertAudioToWav(ctx context.Context, ffmpegPath, inputPath, outputPath string) error {
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-i", inputPath,
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		outputPath,
	)
	subprocess.HideConsoleWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func runWhisperCli(ctx context.Context, whisperCli, modelPath, wavPath, outputBase string) (string, error) {
	cmd := exec.CommandContext(ctx, whisperCli,
		"-m", modelPath,
		"-f", wavPath,
		"-l", "auto",
		"-otxt",
		"-of", outputBase,
		"-nt",
	)
	subprocess.HideConsoleWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("whisper-cli failed: %s", message)
	}
	return stdout.String(), nil
}

func normalizeWhisperOutput(text string) string {
	text = strings.TrimSpace(text)
	var out strings.Builder
	lastSpace := false
	inTimestamp := false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '[' {
			inTimestamp = true
			continue
		}
		if inTimestamp {
			if ch == ']' {
				inTimestamp = false
				if !lastSpace {
					out.WriteByte(' ')
					lastSpace = true
				}
			}
			continue
		}
		if ch == '\r' || ch == '\n' || ch == '\t' || ch == ' ' {
			if !lastSpace {
				out.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		out.WriteByte(ch)
		lastSpace = false
	}
	return strings.TrimSpace(out.String())
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
