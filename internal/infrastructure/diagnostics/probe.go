package diagnostics

import (
	"context"
	"runtime"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// Probe is the concrete ports.DiagnosticsProbe. It is stateless: every method is
// a one-shot read. Per-OS collection lives in the build-tagged probe_<os>.go
// files; this file holds the shared orchestration (notably Report, which reuses
// the other probes so there is no duplicated per-OS report code).
type Probe struct {
	// now is injectable for deterministic tests; defaults to time.Now.
	now func() time.Time
}

var _ ports.DiagnosticsProbe = (*Probe)(nil)

// New builds the native diagnostics probe for the current OS.
func New() *Probe {
	return &Probe{now: time.Now}
}

func (p *Probe) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

// Platform returns the normalized OS family.
func (p *Probe) Platform() string { return platformName() }

func platformName() string {
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		return runtime.GOOS
	default:
		return runtime.GOOS
	}
}

// Probes reports which low-level sources are present on this OS.
func (p *Probe) Probes(ctx context.Context) []domain.DiagnosticsProbeStatus {
	return collectProbes(ctx)
}

func (p *Probe) Summary(ctx context.Context, opts domain.DiagnosticsSummaryOptions) (domain.DiagnosticsSummary, error) {
	s, err := collectSummary(ctx, opts)
	if s.CollectedAt == "" {
		s.CollectedAt = p.clock().UTC().Format(time.RFC3339)
	}
	if s.Platform == "" {
		s.Platform = platformName()
	}
	if s.Warnings == nil {
		s.Warnings = []string{}
	}
	return s, err
}

func (p *Probe) Sensors(ctx context.Context, opts domain.DiagnosticsSensorsOptions) (domain.DiagnosticsSensors, error) {
	return collectSensors(ctx, opts)
}

func (p *Probe) Storage(ctx context.Context, opts domain.DiagnosticsStorageOptions) (domain.DiagnosticsStorage, error) {
	return collectStorage(ctx, opts)
}

func (p *Probe) Processes(ctx context.Context, opts domain.DiagnosticsProcessesOptions) (domain.DiagnosticsProcesses, error) {
	return collectProcesses(ctx, opts)
}

func (p *Probe) Devices(ctx context.Context, opts domain.DiagnosticsDevicesOptions) (domain.DiagnosticsDevices, error) {
	return collectDevices(ctx, opts)
}

func (p *Probe) Logs(ctx context.Context, opts domain.DiagnosticsLogsOptions) (domain.DiagnosticsLogs, error) {
	return collectLogs(ctx, opts)
}

func (p *Probe) LogsSummary(ctx context.Context, opts domain.DiagnosticsLogsSummaryOptions) (domain.DiagnosticsLogsSummary, error) {
	return collectLogsSummary(ctx, opts)
}

// Report aggregates the requested sections. It reuses the individual probes so
// each section gets its own status: a failed or unsupported section becomes a
// partial result and never fails the whole report (spec §4 decisions 5/6).
func (p *Probe) Report(ctx context.Context, opts domain.DiagnosticsReportOptions) (domain.DiagnosticsReport, error) {
	report := domain.DiagnosticsReport{
		CollectedAt: p.clock().UTC().Format(time.RFC3339),
		Platform:    platformName(),
		Sections:    map[string]domain.DiagnosticsSection{},
		Warnings:    []string{},
		Errors:      []string{},
	}
	summary, _ := p.Summary(ctx, domain.DiagnosticsSummaryOptions{IncludeRuntime: true})
	report.Summary = &summary

	want := map[string]bool{}
	for _, s := range opts.Sections {
		want[s] = true
	}

	if want["system"] {
		report.Sections["system"] = okSection(map[string]any{"host": summary.Host, "os": summary.OS})
	}
	if want["cpu"] {
		report.Sections["cpu"] = okSection(summary.CPU)
	}
	if want["memory"] {
		report.Sections["memory"] = okSection(summary.Memory)
	}
	if want["battery"] {
		st := domain.DiagnosticsStatusOK
		if !summary.Battery.Present {
			st = domain.DiagnosticsStatusUnsupported
		}
		report.Sections["battery"] = domain.DiagnosticsSection{Status: st, Data: summary.Battery}
	}
	if want["gpu"] {
		report.Sections["gpu"] = collectGPU(ctx)
	}
	if want["storage"] {
		data, err := p.Storage(ctx, domain.DiagnosticsStorageOptions{IncludeHealth: true, IncludeVolumes: true})
		report.Sections["storage"] = sectionFrom(data.Status, data, data.Warnings, err)
	}
	if want["sensors"] {
		data, err := p.Sensors(ctx, domain.DiagnosticsSensorsOptions{})
		report.Sections["sensors"] = sectionFrom(data.Status, data, data.Warnings, err)
	}
	if want["processes"] {
		data, err := p.Processes(ctx, domain.DiagnosticsProcessesOptions{SortBy: "cpu", Limit: 10, SampleMs: 500})
		report.Sections["processes"] = sectionFrom(data.Status, data, data.Warnings, err)
	}
	if want["devices"] {
		// Default report devices view is the summary+problems view, never a full
		// inventory (spec §5.3).
		data, err := p.Devices(ctx, domain.DiagnosticsDevicesOptions{Mode: "problems", IncludeDrivers: true, IncludeDisabled: true, IncludeProblemDevice: true, Limit: 100})
		report.Sections["devices"] = sectionFrom(data.Status, data, data.Warnings, err)
	}
	if want["logs"] {
		data, err := p.Logs(ctx, domain.DiagnosticsLogsOptions{Since: "24h", Severity: []string{"critical", "error", "warning"}, Sources: []string{"system", "application", "kernel", "storage", "power", "drivers", "devices"}, Limit: 100})
		report.Sections["logs"] = sectionFrom(data.Status, data, data.Warnings, err)
	}

	report.MarkdownSummary = renderMarkdownSummary(summary)
	return report, nil
}

// --- section helpers ---

func okSection(data any) domain.DiagnosticsSection {
	return domain.DiagnosticsSection{Status: domain.DiagnosticsStatusOK, Data: data}
}

func sectionFrom(status domain.DiagnosticsStatus, data any, warnings []string, err error) domain.DiagnosticsSection {
	sec := domain.DiagnosticsSection{Status: status, Data: data, Warnings: warnings}
	if err != nil {
		if sec.Status == "" || sec.Status == domain.DiagnosticsStatusOK {
			sec.Status = domain.DiagnosticsStatusError
		}
		sec.Errors = []string{err.Error()}
	}
	if sec.Status == "" {
		sec.Status = domain.DiagnosticsStatusUnsupported
	}
	return sec
}

func renderMarkdownSummary(s domain.DiagnosticsSummary) string {
	b := &stringsBuilder{}
	b.line("**System diagnostics**")
	if s.Host.Manufacturer != "" || s.Host.Model != "" {
		b.line("- Host: " + nonEmpty(s.Host.Manufacturer+" "+s.Host.Model, "unknown"))
	}
	if s.OS.Name != "" {
		b.line("- OS: " + s.OS.Name + " " + s.OS.Version + " (" + s.OS.Architecture + ")")
	}
	if s.CPU.Model != "" {
		b.line("- CPU: " + s.CPU.Model)
	}
	if s.Memory.TotalBytes > 0 {
		b.line("- Memory: " + humanBytes(s.Memory.UsedBytes) + " / " + humanBytes(s.Memory.TotalBytes))
	}
	if s.Battery.Present {
		b.line("- Battery: " + formatPercent(s.Battery.ChargePercent) + " (" + s.Battery.Status + ")")
	}
	return b.String()
}
