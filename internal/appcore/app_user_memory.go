package appcore

import (
	"aw/internal/application"
	"aw/internal/dto"
)

// GetUserMemoryDoc returns the v2 living memory document for Settings > Memory.
func (a *App) GetUserMemoryDoc() dto.UserMemoryDocResult {
	a.recordActivity()
	doc, err := application.GetUserMemoryDoc(a.vault)
	if err != nil {
		return dto.UserMemoryDocResult{Error: err.Error()}
	}
	return dto.UserMemoryDocResult{Success: true, Doc: &doc}
}

// SetUserMemoryDoc saves the edited document from Settings > Memory. The
// content is scrubbed for secrets before persist.
func (a *App) SetUserMemoryDoc(content string) dto.UserMemoryDocResult {
	a.recordActivity()
	doc, err := application.SetUserMemoryDoc(a.vault, content)
	if err != nil {
		return dto.UserMemoryDocResult{Error: err.Error()}
	}
	return dto.UserMemoryDocResult{Success: true, Doc: &doc}
}

// GetUserMemoryDocBackup returns the one-deep backup document (the version
// before the last condensation), for the undo action in Settings > Memory.
func (a *App) GetUserMemoryDocBackup() dto.UserMemoryDocResult {
	a.recordActivity()
	doc, err := application.GetUserMemoryDoc(a.vault)
	if err != nil {
		return dto.UserMemoryDocResult{Error: err.Error()}
	}
	if doc.Backup == "" {
		return dto.UserMemoryDocResult{Success: true}
	}
	backup := doc
	backup.Content = doc.Backup
	backup.Backup = ""
	return dto.UserMemoryDocResult{Success: true, Doc: &backup}
}
