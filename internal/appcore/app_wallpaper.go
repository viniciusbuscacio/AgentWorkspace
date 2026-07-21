package appcore

import (
	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
	"aw/internal/infrastructure/appconfig"
	"aw/internal/infrastructure/sandbox"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetWallpaper() string {
	id, err := application.GetWallpaper(a.workspace)
	if err != nil {
		return "default"
	}
	return id
}

func (a *App) SetWallpaper(id string) dto.OperationResult {
	a.recordActivity()
	// Custom uploads must still exist on disk; fall back if the file is gone.
	if domain.IsCustomWallpaperID(id) && !application.CustomWallpaperExists(appconfig.Store{}, id) {
		return dto.OperationResult{Error: "that uploaded wallpaper no longer exists"}
	}
	saved, err := application.SetWallpaper(a.workspace, id)
	if err != nil {
		return dto.OperationResult{Error: err.Error()}
	}
	a.emitUIEvent("ui:set-wallpaper", map[string]any{"id": saved})
	return dto.OperationResult{Success: true}
}

// ListCustomWallpapers returns the ids of all user-uploaded wallpapers.
func (a *App) ListCustomWallpapers() dto.WallpaperUploadResult {
	return dto.WallpaperUploadResult{Success: true, Custom: a.customWallpaperIDs()}
}

// GetCustomWallpaperImage returns one user-uploaded wallpaper as a data URI.
func (a *App) GetCustomWallpaperImage(id string) dto.WallpaperImageResult {
	uri, err := application.CustomWallpaperImage(appconfig.Store{}, id)
	if err != nil {
		return dto.WallpaperImageResult{Error: err.Error()}
	}
	return dto.WallpaperImageResult{Success: true, DataURI: uri}
}

// PickAndUploadWallpaper opens a native image picker, imports the chosen file
// as a custom wallpaper and selects it. A canceled picker is not an error.
func (a *App) PickAndUploadWallpaper() dto.WallpaperUploadResult {
	a.recordActivity()
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Upload New Wallpaper",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Images", Pattern: "*.jpg;*.jpeg;*.png;*.gif;*.webp"},
			{DisplayName: "All Files", Pattern: "*"},
		},
	})
	if err != nil {
		return dto.WallpaperUploadResult{Error: err.Error(), Custom: a.customWallpaperIDs()}
	}
	if path == "" {
		return dto.WallpaperUploadResult{Success: true, Canceled: true, Custom: a.customWallpaperIDs()}
	}
	return a.importWallpaper(path)
}

// UploadWallpaperFromPath imports the image at path as a custom wallpaper and
// selects it. Used by the aw tool (the agent supplies an absolute path) —
// unlike the native-picker path, where the USER chose the file, this path is
// AGENT-chosen and must pass the same Permissions fence as fs.*: without it,
// wallpaper upload is an unsandboxed read of any image on disk.
func (a *App) UploadWallpaperFromPath(path string) dto.WallpaperUploadResult {
	a.recordActivity()
	resolved, err := sandbox.ResolveAndCheck(path, a.sandboxPolicyFn(a.workspaceRoot, false)())
	if err != nil {
		return dto.WallpaperUploadResult{Error: err.Error(), Custom: a.customWallpaperIDs()}
	}
	return a.importWallpaper(resolved)
}

// DeleteCustomWallpaper removes a user-uploaded wallpaper. If it was the
// active one, the wallpaper falls back to the default.
func (a *App) DeleteCustomWallpaper(id string) dto.WallpaperUploadResult {
	a.recordActivity()
	if err := application.DeleteCustomWallpaperByID(appconfig.Store{}, id); err != nil {
		return dto.WallpaperUploadResult{Error: err.Error(), Custom: a.customWallpaperIDs()}
	}
	if a.GetWallpaper() == id {
		if saved, err := application.SetWallpaper(a.workspace, "default"); err == nil {
			a.emitUIEvent("ui:set-wallpaper", map[string]any{"id": saved})
		}
	}
	return dto.WallpaperUploadResult{Success: true, Custom: a.customWallpaperIDs()}
}

// importWallpaper copies the image into the data dir, selects it and emits the
// UI event. Shared by the picker and the path-based (aw tool) entry points.
func (a *App) importWallpaper(path string) dto.WallpaperUploadResult {
	id, err := application.ImportCustomWallpaper(appconfig.Store{}, path)
	if err != nil {
		return dto.WallpaperUploadResult{Error: err.Error(), Custom: a.customWallpaperIDs()}
	}
	saved, err := application.SetWallpaper(a.workspace, id)
	if err != nil {
		return dto.WallpaperUploadResult{Error: err.Error(), Custom: a.customWallpaperIDs()}
	}
	a.emitUIEvent("ui:set-wallpaper", map[string]any{"id": saved})
	return dto.WallpaperUploadResult{Success: true, ID: id, Custom: a.customWallpaperIDs()}
}

// customWallpaperIDs lists the stored uploads as "custom:<file>" ids.
func (a *App) customWallpaperIDs() []string {
	return application.CustomWallpaperIDs(appconfig.Store{})
}

// GetWallpaperGlass returns the glass-slider value (0–100; default 20).
func (a *App) GetWallpaperGlass() int {
	return application.GetWallpaperGlass(a.workspace)
}

// SetWallpaperGlass persists the slider value and emits ui:set-wallpaper-glass
// so any open AppShell updates its CSS variable.
func (a *App) SetWallpaperGlass(opacity int) dto.OperationResult {
	a.recordActivity()
	if err := application.SetWallpaperGlass(a.workspace, opacity); err != nil {
		return dto.OperationResult{Error: err.Error()}
	}
	a.emitUIEvent("ui:set-wallpaper-glass", map[string]any{"opacity": opacity})
	return dto.OperationResult{Success: true}
}
