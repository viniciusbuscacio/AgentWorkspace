package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aw/internal/domain/ports"
)

const (
	// condensationMaxOutputTokens is the LLM token budget for a condensation reply.
	condensationMaxOutputTokens int32 = 1200
	// condensationCooldownKey is the secret key that stores the last-condensed date.
	condensationCooldownKey = "__user_memory_last_condensed_date__"
)

// UserMemoryCondenseInput carries the stores needed to run condensation.
type UserMemoryCondenseInput struct {
	DocStore      ports.UserMemoryDocStore
	ProviderStore ports.ProviderSecretStore
	LogStore      ports.LogStore
	SecretStore   ports.SecretStore // for the once-per-day cooldown
	Runtime       ports.ChatTitleRuntime
}

// UserMemoryCondenseResult reports what happened.
type UserMemoryCondenseResult struct {
	Condensed     bool
	SkippedReason string
	CharsBefore   int
	CharsAfter    int
}

// MaybeCondenseUserMemory runs the condensation if and only if:
//   - the document exceeds UserMemoryCondenseThreshold runes, AND
//   - it has not been condensed today (checked via SecretStore cooldown).
//
// A failed LLM call leaves the document untouched. The previous version is
// stored as a one-deep backup and the event is logged.
func MaybeCondenseUserMemory(ctx context.Context, input UserMemoryCondenseInput) (UserMemoryCondenseResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if input.DocStore == nil || input.Runtime == nil || input.ProviderStore == nil {
		return UserMemoryCondenseResult{SkippedReason: "stores not available"}, nil
	}
	if !input.DocStore.IsUnlocked() {
		return UserMemoryCondenseResult{SkippedReason: "vault locked"}, nil
	}

	doc, err := input.DocStore.GetUserMemoryDoc()
	if err != nil {
		return UserMemoryCondenseResult{}, fmt.Errorf("read doc: %w", err)
	}
	runes := []rune(doc.Content)
	if len(runes) < UserMemoryCondenseThreshold {
		return UserMemoryCondenseResult{SkippedReason: "below threshold"}, nil
	}

	// Once-per-day cooldown.
	today := time.Now().UTC().Format("2006-01-02")
	if input.SecretStore != nil {
		if last, ok, _ := input.SecretStore.GetSecret(condensationCooldownKey); ok && last == today {
			return UserMemoryCondenseResult{SkippedReason: "already condensed today"}, nil
		}
	}

	// Resolve LLM config.
	runtimeConfig, err := ResolveProviderRuntimeConfig(input.ProviderStore)
	if err != nil {
		return UserMemoryCondenseResult{SkippedReason: "no provider configured"}, nil
	}
	modelConfig := ModelConfigFromProviderRuntimeConfig(runtimeConfig)

	// Scrub before sending to LLM.
	scrubbed := ScrubChatSecrets(doc.Content)

	prompt := userMemoryCondensePrompt(scrubbed)
	reply, err := input.Runtime.GenerateOneShot(ctx, modelConfig, prompt, condensationMaxOutputTokens)
	if err != nil {
		return UserMemoryCondenseResult{}, fmt.Errorf("condensation LLM: %w", err)
	}
	condensed := strings.TrimSpace(ScrubChatSecrets(reply.Text))
	if condensed == "" {
		return UserMemoryCondenseResult{}, fmt.Errorf("condensation returned empty result")
	}

	charsBefore := len([]rune(doc.Content))

	// Store backup + new content atomically.
	doc.Backup = doc.Content
	doc.Content = condensed
	doc.LastCondensedAt = time.Now().UTC().Format(time.RFC3339)
	if _, err := input.DocStore.SetUserMemoryDoc(doc); err != nil {
		return UserMemoryCondenseResult{}, fmt.Errorf("persist condensed doc: %w", err)
	}

	// Record cooldown date.
	if input.SecretStore != nil {
		_ = input.SecretStore.SetSecret(condensationCooldownKey, today)
	}

	charsAfter := len([]rune(condensed))
	return UserMemoryCondenseResult{
		Condensed:   true,
		CharsBefore: charsBefore,
		CharsAfter:  charsAfter,
	}, nil
}

func userMemoryCondensePrompt(content string) string {
	return "You are rewriting a user memory document to be more concise.\n" +
		"Rules:\n" +
		"- Preserve explicit user preferences verbatim wherever possible.\n" +
		"- Keep the user's language (do not translate).\n" +
		"- Compress narrative and repetition, but never delete facts.\n" +
		"- Output only the rewritten document — no commentary, no headings you didn't receive.\n" +
		"- Keep the same line format as the input.\n\n" +
		"Document to condense:\n" +
		content
}
