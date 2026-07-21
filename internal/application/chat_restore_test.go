package application

import (
	"context"
	"strings"
	"testing"

	"aw/internal/domain"
)

type fakeRestoreStore struct {
	unlocked bool
	messages []domain.Message
}

func (f *fakeRestoreStore) IsUnlocked() bool { return f.unlocked }
func (f *fakeRestoreStore) ListMessages(string) ([]domain.Message, error) {
	return f.messages, nil
}

type fakeRestoreRuntime struct {
	warm   bool
	seeded []domain.HistoryMessage
	resets int
}

func (f *fakeRestoreRuntime) HasSession(context.Context, string) bool { return f.warm }
func (f *fakeRestoreRuntime) ResetSessionWithHistory(_ context.Context, _ domain.ModelConfig, _ string, messages []domain.HistoryMessage) error {
	f.resets++
	f.seeded = messages
	return nil
}

func TestEnsureChatSessionRestoredSeedsColdSession(t *testing.T) {
	store := &fakeRestoreStore{unlocked: true, messages: []domain.Message{
		{Role: "user", Content: "oi"},
		{Role: "assistant", Content: "olá!"},
	}}
	runtime := &fakeRestoreRuntime{}

	seeded, err := EnsureChatSessionRestored(context.Background(), store, runtime, domain.ModelConfig{}, "chat-1")
	if err != nil {
		t.Fatalf("EnsureChatSessionRestored() error = %v", err)
	}
	if seeded != 3 || runtime.resets != 1 {
		t.Fatalf("seeded = %d resets = %d, want note + 2 messages in 1 reset", seeded, runtime.resets)
	}
	if !strings.Contains(runtime.seeded[0].Content, "Chat ID: chat-1") {
		t.Fatalf("seed[0] = %q, want the continuity note first", runtime.seeded[0].Content)
	}
	if runtime.seeded[1].Content != "oi" {
		t.Fatalf("seed[1] = %q, want the conversation right after the note", runtime.seeded[1].Content)
	}
}

func TestBuildRestoreNoteCountsAndFetchHint(t *testing.T) {
	messages := make([]domain.Message, 0, 44)
	messages = append(messages, domain.Message{Role: "system", Content: CompactSummaryPrefix + "\nresumo"})
	for i := 0; i < 20; i++ {
		messages = append(messages,
			domain.Message{Role: "user", Content: "pergunta"},
			domain.Message{Role: "assistant", Content: "resposta"},
		)
	}
	seed := BuildRestoreSeed(messages)
	note := BuildRestoreNote("chat-9", seed, messages)

	if note.Role != "user" {
		t.Fatalf("note role = %q, want user", note.Role)
	}
	for _, want := range []string{
		"Chat ID: chat-9",
		"newest 30 of 40 saved messages",
		"summary of earlier compacted turns",
		`chat.messages with {"chatId": "chat-9"`,
		"Tool results from before the restart were NOT restored",
	} {
		if !strings.Contains(note.Content, want) {
			t.Fatalf("note missing %q:\n%s", want, note.Content)
		}
	}
}

func TestBuildRestoreNoteFullHistorySkipsFetchHint(t *testing.T) {
	messages := []domain.Message{
		{Role: "user", Content: "oi"},
		{Role: "assistant", Content: "olá"},
	}
	seed := BuildRestoreSeed(messages)
	note := BuildRestoreNote("chat-2", seed, messages)

	if !strings.Contains(note.Content, "all 2 saved messages") {
		t.Fatalf("note = %q, want the full-history phrasing", note.Content)
	}
	if strings.Contains(note.Content, "chat.messages") {
		t.Fatalf("note offers chat.messages when the whole history is already in context:\n%s", note.Content)
	}
}

func TestEnsureChatSessionRestoredLeavesWarmSessionAlone(t *testing.T) {
	store := &fakeRestoreStore{unlocked: true, messages: []domain.Message{{Role: "user", Content: "oi"}}}
	runtime := &fakeRestoreRuntime{warm: true}

	seeded, err := EnsureChatSessionRestored(context.Background(), store, runtime, domain.ModelConfig{}, "chat-1")
	if err != nil || seeded != 0 || runtime.resets != 0 {
		t.Fatalf("warm session: seeded=%d resets=%d err=%v, want untouched", seeded, runtime.resets, err)
	}
}

func TestEnsureChatSessionRestoredNoopsOnLockedVaultAndEmptyChat(t *testing.T) {
	runtime := &fakeRestoreRuntime{}
	if seeded, err := EnsureChatSessionRestored(context.Background(), &fakeRestoreStore{unlocked: false}, runtime, domain.ModelConfig{}, "chat-1"); err != nil || seeded != 0 {
		t.Fatalf("locked vault: seeded=%d err=%v, want no-op", seeded, err)
	}
	if seeded, err := EnsureChatSessionRestored(context.Background(), &fakeRestoreStore{unlocked: true}, runtime, domain.ModelConfig{}, "chat-1"); err != nil || seeded != 0 {
		t.Fatalf("empty chat: seeded=%d err=%v, want no-op", seeded, err)
	}
	if runtime.resets != 0 {
		t.Fatalf("resets = %d, want 0", runtime.resets)
	}
}

func TestBuildRestoreSeedCapsAndOpensOnUserMessage(t *testing.T) {
	// 40 alternating messages: the 30-message cap must trim the oldest, and
	// the seed must open on a user turn.
	messages := make([]domain.Message, 0, 40)
	for i := 0; i < 20; i++ {
		messages = append(messages,
			domain.Message{Role: "user", Content: "pergunta"},
			domain.Message{Role: "assistant", Content: "resposta"},
		)
	}
	seed := BuildRestoreSeed(messages)
	if len(seed) == 0 || len(seed) > RestoreSeedMaxMessages {
		t.Fatalf("seed length = %d, want 1..%d", len(seed), RestoreSeedMaxMessages)
	}
	if strings.ToLower(seed[0].Role) != "user" {
		t.Fatalf("seed opens on %q, want user", seed[0].Role)
	}
}

func TestBuildRestoreSeedCharBudgetKeepsNewest(t *testing.T) {
	big := strings.Repeat("x", RestoreSeedMaxChars) // one message fills the budget
	messages := []domain.Message{
		{Role: "user", Content: "antiga"},
		{Role: "assistant", Content: big},
		{Role: "user", Content: "recente"},
		{Role: "assistant", Content: "resposta recente"},
	}
	seed := BuildRestoreSeed(messages)
	// The big message blows the budget for anything older; the seed keeps the
	// newest exchange and opens on the user turn.
	if len(seed) != 2 || seed[0].Content != "recente" {
		t.Fatalf("seed = %+v, want the newest user+assistant pair", seed)
	}
}

func TestBuildRestoreSeedPrependsCompactionSummary(t *testing.T) {
	messages := []domain.Message{
		{Role: "system", Content: CompactSummaryPrefix + "\nresumo antigo"},
		{Role: "system", Content: "marker que não é resumo"},
		{Role: "user", Content: "oi"},
		{Role: "assistant", Content: "olá"},
	}
	seed := BuildRestoreSeed(messages)
	if len(seed) != 3 {
		t.Fatalf("seed length = %d, want summary + 2 messages: %+v", len(seed), seed)
	}
	if !strings.HasPrefix(seed[0].Content, CompactSummaryPrefix) {
		t.Fatalf("seed[0] = %q, want the compaction summary first", seed[0].Content)
	}
	for _, entry := range seed[1:] {
		if strings.Contains(entry.Content, "marker") {
			t.Fatalf("non-summary system marker leaked into the seed: %+v", seed)
		}
	}
}
