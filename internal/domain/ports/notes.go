package ports

import "aw/internal/domain"

// NotesStore persists the Notes module data in the vault.
type NotesStore interface {
	VaultUnlockState
	CreateNote(title string, content string, inPrompt bool) (domain.Note, error)
	// ListNotes returns notes pinned-first, newest first; archived notes only
	// when includeArchived is set.
	ListNotes(includeArchived bool) ([]domain.Note, error)
	GetNote(id string) (domain.Note, error)
	// UpdateNote writes the full row; partial-update merging happens in the
	// application layer.
	UpdateNote(id string, title string, content string, pinned bool, archived bool, inPrompt bool) (domain.Note, error)
	DeleteNote(id string) error
}
