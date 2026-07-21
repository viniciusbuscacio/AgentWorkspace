package ports

import "aw/internal/domain"

// ObsidianVault performs the vault-folder-jailed file operations of the
// Obsidian module. The filesystem mechanics (jail resolution, walking,
// caps) live in infrastructure.
type ObsidianVault interface {
	List(vaultDir, folder string) ([]domain.ObsidianEntry, error)
	Search(vaultDir, query string, limit int) ([]domain.ObsidianMatch, error)
	Read(vaultDir, rel string) (string, error)
	Write(vaultDir, rel, content string) error
	Append(vaultDir, rel, content string) error
	Delete(vaultDir, rel string) error
	AlwaysReadBlock(vaultDir string, files []string) string
	AlwaysReadSize(vaultDir string, files []string) int
}

// ObsidianConfigStore persists the Obsidian module setup.
type ObsidianConfigStore interface {
	LoadObsidianConfig() domain.ObsidianConfig
	SaveObsidianConfig(cfg domain.ObsidianConfig) error
}
