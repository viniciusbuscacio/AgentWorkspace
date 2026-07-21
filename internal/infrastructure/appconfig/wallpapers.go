package appconfig

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// customWallpaperDirName is the on-disk folder (under BaseDir) holding
// user-uploaded wallpapers. It is separate from the bundled gallery, which
// the frontend serves from the embedded /wallpapers/ route.
const customWallpaperDirName = "custom-wallpapers"

// maxWallpaperBytes caps an uploaded image at 25 MB — generous for a photo,
// small enough to refuse a stray huge file before copying it.
const maxWallpaperBytes = 25 << 20

// CustomWallpaperDir returns the directory holding user-uploaded wallpapers.
func CustomWallpaperDir() string {
	return filepath.Join(BaseDir(), customWallpaperDirName)
}

// ImportWallpaper validates the image at srcPath, copies it into the
// custom-wallpaper dir under a sanitized, unique filename, and returns that
// filename. It rejects non-image files and oversized files.
func ImportWallpaper(srcPath string) (string, error) {
	srcPath = strings.TrimSpace(srcPath)
	if srcPath == "" {
		return "", fmt.Errorf("image path is required")
	}
	info, err := os.Stat(srcPath)
	if err != nil {
		return "", fmt.Errorf("cannot read image: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%q is a directory, not an image", srcPath)
	}
	if info.Size() > maxWallpaperBytes {
		return "", fmt.Errorf("image is too large (%d bytes; max %d)", info.Size(), maxWallpaperBytes)
	}
	data, err := os.ReadFile(srcPath) //nolint:gosec // user-chosen image path
	if err != nil {
		return "", fmt.Errorf("cannot read image: %w", err)
	}
	mime := http.DetectContentType(data)
	if !strings.HasPrefix(mime, "image/") {
		return "", fmt.Errorf("file is not an image (detected %s)", mime)
	}

	dir := CustomWallpaperDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	filename := uniqueWallpaperName(dir, sanitizeWallpaperName(filepath.Base(srcPath), mime))
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0o600); err != nil {
		return "", err
	}
	return filename, nil
}

// ListCustomWallpapers returns the stored upload filenames, sorted.
func ListCustomWallpapers() ([]string, error) {
	entries, err := os.ReadDir(CustomWallpaperDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// CustomWallpaperDataURI reads one stored upload and returns it as a
// base64 data URI the frontend can drop straight into a CSS url().
func CustomWallpaperDataURI(filename string) (string, error) {
	clean := sanitizeStoredName(filename)
	if clean == "" {
		return "", fmt.Errorf("invalid wallpaper name")
	}
	data, err := os.ReadFile(filepath.Join(CustomWallpaperDir(), clean)) //nolint:gosec // sanitized name within the data dir
	if err != nil {
		return "", err
	}
	mime := http.DetectContentType(data)
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// DeleteCustomWallpaper removes one stored upload. Missing files are ignored.
func DeleteCustomWallpaper(filename string) error {
	clean := sanitizeStoredName(filename)
	if clean == "" {
		return fmt.Errorf("invalid wallpaper name")
	}
	err := os.Remove(filepath.Join(CustomWallpaperDir(), clean))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CustomWallpaperExists reports whether a stored upload with this filename is
// present.
func CustomWallpaperExists(filename string) bool {
	clean := sanitizeStoredName(filename)
	if clean == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(CustomWallpaperDir(), clean))
	return err == nil && !info.IsDir()
}

// sanitizeStoredName strips any path components and rejects traversal, so a
// caller-supplied filename can only ever name a file inside the data dir.
func sanitizeStoredName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == ".." || name == "" || strings.ContainsAny(name, `/\`) {
		return ""
	}
	return name
}

// sanitizeWallpaperName lowercases the basename, keeps only safe characters,
// and ensures an extension matching the detected image type.
func sanitizeWallpaperName(base string, mime string) string {
	base = strings.ToLower(strings.TrimSpace(base))
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	var b strings.Builder
	for _, r := range stem {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	stem = strings.Trim(b.String(), "-")
	if stem == "" {
		stem = "wallpaper"
	}
	return stem + wallpaperExt(mime)
}

// wallpaperExt maps a detected image mime to a file extension.
func wallpaperExt(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

// uniqueWallpaperName appends -1, -2, ... until the name is free in dir.
func uniqueWallpaperName(dir, name string) string {
	if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
		return name
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
}
