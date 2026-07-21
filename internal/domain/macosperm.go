package domain

// MacosProbeResult is the outcome of a best-effort macOS TCC permission probe.
// Status is one of "granted", "denied" or "unknown".
type MacosProbeResult struct {
	ID     string
	Status string
	Detail string
}
