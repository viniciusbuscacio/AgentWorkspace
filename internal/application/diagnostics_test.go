package application

import (
	"context"
	"errors"
	"testing"

	"aw/internal/domain"
)

// fakeProbe records the options it last received so tests can assert the
// application layer's defaults, caps, and policy gate without touching the OS.
type fakeProbe struct {
	lastReport    domain.DiagnosticsReportOptions
	lastDevices   domain.DiagnosticsDevicesOptions
	lastLogs      domain.DiagnosticsLogsOptions
	lastProcesses domain.DiagnosticsProcessesOptions
	summaryCalls  int
}

func (f *fakeProbe) Platform() string { return "linux" }
func (f *fakeProbe) Probes(context.Context) []domain.DiagnosticsProbeStatus {
	return []domain.DiagnosticsProbeStatus{{ID: "system.identity", Available: true}}
}
func (f *fakeProbe) Summary(context.Context, domain.DiagnosticsSummaryOptions) (domain.DiagnosticsSummary, error) {
	f.summaryCalls++
	return domain.DiagnosticsSummary{Platform: "linux"}, nil
}
func (f *fakeProbe) Report(_ context.Context, o domain.DiagnosticsReportOptions) (domain.DiagnosticsReport, error) {
	f.lastReport = o
	return domain.DiagnosticsReport{}, nil
}
func (f *fakeProbe) Sensors(context.Context, domain.DiagnosticsSensorsOptions) (domain.DiagnosticsSensors, error) {
	return domain.DiagnosticsSensors{}, nil
}
func (f *fakeProbe) Storage(context.Context, domain.DiagnosticsStorageOptions) (domain.DiagnosticsStorage, error) {
	return domain.DiagnosticsStorage{}, nil
}
func (f *fakeProbe) Processes(_ context.Context, o domain.DiagnosticsProcessesOptions) (domain.DiagnosticsProcesses, error) {
	f.lastProcesses = o
	return domain.DiagnosticsProcesses{}, nil
}
func (f *fakeProbe) Devices(_ context.Context, o domain.DiagnosticsDevicesOptions) (domain.DiagnosticsDevices, error) {
	f.lastDevices = o
	return domain.DiagnosticsDevices{}, nil
}
func (f *fakeProbe) Logs(_ context.Context, o domain.DiagnosticsLogsOptions) (domain.DiagnosticsLogs, error) {
	f.lastLogs = o
	return domain.DiagnosticsLogs{}, nil
}
func (f *fakeProbe) LogsSummary(context.Context, domain.DiagnosticsLogsSummaryOptions) (domain.DiagnosticsLogsSummary, error) {
	return domain.DiagnosticsLogsSummary{}, nil
}

func TestCapabilitiesBlockAllExposesOnlyCapabilities(t *testing.T) {
	caps := DiagnosticsCapabilities(context.Background(), &fakeProbe{}, domain.SandboxBlockAll)
	if caps.Policy.Diagnostics != string(domain.DiagnosticsBlocked) {
		t.Fatalf("diagnostics level = %q, want blocked", caps.Policy.Diagnostics)
	}
	if !caps.Actions["diagnostics.capabilities"].Available {
		t.Error("capabilities must be available in block_all")
	}
	for _, action := range []string{"diagnostics.summary", "diagnostics.report", "diagnostics.logs"} {
		if caps.Actions[action].Available {
			t.Errorf("%s must be unavailable in block_all", action)
		}
		if caps.Actions[action].Reason == "" {
			t.Errorf("%s must explain why it is blocked", action)
		}
	}
	if len(caps.Probes) != 0 {
		t.Error("block_all must not enumerate probes")
	}
}

