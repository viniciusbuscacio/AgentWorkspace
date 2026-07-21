package application

import (
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// WallpaperIDs lives in domain so the tools layer can show the agent the
// full gallery (architecture: infrastructure imports domain, not application).
var WallpaperIDs = domain.WallpaperIDs

func NormalizeWallpaper(id string) string {
	id = strings.TrimSpace(id)
	// Custom uploads carry an opaque filename after the prefix; keep it as-is
	// (existence is checked by the composition root, which owns the files).
	if domain.IsCustomWallpaperID(id) {
		return id
	}
	id = strings.ToLower(id)
	if id == "" {
		return "default"
	}
	for _, valid := range WallpaperIDs {
		if id == valid {
			return id
		}
	}
	return "default"
}

func ValidateWallpaper(id string) (string, error) {
	id = strings.TrimSpace(id)
	if domain.IsCustomWallpaperID(id) {
		if domain.CustomWallpaperFilename(id) == "" {
			return "", fmt.Errorf("custom wallpaper id is missing its filename")
		}
		return id, nil
	}
	id = strings.ToLower(id)
	for _, valid := range WallpaperIDs {
		if id == valid {
			return id, nil
		}
	}
	return "", fmt.Errorf("unknown wallpaper %q (wallpapers: %s)", id, strings.Join(WallpaperIDs, ", "))
}

func GetWallpaper(store ports.WallpaperConfigStore) (string, error) {
	if store == nil {
		return "default", nil
	}
	id, err := store.LoadWallpaper()
	if err != nil {
		return "", err
	}
	return NormalizeWallpaper(id), nil
}

func SetWallpaper(store ports.WallpaperConfigStore, id string) (string, error) {
	id, err := ValidateWallpaper(id)
	if err != nil {
		return "", err
	}
	if store == nil {
		return "", fmt.Errorf("wallpaper config store is required")
	}
	if err := store.SaveWallpaper(id); err != nil {
		return "", err
	}
	return id, nil
}

// DefaultWallpaperGlass is the glass-slider default applied when no value has
// been persisted.
const DefaultWallpaperGlass = 20

// GetWallpaperGlass returns the persisted glass-slider value (0–100), or the
// default when unset or on error.
func GetWallpaperGlass(store ports.WallpaperGlassStore) int {
	v, err := store.LoadWallpaperGlass()
	if err != nil {
		return DefaultWallpaperGlass
	}
	return v
}

// SetWallpaperGlass validates and persists the glass-slider value (0–100).
func SetWallpaperGlass(store ports.WallpaperGlassStore, opacity int) error {
	if opacity < 0 || opacity > 100 {
		return fmt.Errorf("wallpaperGlass must be 0–100")
	}
	return store.SaveWallpaperGlass(opacity)
}

// CustomWallpaperExists reports whether the upload behind a custom id still
// exists on disk.
func CustomWallpaperExists(store ports.CustomWallpaperStore, id string) bool {
	name := domain.CustomWallpaperFilename(id)
	if name == "" {
		return false
	}
	return store.CustomWallpaperExists(name)
}

// CustomWallpaperImage returns one upload (by custom id) as a data URI.
func CustomWallpaperImage(store ports.CustomWallpaperStore, id string) (string, error) {
	name := domain.CustomWallpaperFilename(id)
	if name == "" {
		return "", fmt.Errorf("not a custom wallpaper id")
	}
	return store.CustomWallpaperDataURI(name)
}

// CustomWallpaperIDs lists the stored uploads as "custom:<file>" ids.
func CustomWallpaperIDs(store ports.CustomWallpaperStore) []string {
	names, err := store.ListCustomWallpapers()
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(names))
	for _, name := range names {
		ids = append(ids, domain.CustomWallpaperPrefix+name)
	}
	return ids
}

// ImportCustomWallpaper copies the image at path into the custom wallpaper dir
// and returns its "custom:<file>" id.
func ImportCustomWallpaper(store ports.CustomWallpaperStore, path string) (string, error) {
	filename, err := store.ImportWallpaper(path)
	if err != nil {
		return "", err
	}
	return domain.CustomWallpaperPrefix + filename, nil
}

// DeleteCustomWallpaperByID removes the upload behind a custom id.
func DeleteCustomWallpaperByID(store ports.CustomWallpaperStore, id string) error {
	name := domain.CustomWallpaperFilename(id)
	if name == "" {
		return fmt.Errorf("not a custom wallpaper id")
	}
	return store.DeleteCustomWallpaper(name)
}
