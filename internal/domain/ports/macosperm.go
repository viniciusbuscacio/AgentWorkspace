package ports

import "aw/internal/domain"

// MacosPermissions performs the side-effecting macOS TCC operations: opening a
// System Settings pane and probing a permission's status. Both shell out to the
// OS, so the interface layer must reach them through this port rather than the
// infrastructure helper directly. The static permission inventory is pure data
// and is read directly.
type MacosPermissions interface {
	OpenPane(deepLink string) error
	Probe(id string) domain.MacosProbeResult
}
