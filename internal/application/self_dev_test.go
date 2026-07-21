package application

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"aw/internal/domain"
)

type fakeSelfDevStore struct {
	values   map[string]string
	unlocked bool
	chats    []domain.Chat
	chatsErr error
}

func (s *fakeSelfDevStore) SetSecret(name, value string) error { s.values[name] = value; return nil }
func (s *fakeSelfDevStore) GetSecret(name string) (string, bool, error) {
	v, ok := s.values[name]
	return v, ok, nil
}
func (s *fakeSelfDevStore) HasSecret(name string) (bool, error) {
	_, ok := s.values[name]
	return ok, nil
}
func (s *fakeSelfDevStore) DeleteSecret(name string) error { delete(s.values, name); return nil }
func (s *fakeSelfDevStore) IsUnlocked() bool               { return s.unlocked }
func (s *fakeSelfDevStore) ListChats() ([]domain.Chat, error) {
	return s.chats, s.chatsErr
}

var _ SelfDevVaultStore = (*fakeSelfDevStore)(nil)

func TestSelfDevInstructionFilesOnly(t *testing.T) {
	got := SelfDevInstruction("/repo/aw", false, false)
	if strings.Contains(got, "shell.exec") {
		t.Fatalf("instruction must not mention shell.exec when shell is off: %q", got)
	}
	if !strings.Contains(got, "/repo/aw") || !strings.Contains(got, "go run ./tools/buildgate") {
		t.Fatalf("instruction missing repo root or buildgate hint: %q", got)
	}
	if strings.Contains(got, "aw tool") {
		t.Fatalf("instruction must not mention aw tool when selfManage is off: %q", got)
	}
}

func TestSelfDevInstructionShellAndSelfManage(t *testing.T) {
	got := SelfDevInstruction("/repo", true, true)
	if !strings.Contains(got, "shell.exec") {
		t.Fatalf("instruction must mention shell.exec when shell is on: %q", got)
	}
	if !strings.Contains(got, "intentionally enabled") || !strings.Contains(got, "auto-approved") {
		t.Fatalf("instruction must warn about self-dev shell posture: %q", got)
	}
	if !strings.Contains(got, "aw tool") || !strings.Contains(got, "system.selfcode") {
		t.Fatalf("instruction must mention the aw tool when selfManage is on: %q", got)
	}
}

func TestLoadSelfDevVaultStateUnlocked(t *testing.T) {
	store := &fakeSelfDevStore{
		values:   map[string]string{activeProviderSecret: "openrouter", "_config_model_openrouter": "deepseek/deepseek-r1", "openrouter_api_key": "sk-or-test"},
		unlocked: true,
		chats:    []domain.Chat{{ID: "chat-1"}, {ID: "chat-2"}},
	}
	state, err := LoadSelfDevVaultState(store)
	if err != nil {
		t.Fatalf("LoadSelfDevVaultState error = %v", err)
	}
	if !state.Unlocked || len(state.Chats) != 2 {
		t.Fatalf("unlocked/chats = %v/%d, want true/2", state.Unlocked, len(state.Chats))
	}
	if state.RuntimeProvider == nil || state.RuntimeProvider.ProviderID != "openrouter" {
		t.Fatalf("runtime provider = %+v, want openrouter", state.RuntimeProvider)
	}
}

func TestLoadSelfDevVaultStateDoesNotExposeProviderSecretValues(t *testing.T) {
	store := &fakeSelfDevStore{
		values: map[string]string{
			activeProviderSecret:         "openai-codex",
			"_config_model_openai_codex": "gpt-5.5",
			"openai_codex_auth_json":     "self-dev-provider-credential-sentinel",
		},
		unlocked: true,
	}

	state, err := LoadSelfDevVaultState(store)
	if err != nil {
		t.Fatalf("LoadSelfDevVaultState error = %v", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal self-dev vault state: %v", err)
	}
	if strings.Contains(string(data), store.values["openai_codex_auth_json"]) {
		t.Fatal("self-dev vault state exposed a provider secret value")
	}
}

func TestLoadSelfDevVaultStateLockedSkipsChats(t *testing.T) {
	store := &fakeSelfDevStore{values: map[string]string{}, unlocked: false, chats: []domain.Chat{{ID: "x"}}}
	state, err := LoadSelfDevVaultState(store)
	if err != nil {
		t.Fatalf("LoadSelfDevVaultState error = %v", err)
	}
	if state.Unlocked {
		t.Fatal("state should report locked")
	}
	if len(state.Chats) != 0 {
		t.Fatalf("locked vault must expose no chats, got %d", len(state.Chats))
	}
}

func TestLoadSelfDevVaultStatePropagatesChatError(t *testing.T) {
	store := &fakeSelfDevStore{values: map[string]string{}, unlocked: true, chatsErr: errors.New("boom")}
	if _, err := LoadSelfDevVaultState(store); err == nil {
		t.Fatal("expected propagated chat error, got nil")
	}
}
