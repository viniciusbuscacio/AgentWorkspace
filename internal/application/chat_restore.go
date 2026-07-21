package application

import (
	"context"
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

const (
	// RestoreSeedMaxMessages caps how many persisted messages re-seed a cold
	// runtime session after an app restart.
	RestoreSeedMaxMessages = 30
	// RestoreSeedMaxChars caps the seed size in runes; the newest messages
	// win (the tail is walked backwards until either cap trips).
	RestoreSeedMaxChars = 24000
)

// EnsureChatSessionRestored re-seeds the runtime's in-memory session from the
// vault when it is cold (fresh process): without it, the first message after
// an app restart reaches the model with no memory of the conversation the UI
// still shows. The seed opens with a continuity note telling the model the
// chat ID, how much history it is seeing, and how to fetch more. Warm sessions
// are untouched. Returns the number of seeded messages (0 = warm session or
// nothing worth seeding).
func EnsureChatSessionRestored(ctx context.Context, store ports.ChatSessionRestoreStore, runtime ports.ChatSessionRestoreRuntime, cfg domain.ModelConfig, chatID string) (int, error) {
	if store == nil || runtime == nil || strings.TrimSpace(chatID) == "" {
		return 0, nil
	}
	if !store.IsUnlocked() {
		return 0, nil
	}
	if runtime.HasSession(ctx, chatID) {
		return 0, nil
	}
	messages, err := store.ListMessages(chatID)
	if err != nil {
		return 0, err
	}
	seed := BuildRestoreSeed(messages)
	if len(seed) == 0 {
		return 0, nil
	}
	seed = append([]domain.HistoryMessage{BuildRestoreNote(chatID, seed, messages)}, seed...)
	if err := runtime.ResetSessionWithHistory(ctx, cfg, chatID, seed); err != nil {
		return 0, err
	}
	return len(seed), nil
}

// BuildRestoreNote writes the continuity note that opens a restored session:
// which chat this is, how much of the saved history the seed carries, how to
// page in more (the aw action chat.messages), and that pre-restart tool
// results are gone — re-run the tool instead of guessing.
func BuildRestoreNote(chatID string, seed []domain.HistoryMessage, messages []domain.Message) domain.HistoryMessage {
	tailCount := len(seed)
	hasSummary := len(seed) > 0 && IsCompactionSummaryMessage(domain.Message{Role: seed[0].Role, Content: seed[0].Content})
	if hasSummary {
		tailCount--
	}
	totalCount := 0
	for _, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if (role == "user" || role == "assistant") && strings.TrimSpace(message.Content) != "" && !IsCompactionSummaryMessage(message) {
			totalCount++
		}
	}

	var b strings.Builder
	b.WriteString("[Continuity note — this session was restored after an app restart] ")
	fmt.Fprintf(&b, "Chat ID: %s. ", chatID)
	if totalCount > tailCount {
		fmt.Fprintf(&b, "The context below carries the newest %d of %d saved messages", tailCount, totalCount)
	} else {
		fmt.Fprintf(&b, "The context below carries all %d saved messages of this chat", totalCount)
	}
	if hasSummary {
		b.WriteString(", plus a summary of earlier compacted turns")
	}
	b.WriteString(". ")
	if totalCount > tailCount {
		fmt.Fprintf(&b, "Older messages are not in context; to read further back call the aw tool action chat.messages with {\"chatId\": %q, \"limit\": <n>}. ", chatID)
	}
	b.WriteString("Tool results from before the restart were NOT restored — if an earlier turn relied on one (a file, note, web page, or command output), re-run that tool instead of answering from memory.")
	return domain.HistoryMessage{Role: "user", Content: b.String()}
}

// BuildRestoreSeed selects the conversation tail that re-seeds a cold
// session: newest messages within RestoreSeedMaxMessages/RestoreSeedMaxChars,
// trimmed so the seed opens on a user message (a coherent turn boundary),
// with the latest compaction summary always prepended when one exists —
// long-term memory is cheap, the summary is capped at CompactSummaryMaxRunes.
func BuildRestoreSeed(messages []domain.Message) []domain.HistoryMessage {
	var summary *domain.Message
	conversation := make([]domain.Message, 0, len(messages))
	for i := range messages {
		message := messages[i]
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		if IsCompactionSummaryMessage(message) {
			summary = &messages[i] // the last summary wins
			continue
		}
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		conversation = append(conversation, message)
	}

	// Walk backwards accumulating the tail. The newest message always gets in
	// (even oversized); after that, either cap ends the walk.
	start := len(conversation)
	chars := 0
	for start > 0 {
		taken := len(conversation) - start
		if taken >= RestoreSeedMaxMessages {
			break
		}
		candidateChars := len([]rune(conversation[start-1].Content))
		if taken > 0 && chars+candidateChars > RestoreSeedMaxChars {
			break
		}
		start--
		chars += candidateChars
	}
	// Open the seed on a user message so the replayed history starts at a
	// coherent turn boundary (never mid-answer).
	for start < len(conversation) && strings.ToLower(strings.TrimSpace(conversation[start].Role)) != "user" {
		start++
	}
	tail := conversation[start:]

	if len(tail) == 0 && summary == nil {
		return nil
	}
	seed := make([]domain.HistoryMessage, 0, len(tail)+1)
	if summary != nil {
		seed = append(seed, domain.HistoryMessage{Role: summary.Role, Content: summary.Content})
	}
	for _, message := range tail {
		seed = append(seed, domain.HistoryMessage{Role: message.Role, Content: message.Content})
	}
	return seed
}
