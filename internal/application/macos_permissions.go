package application

import (
	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// OpenSystemSettingsPane opens the given macOS System Settings pane through the
// permissions port. The deep link is validated inside the adapter (injection
// guard); unknown links degrade to the Privacy & Security root pane.
func OpenSystemSettingsPane(perms ports.MacosPermissions, deepLink string) error {
	return perms.OpenPane(deepLink)
}

// ProbeMacosPermission runs the cheap best-effort status probe for a single TCC
// permission through the permissions port.
func ProbeMacosPermission(perms ports.MacosPermissions, id string) domain.MacosProbeResult {
	return perms.Probe(id)
}
