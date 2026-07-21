package application

import (
	"strings"
	"testing"

	"aw/internal/domain"
)

type fakeSandboxConfigStore struct {
	unlocked bool
	saved    *domain.SandboxConfig
	config   domain.SandboxConfig
}

func (s *fakeSandboxConfigStore) IsUnlocked() bool { return s.unlocked }
func (s *fakeSandboxConfigStore) LoadSandboxConfig() (domain.SandboxConfig, error) {
	return s.config, nil
}
func (s *fakeSandboxConfigStore) SaveSandboxConfig(config domain.SandboxConfig) error {
	s.saved = &config
	return nil
}

func TestGetSandboxConfigRequiresUnlockedVault(t *testing.T) {
	if _, err := GetSandboxConfig(&fakeSandboxConfigStore{unlocked: false}); err == nil {
		t.Fatal("locked vault did not error")
	}
	if _, err := GetSandboxConfig(nil); err == nil {
		t.Fatal("nil store did not error")
	}
	store := &fakeSandboxConfigStore{unlocked: true, config: domain.DefaultSandboxConfig()}
	config, err := GetSandboxConfig(store)
	if err != nil {
		t.Fatalf("GetSandboxConfig() error = %v", err)
	}
	if config.Mode != domain.SandboxPermitList {
		t.Errorf("mode = %q, want permit_list", config.Mode)
	}
}

func TestSetSandboxConfigValidatesMode(t *testing.T) {
	store := &fakeSandboxConfigStore{unlocked: true}
	_, err := SetSandboxConfig(store, domain.SandboxConfig{Mode: "warp-speed"})
	if err == nil || !strings.Contains(err.Error(), "invalid sandbox mode") {
		t.Fatalf("invalid mode error = %v", err)
	}
	if store.saved != nil {
		t.Fatal("invalid config was saved")
	}
}

func TestSetSandboxConfigNormalizesLists(t *testing.T) {
	store := &fakeSandboxConfigStore{unlocked: true}
	saved, err := SetSandboxConfig(store, domain.SandboxConfig{
		Mode:           domain.SandboxPermitList,
		AllowedFolders: []string{"  ~/Documents ", "", "~/Documents", "~/Projects"},
	})
	if err != nil {
		t.Fatalf("SetSandboxConfig() error = %v", err)
	}
	wantAllowed := []string{"~/Documents", "~/Projects"}
	if len(saved.AllowedFolders) != len(wantAllowed) {
		t.Fatalf("allowed folders = %v, want %v", saved.AllowedFolders, wantAllowed)
	}
	for i, want := range wantAllowed {
		if saved.AllowedFolders[i] != want {
			t.Errorf("allowed[%d] = %q, want %q", i, saved.AllowedFolders[i], want)
		}
	}
	if store.saved == nil {
		t.Fatal("config was not saved")
	}
}

func TestSandboxInstructionPerMode(t *testing.T) {
	workspace := "/data/workspace"

	blockAll := SandboxInstruction(domain.SandboxConfig{Mode: domain.SandboxBlockAll}, workspace)
	for _, want := range []string{"### Filesystem Access", "BLOCK ALL", "CANNOT execute any shell commands", workspace} {
		if !strings.Contains(blockAll, want) {
			t.Errorf("block_all block missing %q:\n%s", want, blockAll)
		}
	}

	permitAll := SandboxInstruction(domain.SandboxConfig{Mode: domain.SandboxPermitAll}, workspace)
	if !strings.Contains(permitAll, "permit_all — no shell/path restrictions.") {
		t.Errorf("permit_all block = %q", permitAll)
	}

	permitList := SandboxInstruction(domain.SandboxConfig{
		Mode:           domain.SandboxPermitList,
		AllowedFolders: []string{"~/Documents"},
	}, workspace)
	for _, want := range []string{"PERMIT LIST", "ONLY run shell commands", workspace, "/tmp", "- ~/Documents"} {
		if !strings.Contains(permitList, want) {
			t.Errorf("permit_list block missing %q:\n%s", want, permitList)
		}
	}
	emptyPermitList := SandboxInstruction(domain.DefaultSandboxConfig(), workspace)
	if !strings.Contains(emptyPermitList, "Settings > Permissions") {
		t.Errorf("empty permit_list should point at Settings: %q", emptyPermitList)
	}

}

func TestSandboxPromptBlockSelfDevOverridesToPermitAll(t *testing.T) {
	store := &fakeSandboxConfigStore{unlocked: true, config: domain.DefaultSandboxConfig()}
	block := SandboxPromptBlock(SandboxPromptInput{Store: store, WorkspaceRoot: "/repo", SelfDev: true})
	if !strings.Contains(block, "permit_all") {
		t.Errorf("self-dev prompt block = %q, want permit_all", block)
	}
	if SandboxPromptBlock(SandboxPromptInput{}) != "" {
		t.Error("no store should yield no block")
	}
	normal := SandboxPromptBlock(SandboxPromptInput{Store: store, WorkspaceRoot: "/ws"})
	if !strings.Contains(normal, "permit_list") {
		t.Errorf("normal prompt block = %q, want permit_list", normal)
	}
}

func TestSetSandboxConfigRequiresUnlockedVault(t *testing.T) {
	store := &fakeSandboxConfigStore{unlocked: false}
	if _, err := SetSandboxConfig(store, domain.DefaultSandboxConfig()); err == nil {
		t.Fatal("locked vault did not error")
	}
	if store.saved != nil {
		t.Fatal("config saved while locked")
	}
}
