package application

import (
	"fmt"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

func ListChats(repo ports.ChatRepository) ([]domain.Chat, error) {
	if repo == nil {
		return nil, fmt.Errorf("chat repository is required")
	}
	return repo.ListChats()
}

func CreateChat(repo ports.ChatRepository, requestedTitle string) (domain.Chat, error) {
	if repo == nil {
		return domain.Chat{}, fmt.Errorf("chat repository is required")
	}
	title := requestedTitle
	if ShouldUseNumberedChatTitle(requestedTitle) {
		chats, err := repo.ListChats()
		if err != nil {
			title = "New Chat"
		} else {
			title = ResolveInitialChatTitle(requestedTitle, chats)
		}
	}
	return repo.CreateChat(title)
}

func ListMessages(repo ports.ChatRepository, chatID string) ([]domain.Message, error) {
	if repo == nil {
		return nil, fmt.Errorf("chat repository is required")
	}
	return repo.ListMessages(chatID)
}

func RecentMessages(repo ports.ChatRepository, chatID string, limit int) ([]domain.Message, error) {
	if repo == nil {
		return nil, fmt.Errorf("chat repository is required")
	}
	return repo.RecentMessages(chatID, limit)
}

func ClearChat(repo ports.ChatRepository, chatID string) error {
	if repo == nil {
		return fmt.Errorf("chat repository is required")
	}
	return repo.ClearChat(chatID)
}

func RenameChat(repo ports.ChatRepository, chatID string, title string) (domain.Chat, error) {
	if repo == nil {
		return domain.Chat{}, fmt.Errorf("chat repository is required")
	}
	return repo.RenameChat(chatID, title)
}

func SetChatArchived(repo ports.ChatRepository, chatID string, archived bool) (domain.Chat, error) {
	if repo == nil {
		return domain.Chat{}, fmt.Errorf("chat repository is required")
	}
	return repo.SetChatArchived(chatID, archived)
}

func DeleteChat(repo ports.ChatRepository, chatID string) error {
	if repo == nil {
		return fmt.Errorf("chat repository is required")
	}
	return repo.DeleteChat(chatID)
}
