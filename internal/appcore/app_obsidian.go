package appcore

import (
	"path/filepath"
	"strings"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
	"aw/internal/infrastructure/appconfig"
	obsidianpkg "aw/internal/infrastructure/obsidian"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetObsidianSettings returns the Obsidian module setup for the module page.
func (a *App) GetObsidianSettings() dto.ObsidianSettings {
	a.recordActivity()
	cfg := application.GetObsidianSettings(appconfig.Store{})
	return dto.ObsidianSettings{
		Enabled:         cfg.Enabled,
		VaultDir:        cfg.VaultDir,
		WriteEnabled:    cfg.WriteEnabled,
		DeleteEnabled:   cfg.DeleteEnabled,
		AlwaysRead:      append([]string(nil), cfg.AlwaysRead...),
		AlwaysReadChars: application.ObsidianAlwaysReadSize(appconfig.Store{}, obsidianpkg.New()),
	}
}

// SetObsidianSettings persists the module setup. Paths are not secrets; they
// live in config.json like the browser paths.
func (a *App) SetObsidianSettings(settings dto.ObsidianSettings) dto.OperationResult {
	a.recordActivity()
	files := make([]string, 0, len(settings.AlwaysRead))
	for _, f := range settings.AlwaysRead {
		if f = strings.TrimSpace(f); f != "" {
			files = append(files, f)
		}
	}
	err := application.SetObsidianSettings(appconfig.Store{}, domain.ObsidianConfig{
		Enabled:       settings.Enabled,
		VaultDir:      settings.VaultDir,
		WriteEnabled:  settings.WriteEnabled,
		DeleteEnabled: settings.DeleteEnabled,
		AlwaysRead:    files,
	})
	return a.basicOperationResult(err)
}

// ChooseObsidianVaultDir opens the native folder picker for the vault folder.
func (a *App) ChooseObsidianVaultDir() (FolderDialogResult, error) {
	a.recordActivity()
	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select your Obsidian vault folder",
	})
	if err != nil {
		return FolderDialogResult{Canceled: true}, err
	}
	if dir == "" {
		return FolderDialogResult{Canceled: true}, nil
	}
	return FolderDialogResult{Canceled: false, Path: dir}, nil
}

// ChooseObsidianAlwaysReadFile opens the native file picker inside the vault
// and returns the picked file as a vault-relative path. Files outside the
// vault are rejected.
func (a *App) ChooseObsidianAlwaysReadFile(vaultDir string) dto.ObsidianFileDialogResult {
	a.recordActivity()
	// The caller passes the page's DRAFT folder so picking notes works before
	// the first Save; empty falls back to the saved config.
	vaultDir = strings.TrimSpace(vaultDir)
	if vaultDir == "" {
		vaultDir = strings.TrimSpace(application.GetObsidianSettings(appconfig.Store{}).VaultDir)
	}
	if vaultDir == "" {
		return dto.ObsidianFileDialogResult{Canceled: true, Error: "set the vault folder first"}
	}
	file, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Pick a note to always load into the agent prompt",
		DefaultDirectory: vaultDir,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Markdown notes (*.md)", Pattern: "*.md"},
		},
	})
	if err != nil {
		return dto.ObsidianFileDialogResult{Canceled: true, Error: err.Error()}
	}
	if file == "" {
		return dto.ObsidianFileDialogResult{Canceled: true}
	}
	rel, err := filepath.Rel(vaultDir, file)
	if err != nil || strings.HasPrefix(rel, "..") {
		return dto.ObsidianFileDialogResult{Canceled: true, Error: "the note must be inside the vault folder"}
	}
	return dto.ObsidianFileDialogResult{Path: filepath.ToSlash(rel)}
}

// TestObsidianAccess verifies the vault folder and always-read notes are
// reachable, for the module page's Test button.
func (a *App) TestObsidianAccess() dto.ObsidianTestResult {
	a.recordActivity()
	summary, err := application.ObsidianTestAccess(appconfig.Store{}, obsidianpkg.New())
	if err != nil {
		return dto.ObsidianTestResult{Success: false, Error: err.Error()}
	}
	return dto.ObsidianTestResult{Success: true, Message: summary}
}
