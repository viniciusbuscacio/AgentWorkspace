//go:build !windows && !darwin && !linux

package diagnostics

import (
	"context"
	"runtime"

	"aw/internal/domain"
)

// This OS has no native collector. Every probe returns an honest unsupported
// result rather than fabricating data (spec risk: honest unsupported beats
// invented readings). Basic identity still comes from the Go runtime.

func collectProbes(_ context.Context) []domain.DiagnosticsProbeStatus {
	return []domain.DiagnosticsProbeStatus{
		{ID: "system.identity", Available: true},
		{ID: "os.logs", Available: false, Reason: "unsupported_os"},
	}
}

func collectSummary(_ context.Context, _ domain.DiagnosticsSummaryOptions) (domain.DiagnosticsSummary, error) {
	return domain.DiagnosticsSummary{
		OS:       domain.DiagnosticsOS{Name: runtime.GOOS, Architecture: runtime.GOARCH},
		CPU:      domain.DiagnosticsCPU{LogicalCores: runtime.NumCPU()},
		Host:     domain.DiagnosticsHost{SKURedacted: true, HostnameRedacted: true},
		Warnings: []string{"native diagnostics are not implemented for this operating system"},
	}, nil
}

func collectGPU(_ context.Context) domain.DiagnosticsSection {
	return domain.DiagnosticsSection{Status: domain.DiagnosticsStatusUnsupported}
}

func collectSensors(_ context.Context, _ domain.DiagnosticsSensorsOptions) (domain.DiagnosticsSensors, error) {
	return domain.DiagnosticsSensors{
		Status:       domain.DiagnosticsStatusUnsupported,
		Temperatures: []domain.DiagnosticsTemperature{},
		Fans:         []domain.DiagnosticsFan{},
		Warnings:     []string{"sensors are not available on this operating system"},
	}, nil
}

func collectStorage(_ context.Context, _ domain.DiagnosticsStorageOptions) (domain.DiagnosticsStorage, error) {
	return domain.DiagnosticsStorage{
		Status:   domain.DiagnosticsStatusUnsupported,
		Disks:    []domain.DiagnosticsDisk{},
		Volumes:  []domain.DiagnosticsVolume{},
		Warnings: []string{"storage inventory is not available on this operating system"},
	}, nil
}

func collectProcesses(_ context.Context, _ domain.DiagnosticsProcessesOptions) (domain.DiagnosticsProcesses, error) {
	return domain.DiagnosticsProcesses{
		Status:     domain.DiagnosticsStatusUnsupported,
		TopCPU:     []domain.DiagnosticsProcess{},
		TopMemory:  []domain.DiagnosticsProcess{},
		Redactions: []string{"commandLine", "executablePath"},
		Warnings:   []string{"process metrics are not available on this operating system"},
	}, nil
}

func collectDevices(_ context.Context, opts domain.DiagnosticsDevicesOptions) (domain.DiagnosticsDevices, error) {
	return domain.DiagnosticsDevices{
		Status:   domain.DiagnosticsStatusUnsupported,
		Mode:     opts.Mode,
		Devices:  []domain.DiagnosticsDevice{},
		Summary:  domain.DiagnosticsDevicesSummary{ByClass: map[string]int{}},
		Warnings: []string{"device inventory is not available on this operating system"},
	}, nil
}

func collectLogs(_ context.Context, opts domain.DiagnosticsLogsOptions) (domain.DiagnosticsLogs, error) {
	logs := domain.DiagnosticsLogs{
		Status:   domain.DiagnosticsStatusUnsupported,
		Platform: platformName(),
		Events:   []domain.DiagnosticsLogEvent{},
		Warnings: []string{"OS logs are not available on this operating system"},
		Errors:   []string{},
	}
	logs.TimeRange.Since = opts.Since
	return logs, nil
}

func collectLogsSummary(_ context.Context, _ domain.DiagnosticsLogsSummaryOptions) (domain.DiagnosticsLogsSummary, error) {
	return domain.DiagnosticsLogsSummary{
		Status:                domain.DiagnosticsStatusUnsupported,
		Summary:               "OS logs are not available on this operating system.",
		Patterns:              []string{},
		RecommendedNextChecks: []string{},
	}, nil
}
