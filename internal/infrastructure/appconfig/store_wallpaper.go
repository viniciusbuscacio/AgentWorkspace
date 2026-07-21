package appconfig

// Store methods wrapping the custom-wallpaper file operations so the interface
// layer reaches them through the same adapter (passed as a domain port) it uses
// for the rest of app config, instead of calling the package functions directly.

// CustomWallpaperExists reports whether the named upload still exists on disk.
func (Store) CustomWallpaperExists(filename string) bool {
	return CustomWallpaperExists(filename)
}

// CustomWallpaperDataURI returns one upload encoded as a data URI.
func (Store) CustomWallpaperDataURI(filename string) (string, error) {
	return CustomWallpaperDataURI(filename)
}

// DeleteCustomWallpaper removes an uploaded wallpaper file.
func (Store) DeleteCustomWallpaper(filename string) error {
	return DeleteCustomWallpaper(filename)
}

// ImportWallpaper copies the image at srcPath into the custom wallpaper dir and
// returns the stored filename.
func (Store) ImportWallpaper(srcPath string) (string, error) {
	return ImportWallpaper(srcPath)
}

// ListCustomWallpapers returns the stored upload filenames.
func (Store) ListCustomWallpapers() ([]string, error) {
	return ListCustomWallpapers()
}
