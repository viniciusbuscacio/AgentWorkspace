package domain

import (
	"testing"
	"time"
)

func TestDiagnosticsLevelForMode(t *testing.T) {
	if got := DiagnosticsLevelForMode(SandboxBlockAll); got != DiagnosticsBlocked {
		t.Errorf("block_all level = %q, want blocked", got)
	}
	for _, mode := range []SandboxMode{SandboxPermitList, SandboxPermitAll} {
		if got := DiagnosticsLevelForMode(mode); got != DiagnosticsDetailed {
			t.Errorf("%s level = %q, want detailed", mode, got)
		}
	}
}

func TestParseDiagnosticsSince(t *testing.T) {
	def := 90 * time.Minute
	cases := map[string]time.Duration{
		"15m":    15 * time.Minute,
		"1h":     time.Hour,
		"6h":     6 * time.Hour,
		"24h":    24 * time.Hour,
		"7d":     7 * 24 * time.Hour,
		"":       def,
		"bogus":  def,
		"  1h  ": time.Hour,
	}
	for token, want := range cases {
		if got := ParseDiagnosticsSince(token, def); got != want {
			t.Errorf("ParseDiagnosticsSince(%q) = %v, want %v", token, got, want)
		}
	}
}
