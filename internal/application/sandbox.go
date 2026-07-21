package application

import (
	"errors"
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// GetSandboxConfig returns the persisted permissions configuration (the store
// falls back to the default permit_list when nothing is saved yet).
func GetSandboxConfig(store ports.SandboxConfigStore) (domain.SandboxConfig, error) {
	if store == nil {
		return domain.SandboxConfig{}, errors.New("sandbox config store is required")
	}
	if !store.IsUnlocked() {
		return domain.SandboxConfig{}, errVaultLocked
	}
	return store.LoadSandboxConfig()
}

// SetSandboxConfig validates and persists the permissions configuration:
// the mode must be one of the three known modes and the folder list is
// normalized (trimmed, empties dropped, duplicates removed). Returns the
// normalized configuration as saved.
func SetSandboxConfig(store ports.SandboxConfigStore, config domain.SandboxConfig) (domain.SandboxConfig, error) {
	if store == nil {
		return domain.SandboxConfig{}, errors.New("sandbox config store is required")
	}
	if !store.IsUnlocked() {
		return domain.SandboxConfig{}, errVaultLocked
	}
	if !config.Mode.Valid() {
		return domain.SandboxConfig{}, fmt.Errorf("invalid sandbox mode %q (use one of: %s)", config.Mode, sandboxModeNames())
	}
	config.AllowedFolders = NormalizeSandboxPathList(config.AllowedFolders)
	if err := store.SaveSandboxConfig(config); err != nil {
		return domain.SandboxConfig{}, err
	}
	return config, nil
}

// SandboxRuntimeStore is the slice of the vault the per-dispatch sandbox
// policy needs: the persisted configuration plus the vault directory (a path
// the fence always protects, wherever the user placed it).
type SandboxRuntimeStore interface {
	ports.SandboxConfigStore
	Dir() string
}

// SandboxRuntime carries the vault-derived inputs of the per-dispatch
// permissions policy.
type SandboxRuntime struct {
	Config   domain.SandboxConfig
	VaultDir string
}

// LoadSandboxRuntime reads the sandbox policy inputs for one tool dispatch.
// Fail-closed: a locked vault or a load error yields the default permit_list
// configuration instead of failing the dispatch open.
func LoadSandboxRuntime(store SandboxRuntimeStore) SandboxRuntime {
	runtime := SandboxRuntime{Config: domain.DefaultSandboxConfig()}
	if store == nil {
		return runtime
	}
	runtime.VaultDir = store.Dir()
	if store.IsUnlocked() {
		if config, err := store.LoadSandboxConfig(); err == nil {
			runtime.Config = config
		}
	}
	return runtime
}

// NormalizeSandboxPathList trims entries, drops empties and removes
// duplicates while preserving order. Entries stay as the user typed them
// (~/... included) — the checker expands at evaluation time.
func NormalizeSandboxPathList(list []string) []string {
	seen := map[string]bool{}
	normalized := []string{}
	for _, entry := range list {
		entry = strings.TrimSpace(entry)
		if entry == "" || seen[entry] {
			continue
		}
		seen[entry] = true
		normalized = append(normalized, entry)
	}
	return normalized
}

func sandboxModeNames() string {
	modes := domain.SandboxModes()
	names := make([]string, len(modes))
	for i, mode := range modes {
		names[i] = string(mode)
	}
	return strings.Join(names, ", ")
}

// SandboxPromptInput carries what the prompt block needs: the persisted
// config, the agent's workspace folder, and whether self-dev overrides the
// mode to permit_all (Decision 9).
type SandboxPromptInput struct {
	Store         ports.SandboxConfigStore
	WorkspaceRoot string
	SelfDev       bool
}

// SandboxPromptBlock resolves the effective configuration and renders the
// "### Filesystem Access" block. Returns "" with no store wired.
func SandboxPromptBlock(input SandboxPromptInput) string {
	if input.Store == nil {
		return ""
	}
	config := domain.DefaultSandboxConfig()
	if input.SelfDev {
		config = domain.SandboxConfig{Mode: domain.SandboxPermitAll}
	} else if loaded, err := GetSandboxConfig(input.Store); err == nil {
		config = loaded
	}
	return SandboxInstruction(config, input.WorkspaceRoot)
}

// SandboxInstruction renders the per-mode "### Filesystem Access" prompt
// block — a port of AW2's build-state-context templates. The prompt only
// informs the mode; enforcement happens in Go regardless of what the model
// believes.
func SandboxInstruction(config domain.SandboxConfig, workspaceRoot string) string {
	var b strings.Builder
	b.WriteString("### Filesystem Access\n")
	switch config.Mode {
	case domain.SandboxBlockAll:
		b.WriteString("Mode: block_all\n\n")
		b.WriteString("⚠️ **SANDBOX: BLOCK ALL** — You CANNOT execute any shell commands. ")
		b.WriteString("Do NOT attempt to run shell commands and do NOT retry blocked ones. ")
		b.WriteString("Tell the user to change Permissions in Settings if they ask for shell execution. ")
		b.WriteString(fmt.Sprintf("You CAN still read and write files within the agent workspace folder (%s) and use the `aw` actions normally.", workspaceRoot))
	case domain.SandboxPermitAll:
		b.WriteString("Mode: permit_all — no shell/path restrictions.")
	default: // permit_list, the default mode
		b.WriteString("Mode: permit_list\n")
		b.WriteString(fmt.Sprintf("SANDBOX: PERMIT LIST — You can ONLY run shell commands that touch files within the allowed folders listed below, the agent workspace folder (%s), %s. ",
			workspaceRoot, strings.Join(domain.SandboxBuiltinAllowedPermitList(), " and ")))
		b.WriteString("Commands accessing any other paths will be blocked. Do NOT attempt commands outside these folders.")
		if len(config.AllowedFolders) == 0 {
			b.WriteString("\nNo extra folders are allowed. The user can allow folders in Settings > Permissions.")
		} else {
			b.WriteString("\nAllowed folders:")
			for _, folder := range config.AllowedFolders {
				b.WriteString("\n- " + folder)
			}
		}
	}
	return b.String()
}
