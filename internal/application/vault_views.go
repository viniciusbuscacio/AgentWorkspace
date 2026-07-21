package application

import (
	"aw/internal/domain"
	"aw/internal/domain/ports"
)

type VaultStatusView struct {
	Status         domain.VaultStatus
	CurrentProfile *domain.ProfileInfo
}

type OperationSnapshot struct {
	Status         domain.VaultStatus
	CurrentProfile *domain.ProfileInfo
	Chats          []domain.Chat
}

func BuildVaultStatusView(vault ports.VaultStatusReader, profiles ports.ProfileReader, currentProfileID string) VaultStatusView {
	var status domain.VaultStatus
	if vault != nil {
		status = vault.Status()
	}
	return VaultStatusView{
		Status:         status,
		CurrentProfile: currentProfile(profiles, currentProfileID),
	}
}

func BuildOperationSnapshot(vault ports.VaultOperationSnapshotStore, profiles ports.ProfileReader, currentProfileID string) OperationSnapshot {
	view := BuildVaultStatusView(vault, profiles, currentProfileID)
	snapshot := OperationSnapshot{
		Status:         view.Status,
		CurrentProfile: view.CurrentProfile,
	}
	if vault != nil && vault.IsUnlocked() {
		if chats, err := vault.ListChats(); err == nil {
			snapshot.Chats = chats
		}
	}
	return snapshot
}

func ListChatsIfUnlocked(vault ports.VaultOperationSnapshotStore) ([]domain.Chat, error) {
	if vault == nil || !vault.IsUnlocked() {
		return []domain.Chat{}, nil
	}
	return vault.ListChats()
}

func currentProfile(profiles ports.ProfileReader, currentProfileID string) *domain.ProfileInfo {
	if profiles == nil || currentProfileID == "" {
		return nil
	}
	if info, ok := profiles.GetInfo(currentProfileID); ok {
		return &info
	}
	return nil
}
