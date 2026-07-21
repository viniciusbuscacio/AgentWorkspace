package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncedPathProvider(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	cases := map[string]string{
		filepath.Join(home, "Library/CloudStorage/OneDrive-Pessoal/Vault"):    "Onedrive",
		filepath.Join(home, "Library/Mobile Documents/com~apple~CloudDocs/x"): "iCloud Drive",
		filepath.Join(home, "Dropbox/notes"):                                  "Dropbox",
		filepath.Join(home, "Vault"):                                          "",
		"/tmp/vault":                                                          "",
		"":                                                                    "",
	}
	for path, want := range cases {
		if got := SyncedPathProvider(path); got != want {
			t.Errorf("SyncedPathProvider(%q) = %q, want %q", path, got, want)
		}
	}
}
