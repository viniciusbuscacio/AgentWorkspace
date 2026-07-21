package appconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pngBytes is the 8-byte PNG signature, enough for http.DetectContentType to
// classify the data as image/png.
var pngBytes = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}

func TestImportWallpaperRoundTrip(t *testing.T) {
	t.Setenv("aw_DATA_DIR", t.TempDir())

	src := filepath.Join(t.TempDir(), "My Photo!.png")
	if err := os.WriteFile(src, pngBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	name, err := ImportWallpaper(src)
	if err != nil {
		t.Fatalf("ImportWallpaper error = %v", err)
	}
	// Sanitized: lowercased, unsafe chars folded, .png extension kept.
	if name != "my-photo.png" {
		t.Fatalf("filename = %q, want my-photo.png", name)
	}

	list, err := ListCustomWallpapers()
	if err != nil {
		t.Fatalf("ListCustomWallpapers error = %v", err)
	}
	if len(list) != 1 || list[0] != name {
		t.Fatalf("list = %v, want [%s]", list, name)
	}

	if !CustomWallpaperExists(name) {
		t.Fatalf("CustomWallpaperExists(%q) = false", name)
	}

	uri, err := CustomWallpaperDataURI(name)
	if err != nil {
		t.Fatalf("CustomWallpaperDataURI error = %v", err)
	}
	if !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Fatalf("data uri = %q, want image/png prefix", uri)
	}

	// A second import of the same name must not collide.
	name2, err := ImportWallpaper(src)
	if err != nil {
		t.Fatalf("second ImportWallpaper error = %v", err)
	}
	if name2 == name {
		t.Fatalf("second import reused the name %q", name2)
	}

	if err := DeleteCustomWallpaper(name); err != nil {
		t.Fatalf("DeleteCustomWallpaper error = %v", err)
	}
	if CustomWallpaperExists(name) {
		t.Fatalf("wallpaper still exists after delete")
	}
}

func TestImportWallpaperRejectsNonImage(t *testing.T) {
	t.Setenv("aw_DATA_DIR", t.TempDir())
	src := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(src, []byte("just text, definitely not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportWallpaper(src); err == nil {
		t.Fatal("ImportWallpaper accepted a non-image file")
	}
}

func TestCustomWallpaperDataURIRejectsTraversal(t *testing.T) {
	t.Setenv("aw_DATA_DIR", t.TempDir())
	for _, bad := range []string{"../secret", "a/b", "..", ""} {
		if _, err := CustomWallpaperDataURI(bad); err == nil {
			t.Fatalf("CustomWallpaperDataURI(%q) should fail", bad)
		}
	}
}
