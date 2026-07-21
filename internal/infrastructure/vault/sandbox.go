package vault

import (
	"encoding/json"

	"aw/internal/domain"
)

// Sandbox (Permissions) configuration persists as internal vault secrets —
// never in config.json, which the agent could edit via fs.write to widen its
// own fence. The "_" prefix keeps the keys out of the Settings secret list
// (the SecurityPage filter), mirroring AW2's _config_* convention.
const (
	sandboxModeSecret           = "_config_sandbox_mode"
	sandboxAllowedFoldersSecret = "_config_sandbox_allowed_folders"
)

// LoadSandboxConfig reads the persisted permissions configuration, falling
// back to the default (permit_list, empty folder list) for missing or
// malformed entries. An unknown stored mode also falls back to the default —
// which is how a vault saved in the removed deny_list mode fails safe to
// permit_list.
func (v *Vault) LoadSandboxConfig() (domain.SandboxConfig, error) {
	config := domain.DefaultSandboxConfig()
	mode, exists, err := v.GetSecret(sandboxModeSecret)
	if err != nil {
		return domain.SandboxConfig{}, err
	}
	if exists && domain.SandboxMode(mode).Valid() {
		config.Mode = domain.SandboxMode(mode)
	}
	if config.AllowedFolders, err = v.loadSandboxList(sandboxAllowedFoldersSecret); err != nil {
		return domain.SandboxConfig{}, err
	}
	return config, nil
}

// SaveSandboxConfig persists the permissions configuration as the two
// internal secrets.
func (v *Vault) SaveSandboxConfig(config domain.SandboxConfig) error {
	if err := v.SetSecret(sandboxModeSecret, string(config.Mode)); err != nil {
		return err
	}
	return v.saveSandboxList(sandboxAllowedFoldersSecret, config.AllowedFolders)
}

func (v *Vault) loadSandboxList(secretName string) ([]string, error) {
	raw, exists, err := v.GetSecret(secretName)
	if err != nil {
		return nil, err
	}
	list := []string{}
	if !exists {
		return list, nil
	}
	// Malformed JSON degrades to an empty list (AW2 behavior) — the user can
	// re-save from Settings; failing here would brick every tool dispatch.
	if err := json.Unmarshal([]byte(raw), &list); err != nil || list == nil {
		return []string{}, nil
	}
	return list, nil
}

func (v *Vault) saveSandboxList(secretName string, list []string) error {
	if list == nil {
		list = []string{}
	}
	data, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return v.SetSecret(secretName, string(data))
}
