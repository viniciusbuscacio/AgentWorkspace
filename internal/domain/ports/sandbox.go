package ports

import "aw/internal/domain"

// SandboxConfigStore persists the permissions sandbox configuration in the
// vault. Loading with nothing stored returns the default configuration.
type SandboxConfigStore interface {
	VaultUnlockState
	LoadSandboxConfig() (domain.SandboxConfig, error)
	SaveSandboxConfig(config domain.SandboxConfig) error
}
