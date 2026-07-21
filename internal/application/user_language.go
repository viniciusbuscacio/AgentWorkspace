package application

import (
	"context"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

const userLanguageDetectTimeout = 8 * time.Second

// HasPreferredLanguage reports whether the memory doc already carries the
// preferred-language line (any case/separator variant).
func HasPreferredLanguage(content string) bool {
	return preferredLanguagePattern.MatchString(content)
}

// MaybeDetectUserLanguage backfills the preferred-language memory line from
// the user's own message via a one-shot LLM call — deterministic app-side
// trigger, independent of the main agent choosing to comply. No-op when the
// line already exists, the message is too short to identify, or anything
// fails (best-effort by design). Returns true when a line was saved.
func MaybeDetectUserLanguage(ctx context.Context, runtime ports.ChatTitleRuntime, cfg domain.ModelConfig, store ports.UserMemoryDocStore, message string) bool {
	if runtime == nil || store == nil {
		return false
	}
	message = strings.TrimSpace(message)
	if len([]rune(message)) < 12 {
		return false // too short to name a language with confidence
	}
	doc, err := GetUserMemoryDoc(store)
	if err != nil || HasPreferredLanguage(doc.Content) {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, userLanguageDetectTimeout)
	defer cancel()
	prompt := "Name the language the following text is written in. Reply with ONLY the language name, written in that language itself (e.g. Português, English, Español, Deutsch). Text:\n" + message
	reply, err := runtime.GenerateOneShot(ctx, cfg, prompt, 16)
	if err != nil {
		return false
	}
	name := strings.Trim(strings.TrimSpace(reply.Text), `"'.`)
	if name == "" || len([]rune(name)) > 30 || strings.ContainsAny(name, "\n:{}") {
		return false // not a plain language name — refuse odd replies
	}
	if _, err := AppendUserMemoryLine(store, "preferred-language", "preference", name); err != nil {
		return false
	}
	return true
}
