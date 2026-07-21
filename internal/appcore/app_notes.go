package appcore

import (
	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
)

// ListNotes returns every note (archived included) for the Notes module view,
// pinned-first then newest; the view filters archived notes out of the main
// list and offers them behind a toggle.
func (a *App) ListNotes() dto.NotesResult {
	a.recordActivity()
	notes, err := application.ListNotes(a.vault, true)
	if err != nil {
		return dto.NotesResult{Error: err.Error()}
	}
	return dto.NotesResult{Success: true, Notes: notes}
}

// CreateNote stores a new note from the Notes view. inPrompt carries the
// editor's "Insert into Agent prompt" toggle so a note created with it
// unchecked is excluded from the agent context from the start.
func (a *App) CreateNote(title string, content string, inPrompt bool) dto.NoteResult {
	a.recordActivity()
	return noteResult(application.CreateNote(a.vault, title, content, inPrompt))
}

// UpdateNote changes a note's title and content from the editor. Content is
// always written (clearing a note is valid); an empty title keeps the current
// one.
func (a *App) UpdateNote(id string, title string, content string) dto.NoteResult {
	a.recordActivity()
	return noteResult(application.UpdateNote(a.vault, id, &title, &content, nil, nil, nil))
}

// UpdateNoteFlags sets a note's pinned/archived flags (AW2 parity: pin sorts
// first, archive hides from the main list).
func (a *App) UpdateNoteFlags(id string, pinned bool, archived bool) dto.NoteResult {
	a.recordActivity()
	return noteResult(application.UpdateNote(a.vault, id, nil, nil, &pinned, &archived, nil))
}

// UpdateNoteInPrompt sets the "Insert into Agent prompt" flag. When true the
// note is injected into the agent system context as part of the User notes
// block (Decision 7 — bounded by caps in application.NotesPromptBlock).
func (a *App) UpdateNoteInPrompt(id string, inPrompt bool) dto.NoteResult {
	a.recordActivity()
	return noteResult(application.UpdateNote(a.vault, id, nil, nil, nil, nil, &inPrompt))
}

// DeleteNote removes a note.
func (a *App) DeleteNote(id string) dto.NoteResult {
	a.recordActivity()
	return noteResult(application.DeleteNote(a.vault, id))
}

func noteResult(note domain.Note, err error) dto.NoteResult {
	if err != nil {
		return dto.NoteResult{Error: err.Error()}
	}
	return dto.NoteResult{Success: true, Note: &note}
}
