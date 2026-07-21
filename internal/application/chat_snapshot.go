package application

import (
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// ChatSnapshotResult is the read-only view of a chat plus its messages.
type ChatSnapshotResult struct {
	Chat     *domain.Chat
	Messages []domain.Message
}

// ChatSnapshot loads a single chat and its messages from an unlocked vault.
// It is a pure use case: all I/O is provided through the store port.
func ChatSnapshot(store ports.ChatConversationStore, chatID string) (ChatSnapshotResult, error) {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return ChatSnapshotResult{}, fmt.Errorf("chat is required")
	}
	if !store.IsUnlocked() {
		return ChatSnapshotResult{}, fmt.Errorf("vault is locked")
	}
	chats, err := store.ListChats()
	if err != nil {
		return ChatSnapshotResult{}, err
	}
	var chat *domain.Chat
	for i := range chats {
		if chats[i].ID == chatID {
			found := chats[i]
			chat = &found
			break
		}
	}
	if chat == nil {
		return ChatSnapshotResult{}, fmt.Errorf("chat not found")
	}
	messages, err := store.ListMessages(chatID)
	if err != nil {
		return ChatSnapshotResult{}, err
	}
	return ChatSnapshotResult{Chat: chat, Messages: messages}, nil
}
