package appcore

import (
	"fmt"
	"strings"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
	"aw/internal/infrastructure/appconfig"
	"aw/internal/infrastructure/macosperm"
	"aw/internal/infrastructure/sandbox"
	"aw/internal/infrastructure/selfdev"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// sandboxSaveWarning is AW2's TouchID-gate message, ported to the native
// Wails dialog. It must stay outside the webview DOM: the agent drives the
// DOM via ui.click, so a web modal would let it approve its own change.
const sandboxSaveWarning = "Only approve this if you personally requested a Permissions change."

// GetSandboxSettings returns the Permissions configuration for Settings.
func (a *App) GetSandboxSettings() dto.SandboxSettingsResult {
	a.recordActivity()
	config, err := application.GetSandboxConfig(a.vault)
	if err != nil {
		return dto.SandboxSettingsResult{Error: err.Error()}
	}
	return dto.SandboxSettingsResult{Success: true, Settings: a.sandboxSettingsView(config)}
}

// SaveSandboxSettings validates and persists the Permissions configuration —
// after a NATIVE confirmation dialog (outside the DOM, so UI automation
// cannot approve it).
func (a *App) SaveSandboxSettings(mode string, allowedFolders []string) dto.SandboxSettingsResult {
	a.recordActivity()
	if a.ctx == nil {
		return dto.SandboxSettingsResult{Error: "the app window is not available for the confirmation dialog"}
	}
	choice, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Change Permissions?",
		Message:       fmt.Sprintf("%s\n\nNew mode: %s", sandboxSaveWarning, mode),
		Buttons:       []string{"Change Permissions", "Cancel"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		return dto.SandboxSettingsResult{Error: err.Error()}
	}
	answer := strings.ToLower(strings.TrimSpace(choice))
	if answer != "change permissions" && answer != "yes" && answer != "ok" {
		return dto.SandboxSettingsResult{Canceled: true, Error: "Permissions change was not approved"}
	}
	saved, err := application.SetSandboxConfig(a.vault, domain.SandboxConfig{
		Mode:           domain.SandboxMode(mode),
		AllowedFolders: allowedFolders,
	})
	if err != nil {
		return dto.SandboxSettingsResult{Error: err.Error()}
	}
	return dto.SandboxSettingsResult{Success: true, Settings: a.sandboxSettingsView(saved)}
}

// TestSandboxCommand dry-runs command against the current permissions sandbox
// without executing it. The Settings test box calls this so the result is
// produced by the same CheckCommand logic the agent tools layer uses — the
// two cannot drift (Spec Risk 2).
func (a *App) TestSandboxCommand(command string) dto.SandboxTestResult {
	a.recordActivity()
	config, err := application.GetSandboxConfig(a.vault)
	if err != nil {
		return dto.SandboxTestResult{
			Reason: "Could not load sandbox config: " + err.Error(),
			Error:  err.Error(),
		}
	}
	workspaceRoot := a.workspaceRoot
	selfDevConfig := application.LoadSelfDevConfig(appconfig.Store{})
	if selfDevConfig.Enabled {
		if repoRoot, err := selfdev.ResolveRepoRoot(selfDevConfig.RepoRoot); err == nil {
			workspaceRoot = repoRoot
		}
	}
	policy := sandbox.Policy{
		Config:        config,
		WorkspaceRoot: workspaceRoot,
		DataDir:       appconfig.BaseDir(),
	}
	check := sandbox.CheckCommand(command, "", policy)
	reason := check.Reason
	if reason == "" && check.Allowed {
		reason = "Allowed — no restricted paths detected."
	}
	return dto.SandboxTestResult{
		Allowed: check.Allowed,
		Reason:  reason,
		Paths:   check.Paths,
	}
}

// sandboxSettingsView assembles the Settings view of a configuration: the
// built-in lists, the effective workspace root (repo in self-dev) and the
// resulting visible roots.
func (a *App) sandboxSettingsView(config domain.SandboxConfig) *dto.SandboxSettings {
	workspaceRoot := a.workspaceRoot
	selfDev := false
	selfDevConfig := application.LoadSelfDevConfig(appconfig.Store{})
	if selfDevConfig.Enabled {
		if repoRoot, err := selfdev.ResolveRepoRoot(selfDevConfig.RepoRoot); err == nil {
			selfDev = true
			workspaceRoot = repoRoot
		}
	}
	effective := config
	if selfDev {
		effective = domain.SandboxConfig{Mode: domain.SandboxPermitAll}
	}
	roots := sandbox.Roots(sandbox.Policy{
		Config:        effective,
		WorkspaceRoot: workspaceRoot,
		DataDir:       appconfig.BaseDir(),
	})
	return &dto.SandboxSettings{
		Mode:           string(config.Mode),
		AllowedFolders: config.AllowedFolders,
		BuiltinDenied:  domain.SandboxBuiltinDenied(),
		BuiltinAllowed: domain.SandboxBuiltinAllowedPermitList(),
		WorkspaceRoot:  workspaceRoot,
		SelfDev:        selfDev,
		Roots:          roots,
	}
}

// GetMacosPermissions returns the macOS TCC permission inventory for the
// Settings security card. Returns an empty Items slice on non-macOS platforms
// so the frontend hides the card automatically (Decision 1).
func (a *App) GetMacosPermissions() dto.MacosPermissionsResult {
	a.recordActivity()
	perms := macosperm.All()
	items := make([]dto.MacosPermission, len(perms))
	for i, p := range perms {
		items[i] = dto.MacosPermission{
			ID:           p.ID,
			Name:         p.Name,
			Why:          p.Why,
			SettingsPane: p.SettingsPane,
			DeepLink:     p.DeepLink,
			HasProbe:     p.HasProbe,
		}
	}
	return dto.MacosPermissionsResult{Items: items}
}

// OpenSystemSettingsPane opens the given macOS System Settings pane by its
// deep link. The link is validated against the known permission inventory
// (injection guard); unknown links degrade to the Privacy & Security root
// pane. No-op on non-macOS platforms.
func (a *App) OpenSystemSettingsPane(deepLink string) dto.OperationResult {
	a.recordActivity()
	if err := application.OpenSystemSettingsPane(macosperm.NewAdapter(), strings.TrimSpace(deepLink)); err != nil {
		return dto.OperationResult{Error: fmt.Sprintf("open System Settings pane: %v", err)}
	}
	return dto.OperationResult{Success: true}
}

// ProbeMacosPermission runs the cheap best-effort status probe for a single
// TCC permission. Only fires when the user explicitly clicks Test — never on
// page load. Returns ProbeUnknown on non-macOS or for ids without a probe.
func (a *App) ProbeMacosPermission(id string) dto.MacosProbeResult {
	a.recordActivity()
	res := application.ProbeMacosPermission(macosperm.NewAdapter(), strings.TrimSpace(id))
	return dto.MacosProbeResult{
		ID:     res.ID,
		Status: res.Status,
		Detail: res.Detail,
	}
}
