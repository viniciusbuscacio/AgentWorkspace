//go:build !awd_embed

package main

import (
	"io/fs"
	"os"
	"path/filepath"
)

// loadAssets (disk variant) serves the frontend from a directory on disk, so a
// plain `go build ./cmd/awd` works without staging dist into the source tree.
// It looks at AW_WEB_ASSETS_DIR, then a "dist" dir next to the binary. Returns
// (nil, nil) when no assets are found — the web server then answers 503 for the
// SPA while /login/healthz still work, which is enough to verify the daemon.
//
// Release builds embed the frontend instead: build with -tags awd_embed after
// staging frontend/dist into cmd/awd/dist (see assets_embed.go).
func loadAssets() (fs.FS, error) {
	if dir := os.Getenv("AW_WEB_ASSETS_DIR"); dir != "" {
		if isDir(dir) {
			return os.DirFS(dir), nil
		}
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "dist")
		if isDir(candidate) {
			return os.DirFS(candidate), nil
		}
	}
	return nil, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
