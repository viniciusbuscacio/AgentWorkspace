package ports

import (
	"context"

	"aw/internal/domain"
)

type ChatRuntime interface {
	SendMessage(ctx context.Context, cfg domain.ModelConfig, chatID string, text string) (domain.AgentReply, error)
	StreamMessage(ctx context.Context, cfg domain.ModelConfig, chatID string, text string, onDelta func(delta string)) (domain.AgentReply, error)
}

// ChatSessionResetter drops every in-memory chat session (used after a vault
// resync so each chat's next turn re-seeds from the vault).
type ChatSessionResetter interface {
	ResetAllSessions()
}

// ChatHistoryAppender is implemented by runtimes that can append an
// assistant-authored entry to a chat's live in-memory session. Used after an
// interrupted run persists partial streamed text to the vault: the ADK runner
// only records completed events, so without this the next turn in the same
// process would see less history than the vault (and the UI) holds.
type ChatHistoryAppender interface {
	AppendAssistantHistory(ctx context.Context, chatID string, text string) error
}

type ChatCompactionRuntime interface {
	ChatTitleRuntime
	ResetSessionWithHistory(ctx context.Context, cfg domain.ModelConfig, chatID string, messages []domain.HistoryMessage) error
}

// ChatSessionRestoreRuntime is what the after-restart session restore needs:
// detect a cold in-memory session and re-seed it from persisted messages.
type ChatSessionRestoreRuntime interface {
	HasSession(ctx context.Context, chatID string) bool
	ResetSessionWithHistory(ctx context.Context, cfg domain.ModelConfig, chatID string, messages []domain.HistoryMessage) error
}

// ChatSessionRestoreStore is the vault slice the restore reads.
type ChatSessionRestoreStore interface {
	IsUnlocked() bool
	ListMessages(chatID string) ([]domain.Message, error)
}

type ChatTitleRuntime interface {
	GenerateOneShot(ctx context.Context, cfg domain.ModelConfig, text string, maxOutputTokens int32) (domain.AgentReply, error)
}
