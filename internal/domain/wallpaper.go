package domain

import "strings"

// WallpaperIDs is the bundled gallery — the single source for validation,
// the agent-facing tool description and the docs. The frontend mirrors it
// in wallpaper-data.ts; keep both in sync.
var WallpaperIDs = []string{
	"default", "mountain-lake", "northern-lights", "ocean-waves", "forest-path",
	"desert-dunes", "starry-night", "misty-mountains", "tropical-beach", "autumn-forest",
	"cherry-blossoms", "waterfall", "lavender-fields", "city-lights", "tokyo-night",
	"new-york-skyline", "hong-kong", "london-bridge", "earth-from-space", "nebula",
	"milky-way", "full-moon", "dark-geometry", "gradient-blur",
}

// CustomWallpaperPrefix marks a user-uploaded wallpaper. The id is
// "custom:<filename>", where <filename> lives in the custom-wallpapers data
// dir and is served to the frontend as a data URI (not a bundled asset).
const CustomWallpaperPrefix = "custom:"

// IsCustomWallpaperID reports whether id refers to a user-uploaded wallpaper.
func IsCustomWallpaperID(id string) bool {
	return strings.HasPrefix(id, CustomWallpaperPrefix)
}

// CustomWallpaperFilename extracts the stored filename from a custom id, or
// "" when id is not a custom wallpaper.
func CustomWallpaperFilename(id string) string {
	if !IsCustomWallpaperID(id) {
		return ""
	}
	return strings.TrimPrefix(id, CustomWallpaperPrefix)
}
