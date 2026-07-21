package application

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

const PasswordSecretPrefix = "_password:"

func ListPasswords(store ports.SecretStore) ([]domain.PasswordEntry, error) {
	if store == nil {
		return nil, fmt.Errorf("secret store is required")
	}
	names, err := store.ListSecrets()
	if err != nil {
		return nil, err
	}
	entries := make([]domain.PasswordEntry, 0)
	for _, name := range names {
		if !strings.HasPrefix(name, PasswordSecretPrefix) {
			continue
		}
		raw, exists, err := store.GetSecret(name)
		if err != nil || !exists {
			continue
		}
		var entry domain.PasswordEntry
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			continue
		}
		if entry.ID == "" {
			entry.ID = strings.TrimPrefix(name, PasswordSecretPrefix)
		}
		if strings.TrimSpace(entry.Name) == "" {
			entry.Name = entry.ID
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].UpdatedAt == entries[j].UpdatedAt {
			return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		}
		return entries[i].UpdatedAt > entries[j].UpdatedAt
	})
	return entries, nil
}

// ListPasswordSummaries is the agent-facing catalog: every credential the
// user stored (the module is shared-with-the-agent by design), but only
// metadata — values travel exclusively through GetPassword.
func ListPasswordSummaries(store ports.SecretStore) ([]domain.PasswordSummary, error) {
	entries, err := ListPasswords(store)
	if err != nil {
		return nil, err
	}
	summaries := make([]domain.PasswordSummary, 0, len(entries))
	for _, entry := range entries {
		summaries = append(summaries, entry.Summary())
	}
	return summaries, nil
}

// GetPassword resolves a credential by id, or by exact (case-insensitive)
// name when the id does not match.
func GetPassword(store ports.SecretStore, idOrName string) (domain.PasswordEntry, error) {
	idOrName = strings.TrimSpace(idOrName)
	if idOrName == "" {
		return domain.PasswordEntry{}, fmt.Errorf("id (or name) is required")
	}
	entries, err := ListPasswords(store)
	if err != nil {
		return domain.PasswordEntry{}, err
	}
	for _, entry := range entries {
		if entry.ID == idOrName {
			return entry, nil
		}
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name, idOrName) {
			return entry, nil
		}
	}
	return domain.PasswordEntry{}, fmt.Errorf("no credential with id or name %q", idOrName)
}

func SavePassword(store ports.SecretStore, entry domain.PasswordEntry) (domain.PasswordEntry, error) {
	if store == nil {
		return domain.PasswordEntry{}, fmt.Errorf("secret store is required")
	}
	entry.ID = strings.TrimSpace(entry.ID)
	if entry.ID == "" {
		entry.ID = newPasswordID()
	}
	entry.Name = strings.TrimSpace(entry.Name)
	if entry.Name == "" {
		return domain.PasswordEntry{}, fmt.Errorf("name is required")
	}
	entry.UpdatedAt = time.Now().UnixMilli()
	data, err := json.Marshal(entry)
	if err != nil {
		return domain.PasswordEntry{}, err
	}
	if err := store.SetSecret(PasswordSecretPrefix+entry.ID, string(data)); err != nil {
		return domain.PasswordEntry{}, err
	}
	return entry, nil
}

func DeletePassword(store ports.SecretStore, id string) error {
	if store == nil {
		return fmt.Errorf("secret store is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return store.DeleteSecret(PasswordSecretPrefix + id)
}

func newPasswordID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
