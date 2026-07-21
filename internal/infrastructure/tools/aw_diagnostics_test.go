package tools

import (
	"context"
	"strings"
	"testing"

	"aw/internal/domain"
	"aw/internal/infrastructure/sandbox"
)

func diagnosticsWorkspace(mode domain.SandboxMode, funcs *DiagnosticsFuncs) *workspace {
	return &workspace{
		autoApprove:     true,
		diagnostics:     funcs,
		sandboxPolicyFn: func() sandbox.Policy { return sandbox.Policy{Config: domain.SandboxConfig{Mode: mode}} },
	}
}

func TestDiagnosticsRegisteredIndependentOfSelfManage(t *testing.T) {
	ws := diagnosticsWorkspace(domain.SandboxBlockAll, &DiagnosticsFuncs{
		Capabilities: func(context.Context, domain.SandboxMode) (any, error) { return map[string]any{"ok": true}, nil },
	})
	if ws.selfManage {
		t.Fatal("test precondition: selfManage should be false")
	}
	reg := ws.awRegistry()
	for _, action := range []string{"diagnostics.capabilities", "diagnostics.summary", "diagnostics.report", "diagnostics.logs", "diagnostics.logs.summary"} {
		if _, ok := reg[action]; !ok {
			t.Errorf("action %q not registered", action)
		}
	}
}

func TestDiagnosticsNotRegisteredWhenNil(t *testing.T) {
	ws := diagnosticsWorkspace(domain.SandboxPermitList, nil)
	if _, ok := ws.awRegistry()["diagnostics.capabilities"]; ok {
		t.Error("diagnostics.capabilities must not register without a probe")
	}
}

func TestDiagnosticsCapabilitiesForwardsMode(t *testing.T) {
	var gotMode domain.SandboxMode
	ws := diagnosticsWorkspace(domain.SandboxBlockAll, &DiagnosticsFuncs{
		Capabilities: func(_ context.Context, mode domain.SandboxMode) (any, error) {
			gotMode = mode
			return map[string]any{"platform": "test"}, nil
		},
	})
	out, err := ws.awRegistry()["diagnostics.capabilities"](context.Background(), nil, ws)
	if err != nil {
		t.Fatalf("capabilities error = %v", err)
	}
	if gotMode != domain.SandboxBlockAll {
		t.Errorf("mode forwarded = %q, want block_all", gotMode)
	}
	if !strings.Contains(out, "platform") {
		t.Errorf("capabilities output missing payload: %s", out)
	}
}

func TestDiagnosticsReportParsesSections(t *testing.T) {
	var got domain.DiagnosticsReportOptions
	ws := diagnosticsWorkspace(domain.SandboxPermitList, &DiagnosticsFuncs{
		Report: func(_ context.Context, _ domain.SandboxMode, o domain.DiagnosticsReportOptions) (any, error) {
			got = o
			return map[string]any{}, nil
		},
	})
	_, err := ws.awRegistry()["diagnostics.report"](context.Background(), map[string]any{
		"sections":  []any{"cpu", "logs"},
		"timeoutMs": float64(8000),
	}, ws)
	if err != nil {
		t.Fatalf("report error = %v", err)
	}
	if strings.Join(got.Sections, ",") != "cpu,logs" {
		t.Errorf("sections = %v", got.Sections)
	}
	if got.TimeoutMs != 8000 {
		t.Errorf("timeoutMs = %d", got.TimeoutMs)
	}
}

func TestDiagnosticsSummaryDefaultsIncludeRuntime(t *testing.T) {
	var got domain.DiagnosticsSummaryOptions
	ws := diagnosticsWorkspace(domain.SandboxPermitList, &DiagnosticsFuncs{
		Summary: func(_ context.Context, _ domain.SandboxMode, o domain.DiagnosticsSummaryOptions) (any, error) {
			got = o
			return map[string]any{}, nil
		},
	})
	if _, err := ws.awRegistry()["diagnostics.summary"](context.Background(), map[string]any{}, ws); err != nil {
		t.Fatalf("summary error = %v", err)
	}
	if !got.IncludeRuntime {
		t.Error("includeRuntime should default to true")
	}
}

func TestAwStringSliceArg(t *testing.T) {
	arr, err := awStringSliceArg(map[string]any{"x": []any{"a", " b ", "c"}}, "x")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(arr, ",") != "a,b,c" {
		t.Errorf("array parse = %v", arr)
	}
	csv, err := awStringSliceArg(map[string]any{"x": "a, b ,c"}, "x")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(csv, ",") != "a,b,c" {
		t.Errorf("csv parse = %v", csv)
	}
	if v, err := awStringSliceArg(map[string]any{}, "x"); err != nil || v != nil {
		t.Errorf("absent = %v, %v", v, err)
	}
	if _, err := awStringSliceArg(map[string]any{"x": []any{"a", 1}}, "x"); err == nil {
		t.Error("non-string element should error")
	}
}
