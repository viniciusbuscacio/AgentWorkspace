package application

import (
	"strings"
	"testing"

	"aw/internal/domain"
)

// fakeUserMemoryDocStore is the test double for ports.UserMemoryDocStore.
type fakeUserMemoryDocStore struct {
	unlocked bool
	doc      domain.UserMemoryDoc
}

func newFakeUserMemoryDocStore() *fakeUserMemoryDocStore {
	return &fakeUserMemoryDocStore{unlocked: true}
}

func (s *fakeUserMemoryDocStore) IsUnlocked() bool { return s.unlocked }

func (s *fakeUserMemoryDocStore) GetUserMemoryDoc() (domain.UserMemoryDoc, error) {
	return s.doc, nil
}

func (s *fakeUserMemoryDocStore) SetUserMemoryDoc(doc domain.UserMemoryDoc) (domain.UserMemoryDoc, error) {
	doc.UpdatedAt = "2026-06-10T00:00:00Z"
	s.doc = doc
	return doc, nil
}

// --- GetUserMemoryDoc ---

func TestGetUserMemoryDocRequiresUnlocked(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	store.unlocked = false
	if _, err := GetUserMemoryDoc(store); !isVaultLockedError(err) {
		t.Fatalf("GetUserMemoryDoc(locked) error = %v, want errVaultLocked", err)
	}
}

func TestGetUserMemoryDocReturnsDoc(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	store.doc = domain.UserMemoryDoc{Content: "hello"}
	doc, err := GetUserMemoryDoc(store)
	if err != nil {
		t.Fatalf("GetUserMemoryDoc() error = %v", err)
	}
	if doc.Content != "hello" {
		t.Fatalf("content = %q, want %q", doc.Content, "hello")
	}
}

// --- SetUserMemoryDoc ---

func TestSetUserMemoryDocScrubsSecrets(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	_, err := SetUserMemoryDoc(store, "token: sk-abcdefghijklmnopqrstuvwxyz123456")
	if err != nil {
		t.Fatalf("SetUserMemoryDoc() error = %v", err)
	}
	if strings.Contains(store.doc.Content, "sk-") {
		t.Fatalf("secret not scrubbed from doc: %q", store.doc.Content)
	}
}

func TestSetUserMemoryDocRequiresUnlocked(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	store.unlocked = false
	if _, err := SetUserMemoryDoc(store, "hello"); !isVaultLockedError(err) {
		t.Fatalf("SetUserMemoryDoc(locked) error = %v, want errVaultLocked", err)
	}
}

func TestSetUserMemoryDocTrimsContent(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	if _, err := SetUserMemoryDoc(store, "  hello  "); err != nil {
		t.Fatalf("SetUserMemoryDoc() error = %v", err)
	}
	if store.doc.Content != "hello" {
		t.Fatalf("content not trimmed: %q", store.doc.Content)
	}
}

// --- AppendUserMemoryLine ---

func TestAppendUserMemoryLineAppendsToEmpty(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	doc, err := AppendUserMemoryLine(store, "preferred-language", "preference", "Português.")
	if err != nil {
		t.Fatalf("AppendUserMemoryLine() error = %v", err)
	}
	if !strings.Contains(doc.Content, "preferred-language") {
		t.Fatalf("appended line missing key: %q", doc.Content)
	}
	if !strings.Contains(doc.Content, "Português.") {
		t.Fatalf("appended line missing content: %q", doc.Content)
	}
}

func TestAppendUserMemoryLineAppendsToExisting(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	store.doc = domain.UserMemoryDoc{Content: "- existing: line"}
	_, err := AppendUserMemoryLine(store, "editor", "preference", "Neovim")
	if err != nil {
		t.Fatalf("AppendUserMemoryLine() error = %v", err)
	}
	if !strings.Contains(store.doc.Content, "existing") {
		t.Fatalf("existing content lost: %q", store.doc.Content)
	}
	if !strings.Contains(store.doc.Content, "editor") {
		t.Fatalf("new line missing: %q", store.doc.Content)
	}
	// Two lines separated by newline.
	if !strings.Contains(store.doc.Content, "\n") {
		t.Fatalf("lines not newline-separated: %q", store.doc.Content)
	}
}

