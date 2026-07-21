package application

import (
	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// LoadSelfDevConfig reads the persisted self-development posture through the
// config store port.
func LoadSelfDevConfig(store ports.SelfDevConfigStore) domain.SelfDevConfig {
	return store.LoadSelfDev()
}
