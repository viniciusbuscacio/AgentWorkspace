package application

import (
	"fmt"
	"testing"

	"aw/internal/domain"
)

type fakeChatRepository struct {
	chats           []domain.Chat
	messages        []domain.Message
	listErr         error
	lastCreated     string
	lastRenamedID   string
	lastRenamed     string
	lastArchivedID  string
	lastArchived    bool
	lastClearedID   string
	lastDeletedID   string
	lastMessagesID  string
	lastRecentID    string
	lastRecentLimit int
}

func (r *fakeChatRepository) ListChats() ([]domain.Chat, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return append([]domain.Chat(nil), r.chats...), nil
}

func (r *fakeChatRepository) CreateChat(title string) (domain.Chat, error) {
	r.lastCreated = title
	chat := domain.Chat{ID: "created-chat", Title: title}
	r.chats = append(r.chats, chat)
	return chat, nil
}

func (r *fakeChatRepository) RenameChat(chatID string, title string) (domain.Chat, error) {
	r.lastRenamedID = chatID
	r.lastRenamed = title
	return domain.Chat{ID: chatID, Title: title}, nil
}

func (r *fakeChatRepository) SetChatArchived(chatID string, archived bool) (domain.Chat, error) {
	r.lastArchivedID = chatID
	r.lastArchived = archived
	return domain.Chat{ID: chatID, Archived: archived}, nil
}

func (r *fakeChatRepository) ClearChat(chatID string) error {
	r.lastClearedID = chatID
	return nil
}

func (r *fakeChatRepository) DeleteChat(chatID string) error {
	r.lastDeletedID = chatID
	return nil
}

func (r *fakeChatRepository) ListMessages(chatID string) ([]domain.Message, error) {
	r.lastMessagesID = chatID
	return append([]domain.Message(nil), r.messages...), nil
}

func (r *fakeChatRepository) RecentMessages(chatID string, limit int) ([]domain.Message, error) {
	r.lastRecentID = chatID
	r.lastRecentLimit = limit
	return append([]domain.Message(nil), r.messages...), nil
}

func (r *fakeChatRepository) AddMessage(chatID string, role string, content string) (domain.Message, error) {
	return domain.Message{SessionID: chatID, Role: role, Content: content}, nil
}

func (r *fakeChatRepository) AddMessageWithAttachments(chatID string, role string, content string, attachments []domain.Attachment) (domain.Message, error) {
	return domain.Message{SessionID: chatID, Role: role, Content: content, Attachments: attachments}, nil
}

func TestCreateChatResolvesInitialTitle(t *testing.T) {
	repo := &fakeChatRepository{
		chats: []domain.Chat{
			{ID: "1", Title: "Chat 1"},
			{ID: "2", Title: "Chat 3"},
			{ID: "archived", Title: "Chat 2", Archived: true},
		},
	}

	chat, err := CreateChat(repo, "")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	if chat.Title != "Chat 2" || repo.lastCreated != "Chat 2" {
		t.Fatalf("created title = %q, lastCreated = %q", chat.Title, repo.lastCreated)
	}
}

func TestCreateChatUsesCustomTitle(t *testing.T) {
	repo := &fakeChatRepository{}

	chat, err := CreateChat(repo, "Spec: billing")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	if chat.Title != "Spec: billing" {
		t.Fatalf("created title = %q", chat.Title)
	}
}

func TestCreateChatFallsBackWhenExistingChatsCannotBeListed(t *testing.T) {
	repo := &fakeChatRepository{listErr: fmt.Errorf("database is locked")}

	chat, err := CreateChat(repo, "New Chat")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	if chat.Title != "New Chat" {
		t.Fatalf("created title = %q", chat.Title)
	}
}

func TestChatManagementUseCasesCallRepository(t *testing.T) {
	repo := &fakeChatRepository{
		chats:    []domain.Chat{{ID: "chat-1", Title: "Chat 1"}},
		messages: []domain.Message{{ID: "message-1", SessionID: "chat-1", Role: "user"}},
	}

	if chats, err := ListChats(repo); err != nil || len(chats) != 1 {
		t.Fatalf("ListChats() = %v, %v", chats, err)
	}
	if _, err := RenameChat(repo, "chat-1", "Renamed"); err != nil {
		t.Fatalf("RenameChat() error = %v", err)
	}
	if repo.lastRenamedID != "chat-1" || repo.lastRenamed != "Renamed" {
		t.Fatalf("rename args = %q, %q", repo.lastRenamedID, repo.lastRenamed)
	}
	if _, err := SetChatArchived(repo, "chat-1", true); err != nil {
		t.Fatalf("SetChatArchived() error = %v", err)
	}
	if repo.lastArchivedID != "chat-1" || !repo.lastArchived {
		t.Fatalf("archive args = %q, %v", repo.lastArchivedID, repo.lastArchived)
	}
	if messages, err := ListMessages(repo, "chat-1"); err != nil || len(messages) != 1 || repo.lastMessagesID != "chat-1" {
		t.Fatalf("ListMessages() = %v, %v, last=%q", messages, err, repo.lastMessagesID)
	}
	if messages, err := RecentMessages(repo, "chat-1", 7); err != nil || len(messages) != 1 || repo.lastRecentID != "chat-1" || repo.lastRecentLimit != 7 {
		t.Fatalf("RecentMessages() = %v, %v, last=%q limit=%d", messages, err, repo.lastRecentID, repo.lastRecentLimit)
	}
	if err := ClearChat(repo, "chat-1"); err != nil || repo.lastClearedID != "chat-1" {
		t.Fatalf("ClearChat() error = %v, last=%q", err, repo.lastClearedID)
	}
	if err := DeleteChat(repo, "chat-1"); err != nil || repo.lastDeletedID != "chat-1" {
		t.Fatalf("DeleteChat() error = %v, last=%q", err, repo.lastDeletedID)
	}
}

func TestChatManagementUseCasesRejectMissingRepository(t *testing.T) {
	if _, err := ListChats(nil); err == nil {
		t.Fatalf("ListChats(nil) error = nil")
	}
	if _, err := CreateChat(nil, ""); err == nil {
		t.Fatalf("CreateChat(nil) error = nil")
	}
	if _, err := ListMessages(nil, "chat"); err == nil {
		t.Fatalf("ListMessages(nil) error = nil")
	}
	if _, err := RecentMessages(nil, "chat", 1); err == nil {
		t.Fatalf("RecentMessages(nil) error = nil")
	}
	if err := ClearChat(nil, "chat"); err == nil {
		t.Fatalf("ClearChat(nil) error = nil")
	}
	if _, err := RenameChat(nil, "chat", "title"); err == nil {
		t.Fatalf("RenameChat(nil) error = nil")
	}
	if _, err := SetChatArchived(nil, "chat", true); err == nil {
		t.Fatalf("SetChatArchived(nil) error = nil")
	}
	if err := DeleteChat(nil, "chat"); err == nil {
		t.Fatalf("DeleteChat(nil) error = nil")
	}
}
