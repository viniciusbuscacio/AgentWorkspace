package ports

// WorkspaceModuleStore persists which modules the user added to the workspace,
// which added modules are hidden (closed in the sidebar but still added — the
// agent actions stay registered) and the user's sidebar order. This is
// per-instance UI state (appconfig), not vault data. An absent or empty added
// list means "core modules only" — never an empty workspace.
type WorkspaceModuleStore interface {
	LoadAddedModules() ([]string, error)
	SaveAddedModules(ids []string) error
	LoadHiddenModules() ([]string, error)
	SaveHiddenModules(ids []string) error
	LoadModuleOrder() ([]string, error)
	SaveModuleOrder(ids []string) error
}

type WallpaperConfigStore interface {
	LoadWallpaper() (string, error)
	SaveWallpaper(id string) error
}
