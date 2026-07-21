package application

import (
	"fmt"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// SelfDevInstruction builds the extra agent instruction injected when self-dev
// mode is enabled. It encodes how aw should behave when editing its own source
// code, parameterised by the capabilities the interface layer wired in. Keeping
// it here (instead of the composition root) makes the rule pure and testable.
func SelfDevInstruction(repoRoot string, allowShell bool, selfManage bool) string {
	capabilityText := "read and write files anywhere on this machine"
	toolsText := "aw actions fs.read, fs.write, fs.edit and fs.list"
	if allowShell {
		capabilityText = "read, write and execute code anywhere on this machine"
		toolsText = "aw actions fs.read, fs.write, fs.edit, fs.list and shell.exec"
	}
	instruction := fmt.Sprintf(`Self-dev mode is ON. You can %s via %s. You are editing your own source code at "%s". After changing Go code, validate with go run ./tools/buildgate before declaring success.`, capabilityText, toolsText, repoRoot)
	if allowShell {
		instruction += ` Shell execution is intentionally enabled for self-dev and may be auto-approved by runtime configuration; keep commands scoped to the requested change and do not use shell output to expose secrets.`
	}
	if selfManage {
		instruction += `

You can self-manage aw via the aw tool. Always start self-management tasks by calling aw with action system.selfcode to load the repo map, check commands and change rules. Use system.state to inspect the running app, fs.* to read/write source, shell.exec to run builds/tests, and git.* to version.`
	}
	return instruction
}

// SelfDevRuntimeProvider is the typed, serialisable view of the active provider
// runtime configuration exposed to the self-dev `system.state` action.
type SelfDevRuntimeProvider struct {
	ProviderID   string `json:"providerId"`
	ProviderName string `json:"providerName"`
	AuthType     string `json:"authType"`
	Model        string `json:"model"`
	BaseURL      string `json:"baseURL"`
}

// SelfDevVaultStore is the union of ports needed to assemble the vault-derived
// portion of the self-dev state snapshot.
type SelfDevVaultStore interface {
	ports.ProviderSecretStore
	ports.VaultUnlockState
	ports.ChatReader
}

// SelfDevVaultState is the business slice of the self-dev state: provider status,
// active runtime provider and (when unlocked) the chat list. The composition
// root combines it with its own runtime fields.
type SelfDevVaultState struct {
	ProviderStatus       domain.ProviderStatus
	RuntimeProvider      *SelfDevRuntimeProvider
	RuntimeProviderError string
	Unlocked             bool
	Chats                []domain.Chat
}

// LoadSelfDevVaultState gathers the vault-derived self-dev state from the store.
func LoadSelfDevVaultState(store SelfDevVaultStore) (SelfDevVaultState, error) {
	state := SelfDevVaultState{
		ProviderStatus: GetProviderStatus(store),
		Unlocked:       store.IsUnlocked(),
		Chats:          []domain.Chat{},
	}
	if cfg, err := ResolveProviderRuntimeConfig(store); err == nil {
		state.RuntimeProvider = &SelfDevRuntimeProvider{
			ProviderID:   cfg.ProviderID,
			ProviderName: cfg.ProviderName,
			AuthType:     cfg.AuthType,
			Model:        cfg.Model,
			BaseURL:      cfg.BaseURL,
		}
	} else {
		state.RuntimeProviderError = err.Error()
	}
	if state.Unlocked {
		chats, err := store.ListChats()
		if err != nil {
			return SelfDevVaultState{}, err
		}
		state.Chats = chats
	}
	return state, nil
}
