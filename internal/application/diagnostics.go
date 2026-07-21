// Package application — diagnostics use cases. This layer owns defaults,
// timeouts, hard caps, and the Permissions policy gate for the native system
// diagnostics surface (spec docs/plans/hardware-diagnostics-tools-spec.md §8.2).
// It never touches OS APIs directly; all collection goes through the
// ports.DiagnosticsProbe. Redaction happens at the probe (the source), so no
// sensitive value ever reaches this layer to leak.
package application

import (
	"context"
	"errors"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// ErrDiagnosticsBlocked is returned when a detailed diagnostics action is called
// while the Permissions policy blocks local inspection (block_all). Only
// diagnostics.capabilities works in that mode (spec §4 decision 10).
var ErrDiagnosticsBlocked = errors.New("detailed diagnostics are blocked by the current Permissions policy (block_all); only diagnostics.capabilities is available")

// Diagnostics caps and defaults (spec §5 / §9.4).
const (
	diagnosticsLogLimitDefault    = 100
	diagnosticsLogLimitMax        = 500
	diagnosticsDeviceLimitMax     = 1000
	diagnosticsDeviceLimitDefault = 100
	diagnosticsProcLimitDefault   = 10
	diagnosticsProcLimitMax       = 50
	diagnosticsProcSampleDefault  = 1000
	diagnosticsProcSampleMax      = 2000

	diagnosticsSummaryTimeout   = 3 * time.Second
	diagnosticsSensorsTimeout   = 5 * time.Second
	diagnosticsStorageTimeout   = 5 * time.Second
	diagnosticsProcessTimeout   = 4 * time.Second
	diagnosticsDevicesTimeout   = 5 * time.Second
	diagnosticsLogsTimeout      = 10 * time.Second
	diagnosticsLogsSumTimeout   = 5 * time.Second
	diagnosticsReportTimeout    = 15 * time.Second
	diagnosticsReportTimeoutMax = 60 * time.Second
)

// diagnosticsBlocked reports whether detailed diagnostics are blocked for mode.
func diagnosticsBlocked(mode domain.SandboxMode) bool {
	return domain.DiagnosticsLevelForMode(mode) == domain.DiagnosticsBlocked
}

// DiagnosticsCapabilities builds the capabilities report. It is always available,
// including in block_all, and explains what the current policy blocks (spec §5.1).
func DiagnosticsCapabilities(ctx context.Context, probe ports.DiagnosticsProbe, mode domain.SandboxMode) domain.DiagnosticsCapabilities {
	level := domain.DiagnosticsLevelForMode(mode)
	caps := domain.DiagnosticsCapabilities{
		Platform: probe.Platform(),
		Actions:  map[string]domain.DiagnosticsActionAvailability{},
	}
	caps.Policy.Mode = string(mode)
	caps.Policy.Diagnostics = string(level)

	detailed := level != domain.DiagnosticsBlocked
	blockReason := ""
	if !detailed {
		blockReason = "blocked by Permissions policy (block_all)"
	}
	withNotes := func(notes ...string) domain.DiagnosticsActionAvailability {
		a := domain.DiagnosticsActionAvailability{Available: detailed}
		if detailed {
			a.Notes = notes
		} else {
			a.Reason = blockReason
		}
		return a
	}
	// capabilities itself is always available.
	caps.Actions["diagnostics.capabilities"] = domain.DiagnosticsActionAvailability{Available: true}
	caps.Actions["diagnostics.summary"] = withNotes()
	caps.Actions["diagnostics.report"] = withNotes()
	caps.Actions["diagnostics.sensors"] = withNotes("temperature and fans are best effort")
	caps.Actions["diagnostics.storage"] = withNotes()
	caps.Actions["diagnostics.processes"] = withNotes()
	caps.Actions["diagnostics.devices"] = withNotes("device and driver inventory is best effort")
	caps.Actions["diagnostics.logs"] = withNotes("logs are filtered, capped, and redacted")
	caps.Actions["diagnostics.logs.summary"] = withNotes("logs are filtered, capped, and redacted")

	if detailed {
		caps.Probes = probe.Probes(ctx)
	} else {
		caps.Probes = []domain.DiagnosticsProbeStatus{}
	}
	return caps
}

// DiagnosticsSummary runs the lightweight identity/summary probe.
func DiagnosticsSummary(ctx context.Context, probe ports.DiagnosticsProbe, mode domain.SandboxMode, opts domain.DiagnosticsSummaryOptions) (domain.DiagnosticsSummary, error) {
	if diagnosticsBlocked(mode) {
		return domain.DiagnosticsSummary{}, ErrDiagnosticsBlocked
	}
	ctx, cancel := context.WithTimeout(diagCtx(ctx), diagnosticsSummaryTimeout)
	defer cancel()
	return probe.Summary(ctx, opts)
}

// DiagnosticsReport runs the aggregated report over the requested sections.
func DiagnosticsReport(ctx context.Context, probe ports.DiagnosticsProbe, mode domain.SandboxMode, opts domain.DiagnosticsReportOptions) (domain.DiagnosticsReport, error) {
	if diagnosticsBlocked(mode) {
		return domain.DiagnosticsReport{}, ErrDiagnosticsBlocked
	}
	opts.Sections = normalizeReportSections(opts.Sections)
	timeout := diagnosticsReportTimeout
	if opts.TimeoutMs > 0 {
		timeout = clampDuration(time.Duration(opts.TimeoutMs)*time.Millisecond, time.Second, diagnosticsReportTimeoutMax)
	}
	ctx, cancel := context.WithTimeout(diagCtx(ctx), timeout)
	defer cancel()
	return probe.Report(ctx, opts)
}

// DiagnosticsSensors runs the best-effort sensors probe.
func DiagnosticsSensors(ctx context.Context, probe ports.DiagnosticsProbe, mode domain.SandboxMode, opts domain.DiagnosticsSensorsOptions) (domain.DiagnosticsSensors, error) {
	if diagnosticsBlocked(mode) {
		return domain.DiagnosticsSensors{}, ErrDiagnosticsBlocked
	}
	timeout := diagnosticsSensorsTimeout
	if opts.TimeoutMs > 0 {
		timeout = clampDuration(time.Duration(opts.TimeoutMs)*time.Millisecond, time.Second, 15*time.Second)
	}
	ctx, cancel := context.WithTimeout(diagCtx(ctx), timeout)
	defer cancel()
	return probe.Sensors(ctx, opts)
}

// DiagnosticsStorage runs the storage probe.
func DiagnosticsStorage(ctx context.Context, probe ports.DiagnosticsProbe, mode domain.SandboxMode, opts domain.DiagnosticsStorageOptions) (domain.DiagnosticsStorage, error) {
	if diagnosticsBlocked(mode) {
		return domain.DiagnosticsStorage{}, ErrDiagnosticsBlocked
	}
	ctx, cancel := context.WithTimeout(diagCtx(ctx), diagnosticsStorageTimeout)
	defer cancel()
	return probe.Storage(ctx, opts)
}

// DiagnosticsProcesses runs the top-processes probe.
func DiagnosticsProcesses(ctx context.Context, probe ports.DiagnosticsProbe, mode domain.SandboxMode, opts domain.DiagnosticsProcessesOptions) (domain.DiagnosticsProcesses, error) {
	if diagnosticsBlocked(mode) {
		return domain.DiagnosticsProcesses{}, ErrDiagnosticsBlocked
	}
	opts = normalizeProcessOptions(opts)
	ctx, cancel := context.WithTimeout(diagCtx(ctx), diagnosticsProcessTimeout)
	defer cancel()
	return probe.Processes(ctx, opts)
}

// DiagnosticsDevices runs the device/driver inventory probe.
func DiagnosticsDevices(ctx context.Context, probe ports.DiagnosticsProbe, mode domain.SandboxMode, opts domain.DiagnosticsDevicesOptions) (domain.DiagnosticsDevices, error) {
	if diagnosticsBlocked(mode) {
		return domain.DiagnosticsDevices{}, ErrDiagnosticsBlocked
	}
	opts = normalizeDeviceOptions(opts)
	ctx, cancel := context.WithTimeout(diagCtx(ctx), diagnosticsDevicesTimeout)
	defer cancel()
	return probe.Devices(ctx, opts)
}

// DiagnosticsLogs runs the detailed OS-log read.
func DiagnosticsLogs(ctx context.Context, probe ports.DiagnosticsProbe, mode domain.SandboxMode, opts domain.DiagnosticsLogsOptions) (domain.DiagnosticsLogs, error) {
	if diagnosticsBlocked(mode) {
		return domain.DiagnosticsLogs{}, ErrDiagnosticsBlocked
	}
	opts = normalizeLogOptions(opts)
	ctx, cancel := context.WithTimeout(diagCtx(ctx), diagnosticsLogsTimeout)
	defer cancel()
	return probe.Logs(ctx, opts)
}

// DiagnosticsLogsSummary runs the aggregated OS-log summary.
func DiagnosticsLogsSummary(ctx context.Context, probe ports.DiagnosticsProbe, mode domain.SandboxMode, opts domain.DiagnosticsLogsSummaryOptions) (domain.DiagnosticsLogsSummary, error) {
	if diagnosticsBlocked(mode) {
		return domain.DiagnosticsLogsSummary{}, ErrDiagnosticsBlocked
	}
	if opts.Focus == "" {
		opts.Focus = "all"
	}
	if opts.Since == "" {
		opts.Since = "24h"
	}
	ctx, cancel := context.WithTimeout(diagCtx(ctx), diagnosticsLogsSumTimeout)
	defer cancel()
	return probe.LogsSummary(ctx, opts)
}

// --- option normalization ---

func normalizeReportSections(sections []string) []string {
	if len(sections) == 0 {
		return append([]string(nil), domain.DiagnosticsReportSections...)
	}
	allowed := map[string]bool{}
	for _, s := range domain.DiagnosticsAllReportSections {
		allowed[s] = true
	}
	out := make([]string, 0, len(sections))
	seen := map[string]bool{}
	for _, s := range sections {
		if allowed[s] && !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	if len(out) == 0 {
		return append([]string(nil), domain.DiagnosticsReportSections...)
	}
	return out
}

func normalizeProcessOptions(opts domain.DiagnosticsProcessesOptions) domain.DiagnosticsProcessesOptions {
	if opts.SortBy != "memory" {
		opts.SortBy = "cpu"
	}
	if opts.Limit <= 0 {
		opts.Limit = diagnosticsProcLimitDefault
	}
	if opts.Limit > diagnosticsProcLimitMax {
		opts.Limit = diagnosticsProcLimitMax
	}
	if opts.SampleMs <= 0 {
		opts.SampleMs = diagnosticsProcSampleDefault
	}
	if opts.SampleMs > diagnosticsProcSampleMax {
		opts.SampleMs = diagnosticsProcSampleMax
	}
	return opts
}

func normalizeDeviceOptions(opts domain.DiagnosticsDevicesOptions) domain.DiagnosticsDevicesOptions {
	switch opts.Mode {
	case "summary", "all", "problems":
	default:
		opts.Mode = "problems"
	}
	if opts.Limit <= 0 {
		opts.Limit = diagnosticsDeviceLimitDefault
	}
	if opts.Limit > diagnosticsDeviceLimitMax {
		opts.Limit = diagnosticsDeviceLimitMax
	}
	opts.Classes = filterEnum(opts.Classes, domain.DiagnosticsDeviceClasses)
	return opts
}

func normalizeLogOptions(opts domain.DiagnosticsLogsOptions) domain.DiagnosticsLogsOptions {
	if opts.Since == "" {
		opts.Since = "1h"
	}
	if opts.Limit <= 0 {
		opts.Limit = diagnosticsLogLimitDefault
	}
	if opts.Limit > diagnosticsLogLimitMax {
		opts.Limit = diagnosticsLogLimitMax
	}
	opts.Severity = filterEnum(opts.Severity, domain.DiagnosticsLogSeverities)
	if len(opts.Severity) == 0 {
		opts.Severity = []string{"critical", "error", "warning"}
	}
	opts.Sources = filterEnum(opts.Sources, domain.DiagnosticsLogSources)
	// security is opt-in: it is honored only when explicitly requested, which it
	// is here (it survived filterEnum). Default sources (when none given) exclude
	// it entirely.
	if len(opts.Sources) == 0 {
		opts.Sources = []string{"system", "application", "kernel", "drivers", "devices", "storage", "power"}
	}
	return opts
}

// filterEnum keeps only the values present in allowed, preserving order and
// dropping duplicates.
func filterEnum(values, allowed []string) []string {
	if len(values) == 0 {
		return nil
	}
	ok := map[string]bool{}
	for _, a := range allowed {
		ok[a] = true
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		if ok[v] && !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	return out
}

func clampDuration(d, lo, hi time.Duration) time.Duration {
	if d < lo {
		return lo
	}
	if d > hi {
		return hi
	}
	return d
}

func diagCtx(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
