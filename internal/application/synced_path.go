package application

import (
	"os"
	"path/filepath"
	"strings"
)

// SyncedPathProvider names the cloud-sync service a path lives under, or ""
// when the path looks local. A live SQLite vault inside a synced folder is
// how "sqlite3: disk I/O error" (and eventual corruption) happens — the sync
// client locks/evicts/replaces file blocks under the open connection.
func SyncedPathProvider(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return ""
	}
	home, _ := os.UserHomeDir()
	lower := strings.ToLower(filepath.ToSlash(path))
	if home != "" {
		h := strings.ToLower(filepath.ToSlash(home))
		// macOS: every modern File Provider sync (OneDrive, Google Drive,
		// Dropbox, Box) mounts under ~/Library/CloudStorage/<Provider>-...
		if strings.HasPrefix(lower, h+"/library/cloudstorage/") {
			rest := strings.TrimPrefix(lower, h+"/library/cloudstorage/")
			if i := strings.IndexAny(rest, "-/"); i > 0 {
				return strings.Title(rest[:i]) //nolint:staticcheck // provider slug, ASCII
			}
			return "CloudStorage"
		}
		// iCloud Drive.
		if strings.HasPrefix(lower, h+"/library/mobile documents/") {
			return "iCloud Drive"
		}
		// Legacy direct-folder clients.
		for name, provider := range map[string]string{
			"/dropbox": "Dropbox", "/onedrive": "OneDrive", "/google drive": "Google Drive",
		} {
			if strings.HasPrefix(lower, h+name+"/") || lower == h+name {
				return provider
			}
		}
	}
	// Windows: the OneDrive client exports its root via environment variables.
	for _, env := range []string{"OneDrive", "OneDriveConsumer", "OneDriveCommercial"} {
		if root := strings.TrimSpace(os.Getenv(env)); root != "" {
			r := strings.ToLower(filepath.ToSlash(filepath.Clean(root)))
			if strings.HasPrefix(lower, r+"/") || lower == r {
				return "OneDrive"
			}
		}
	}
	return ""
}
