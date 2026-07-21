package appcore

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tinyPNG is a valid 1x1 transparent PNG.
var tinyPNG, _ = base64.StdEncoding.DecodeString(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==")

// TestUploadWallpaperFromPathIsSandboxed pins the fence: the aw action's path
// is AGENT-chosen, so it must pass the same Permissions check as fs.* — a
// path outside the allowed roots is denied, one inside is imported.
func TestUploadWallpaperFromPathIsSandboxed(t *testing.T) {
	t.Setenv("aw_DATA_DIR", t.TempDir())
	workspace := t.TempDir()
	app := &App{workspaceRoot: workspace}

	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, tinyPNG, 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	result := app.UploadWallpaperFromPath(outside)
	if result.Success {
		t.Fatalf("agent upload outside the sandbox must be denied, got %+v", result)
	}
	if !strings.Contains(strings.ToLower(result.Error), "denied") && !strings.Contains(strings.ToLower(result.Error), "not allowed") {
		t.Logf("denial message: %s", result.Error)
	}

	inside := filepath.Join(workspace, "inside.png")
	if err := os.WriteFile(inside, tinyPNG, 0o600); err != nil {
		t.Fatalf("write inside: %v", err)
	}
	result = app.UploadWallpaperFromPath(inside)
	if !result.Success {
		t.Fatalf("upload inside the workspace should succeed, got error: %s", result.Error)
	}
	if !strings.HasPrefix(result.ID, "custom:") {
		t.Fatalf("expected a custom wallpaper id, got %q", result.ID)
	}
}
