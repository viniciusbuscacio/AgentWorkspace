package application

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"aw/internal/domain"
)

type fakeNotesStore struct {
	unlocked bool
	notes    map[string]domain.Note
	seq      int
}

func newFakeNotesStore() *fakeNotesStore {
	return &fakeNotesStore{unlocked: true, notes: map[string]domain.Note{}}
}

func (s *fakeNotesStore) IsUnlocked() bool { return s.unlocked }

func (s *fakeNotesStore) CreateNote(title string, content string, inPrompt bool) (domain.Note, error) {
	s.seq++
	note := domain.Note{ID: fmt.Sprintf("note-%d", s.seq), Title: title, Content: content, UpdatedAt: "now", InPrompt: inPrompt}
	s.notes[note.ID] = note
	return note, nil
}

func (s *fakeNotesStore) ListNotes(includeArchived bool) ([]domain.Note, error) {
	notes := make([]domain.Note, 0, len(s.notes))
	for _, note := range s.notes {
		if note.Archived && !includeArchived {
			continue
		}
		notes = append(notes, note)
	}
	return notes, nil
}

func (s *fakeNotesStore) GetNote(id string) (domain.Note, error) {
	note, ok := s.notes[id]
	if !ok {
		return domain.Note{}, errors.New("note not found")
	}
	return note, nil
}

func (s *fakeNotesStore) UpdateNote(id string, title string, content string, pinned bool, archived bool, inPrompt bool) (domain.Note, error) {
	note, ok := s.notes[id]
	if !ok {
		return domain.Note{}, errors.New("note not found")
	}
	note.Title, note.Content = title, content
	note.Pinned, note.Archived = pinned, archived
	note.InPrompt = inPrompt
	s.notes[id] = note
	return note, nil
}

func (s *fakeNotesStore) DeleteNote(id string) error {
	delete(s.notes, id)
	return nil
}

func TestCreateNoteValidatesTitle(t *testing.T) {
	store := newFakeNotesStore()
	if _, err := CreateNote(store, "   ", "x", true); err == nil {
		t.Fatal("empty title should fail")
	}
	note, err := CreateNote(store, " Plano ", "conteúdo", true)
	if err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}
	if note.Title != "Plano" {
		t.Fatalf("title = %q, want trimmed", note.Title)
	}
}

func TestUpdateNoteMergesPartialFields(t *testing.T) {
	store := newFakeNotesStore()
	note, _ := CreateNote(store, "Título", "Original", true)

	content := "Novo conteúdo"
	updated, err := UpdateNote(store, note.ID, nil, &content, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateNote(content only) error = %v", err)
	}
	if updated.Title != "Título" || updated.Content != "Novo conteúdo" {
		t.Fatalf("partial update = %+v", updated)
	}

	// Provided-but-empty content clears the note (autosave of an emptied
	// editor); the title stays.
	empty := ""
	cleared, err := UpdateNote(store, note.ID, nil, &empty, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateNote(clear content) error = %v", err)
	}
	if cleared.Content != "" || cleared.Title != "Título" {
		t.Fatalf("cleared note = %+v", cleared)
	}

	if _, err := UpdateNote(store, note.ID, nil, nil, nil, nil, nil); err == nil {
		t.Fatal("update with no fields should fail")
	}
	title := "x"
	if _, err := UpdateNote(store, "missing", &title, nil, nil, nil, nil); err == nil {
		t.Fatal("update of missing note should fail")
	}
}

func TestUpdateNoteTogglesFlagsWithoutTouchingText(t *testing.T) {
	store := newFakeNotesStore()
	note, _ := CreateNote(store, "Fixa", "Conteúdo", true)

	pinned := true
	updated, err := UpdateNote(store, note.ID, nil, nil, &pinned, nil, nil)
	if err != nil {
		t.Fatalf("UpdateNote(pin only) error = %v", err)
	}
	if !updated.Pinned || updated.Archived || updated.Title != "Fixa" || updated.Content != "Conteúdo" {
		t.Fatalf("pin-only update = %+v", updated)
	}

	archived := true
	updated, err = UpdateNote(store, note.ID, nil, nil, nil, &archived, nil)
	if err != nil {
		t.Fatalf("UpdateNote(archive only) error = %v", err)
	}
	if !updated.Pinned || !updated.Archived {
		t.Fatalf("archive-only update should keep pinned = %+v", updated)
	}
}

