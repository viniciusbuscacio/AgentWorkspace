package application

import (
	"errors"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// CreateNote validates and stores a new note.
func CreateNote(store ports.NotesStore, title string, content string, inPrompt bool) (domain.Note, error) {
	if store == nil {
		return domain.Note{}, errors.New("notes store is required")
	}
	if !store.IsUnlocked() {
		return domain.Note{}, errVaultLocked
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.Note{}, errors.New("note title is required")
	}
	return store.CreateNote(title, content, inPrompt)
}

// ListNotes returns notes pinned-first, most recently updated first. Archived
// notes are included only when includeArchived is set (AW2 parity).
func ListNotes(store ports.NotesStore, includeArchived bool) ([]domain.Note, error) {
	if store == nil {
		return nil, errors.New("notes store is required")
	}
	if !store.IsUnlocked() {
		return nil, errVaultLocked
	}
	return store.ListNotes(includeArchived)
}

// GetNote returns one note by id.
func GetNote(store ports.NotesStore, id string) (domain.Note, error) {
	if store == nil {
		return domain.Note{}, errors.New("notes store is required")
	}
	if !store.IsUnlocked() {
		return domain.Note{}, errVaultLocked
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Note{}, errors.New("note id is required")
	}
	return store.GetNote(id)
}

// UpdateNote changes a note partially: nil fields keep the current value. A
// provided content may be empty (clearing the note is valid); a provided title
// that trims to empty keeps the current title (AW2 parity). At least one field
// must be provided.
func UpdateNote(store ports.NotesStore, id string, title *string, content *string, pinned *bool, archived *bool, inPrompt *bool) (domain.Note, error) {
	if store == nil {
		return domain.Note{}, errors.New("notes store is required")
	}
	if !store.IsUnlocked() {
		return domain.Note{}, errVaultLocked
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Note{}, errors.New("note id is required")
	}
	if title == nil && content == nil && pinned == nil && archived == nil && inPrompt == nil {
		return domain.Note{}, errors.New("title, content, pinned, archived and/or inPrompt is required")
	}
	current, err := store.GetNote(id)
	if err != nil {
		return domain.Note{}, err
	}
	next := current
	if title != nil && strings.TrimSpace(*title) != "" {
		next.Title = strings.TrimSpace(*title)
	}
	if content != nil {
		next.Content = *content
	}
	if pinned != nil {
		next.Pinned = *pinned
	}
	if archived != nil {
		next.Archived = *archived
	}
	if inPrompt != nil {
		next.InPrompt = *inPrompt
	}
	return store.UpdateNote(id, next.Title, next.Content, next.Pinned, next.Archived, next.InPrompt)
}

// DeleteNote removes a note and returns the deleted note (so callers can echo
// its title in results).
func DeleteNote(store ports.NotesStore, id string) (domain.Note, error) {
	if store == nil {
		return domain.Note{}, errors.New("notes store is required")
	}
	if !store.IsUnlocked() {
		return domain.Note{}, errVaultLocked
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Note{}, errors.New("note id is required")
	}
	note, err := store.GetNote(id)
	if err != nil {
		return domain.Note{}, err
	}
	if err := store.DeleteNote(id); err != nil {
		return domain.Note{}, err
	}
	return note, nil
}