func TestAppendUserMemoryLineScrubsSecrets(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	_, err := AppendUserMemoryLine(store, "token", "reference", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9xxxx")
	if err != nil {
		t.Fatalf("AppendUserMemoryLine() error = %v", err)
	}
	// The long token-like string should be scrubbed.
	if strings.Contains(store.doc.Content, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9xxxx") {
		t.Fatalf("secret not scrubbed from appended line: %q", store.doc.Content)
	}
}

func TestAppendUserMemoryLineRequiresUnlocked(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	store.unlocked = false
	if _, err := AppendUserMemoryLine(store, "k", "preference", "v"); !isVaultLockedError(err) {
		t.Fatalf("AppendUserMemoryLine(locked) error = %v, want errVaultLocked", err)
	}
}

func TestAppendUserMemoryLineValidation(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	if _, err := AppendUserMemoryLine(store, "   ", "preference", "v"); err == nil {
		t.Fatalf("expected error for empty key")
	}
	if _, err := AppendUserMemoryLine(store, "k", "preference", "   "); err == nil {
		t.Fatalf("expected error for empty content")
	}
}

func TestAppendUserMemoryLineNormalizesKey(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	_, err := AppendUserMemoryLine(store, "  Preferred Language  ", "preference", "Portuguese")
	if err != nil {
		t.Fatalf("AppendUserMemoryLine() error = %v", err)
	}
	if !strings.Contains(store.doc.Content, "preferred-language") {
		t.Fatalf("key not normalized: %q", store.doc.Content)
	}
}

// --- UserMemoryInstruction ---

func TestUserMemoryInstructionEmptyDoc(t *testing.T) {
	if got := UserMemoryInstruction(""); got != "" {
		t.Fatalf("UserMemoryInstruction(\"\") = %q, want empty", got)
	}
}

func TestUserMemoryInstructionRendersHeader(t *testing.T) {
	got := UserMemoryInstruction("- preferred-language: Português.")
	if !strings.Contains(got, "Information loaded from Settings → Memory") {
		t.Fatalf("missing header: %q", got)
	}
	if !strings.Contains(got, "preferred-language") {
		t.Fatalf("missing content: %q", got)
	}
}

func TestUserMemoryInstructionCapAt8000Runes(t *testing.T) {
	// Build a document larger than 8000 runes.
	big := strings.Repeat("abcdefghij", 1000) // 10000 runes
	got := UserMemoryInstruction(big)
	if len([]rune(got)) > userMemoryPromptCap {
		t.Fatalf("result exceeds cap: %d runes (cap %d)", len([]rune(got)), userMemoryPromptCap)
	}
}

// --- NormalizeUserFactKey ---

func TestNormalizeUserFactKey(t *testing.T) {
	if got := NormalizeUserFactKey("  Linguagem   Preferida do User "); got != "linguagem-preferida-do-user" {
		t.Fatalf("NormalizeUserFactKey() = %q", got)
	}
	if got := NormalizeUserFactKey("    "); got != "" {
		t.Fatalf("NormalizeUserFactKey(blank) = %q, want empty", got)
	}
}

// helper so tests don't depend on the concrete error value.
func isVaultLockedError(err error) bool {
	return err != nil && err.Error() == errVaultLocked.Error()
}

func TestAppendUserMemoryLineUpsertsAcrossCategories(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	if _, err := AppendUserMemoryLine(store, "preferred-language", "preferences", "English"); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendUserMemoryLine(store, "preferred-language", "preference", "Português"); err != nil {
		t.Fatal(err)
	}
	doc, _ := store.GetUserMemoryDoc()
	if strings.Count(doc.Content, "preferred-language") != 1 {
		t.Fatalf("duplicate key lines survived upsert: %q", doc.Content)
	}
	if !strings.Contains(doc.Content, "Português") || strings.Contains(doc.Content, "English") {
		t.Fatalf("latest value must win: %q", doc.Content)
	}
}

func TestRemoveUserMemoryLine(t *testing.T) {
	store := newFakeUserMemoryDocStore()
	_, _ = AppendUserMemoryLine(store, "preferred-language", "preference", "English")
	_, _ = AppendUserMemoryLine(store, "user-name", "profile", "Vini")
	removed, err := RemoveUserMemoryLine(store, "Preferred Language")
	if err != nil || removed != 1 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	doc, _ := store.GetUserMemoryDoc()
	if strings.Contains(doc.Content, "preferred-language") || !strings.Contains(doc.Content, "user-name") {
		t.Fatalf("wrong line removed: %q", doc.Content)
	}
}
