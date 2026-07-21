package appcore

import (
	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
)

func (a *App) ListPasswords() dto.PasswordsResult {
	a.recordActivity()
	entries, err := application.ListPasswords(a.vault)
	if err != nil {
		return dto.PasswordsResult{Error: err.Error()}
	}
	return dto.PasswordsResult{Success: true, Passwords: entries}
}

func (a *App) SavePassword(entry domain.PasswordEntry) dto.PasswordResult {
	a.recordActivity()
	saved, err := application.SavePassword(a.vault, entry)
	if err != nil {
		return dto.PasswordResult{Error: err.Error()}
	}
	return dto.PasswordResult{Success: true, Password: &saved}
}

func (a *App) DeletePassword(id string) dto.OperationResult {
	a.recordActivity()
	if err := application.DeletePassword(a.vault, id); err != nil {
		return dto.OperationResult{Error: err.Error()}
	}
	return dto.OperationResult{Success: true}
}
