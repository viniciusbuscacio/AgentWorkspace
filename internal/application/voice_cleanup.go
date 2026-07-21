package application

import (
	"context"
	"regexp"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// VoiceCleanupTimeout bounds the LLM call; past it the raw transcript wins.
const VoiceCleanupTimeout = 12 * time.Second
const voiceCleanupMaxOutputTokens = 1024

// VoiceCleanupPrompt mirrors AW2's cleanup step: the local whisper model is
// weak, so the active LLM fixes obvious dictation errors before the text
// reaches the composer. The prompt is English (models follow it better) but
// the transcript must stay in the speaker's language.
func VoiceCleanupPrompt(rawText string, preferredLanguage string) string {
	language := strings.TrimSpace(preferredLanguage)
	if language == "" {
		language = DefaultVoiceLanguage
	}
	return strings.Join([]string{
		"Fix a short dictation transcript before it goes into a chat input field.",
		"Rules:",
		"- Return only the corrected text, nothing else.",
		"- The speaker's preferred language is " + language + "; use it as the hint when a word is ambiguous or misrecognized.",
		"- Keep the transcript in its original language; never translate it.",
		"- Preserve the intent, proper names and technical terms whenever possible.",
		"- Fix punctuation, capitalization, obvious agreement issues and clear speech-recognition errors.",
		"- Do not answer the message and do not add new information.",
		"",
		"Raw transcript: " + rawText,
	}, "\n")
}

// CleanVoiceTranscript is best-effort: any failure (no runtime, provider not
// ready, timeout, empty reply) returns the raw text unchanged.
func CleanVoiceTranscript(ctx context.Context, runtime ports.ChatTitleRuntime, cfg domain.ModelConfig, rawText string, preferredLanguage string) (string, bool) {
	raw := strings.TrimSpace(rawText)
	if runtime == nil || raw == "" {
		return raw, false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, VoiceCleanupTimeout)
	defer cancel()
	reply, err := runtime.GenerateOneShot(ctx, cfg, VoiceCleanupPrompt(raw, preferredLanguage), voiceCleanupMaxOutputTokens)
	if err != nil {
		return raw, false
	}
	cleaned := normalizeVoiceCleanup(reply.Text)
	if cleaned == "" || cleaned == raw {
		return raw, false
	}
	return cleaned, true
}

func normalizeVoiceCleanup(text string) string {
	text = strings.TrimSpace(text)
	text = strings.Trim(text, `"'`)
	return strings.Join(strings.Fields(text), " ")
}

// DefaultVoiceLanguage is used when the user memory carries no
// preferred-language line.
const DefaultVoiceLanguage = "English (US)"

var preferredLanguagePattern = regexp.MustCompile(`(?i)preferred[-_ ]?language\s*[:=]\s*(.+)`)

// PreferredLanguageFromMemory extracts the "preferred-language: X" line the
// Settings › Memory doc carries (same convention the module-help button uses
// on the frontend). Defaults to DefaultVoiceLanguage when absent or empty.
func PreferredLanguageFromMemory(content string) string {
	match := preferredLanguagePattern.FindStringSubmatch(content)
	if len(match) < 2 {
		return DefaultVoiceLanguage
	}
	value := strings.TrimRight(strings.TrimSpace(match[1]), ".;,")
	if value == "" {
		return DefaultVoiceLanguage
	}
	return value
}
