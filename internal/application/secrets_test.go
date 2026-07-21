package application

import "testing"

type memorySecretStore struct {
	values map[string]string
}

func newMemorySecretStore() *memorySecretStore {
	return &memorySecretStore{values: map[string]string{}}
}

func (store *memorySecretStore) SetSecret(name string, value string) error {
	store.values[name] = value
	return nil
}

func (store *memorySecretStore) GetSecret(name string) (string, bool, error) {
	value, exists := store.values[name]
	return value, exists, nil
}

func (store *memorySecretStore) HasSecret(name string) (bool, error) {
	_, exists := store.values[name]
	return exists, nil
}

func (store *memorySecretStore) DeleteSecret(name string) error {
	delete(store.values, name)
	return nil
}

func (store *memorySecretStore) ListSecrets() ([]string, error) {
	names := make([]string, 0, len(store.values))
	for name := range store.values {
		names = append(names, name)
	}
	return names, nil
}

func TestSecretUseCasesCallStore(t *testing.T) {
	store := newMemorySecretStore()
	if err := SetSecret(store, "token", "abc"); err != nil {
		t.Fatalf("SetSecret() error = %v", err)
	}
	value, err := GetSecret(store, "token")
	if err != nil || !value.Exists || value.Value != "abc" {
		t.Fatalf("GetSecret() = %+v, %v; want stored value", value, err)
	}
	exists, err := HasSecret(store, "token")
	if err != nil || !exists {
		t.Fatalf("HasSecret() = %v, %v; want true", exists, err)
	}
	names, err := ListSecrets(store)
	if err != nil || len(names) != 1 || names[0] != "token" {
		t.Fatalf("ListSecrets() = %v, %v; want [token]", names, err)
	}
	if err := DeleteSecret(store, "token"); err != nil {
		t.Fatalf("DeleteSecret() error = %v", err)
	}
	if exists, _ := HasSecret(store, "token"); exists {
		t.Fatalf("secret should be deleted")
	}
}
