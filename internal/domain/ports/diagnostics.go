package ports

import (
	"context"

	"aw/internal/domain"
)

// DiagnosticsProbe is the OS-facing port for native system diagnostics. It is
// implemented by internal/infrastructure/diagnostics with per-OS build-tagged
// backends. Every method is read-only and one-shot: it collects currently
// available data once and returns a structured DTO with per-section status.
//
// Implementations must never return raw sensitive data (serials, MAC addresses,
// hostnames, hardware/device IDs, driver paths, command lines, large raw log
// payloads) — redaction happens at the source. Missing sensors, unavailable
// logs, and permission failures are normal partial results, not errors.
type DiagnosticsProbe interface {
	// Platform reports the OS family ("windows" | "darwin" | "linux" | other).
	Platform() string
	// Probes reports which low-level data sources are present, for the
	// capabilities action (e.g. nvidia-smi found, journalctl available).
	Probes(ctx context.Context) []domain.DiagnosticsProbeStatus

	Summary(ctx context.Context, opts domain.DiagnosticsSummaryOptions) (domain.DiagnosticsSummary, error)
	Report(ctx context.Context, opts domain.DiagnosticsReportOptions) (domain.DiagnosticsReport, error)
	Sensors(ctx context.Context, opts domain.DiagnosticsSensorsOptions) (domain.DiagnosticsSensors, error)
	Storage(ctx context.Context, opts domain.DiagnosticsStorageOptions) (domain.DiagnosticsStorage, error)
	Processes(ctx context.Context, opts domain.DiagnosticsProcessesOptions) (domain.DiagnosticsProcesses, error)
	Devices(ctx context.Context, opts domain.DiagnosticsDevicesOptions) (domain.DiagnosticsDevices, error)
	Logs(ctx context.Context, opts domain.DiagnosticsLogsOptions) (domain.DiagnosticsLogs, error)
	LogsSummary(ctx context.Context, opts domain.DiagnosticsLogsSummaryOptions) (domain.DiagnosticsLogsSummary, error)
}
