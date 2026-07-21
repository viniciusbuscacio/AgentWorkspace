package vault_test

import (
	"strings"
	"testing"

	"aw/internal/application"
	vaultpkg "aw/internal/infrastructure/vault"
	"aw/skills"
)

// fakeSkillsSetter captures what the agent runtime would receive.
type fakeSkillsSetter struct{ got string }

func (f *fakeSkillsSetter) SetSkillsContext(extra string) { f.got = extra }

// TestEndToEndSeedToInjectedPrompt proves the real runtime path the app runs on
// create/unlock: embedded seed -> real encrypted vault -> effective catalog ->
// rendered into the agent's skills slot. No mocks of the seed or the storage.
func TestEndToEndSeedToInjectedPrompt(t *testing.T) {
	v := vaultpkg.New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	// What app.bootstrapAndRefreshSkills does, with the REAL embedded seed.
	seeds, err := skills.BundledSkills()
	if err != nil {
		t.Fatalf("BundledSkills() error = %v", err)
	}
	agentsDoc, hasAgents, err := skills.BundledAgentsDoc()
	if err != nil {
		t.Fatalf("BundledAgentsDoc() error = %v", err)
	}
	if err := application.BootstrapSkills(v, seeds, agentsDoc, hasAgents); err != nil {
		t.Fatalf("BootstrapSkills() error = %v", err)
	}

	// The vault now actually holds the seeded skills.
	stored, err := v.ListSkills(false)
	if err != nil {
		t.Fatalf("ListSkills() error = %v", err)
	}
	if len(stored) < 2 {
		t.Fatalf("expected the vault to be seeded with the builtin skills, got %d", len(stored))
	}

	// Injection: what the agent runtime receives must carry the skills + AGENTS.
	setter := &fakeSkillsSetter{}
	application.RefreshSkillsContext(setter, v, application.SkillsPromptInput{})
	for _, want := range []string{"## Skills", "`gmail-web`", "`web-read`", "skill.read", "Runtime agent guide"} {
		if !strings.Contains(setter.got, want) {
			t.Fatalf("injected skills prompt missing %q:\n%s", want, setter.got)
		}
	}
	// Progressive disclosure: the heavy body is NOT in the prompt; it loads via skill.read.
	if strings.Contains(setter.got, "Do not try `gws.gmail.inbox`") {
		t.Fatalf("injected prompt must be the index, not the full body:\n%s", setter.got)
	}

	// Locking clears the injected block (no sensitive skill text while locked).
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	application.RefreshSkillsContext(setter, v, application.SkillsPromptInput{})
	if setter.got != "" {
		t.Fatalf("locked vault should clear the skills prompt, got:\n%s", setter.got)
	}
}
