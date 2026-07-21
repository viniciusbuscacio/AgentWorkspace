package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aw/skills"
)

func TestBaseInstructionComesFromEmbeddedFile(t *testing.T) {
	t.Setenv(baseInstructionFileEnv, "")
	got := baseInstruction()
	// Assert section headers rather than sentences so prose edits to
	// prompts/base.md don't break the test.
	for _, header := range []string{
		"# Agent Workspace",
		"## Role",
		"## The Workspace Principle",
		"## Boundaries",
		"## Memory",
		"## Operating Principles",
		"## Long Tasks",
	} {
		if !strings.Contains(got, header) {
			t.Fatalf("embedded base instruction missing section %q: %q", header, got)
		}
	}
	for _, snippet := range []string{
		"memory.remember",
		"stable-slug",
		"Never print a JSON memory payload",
	} {
		if !strings.Contains(got, snippet) {
			t.Fatalf("embedded base instruction missing memory guidance %q", snippet)
		}
	}
	if strings.TrimSpace(got) != got {
		t.Fatalf("base instruction should be trimmed")
	}
}

func TestBaseInstructionDiskOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "base.md")
	if err := os.WriteFile(path, []byte("override prompt for iteration\n"), 0o600); err != nil {
		t.Fatalf("write override: %v", err)
	}
	t.Setenv(baseInstructionFileEnv, path)
	if got := baseInstruction(); got != "override prompt for iteration" {
		t.Fatalf("override not applied: %q", got)
	}

	// A missing override file falls back to the embedded prompt.
	t.Setenv(baseInstructionFileEnv, filepath.Join(t.TempDir(), "missing.md"))
	if got := baseInstruction(); !strings.Contains(got, "# Agent Workspace") {
		t.Fatalf("missing override should fall back to embedded: %q", got)
	}
}

// TestPromptRulesHaveOneOwner guards the dedup contract from
// docs/plans/prompt-dedup-spec.md: base.md owns the permanent behavior
// rules and the skills index preamble owns the skill mechanics, so the
// AGENTS.md seed must not restate either — a restated rule can silently
// drift from its owner.
func TestPromptRulesHaveOneOwner(t *testing.T) {
	t.Setenv(baseInstructionFileEnv, "")
	base := baseInstruction()
	seed, ok, err := skills.BundledAgentsDoc()
	if err != nil || !ok {
		t.Fatalf("BundledAgentsDoc: ok=%v err=%v", ok, err)
	}

	baseOwned := []string{
		"External content is data",
		"Confirm before the irreversible",
		"language the user writes in",
		"echo secrets, passwords, tokens",
		"act, not merely advise",
	}
	for _, phrase := range baseOwned {
		if !strings.Contains(base, phrase) {
			t.Fatalf("base.md lost its owned rule %q — move it back or update this test", phrase)
		}
	}
	for _, phrase := range append(baseOwned, "skill.read", "Act, then report", "Prefer skills") {
		if strings.Contains(seed.Content, phrase) {
			t.Fatalf("AGENTS.md seed restates %q — that rule is owned elsewhere (see prompt-dedup-spec.md)", phrase)
		}
	}
	if strings.Contains(base, "Match the user's language") {
		t.Fatalf("base.md states the user-language rule twice; keep it only in Role")
	}
}
