package macosperm

import "aw/internal/domain"

// Adapter is the ports.MacosPermissions adapter over the package's
// side-effecting helpers. The static inventory (All) is pure data and is read
// directly by callers; only the OS-touching operations go through this seam.
type Adapter struct{}

// NewAdapter returns the macOS permissions adapter.
func NewAdapter() Adapter { return Adapter{} }

// OpenPane implements ports.MacosPermissions.
func (Adapter) OpenPane(deepLink string) error { return OpenPane(deepLink) }

// Probe implements ports.MacosPermissions, mapping the result onto domain types.
func (Adapter) Probe(id string) domain.MacosProbeResult {
	result := Probe(id)
	return domain.MacosProbeResult{
		ID:     result.ID,
		Status: string(result.Status),
		Detail: result.Detail,
	}
}
