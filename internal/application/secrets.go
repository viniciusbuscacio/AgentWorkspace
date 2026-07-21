package application

import (
	"fmt"

	"aw/internal/domain/ports"
)

type SecretValue struct {
	Value  string
	Exists bool
}

func SetSecret(store ports.SecretStore, name string, value string) error {
	if store == nil {
		return fmt.Errorf("secret store is required")
	}
	return store.SetSecret(name, value)
}

func GetSecret(store ports.SecretStore, name string) (SecretValue, error) {
	if store == nil {
		return SecretValue{}, fmt.Errorf("secret store is required")
	}
	value, exists, err := store.GetSecret(name)
	return SecretValue{Value: value, Exists: exists}, err
}

func HasSecret(store ports.SecretStore, name string) (bool, error) {
	if store == nil {
		return false, fmt.Errorf("secret store is required")
	}
	return store.HasSecret(name)
}

func ListSecrets(store ports.SecretStore) ([]string, error) {
	if store == nil {
		return nil, fmt.Errorf("secret store is required")
	}
	return store.ListSecrets()
}

func DeleteSecret(store ports.SecretStore, name string) error {
	if store == nil {
		return fmt.Errorf("secret store is required")
	}
	return store.DeleteSecret(name)
}
