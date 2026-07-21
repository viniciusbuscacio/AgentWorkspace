package domain

// ObsidianConfig is the Obsidian module's setup: the vault folder every
// obsidian.* action is jailed to, whether the agent may write notes, and the
// notes injected into the agent prompt on every turn.
type ObsidianConfig struct {
	Enabled      bool   `json:"enabled,omitempty"`
	VaultDir     string `json:"vaultDir,omitempty"`
	WriteEnabled bool   `json:"writeEnabled,omitempty"`
	// DeleteEnabled gates obsidian.delete (notes and folders) separately
	// from create/update.
	DeleteEnabled bool     `json:"deleteEnabled,omitempty"`
	AlwaysRead    []string `json:"alwaysRead,omitempty"`
}

// IsZero reports an entirely unset config (used by the config-save merge).
func (c ObsidianConfig) IsZero() bool {
	return !c.Enabled && c.VaultDir == "" && !c.WriteEnabled && !c.DeleteEnabled && len(c.AlwaysRead) == 0
}

// ObsidianEntry is one list result (vault-relative path).
type ObsidianEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size,omitempty"`
}

// ObsidianMatch is one search result.
type ObsidianMatch struct {
	Path    string `json:"path"`
	Snippet string `json:"snippet,omitempty"`
}
