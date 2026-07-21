package vault

import (
	"database/sql"
	"errors"

	"aw/internal/domain"
)

type Note = domain.Note

var errNoteNotFound = errors.New("note not found")

// CreateNote stores a new note and returns it with id and timestamp set.
// New notes start unpinned and unarchived; inPrompt controls whether the note
// feeds into the agent system context (AW2 parity).
func (v *Vault) CreateNote(title string, content string, inPrompt bool) (Note, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return Note{}, errLocked
	}
	note := Note{
		ID:        "note-" + randomHex(6),
		Title:     title,
		Content:   content,
		UpdatedAt: nowString(),
		InPrompt:  inPrompt,
	}
	_, err := v.db.Exec(
		`INSERT INTO notes (id, title, content, updated_at, pinned, archived, in_prompt) VALUES (?, ?, ?, ?, 0, 0, ?)`,
		note.ID, note.Title, note.Content, note.UpdatedAt, boolToInt(inPrompt),
	)
	if err != nil {
		return Note{}, err
	}
	return note, nil
}

// ListNotes returns notes pinned-first, then most recently updated first.
// Archived notes are excluded unless includeArchived is set (AW2 parity).
func (v *Vault) ListNotes(includeArchived bool) ([]Note, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	archivedFilter := 0
	if includeArchived {
		archivedFilter = 1
	}
	rows, err := v.db.Query(
		`SELECT id, title, content, updated_at, pinned, archived, in_prompt FROM notes
		  WHERE (? = 1 OR archived = 0)
		  ORDER BY pinned DESC, updated_at DESC, id ASC`,
		archivedFilter,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := make([]Note, 0)
	for rows.Next() {
		note, err := scanNote(rows.Scan)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

// GetNote returns one note by id.
func (v *Vault) GetNote(id string) (Note, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return Note{}, errLocked
	}
	return v.getNoteLocked(id)
}

// UpdateNote replaces a note's title, content, flags and in_prompt, bumping
// updated_at. Partial-update merging is the application layer's job.
func (v *Vault) UpdateNote(id string, title string, content string, pinned bool, archived bool, inPrompt bool) (Note, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return Note{}, errLocked
	}
	now := nowString()
	result, err := v.db.Exec(
		`UPDATE notes SET title = ?, content = ?, updated_at = ?, pinned = ?, archived = ?, in_prompt = ? WHERE id = ?`,
		title, content, now, boolToInt(pinned), boolToInt(archived), boolToInt(inPrompt), id,
	)
	if err != nil {
		return Note{}, err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return Note{}, errNoteNotFound
	}
	return v.getNoteLocked(id)
}

// DeleteNote removes one note by id. Deleting a missing note is a no-op.
func (v *Vault) DeleteNote(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	_, err := v.db.Exec(`DELETE FROM notes WHERE id = ?`, id)
	return err
}

func (v *Vault) getNoteLocked(id string) (Note, error) {
	row := v.db.QueryRow(
		`SELECT id, title, content, updated_at, pinned, archived, in_prompt FROM notes WHERE id = ?`, id,
	)
	note, err := scanNote(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, errNoteNotFound
	}
	if err != nil {
		return Note{}, err
	}
	return note, nil
}

func scanNote(scan func(dest ...any) error) (Note, error) {
	var note Note
	var pinned, archived, inPrompt int
	if err := scan(&note.ID, &note.Title, &note.Content, &note.UpdatedAt, &pinned, &archived, &inPrompt); err != nil {
		return Note{}, err
	}
	note.Pinned = pinned != 0
	note.Archived = archived != 0
	note.InPrompt = inPrompt != 0
	return note, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
