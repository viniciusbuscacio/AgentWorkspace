package application

import (
	"strings"
	"testing"
)

func TestSubagentInstructionModes(t *testing.T) {
	// Off means off: no delegation policy is injected at all (the browser-read
	// subagent is gone — page reads are direct, so there is nothing to keep).
	if off := SubagentInstruction("off"); off != "" {
		t.Fatalf("off instruction should be empty, got:\n%s", off)
	}

	balanced := SubagentInstruction("balanced")
	for _, want := range []string{
		"## Generic subagents",
		"Balanced mode",
		"Use at most 3 parallel subagents",
		"system.spawn",
		"::subagent{task=",
		"nao criar branches",
	} {
		if !strings.Contains(balanced, want) {
			t.Fatalf("balanced instruction missing %q:\n%s", want, balanced)
		}
	}
	if strings.Contains(balanced, "subagent.run") {
		t.Fatalf("balanced instruction still references the removed subagent.run:\n%s", balanced)
	}

	aggressive := SubagentInstruction("aggressive")
	for _, want := range []string{
		"Aggressive mode",
		"Use at most 5 parallel subagents",
	} {
		if !strings.Contains(aggressive, want) {
			t.Fatalf("aggressive instruction missing %q:\n%s", want, aggressive)
		}
	}
}
