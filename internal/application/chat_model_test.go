package application

import "testing"

type memoryOverrideStore struct {
	provider map[string]string
	model    map[string]string
}

func newMemoryOverrideStore() *memoryOverrideStore {
	return &memoryOverrideStore{provider: map[string]string{}, model: map[string]string{}}
}

func (s *memoryOverrideStore) ChatModelOverride(chatID string) (string, string, error) {
	return s.provider[chatID], s.model[chatID], nil
}

func (s *memoryOverrideStore) SetChatModelOverride(chatID, provider, model string) error {
	s.provider[chatID] = provider
	s.model[chatID] = model
	return nil
}

func TestSetChatModelPersistsOverride(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openrouter", "openrouter_api_key", "sk-or", "_config_model_openrouter", "deepseek/deepseek-r1")
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")
	store.values[activeProviderSecret] = "openrouter"
	overrides := newMemoryOverrideStore()

	result, err := SetChatModel(store, overrides, "chat-1", "openai", "")
	if err != nil {
		t.Fatalf("SetChatModel() error = %v", err)
	}
	if result.Cleared {
		t.Fatalf("setting an override should not report Cleared: %+v", result)
	}
	if result.Provider != "openai" {
		t.Fatalf("result.Provider = %q, want openai", result.Provider)
	}
	if result.Model != "gpt-5.5" {
		t.Fatalf("result.Model = %q, want gpt-5.5 (the provider's configured model)", result.Model)
	}
	if overrides.provider["chat-1"] != "openai" {
		t.Fatalf("stored provider = %q, want openai", overrides.provider["chat-1"])
	}
}

func TestSetChatModelByDisplayName(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")
	store.values[activeProviderSecret] = "openai"
	overrides := newMemoryOverrideStore()

	// The display name is resolved case-insensitively to the provider ID.
	result, err := SetChatModel(store, overrides, "chat-1", "OpenAI", "")
	if err != nil {
		t.Fatalf("SetChatModel() error = %v", err)
	}
	if result.Provider != "openai" {
		t.Fatalf("result.Provider = %q, want openai", result.Provider)
	}
}

func TestSetChatModelClearsOverride(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")
	store.values[activeProviderSecret] = "openai"
	overrides := newMemoryOverrideStore()
	overrides.provider["chat-1"] = "openrouter"
	overrides.model["chat-1"] = "deepseek/deepseek-r1"

	result, err := SetChatModel(store, overrides, "chat-1", "", "")
	if err != nil {
		t.Fatalf("SetChatModel() error = %v", err)
	}
	if !result.Cleared {
		t.Fatalf("clearing should report Cleared: %+v", result)
	}
	if overrides.provider["chat-1"] != "" {
		t.Fatalf("override provider should be cleared, got %q", overrides.provider["chat-1"])
	}
	if result.Provider != "openai" {
		t.Fatalf("cleared result should report the global default (openai), got %q", result.Provider)
	}
}

func TestSetChatModelRejectsUnconfiguredProvider(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")
	store.values[activeProviderSecret] = "openai"
	overrides := newMemoryOverrideStore()

	// anthropic is a known provider but has no credential here.
	_, err := SetChatModel(store, overrides, "chat-1", "anthropic", "")
	if err == nil {
		t.Fatal("expected error setting an unconfigured provider")
	}
	if _, ok := overrides.provider["chat-1"]; ok {
		t.Fatal("a rejected change must not persist an override")
	}
}

func TestSetChatModelRejectsUnknownProvider(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")
	overrides := newMemoryOverrideStore()

	if _, err := SetChatModel(store, overrides, "chat-1", "not-a-provider", ""); err == nil {
		t.Fatal("expected error for an unknown provider id")
	}
}

func TestResolveChatFallbackChainPutsOverrideFirst(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openrouter", "openrouter_api_key", "sk-or", "_config_model_openrouter", "deepseek/deepseek-r1")
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")
	store.values[activeProviderSecret] = "openrouter"

	chain, err := ResolveChatFallbackChain(store, []string{"openrouter", "openai"}, "openai", "gpt-override")
	if err != nil {
		t.Fatalf("ResolveChatFallbackChain() error = %v", err)
	}
	if chain[0].ProviderID != "openai" {
		t.Fatalf("override provider should lead the chain, got %q", chain[0].ProviderID)
	}
	if chain[0].Model != "gpt-override" {
		t.Fatalf("override model should be applied, got %q", chain[0].Model)
	}
	// The overridden provider must not appear twice.
	for _, cfg := range chain[1:] {
		if cfg.ProviderID == "openai" {
			t.Fatalf("override provider duplicated in chain: %+v", chain)
		}
	}
}

func TestResolveEffectiveProviderRuntimeConfigUsesOverride(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openrouter", "openrouter_api_key", "sk-or", "_config_model_openrouter", "deepseek/deepseek-r1")
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")
	store.values[activeProviderSecret] = "openrouter"

	cfg, err := ResolveEffectiveProviderRuntimeConfig(store, "openai", "gpt-override")
	if err != nil {
		t.Fatalf("ResolveEffectiveProviderRuntimeConfig() error = %v", err)
	}
	if cfg.ProviderID != "openai" || cfg.Model != "gpt-override" {
		t.Fatalf("effective config = %q/%q, want openai/gpt-override", cfg.ProviderID, cfg.Model)
	}
}

func TestResolveEffectiveProviderRuntimeConfigFallsBackWhenOverrideBroken(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openrouter", "openrouter_api_key", "sk-or", "_config_model_openrouter", "deepseek/deepseek-r1")
	store.values[activeProviderSecret] = "openrouter"

	// openai has no credential — a broken override should transparently fall
	// back to the global active provider.
	cfg, err := ResolveEffectiveProviderRuntimeConfig(store, "openai", "")
	if err != nil {
		t.Fatalf("ResolveEffectiveProviderRuntimeConfig() error = %v", err)
	}
	if cfg.ProviderID != "openrouter" {
		t.Fatalf("broken override should fall back to openrouter, got %q", cfg.ProviderID)
	}
}
