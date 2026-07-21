package application

import (
	"encoding/json"
	"strings"
	"testing"

	"aw/internal/domain"
)

func TestPasswordsUsePasswordSecretPrefixAndSkipCorruptValues(t *testing.T) {
	store := newMemorySecretStore()
	store.values["_password:broken"] = "{not json"
	store.values["openrouter_api_key"] = "sk-test"

	saved, err := SavePassword(store, domain.PasswordEntry{Name: "GitHub", Username: "vini", Password: "secret"})
	if err != nil {
		t.Fatalf("SavePassword() error = %v", err)
	}
	if saved.ID == "" || !strings.HasPrefix(store.values[PasswordSecretPrefix+saved.ID], "{") {
		t.Fatalf("password not stored under %s*: id=%q values=%v", PasswordSecretPrefix, saved.ID, store.values)
	}

	entries, err := ListPasswords(store)
	if err != nil {
		t.Fatalf("ListPasswords() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "GitHub" || entries[0].Password != "secret" {
		t.Fatalf("entries = %+v, want saved password only", entries)
	}
}

func TestDeletePasswordOnlyDeletesPasswordPrefixedKey(t *testing.T) {
	store := newMemorySecretStore()
	store.values["_password:abc"] = "{}"
	store.values["abc"] = "ordinary"

	if err := DeletePassword(store, "abc"); err != nil {
		t.Fatalf("DeletePassword() error = %v", err)
	}
	if _, exists := store.values["_password:abc"]; exists {
		t.Fatalf("password key still exists")
	}
	if store.values["abc"] != "ordinary" {
		t.Fatalf("ordinary secret was changed")
	}
}

func TestListPasswordSummariesNeverCarriesValues(t *testing.T) {
	store := newMemorySecretStore()
	saved, err := SavePassword(store, domain.PasswordEntry{Name: "Email", Username: "u@x.test", Password: "s3cret", URL: "https://mail.test"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	summaries, err := ListPasswordSummaries(store)
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != saved.ID || summaries[0].Name != "Email" {
		t.Fatalf("summaries = %+v", summaries)
	}
	encoded, err := json.Marshal(summaries)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "s3cret") || strings.Contains(strings.ToLower(string(encoded)), "password") {
		t.Fatalf("summary payload leaks secret material: %s", encoded)
	}
}

func TestGetPasswordByIDAndName(t *testing.T) {
	store := newMemorySecretStore()
	saved, err := SavePassword(store, domain.PasswordEntry{Name: "Portal RH", Password: "valor"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	byID, err := GetPassword(store, saved.ID)
	if err != nil || byID.Password != "valor" {
		t.Fatalf("by id: %+v err=%v", byID, err)
	}
	byName, err := GetPassword(store, "portal rh")
	if err != nil || byName.ID != saved.ID {
		t.Fatalf("by name: %+v err=%v", byName, err)
	}
	if _, err := GetPassword(store, "inexistente"); err == nil {
		t.Fatalf("expected not-found error")
	}
	if _, err := GetPassword(store, "  "); err == nil {
		t.Fatalf("expected error on blank id")
	}
}
