//go:build awd_embed

package main

import (
	"embed"
	"io/fs"
)

// dist is the staged frontend build, embedded for single-file release binaries.
// go:embed is relative to this file's directory and cannot reach
// ../../frontend/dist, so the build pipeline stages it here first:
//
//	cp -r frontend/dist cmd/awd/dist && go build -tags awd_embed ./cmd/awd
//
//go:embed all:dist
var dist embed.FS

func loadAssets() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}