func TestListNotesFiltersArchived(t *testing.T) {
	store := newFakeNotesStore()
	keep, _ := CreateNote(store, "Ativa", "x", true)
	gone, _ := CreateNote(store, "Arquivada", "y", true)
	archived := true
	if _, err := UpdateNote(store, gone.ID, nil, nil, nil, &archived, nil); err != nil {
		t.Fatalf("archive error = %v", err)
	}

	visible, err := ListNotes(store, false)
	if err != nil {
		t.Fatalf("ListNotes(false) error = %v", err)
	}
	if len(visible) != 1 || visible[0].ID != keep.ID {
		t.Fatalf("visible notes = %+v", visible)
	}

	all, err := ListNotes(store, true)
	if err != nil {
		t.Fatalf("ListNotes(true) error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("all notes = %d, want 2", len(all))
	}
}

func TestDeleteNoteEchoesDeletedNote(t *testing.T) {
	store := newFakeNotesStore()
	note, _ := CreateNote(store, "Para apagar", "x", true)

	deleted, err := DeleteNote(store, note.ID)
	if err != nil {
		t.Fatalf("DeleteNote() error = %v", err)
	}
	if deleted.Title != "Para apagar" {
		t.Fatalf("deleted note = %+v", deleted)
	}
	if _, err := DeleteNote(store, note.ID); err == nil {
		t.Fatal("deleting a missing note should fail (it was already gone)")
	}
}

func TestNotesRequireUnlockedVault(t *testing.T) {
	store := newFakeNotesStore()
	store.unlocked = false
	if _, err := CreateNote(store, "x", "y", true); !errors.Is(err, errVaultLocked) {
		t.Fatalf("CreateNote error = %v, want errVaultLocked", err)
	}
	if _, err := ListNotes(store, false); !errors.Is(err, errVaultLocked) {
		t.Fatalf("ListNotes error = %v, want errVaultLocked", err)
	}
}

func TestUpdateNoteInPromptFlagRoundTrip(t *testing.T) {
	store := newFakeNotesStore()
	note, _ := CreateNote(store, "Plano", "conteudo", true)
	if !note.InPrompt {
		t.Fatal("new note should default to InPrompt=true")
	}

	f := false
	updated, err := UpdateNote(store, note.ID, nil, nil, nil, nil, &f)
	if err != nil {
		t.Fatalf("UpdateNote(inPrompt=false) error = %v", err)
	}
	if updated.InPrompt {
		t.Fatal("InPrompt should be false after explicit update")
	}

	tr := true
	restored, err := UpdateNote(store, note.ID, nil, nil, nil, nil, &tr)
	if err != nil {
		t.Fatalf("UpdateNote(inPrompt=true) error = %v", err)
	}
	if !restored.InPrompt {
		t.Fatal("InPrompt should be restored to true")
	}
}

func TestNotesPromptBlockCapsAndSkipsArchived(t *testing.T) {
	long := string(make([]rune, 4100))
	notes := []domain.Note{
		{ID: "n1", Title: "Pinned", Content: "short", Pinned: true, InPrompt: true},
		{ID: "n2", Title: "Long", Content: long, InPrompt: true},
		{ID: "n3", Title: "Archived", Content: "should not appear", Archived: true, InPrompt: true},
		{ID: "n4", Title: "NoPrompt", Content: "excluded", InPrompt: false},
	}
	block := NotesPromptBlock(notes)

	if !strings.Contains(block, "## Information loaded from the Notes module") {
		t.Fatalf("block missing header: %q", block)
	}
	if !strings.Contains(block, "### Note: Pinned") {
		t.Fatalf("block missing pinned note: %q", block)
	}
	if strings.Contains(block, "Archived") {
		t.Fatalf("block must not include archived note: %q", block)
	}
	if strings.Contains(block, "NoPrompt") {
		t.Fatalf("block must not include inPrompt=false note: %q", block)
	}
	// The long note's block entry must be capped at 4000 runes + [truncated].
	if !strings.Contains(block, "[truncated]") {
		t.Fatal("long note should produce [truncated] marker")
	}
	// Whole block must not exceed 12000 runes + [truncated] suffix.
	if len([]rune(block)) > 12000+len([]rune("[truncated]")) {
		t.Fatalf("block rune length = %d, exceeds whole-block cap", len([]rune(block)))
	}
}

func TestNotesPromptBlockEmptyWhenNoEligibleNotes(t *testing.T) {
	notes := []domain.Note{
		{ID: "n1", Title: "Archived", Content: "x", Archived: true, InPrompt: true},
		{ID: "n2", Title: "Off", Content: "y", InPrompt: false},
	}
	if got := NotesPromptBlock(notes); got != "" {
		t.Fatalf("expected empty block, got: %q", got)
	}
	if got := NotesPromptBlock(nil); got != "" {
		t.Fatalf("expected empty block for nil input, got: %q", got)
	}
}
