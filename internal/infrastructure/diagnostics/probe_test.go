package diagnostics

import (
	"context"
	"runtime"
	"testing"
	"time"

	"aw/internal/domain"
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestProbeSummaryShape(t *testing.T) {
	p := New()
	s, err := p.Summary(testCtx(t), domain.DiagnosticsSummaryOptions{IncludeRuntime: true})
	if err != nil {
		t.Fatalf("Summary error = %v", err)
	}
	if s.Platform != runtime.GOOS {
		t.Errorf("platform = %q, want %q", s.Platform, runtime.GOOS)
	}
	if s.CollectedAt == "" {
		t.Error("collectedAt should be set")
	}
	// Host identity is always redaction-flagged regardless of OS.
	if !s.Host.SKURedacted || !s.Host.HostnameRedacted {
		t.Error("host SKU/hostname must be flagged redacted")
	}
}

func TestProbeReportAssemblesRequestedSections(t *testing.T) {
	p := New()
	report, err := p.Report(testCtx(t), domain.DiagnosticsReportOptions{Sections: []string{"cpu", "memory"}})
	if err != nil {
		t.Fatalf("Report error = %v", err)
	}
	for _, want := range []string{"cpu", "memory"} {
		if _, ok := report.Sections[want]; !ok {
			t.Errorf("report missing section %q", want)
		}
	}
	// A section we did not request must be absent.
	if _, ok := report.Sections["logs"]; ok {
		t.Error("logs must not appear unless requested")
	}
	if report.Summary == nil {
		t.Error("report should carry a summary")
	}
}

func TestProbeProbesNotEmpty(t *testing.T) {
	if got := New().Probes(testCtx(t)); len(got) == 0 {
		t.Error("Probes should report at least system.identity")
	}
}

func TestProbeProcessesRedactionsDeclared(t *testing.T) {
	p := New()
	procs, err := p.Processes(testCtx(t), domain.DiagnosticsProcessesOptions{SortBy: "cpu", Limit: 5, SampleMs: 200})
	if err != nil {
		t.Fatalf("Processes error = %v", err)
	}
	if len(procs.Redactions) == 0 {
		t.Error("processes must declare redactions (commandLine/executablePath)")
	}
}