func TestCapabilitiesDetailedExposesActionsAndProbes(t *testing.T) {
	caps := DiagnosticsCapabilities(context.Background(), &fakeProbe{}, domain.SandboxPermitList)
	if caps.Policy.Diagnostics != string(domain.DiagnosticsDetailed) {
		t.Fatalf("diagnostics level = %q, want detailed", caps.Policy.Diagnostics)
	}
	if !caps.Actions["diagnostics.summary"].Available {
		t.Error("summary must be available in permit_list")
	}
	if len(caps.Probes) == 0 {
		t.Error("detailed mode must enumerate probes")
	}
}

func TestDetailedActionsBlockedInBlockAll(t *testing.T) {
	probe := &fakeProbe{}
	ctx := context.Background()
	if _, err := DiagnosticsSummary(ctx, probe, domain.SandboxBlockAll, domain.DiagnosticsSummaryOptions{}); !errors.Is(err, ErrDiagnosticsBlocked) {
		t.Errorf("summary err = %v, want ErrDiagnosticsBlocked", err)
	}
	if _, err := DiagnosticsLogs(ctx, probe, domain.SandboxBlockAll, domain.DiagnosticsLogsOptions{}); !errors.Is(err, ErrDiagnosticsBlocked) {
		t.Errorf("logs err = %v, want ErrDiagnosticsBlocked", err)
	}
	if probe.summaryCalls != 0 {
		t.Error("blocked action must not reach the probe")
	}
}

func TestReportDefaultSectionsExcludeLogs(t *testing.T) {
	probe := &fakeProbe{}
	if _, err := DiagnosticsReport(context.Background(), probe, domain.SandboxPermitList, domain.DiagnosticsReportOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, s := range probe.lastReport.Sections {
		if s == "logs" {
			t.Fatal("default report sections must not include logs")
		}
	}
	if len(probe.lastReport.Sections) == 0 {
		t.Fatal("default report sections must be populated")
	}
}

func TestLogsDefaultsExcludeSecurityAndCapLimit(t *testing.T) {
	probe := &fakeProbe{}
	if _, err := DiagnosticsLogs(context.Background(), probe, domain.SandboxPermitList, domain.DiagnosticsLogsOptions{Limit: 99999}); err != nil {
		t.Fatal(err)
	}
	if probe.lastLogs.Limit != diagnosticsLogLimitMax {
		t.Errorf("limit = %d, want capped to %d", probe.lastLogs.Limit, diagnosticsLogLimitMax)
	}
	for _, src := range probe.lastLogs.Sources {
		if src == "security" {
			t.Fatal("security must never be a default log source")
		}
	}
	if len(probe.lastLogs.Severity) == 0 {
		t.Error("severity must default to a non-empty set")
	}
}

func TestLogsSecurityHonoredWhenExplicit(t *testing.T) {
	probe := &fakeProbe{}
	if _, err := DiagnosticsLogs(context.Background(), probe, domain.SandboxPermitList, domain.DiagnosticsLogsOptions{Sources: []string{"security"}}); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, src := range probe.lastLogs.Sources {
		if src == "security" {
			found = true
		}
	}
	if !found {
		t.Error("explicitly requested security source must be honored")
	}
}

func TestDevicesDefaultModeIsProblems(t *testing.T) {
	probe := &fakeProbe{}
	if _, err := DiagnosticsDevices(context.Background(), probe, domain.SandboxPermitList, domain.DiagnosticsDevicesOptions{}); err != nil {
		t.Fatal(err)
	}
	if probe.lastDevices.Mode != "problems" {
		t.Errorf("default devices mode = %q, want problems", probe.lastDevices.Mode)
	}
}

func TestProcessesLimitCapped(t *testing.T) {
	probe := &fakeProbe{}
	if _, err := DiagnosticsProcesses(context.Background(), probe, domain.SandboxPermitList, domain.DiagnosticsProcessesOptions{Limit: 9999}); err != nil {
		t.Fatal(err)
	}
	if probe.lastProcesses.Limit != diagnosticsProcLimitMax {
		t.Errorf("processes limit = %d, want capped to %d", probe.lastProcesses.Limit, diagnosticsProcLimitMax)
	}
}
