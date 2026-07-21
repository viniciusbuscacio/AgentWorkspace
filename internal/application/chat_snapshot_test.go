package application

import (
	"errors"
	"testing"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// fakeSnapshotStore implements ports.ChatConversationStore for snapshot tests.
// Only IsUnlocked, ListChats and ListMessages carry behavior; the remaining
// ChatRepository methods are unused no-ops to satisfy the interface.
type fakeSnapshotStore struct {
	unlocked     bool
	chats        []domain.Chat
	messages     []domain.Message
	listChatsErr error
	listMsgsErr  error
}

func (s fakeSnapshotStore) IsUnlocked() bool { return s.unlocked }

func (s fakeSnapshotStore) ListChats() ([]domain.Chat, error) {
	return s.chats, s.listChatsErr
}

func (s fakeSnapshotStore) ListMessages(string) ([]domain.Message, error) {
	return s.messages, s.listMsgsErr
}

func (s fakeSnapshotStore) RecentMessages(string, int) ([]domain.Message, error) {
	return nil, nil
}
func (s fakeSnapshotStore) CreateChat(string) (domain.Chat, error) { return domain.Chat{}, nil }
func (s fakeSnapshotStore) RenameChat(string, string) (domain.Chat, error) {
	return domain.Chat{}, nil
}
func (s fakeSnapshotStore) SetChatArchived(string, bool) (domain.Chat, error) {
	return domain.Chat{}, nil
}
func (s fakeSnapshotStore) ClearChat(string) error  { return nil }
func (s fakeSnapshotStore) DeleteChat(string) error { return nil }
func (s fakeSnapshotStore) AddMessage(string, string, string) (domain.Message, error) {
	return domain.Message{}, nil
}
func (s fakeSnapshotStore) AddMessageWithAttachments(string, string, string, []domain.Attachment) (domain.Message, error) {
	return domain.Message{}, nil
}

var _ ports.ChatConversationStore = fakeSnapshotStore{}

func TestChatSnapshotReturnsChatAndMessages(t *testing.T) {
	store := fakeSnapshotStore{
		unlocked: true,
		chats:    []domain.Chat{{ID: "chat-1", Title: "First"}, {ID: "chat-2", Title: "Second"}},
		messages: []domain.Message{{ID: "msg-1", Role: "user", Content: "hi"}},
	}

	result, err := ChatSnapshot(store, "chat-2")
	if err != nil {
		t.Fatalf("ChatSnapshot error = %v", err)
	}
	if result.Chat == nil || result.Chat.ID != "chat-2" || result.Chat.Title != "Second" {
		t.Fatalf("chat = %+v, want chat-2/Second", result.Chat)
	}
	if len(result.Messages) != 1 || result.Messages[0].ID != "msg-1" {
		t.Fatalf("messages = %+v, want 1 message msg-1", result.Messages)
	}
}

func TestChatSnapshotRejectsLockedVault(t *testing.T) {
	store := fakeSnapshotStore{unlocked: false}
	if _, err := ChatSnapshot(store, "chat-1"); err == nil {
		t.Fatal("expected error for locked vault, got nil")
	}
}

func TestChatSnapshotRequiresChatID(t *testing.T) {
	store := fakeSnapshotStore{unlocked: true}
	if _, err := ChatSnapshot(store, "   "); err == nil {
		t.Fatal("expected error for empty chat id, got nil")
	}
}

func TestChatSnapshotChatNotFound(t *testing.T) {
	store := fakeSnapshotStore{unlocked: true, chats: []domain.Chat{{ID: "chat-1"}}}
	if _, err := ChatSnapshot(store, "missing"); err == nil {
		t.Fatal("expected error for missing chat, got nil")
	}
}

func TestChatSnapshotPropagatesListError(t *testing.T) {
	store := fakeSnapshotStore{unlocked: true, listChatsErr: errors.New("boom")}
	if _, err := ChatSnapshot(store, "chat-1"); err == nil {
		t.Fatal("expected propagated list error, got nil")
	}
}
