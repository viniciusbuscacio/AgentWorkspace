package domain

import "testing"

func TestExternalTaintTracksWorstRiskAndGatesLaterActions(t *testing.T) {
	taint := ExternalTaint{}
	// A benign read must not contaminate the session.
	taint = taint.WithObservation(ExternalContentSafety{RiskLevel: ExternalRiskLow})
	if taint.Suspicious || taint.RiskLevel != "" {
		t.Fatalf("benign read must not taint: %+v", taint)
	}
	if clean := taint.Safety(); clean.Untrusted || clean.Suspicious || clean.RiskLevel != "" {
		t.Fatalf("clean taint must render as zero safety: %+v", clean)
	}

	// A suspicious/high-risk read contaminates the session.
	taint = taint.WithObservation(ExternalContentSafety{Suspicious: true, RiskLevel: ExternalRiskHigh, Origin: "https://evil.test"})
	s := taint.Safety()
	if !s.Untrusted || !s.Suspicious || s.RiskLevel != ExternalRiskHigh {
		t.Fatalf("tainted session safety wrong: %+v", s)
	}

	// A later CLEAN sensitive action, merged with the session taint, must be gated.
	merged := MergeSafety(ExternalContentSafety{}, s)
	if !DecideExternalAction(ExternalActionPersist, merged).RequireConfirmation {
		t.Fatal("session taint should gate a later clean sensitive action")
	}
	// Reads stay unguarded even under taint.
	if DecideExternalAction(ExternalActionRead, merged).RequireConfirmation {
		t.Fatal("reads must remain unguarded even under taint")
	}
}

func TestClampSandboxModeForUntrusted(t *testing.T) {
	cases := map[SandboxMode]SandboxMode{
		SandboxPermitAll:  SandboxPermitList, // looser -> clamped
		SandboxPermitList: SandboxPermitList, // already tight -> unchanged
		SandboxBlockAll:   SandboxBlockAll,   // tighter -> unchanged
	}
	for in, want := range cases {
		if got := ClampSandboxModeForUntrusted(in); got != want {
			t.Fatalf("ClampSandboxModeForUntrusted(%s) = %s, want %s", in, got, want)
		}
	}
}

// Local tool-output taint stays local (no sandbox clamp downstream); any
// genuinely external observation flips the rendered source type.
func TestExternalTaintTracksExternalSource(t *testing.T) {
	var taint ExternalTaint
	taint = taint.WithObservation(ExternalContentSafety{
		SourceType: ExternalSourceToolOutput, Suspicious: true, RiskLevel: ExternalRiskHigh,
	})
	if taint.SawExternal {
		t.Fatal("tool-output-only taint must not count as external")
	}
	if got := taint.Safety().SourceType; got != ExternalSourceToolOutput {
		t.Fatalf("local-only taint renders SourceType %q, want tool_output", got)
	}

	taint = taint.WithObservation(ExternalContentSafety{
		SourceType: ExternalSourceWeb, Suspicious: true, RiskLevel: ExternalRiskHigh,
	})
	if !taint.SawExternal {
		t.Fatal("a web observation must mark the taint external")
	}
	if got := taint.Safety().SourceType; got == ExternalSourceToolOutput {
		t.Fatalf("external taint must not render as tool_output, got %q", got)
	}

	// An empty/unknown source counts as external (fail closed).
	var unknown ExternalTaint
	unknown = unknown.WithObservation(ExternalContentSafety{Suspicious: true})
	if !unknown.SawExternal {
		t.Fatal("unknown source must fail closed as external")
	}
}

func TestMergeSafetyKeepsWorstRisk(t *testing.T) {
	base := ExternalContentSafety{Untrusted: true, RiskLevel: ExternalRiskLow}
	other := ExternalContentSafety{Untrusted: true, Suspicious: true, RiskLevel: ExternalRiskHigh, Origin: "x"}
	out := MergeSafety(base, other)
	if out.RiskLevel != ExternalRiskHigh || !out.Suspicious {
		t.Fatalf("merge should keep the worst risk: %+v", out)
	}
	// Worse base risk is not downgraded by a milder other.
	out = MergeSafety(other, base)
	if out.RiskLevel != ExternalRiskHigh {
		t.Fatalf("merge must not downgrade risk: %+v", out)
	}
}
