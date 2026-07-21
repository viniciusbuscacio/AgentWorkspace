package vault

import "testing"

func TestNotesCrudRoundTrip(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	note, err := v.CreateNote("Plano da viagem", "Roteiro: Lisboa, Porto.", true)
	if err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}
	if note.ID == "" || note.UpdatedAt == "" {
		t.Fatalf("CreateNote() missing id/timestamp: %+v", note)
	}

	// A note created with inPrompt=false must persist that way (the "Insert into
	// Agent prompt" toggle honored at creation, not only on later edits).
	quiet, err := v.CreateNote("Privada", "Fora do prompt.", false)
	if err != nil {
		t.Fatalf("CreateNote(inPrompt=false) error = %v", err)
	}
	if quiet.InPrompt {
		t.Fatal("note created with inPrompt=false must not be InPrompt")
	}
	if reread, err := v.GetNote(quiet.ID); err != nil || reread.InPrompt {
		t.Fatalf("reread inPrompt=false note = %+v (err %v)", reread, err)
	}
	// Remove it so the round-trip's later count assertions see one note.
	if err := v.DeleteNote(quiet.ID); err != nil {
		t.Fatalf("DeleteNote(quiet) error = %v", err)
	}

	updated, err := v.UpdateNote(note.ID, "Plano da viagem 2026", "Roteiro: Lisboa, Porto, Faro.", false, false, true)
	if err != nil {
		t.Fatalf("UpdateNote() error = %v", err)
	}
	if updated.Title != "Plano da viagem 2026" || updated.Content != "Roteiro: Lisboa, Porto, Faro." {
		t.Fatalf("UpdateNote() = %+v", updated)
	}

	if _, err := v.UpdateNote("note-missing", "x", "y", false, false, true); err == nil {
		t.Fatal("UpdateNote() on missing note should fail")
	}

	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if _, err := v.ListNotes(false); err == nil {
		t.Fatal("ListNotes() on locked vault succeeded")
	}
	if err := v.Unlock("senha1234"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	got, err := v.GetNote(note.ID)
	if err != nil {
		t.Fatalf("GetNote() error = %v", err)
	}
	if got.Content != "Roteiro: Lisboa, Porto, Faro." {
		t.Fatalf("note after unlock = %+v", got)
	}

	notes, err := v.ListNotes(false)
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("notes = %d, want 1", len(notes))
	}

	if err := v.DeleteNote(note.ID); err != nil {
		t.Fatalf("DeleteNote() error = %v", err)
	}
	notes, _ = v.ListNotes(false)
	if len(notes) != 0 {
		t.Fatalf("notes after delete = %d, want 0", len(notes))
	}
}

func TestNotesPinnedOrderingAndArchivedFilter(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	oldest, _ := v.CreateNote("Antiga", "a", true)
	middle, _ := v.CreateNote("Fixada", "b", true)
	newest, _ := v.CreateNote("Recente", "c", true)

	// Pin the middle note: it must sort first even though newer notes exist.
	pinned, err := v.UpdateNote(middle.ID, middle.Title, middle.Content, true, false, true)
	if err != nil {
		t.Fatalf("UpdateNote(pin) error = %v", err)
	}
	if !pinned.Pinned {
		t.Fatalf("pinned note = %+v", pinned)
	}
	// Archive the oldest: it leaves the default list.
	if _, err := v.UpdateNote(oldest.ID, oldest.Title, oldest.Content, false, true, true); err != nil {
		t.Fatalf("UpdateNote(archive) error = %v", err)
	}

	visible, err := v.ListNotes(false)
	if err != nil {
		t.Fatalf("ListNotes(false) error = %v", err)
	}
	if len(visible) != 2 || visible[0].ID != middle.ID || visible[1].ID != newest.ID {
		t.Fatalf("visible order = %+v", visible)
	}

	all, err := v.ListNotes(true)
	if err != nil {
		t.Fatalf("ListNotes(true) error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("all notes = %d, want 3", len(all))
	}
	archivedSeen := false
	for _, note := range all {
		if note.ID == oldest.ID && note.Archived {
			archivedSeen = true
		}
	}
	if !archivedSeen {
		t.Fatalf("archived note missing or flag lost: %+v", all)
	}
}

// TestNotesFlagColumnsMigration proves the pinned/archived migration is
// additive and idempotent: a vault whose notes table predates the columns
// gains them on the next initSchema run, keeping existing rows.
func TestNotesFlagColumnsMigration(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	note, err := v.CreateNote("Pré-migração", "Sobrevive à migração.", true)
	if err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}

	// Rebuild the pre-flags table shape (old vaults).
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS idx_notes_visible`,
		`ALTER TABLE notes DROP COLUMN pinned`,
		`ALTER TABLE notes DROP COLUMN archived`,
	} {
		if _, err := v.db.Exec(stmt); err != nil {
			t.Fatalf("downgrade %q error = %v", stmt, err)
		}
	}

	// Unlock path runs initSchema again — the additive guards must restore the
	// columns without touching the row.
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if err := v.Unlock("senha1234"); err != nil {
		t.Fatalf("Unlock() (migration) error = %v", err)
	}
	// Idempotence: a second run over the migrated schema must not fail.
	if err := initSchema(v.db); err != nil {
		t.Fatalf("initSchema() second run error = %v", err)
	}

	got, err := v.GetNote(note.ID)
	if err != nil {
		t.Fatalf("GetNote() after migration error = %v", err)
	}
	if got.Content != "Sobrevive à migração." || got.Pinned || got.Archived {
		t.Fatalf("migrated note = %+v", got)
	}
}

// TestNotesInPromptColumnMigration proves the in_prompt migration is additive
// and idempotent: a vault whose notes table predates the column gains it on
// the next initSchema run, defaulting existing rows to 1 (opt-in).
func TestNotesInPromptColumnMigration(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	note, err := v.CreateNote("Pré-migração inPrompt", "Sobrevive à migração.", true)
	if err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}
	if !note.InPrompt {
		t.Fatal("new note should default InPrompt=true")
	}

	// Simulate a pre-in_prompt vault by dropping the column.
	if _, err := v.db.Exec(`ALTER TABLE notes DROP COLUMN in_prompt`); err != nil {
		t.Fatalf("downgrade error = %v", err)
	}

	// Lock + unlock triggers initSchema (the additive guard re-adds the column).
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if err := v.Unlock("senha1234"); err != nil {
		t.Fatalf("Unlock() (migration) error = %v", err)
	}
	// Idempotence check.
	if err := initSchema(v.db); err != nil {
		t.Fatalf("initSchema() second run error = %v", err)
	}

	got, err := v.GetNote(note.ID)
	if err != nil {
		t.Fatalf("GetNote() after migration error = %v", err)
	}
	if !got.InPrompt {
		t.Fatalf("migrated note should have InPrompt=true (column default): %+v", got)
	}
}
