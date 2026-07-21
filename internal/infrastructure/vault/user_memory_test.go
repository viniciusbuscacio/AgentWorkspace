package vault

import (
	"strings"
	"testing"
)

// --- v1 fact API (retained for migration coverage) ---

func TestUserMemoryV1FactsUpsertAndList(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	fact, err := v.UpsertUserFact(UserMemoryFact{
		Key:      "preferred-language",
		Category: "preference",
		Content:  "Responde sempre em português.",
		Source:   "agent",
	})
	if err != nil {
		t.Fatalf("UpsertUserFact() error = %v", err)
	}
	if fact.UpdatedAt == "" {
		t.Fatal("UpsertUserFact() did not stamp updated_at")
	}

	// Upsert on the same key must update, not duplicate.
	if _, err := v.UpsertUserFact(UserMemoryFact{
		Key:      "preferred-language",
		Category: "preference",
		Content:  "Prefere inglês em código.",
		Source:   "user",
	}); err != nil {
		t.Fatalf("UpsertUserFact() update error = %v", err)
	}

	facts, err := v.ListUserFacts()
	if err != nil {
		t.Fatalf("ListUserFacts() error = %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("facts = %d, want 1 (upsert by key)", len(facts))
	}
	got := facts[0]
	if got.Content != "Prefere inglês em código." || got.Source != "user" || got.Category != "preference" {
		t.Fatalf("fact = %+v, want updated content and source", got)
	}

	if err := v.DeleteUserFact("preferred-language"); err != nil {
		t.Fatalf("DeleteUserFact() error = %v", err)
	}
	facts, err = v.ListUserFacts()
	if err != nil {
		t.Fatalf("ListUserFacts() after delete error = %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("facts = %d, want 0 after delete", len(facts))
	}
}

func TestListUserFactsOrdersByCategoryThenNewest(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	seeds := []UserMemoryFact{
		{Key: "projeto-atual", Category: "context", Content: "aw", Source: "agent"},
		{Key: "nome", Category: "profile", Content: "Vinicius", Source: "user"},
		{Key: "editor", Category: "preference", Content: "Neovim", Source: "user"},
	}
	for _, seed := range seeds {
		if _, err := v.UpsertUserFact(seed); err != nil {
			t.Fatalf("UpsertUserFact(%s) error = %v", seed.Key, err)
		}
	}

	facts, err := v.ListUserFacts()
	if err != nil {
		t.Fatalf("ListUserFacts() error = %v", err)
	}
	if len(facts) != 3 {
		t.Fatalf("facts = %d, want 3", len(facts))
	}
	wantOrder := []string{"projeto-atual", "editor", "nome"} // context < preference < profile
	for i, want := range wantOrder {
		if facts[i].Key != want {
			t.Fatalf("facts[%d].Key = %q, want %q (full order %+v)", i, facts[i].Key, want, facts)
		}
	}
}

// --- v2 living document tests ---

func TestGetUserMemoryDocEmptyOnFreshVault(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	doc, err := v.GetUserMemoryDoc()
	if err != nil {
		t.Fatalf("GetUserMemoryDoc() error = %v", err)
	}
	if doc.Content != "" {
		t.Fatalf("expected empty doc on fresh vault, got %q", doc.Content)
	}
}

func TestSetAndGetUserMemoryDocRoundtrip(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	input := UserMemoryDoc{
		Content: "- preferred-language: Português\n- editor: Neovim",
		Backup:  "old backup",
	}
	stored, err := v.SetUserMemoryDoc(input)
	if err != nil {
		t.Fatalf("SetUserMemoryDoc() error = %v", err)
	}
	if stored.UpdatedAt == "" {
		t.Fatalf("SetUserMemoryDoc() did not stamp updated_at")
	}

	got, err := v.GetUserMemoryDoc()
	if err != nil {
		t.Fatalf("GetUserMemoryDoc() error = %v", err)
	}
	if got.Content != input.Content {
		t.Fatalf("content = %q, want %q", got.Content, input.Content)
	}
	if got.Backup != input.Backup {
		t.Fatalf("backup = %q, want %q", got.Backup, input.Backup)
	}
	if got.UpdatedAt == "" {
		t.Fatalf("updated_at is empty after roundtrip")
	}
}

func TestUserMemoryDocSurvivesLockUnlock(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	if _, err := v.SetUserMemoryDoc(UserMemoryDoc{Content: "- editor: Neovim"}); err != nil {
		t.Fatalf("SetUserMemoryDoc() error = %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if err := v.Unlock("senha1234"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	doc, err := v.GetUserMemoryDoc()
	if err != nil {
		t.Fatalf("GetUserMemoryDoc() after unlock error = %v", err)
	}
	if doc.Content != "- editor: Neovim" {
		t.Fatalf("doc content after lock/unlock = %q", doc.Content)
	}
}

func TestUserMemoryDocLockedVaultErrors(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if _, err := v.GetUserMemoryDoc(); err == nil {
		t.Fatal("GetUserMemoryDoc() on locked vault succeeded")
	}
	if _, err := v.SetUserMemoryDoc(UserMemoryDoc{Content: "x"}); err == nil {
		t.Fatal("SetUserMemoryDoc() on locked vault succeeded")
	}
}

func TestMigrateUserMemoryFactsToDoc(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	// Seed v1 facts directly into the table (bypassing schema migration).
	v.mu.Lock()
	_, err := v.db.Exec(`INSERT INTO user_memory (key, category, content, source, updated_at) VALUES
		('preferred-language', 'preference', 'Português.', 'agent', '2026-01-01'),
		('nome', 'profile', 'Vinicius', 'user', '2026-01-01')`)
	v.mu.Unlock()
	if err != nil {
		t.Fatalf("seed facts error = %v", err)
	}

	// Trigger migration.
	v.mu.Lock()
	v.migrateUserMemoryFactsLocked()
	v.mu.Unlock()

	doc, err := v.GetUserMemoryDoc()
	if err != nil {
		t.Fatalf("GetUserMemoryDoc() error = %v", err)
	}
	if !strings.Contains(doc.Content, "preferred-language") {
		t.Fatalf("migrated doc missing preferred-language: %q", doc.Content)
	}
	if !strings.Contains(doc.Content, "nome") {
		t.Fatalf("migrated doc missing nome: %q", doc.Content)
	}

	// v1 table must be empty after migration.
	v.mu.Lock()
	var count int
	_ = v.db.QueryRow(`SELECT COUNT(*) FROM user_memory`).Scan(&count)
	v.mu.Unlock()
	if count != 0 {
		t.Fatalf("user_memory has %d rows after migration, want 0", count)
	}
}

func TestMigrateUserMemoryFactsIdempotent(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	// Seed v1 facts.
	v.mu.Lock()
	_, _ = v.db.Exec(`INSERT INTO user_memory (key, category, content, source, updated_at) VALUES
		('k', 'preference', 'v', 'agent', '2026-01-01')`)
	v.mu.Unlock()

	// Run migration once.
	v.mu.Lock()
	v.migrateUserMemoryFactsLocked()
	v.mu.Unlock()

	// Override doc content so second run can't overwrite.
	if _, err := v.SetUserMemoryDoc(UserMemoryDoc{Content: "manually set content"}); err != nil {
		t.Fatalf("SetUserMemoryDoc() error = %v", err)
	}

	// Second run: doc already has content → must skip.
	v.mu.Lock()
	v.migrateUserMemoryFactsLocked()
	v.mu.Unlock()

	doc, err := v.GetUserMemoryDoc()
	if err != nil {
		t.Fatalf("GetUserMemoryDoc() error = %v", err)
	}
	if doc.Content != "manually set content" {
		t.Fatalf("idempotency broken: content = %q", doc.Content)
	}
}
