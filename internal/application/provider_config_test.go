package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"aw/internal/domain"
)

type memoryProviderStore struct {
	values map[string]string
}

func newMemoryProviderStore() *memoryProviderStore {
	return &memoryProviderStore{values: map[string]string{}}
}

func (s *memoryProviderStore) SetSecret(name string, value string) error {
	s.values[name] = value
	return nil
}

func (s *memoryProviderStore) GetSecret(name string) (string, bool, error) {
	value, exists := s.values[name]
	return value, exists, nil
}

func (s *memoryProviderStore) HasSecret(name string) (bool, error) {
	_, exists := s.values[name]
	return exists, nil
}

func (s *memoryProviderStore) DeleteSecret(name string) error {
	delete(s.values, name)
	return nil
}

type fakeProviderValidator struct {
	err     error
	calls   int
	def     domain.ProviderDefinition
	apiKey  string
	model   string
	baseURL string
}

func (v *fakeProviderValidator) ValidateAPIKeyProvider(_ context.Context, def domain.ProviderDefinition, apiKey string, model string, baseURL string) error {
	v.calls++
	v.def = def
	v.apiKey = apiKey
	v.model = model
	v.baseURL = baseURL
	return v.err
}

func TestUnconfiguredProviderStartsDisabled(t *testing.T) {
	store := newMemoryProviderStore()
	// A brand-new vault has no configured provider — every slot reports disabled.
	for _, p := range GetProviderStatus(store).Providers {
		if p.Connected {
			t.Fatalf("nothing should be connected on a fresh vault: %q", p.ID)
		}
		if p.Enabled {
			t.Fatalf("unconfigured provider %q must start disabled", p.ID)
		}
	}
	// Once configured (api key present) it connects and is enabled by default.
	store.values["openai_api_key"] = "sk-123"
	var found bool
	for _, p := range GetProviderStatus(store).Providers {
		if p.ID == "openai" {
			found = true
			if !p.Connected || !p.Enabled {
				t.Fatalf("configured openai should be connected and enabled: %+v", p)
			}
		}
	}
	if !found {
		t.Fatal("openai provider missing from status")
	}
}

func TestSetProviderEnabledDisableExcludesFromChainAndStatus(t *testing.T) {
	store := newMemoryProviderStore()
	store.values["openrouter_api_key"] = "sk-or-1"
	store.values["openai_api_key"] = "sk-2"
	store.values[activeProviderSecret] = "openrouter"

	result := SetProviderEnabled(store, []string{"openrouter", "openai"}, "openai", false)
	if !result.Success {
		t.Fatalf("SetProviderEnabled() = %+v", result)
	}

	status := GetProviderStatus(store)
	for _, info := range status.Providers {
		if info.ID == "openai" && info.Enabled {
			t.Fatalf("openai should report enabled=false: %+v", info)
		}
		if info.ID == "openrouter" && !info.Enabled {
			t.Fatalf("openrouter should stay enabled: %+v", info)
		}
	}

	chain, err := ResolveProviderFallbackChain(store, []string{"openrouter", "openai"})
	if err != nil {
		t.Fatalf("ResolveProviderFallbackChain() error = %v", err)
	}
	for _, cfg := range chain {
		if cfg.ProviderID == "openai" {
			t.Fatalf("disabled provider must not join the chain: %+v", chain)
		}
	}
}

func TestSetProviderEnabledDisablingActiveHandsOverByPriority(t *testing.T) {
	store := newMemoryProviderStore()
	store.values["openrouter_api_key"] = "sk-or-1"
	store.values["openai_api_key"] = "sk-2"
	store.values[activeProviderSecret] = "openrouter"

	result := SetProviderEnabled(store, []string{"openrouter", "openai"}, "openrouter", false)
	if !result.Success || !result.Activated || result.ProviderID != "openai" {
		t.Fatalf("SetProviderEnabled() = %+v, want handover to openai", result)
	}
	if got := store.values[activeProviderSecret]; got != "openai" {
		t.Fatalf("active provider = %q, want openai", got)
	}
}

func TestSetProviderEnabledDisablingLastClearsActive(t *testing.T) {
	store := newMemoryProviderStore()
	store.values["openrouter_api_key"] = "sk-or-1"
	store.values[activeProviderSecret] = "openrouter"

	result := SetProviderEnabled(store, []string{"openrouter"}, "openrouter", false)
	if !result.Success || result.Warning == "" {
		t.Fatalf("SetProviderEnabled() = %+v, want success with warning", result)
	}
	if _, exists := store.values[activeProviderSecret]; exists {
		t.Fatalf("active provider should be cleared")
	}
}

func TestSetProviderEnabledReenableBecomesActiveOnlyWhenNothingActive(t *testing.T) {
	store := newMemoryProviderStore()
	store.values["openrouter_api_key"] = "sk-or-1"
	store.values["openai_api_key"] = "sk-2"
	store.values[activeProviderSecret] = "openai"

	// Disable then re-enable openrouter while openai is active: no takeover.
	if result := SetProviderEnabled(store, nil, "openrouter", false); !result.Success {
		t.Fatalf("disable: %+v", result)
	}
	if result := SetProviderEnabled(store, nil, "openrouter", true); !result.Success || result.Activated {
		t.Fatalf("re-enable with an active provider = %+v, want no takeover", result)
	}
	if got := store.values[activeProviderSecret]; got != "openai" {
		t.Fatalf("active provider = %q, want openai", got)
	}

	// With nothing active, enabling a configured provider activates it.
	delete(store.values, activeProviderSecret)
	if result := SetProviderEnabled(store, nil, "openrouter", false); !result.Success {
		t.Fatalf("disable again: %+v", result)
	}
	if result := SetProviderEnabled(store, nil, "openrouter", true); !result.Success || !result.Activated {
		t.Fatalf("enable with nothing active = %+v, want activation", result)
	}
	if got := store.values[activeProviderSecret]; got != "openrouter" {
		t.Fatalf("active provider = %q, want openrouter", got)
	}
}

func TestSaveAPIKeyProviderStoresSecretAndModel(t *testing.T) {
	store := newMemoryProviderStore()

	result := SaveProviderConfig(context.Background(), store, nil, domain.ProviderSaveConfigInput{
		Provider:  "openrouter",
		Model:     "deepseek/deepseek-r1",
		APIKey:    "sk-or-test",
		SetActive: false,
	})

	if !result.Success || !result.Saved || result.Activated {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := store.values["openrouter_api_key"]; got != "sk-or-test" {
		t.Fatalf("openrouter key not saved, got %q", got)
	}
	if got := store.values["_config_model_openrouter"]; got != "deepseek/deepseek-r1" {
		t.Fatalf("model not saved, got %q", got)
	}
	if _, exists := store.values[activeProviderSecret]; exists {
		t.Fatalf("provider should not be active when setActive=false")
	}
}

func TestSaveCustomProviderActivatesAfterValidation(t *testing.T) {
	store := newMemoryProviderStore()
	validator := &fakeProviderValidator{}

	result := SaveProviderConfig(context.Background(), store, validator, domain.ProviderSaveConfigInput{
		Provider:  "custom-openai",
		Model:     "demo-model",
		APIKey:    "secret",
		BaseURL:   "https://example.test/v1",
		SetActive: true,
	})

	if !result.Success || !result.Saved || !result.Activated || result.Warning != "" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if validator.calls != 1 {
		t.Fatalf("validator calls = %d, want 1", validator.calls)
	}
	if validator.def.ID != "custom-openai" || validator.apiKey != "secret" || validator.model != "demo-model" || validator.baseURL != "https://example.test/v1" {
		t.Fatalf("validator received unexpected input: %+v", validator)
	}
	if got := store.values[activeProviderSecret]; got != "custom-openai" {
		t.Fatalf("active provider not saved, got %q", got)
	}
	if got := store.values["_config_base_url_custom_openai"]; got != "https://example.test/v1" {
		t.Fatalf("base URL not saved, got %q", got)
	}
}

func TestSaveProviderKeepsConfigurationWhenValidationFails(t *testing.T) {
	store := newMemoryProviderStore()
	validator := &fakeProviderValidator{err: fmt.Errorf("401: bad key")}

	result := SaveProviderConfig(context.Background(), store, validator, domain.ProviderSaveConfigInput{
		Provider:  "custom-openai",
		Model:     "demo-model",
		APIKey:    "wrong",
		BaseURL:   "https://example.test/v1",
		SetActive: true,
	})

	if !result.Success || !result.Saved || result.Activated {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(result.Warning, "401") {
		t.Fatalf("warning should include validation status, got %q", result.Warning)
	}
	if got := store.values["custom_openai_api_key"]; got != "wrong" {
		t.Fatalf("api key should still be saved, got %q", got)
	}
	if _, exists := store.values[activeProviderSecret]; exists {
		t.Fatalf("provider should not be active after failed validation")
	}
}

func TestVaultOnlyOAuthProviderRequiresCredentialForActivation(t *testing.T) {
	store := newMemoryProviderStore()

	result := SaveProviderConfig(context.Background(), store, nil, domain.ProviderSaveConfigInput{
		Provider:  "github-copilot",
		Model:     "claude-sonnet-4",
		SetActive: true,
	})

	if !result.Success || !result.Saved || result.Activated {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(strings.ToLower(result.Warning), "credential") {
		t.Fatalf("warning should mention credential, got %q", result.Warning)
	}

	result = SaveProviderConfig(context.Background(), store, nil, domain.ProviderSaveConfigInput{
		Provider:   "github-copilot",
		Model:      "claude-sonnet-4",
		Credential: `{"access_token":"token"}`,
		SetActive:  true,
	})

	if !result.Success || !result.Activated {
		t.Fatalf("provider should activate with vault credential, got %+v", result)
	}
	if got := store.values["github_copilot_auth_json"]; got == "" {
		t.Fatalf("copilot credential was not stored")
	}
}

func TestResolveProviderRuntimeConfigReadsActiveProviderSecrets(t *testing.T) {
	store := newMemoryProviderStore()
	store.values[activeProviderSecret] = "custom-openai"
	store.values["_config_model_custom_openai"] = "demo-model"
	store.values["custom_openai_api_key"] = "secret"
	store.values["_config_base_url_custom_openai"] = "https://example.test/v1"

	config, err := ResolveProviderRuntimeConfig(store)
	if err != nil {
		t.Fatalf("ResolveProviderRuntimeConfig() error = %v", err)
	}
	if config.ProviderID != "custom-openai" {
		t.Fatalf("ProviderID = %q", config.ProviderID)
	}
	if config.Model != "demo-model" {
		t.Fatalf("Model = %q", config.Model)
	}
	if config.APIKey != "secret" {
		t.Fatalf("APIKey = %q", config.APIKey)
	}
	if config.BaseURL != "https://example.test/v1" {
		t.Fatalf("BaseURL = %q", config.BaseURL)
	}
}

func TestProviderStatusDoesNotExposeProviderSecretValues(t *testing.T) {
	store := newMemoryProviderStore()
	secretSentinel := "provider-secret-status-sentinel"
	credentialSentinel := "provider-credential-status-sentinel"
	store.values[activeProviderSecret] = "openai"
	store.values["_config_model_openai"] = "gpt-5.5"
	store.values["openai_api_key"] = secretSentinel
	store.values["openai_codex_auth_json"] = credentialSentinel

	status := GetProviderStatus(store)
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal provider status: %v", err)
	}
	out := string(data)
	if strings.Contains(out, secretSentinel) || strings.Contains(out, credentialSentinel) {
		t.Fatal("provider status exposed a provider secret value")
	}
}

func TestProviderRuntimeConfigJSONOmitsSecretValues(t *testing.T) {
	config := domain.ProviderRuntimeConfig{
		ProviderID:   "custom-openai",
		ProviderName: "Custom OpenAI-compatible",
		AuthType:     "api-key",
		Model:        "demo-model",
		APIKey:       "provider-runtime-api-key-sentinel",
		Credential:   "provider-runtime-credential-sentinel",
		BaseURL:      "https://example.test/v1",
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal runtime config: %v", err)
	}
	out := string(data)
	if strings.Contains(out, config.APIKey) || strings.Contains(out, config.Credential) {
		t.Fatal("runtime config JSON exposed a provider secret value")
	}
	if strings.Contains(out, "apiKey") || strings.Contains(out, "credential") {
		t.Fatalf("runtime config JSON contains secret field names: %s", out)
	}
}

func TestResolveProviderRuntimeConfigFailsWhenNoProviderConfigured(t *testing.T) {
	store := newMemoryProviderStore()

	_, err := ResolveProviderRuntimeConfig(store)
	if err == nil {
		t.Fatalf("ResolveProviderRuntimeConfig() error = nil, want no-provider error")
	}
	if strings.Contains(err.Error(), "GitHub Copilot") || strings.Contains(err.Error(), "github-copilot") {
		t.Fatalf("error should not assume a specific provider, got %q", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no provider") {
		t.Fatalf("error = %q, want a clear no-provider message", err)
	}
}

func TestResolveProviderRuntimeConfigFailsWithoutActiveCredential(t *testing.T) {
	store := newMemoryProviderStore()
	store.values[activeProviderSecret] = "openai"
	store.values["_config_model_openai"] = "gpt-5.5"

	_, err := ResolveProviderRuntimeConfig(store)
	if err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("ResolveProviderRuntimeConfig() error = %v, want API key error", err)
	}
}

func TestResolveNamedProviderRuntimeConfigUsesNamedProvider(t *testing.T) {
	store := newMemoryProviderStore()
	// Active provider is copilot, but we want openrouter config.
	store.values[activeProviderSecret] = "github-copilot"
	store.values["openrouter_api_key"] = "sk-or-named"
	store.values["_config_model_openrouter"] = "deepseek/deepseek-r1"

	cfg, err := ResolveNamedProviderRuntimeConfig(store, "openrouter")
	if err != nil {
		t.Fatalf("ResolveNamedProviderRuntimeConfig() error = %v", err)
	}
	if cfg.ProviderID != "openrouter" {
		t.Fatalf("ProviderID = %q, want %q", cfg.ProviderID, "openrouter")
	}
	if cfg.Model != "deepseek/deepseek-r1" {
		t.Fatalf("Model = %q, want %q", cfg.Model, "deepseek/deepseek-r1")
	}
	if cfg.APIKey != "sk-or-named" {
		t.Fatalf("APIKey not resolved correctly")
	}
}

func TestResolveNamedProviderRuntimeConfigFailsForUnsupported(t *testing.T) {
	store := newMemoryProviderStore()
	_, err := ResolveNamedProviderRuntimeConfig(store, "nonexistent-provider")
	if err == nil || !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("ResolveNamedProviderRuntimeConfig() error = %v, want unsupported error", err)
	}
}

func TestResolveNamedProviderRuntimeConfigFailsWithoutCredential(t *testing.T) {
	store := newMemoryProviderStore()
	// openrouter configured as active but no key
	store.values[activeProviderSecret] = "openrouter"

	_, err := ResolveNamedProviderRuntimeConfig(store, "openrouter")
	if err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("ResolveNamedProviderRuntimeConfig() error = %v, want API key error", err)
	}
}

func configureAPIKeyProvider(store *memoryProviderStore, _, apiKeySecret, key, modelSecret, model string) {
	store.values[apiKeySecret] = key
	store.values[modelSecret] = model
}

func TestResolveProviderFallbackChainFollowsOrder(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openrouter", "openrouter_api_key", "sk-or", "_config_model_openrouter", "deepseek/deepseek-r1")
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")
	configureAPIKeyProvider(store, "nvidia", "nvidia_api_key", "nv", "_config_model_nvidia", "nvidia/llama-3.1-nemotron-70b-instruct")

	chain, err := ResolveProviderFallbackChain(store, []string{"openai", "openrouter", "nvidia"})
	if err != nil {
		t.Fatalf("ResolveProviderFallbackChain() error = %v", err)
	}
	if len(chain) != 3 {
		t.Fatalf("chain length = %d, want 3", len(chain))
	}
	wantOrder := []string{"openai", "openrouter", "nvidia"}
	for i, want := range wantOrder {
		if chain[i].ProviderID != want {
			t.Errorf("chain[%d] = %q, want %q", i, chain[i].ProviderID, want)
		}
	}
}

func TestResolveProviderFallbackChainOrderIsThePriority(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openrouter", "openrouter_api_key", "sk-or", "_config_model_openrouter", "deepseek/deepseek-r1")
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")
	configureAPIKeyProvider(store, "nvidia", "nvidia_api_key", "nv", "_config_model_nvidia", "nvidia/llama-3.1-nemotron-70b-instruct")
	// The numbered order IS the priority: #1 runs first even when the active
	// secret points elsewhere (SyncActiveProviderToOrder re-aligns it).
	store.values[activeProviderSecret] = "nvidia"

	chain, err := ResolveProviderFallbackChain(store, []string{"openai", "openrouter"})
	if err != nil {
		t.Fatalf("ResolveProviderFallbackChain() error = %v", err)
	}
	if len(chain) < 3 {
		t.Fatalf("chain length = %d, want >= 3", len(chain))
	}
	wantOrder := []string{"openai", "openrouter", "nvidia"}
	for i, want := range wantOrder {
		if chain[i].ProviderID != want {
			t.Errorf("chain[%d] = %q, want %q", i, chain[i].ProviderID, want)
		}
	}
}

func TestSyncActiveProviderToOrder(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openrouter", "openrouter_api_key", "sk-or", "_config_model_openrouter", "deepseek/deepseek-r1")
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")
	store.values[activeProviderSecret] = "openai"

	// #1 unconfigured (nvidia) is skipped; openrouter becomes the default.
	activated, err := SyncActiveProviderToOrder(store, []string{"nvidia", "openrouter", "openai"})
	if err != nil {
		t.Fatalf("SyncActiveProviderToOrder() error = %v", err)
	}
	if activated != "openrouter" {
		t.Fatalf("activated = %q, want openrouter", activated)
	}
	if store.values[activeProviderSecret] != "openrouter" {
		t.Fatalf("active secret = %q, want openrouter", store.values[activeProviderSecret])
	}

	// Idempotent: the effective head is already active.
	activated, err = SyncActiveProviderToOrder(store, []string{"nvidia", "openrouter", "openai"})
	if err != nil || activated != "" {
		t.Fatalf("second sync = (%q, %v), want no-op", activated, err)
	}

	// Nothing usable in the order: the current active stays.
	activated, err = SyncActiveProviderToOrder(store, []string{"nvidia", "azure-openai"})
	if err != nil || activated != "" {
		t.Fatalf("unusable order sync = (%q, %v), want no-op", activated, err)
	}
	if store.values[activeProviderSecret] != "openrouter" {
		t.Fatalf("active secret changed to %q on unusable order", store.values[activeProviderSecret])
	}
}

type fakeFallbackStore struct {
	order []string
}

func (f *fakeFallbackStore) LoadProviderFallbackOrder() ([]string, error) { return f.order, nil }
func (f *fakeFallbackStore) SaveProviderFallbackOrder(ids []string) error {
	f.order = ids
	return nil
}
func (f *fakeFallbackStore) LoadProviderCooldownMinutes() int      { return 0 }
func (f *fakeFallbackStore) SaveProviderCooldownMinutes(int) error { return nil }

func TestMoveProviderToOrderHead(t *testing.T) {
	store := &fakeFallbackStore{order: []string{"openai", "openrouter"}}
	if err := MoveProviderToOrderHead(store, "openrouter"); err != nil {
		t.Fatalf("MoveProviderToOrderHead() error = %v", err)
	}
	if len(store.order) != 2 || store.order[0] != "openrouter" || store.order[1] != "openai" {
		t.Fatalf("order = %v, want [openrouter openai]", store.order)
	}
	// Already at the head: no rewrite needed, order unchanged.
	if err := MoveProviderToOrderHead(store, "openrouter"); err != nil {
		t.Fatalf("MoveProviderToOrderHead() no-op error = %v", err)
	}
	// A provider not yet listed is inserted at the head.
	if err := MoveProviderToOrderHead(store, "nvidia"); err != nil {
		t.Fatalf("MoveProviderToOrderHead() insert error = %v", err)
	}
	if store.order[0] != "nvidia" || len(store.order) != 3 {
		t.Fatalf("order = %v, want nvidia first of 3", store.order)
	}
}

func TestResolveProviderFallbackChainSkipsUnconfigured(t *testing.T) {
	store := newMemoryProviderStore()
	// Only openrouter is configured; openai is listed in order but has no key.
	configureAPIKeyProvider(store, "openrouter", "openrouter_api_key", "sk-or", "_config_model_openrouter", "deepseek/deepseek-r1")

	chain, err := ResolveProviderFallbackChain(store, []string{"openai", "openrouter"})
	if err != nil {
		t.Fatalf("ResolveProviderFallbackChain() error = %v", err)
	}
	for _, cfg := range chain {
		if cfg.ProviderID == "openai" {
			t.Fatal("unconfigured openai should be skipped from the chain")
		}
	}
	if chain[0].ProviderID != "openrouter" {
		t.Fatalf("chain[0] = %q, want openrouter", chain[0].ProviderID)
	}
}

func TestResolveProviderFallbackChainEmptyOrderUsesActiveFirst(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openrouter", "openrouter_api_key", "sk-or", "_config_model_openrouter", "deepseek/deepseek-r1")
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")
	store.values[activeProviderSecret] = "openai"

	chain, err := ResolveProviderFallbackChain(store, nil)
	if err != nil {
		t.Fatalf("ResolveProviderFallbackChain() error = %v", err)
	}
	if chain[0].ProviderID != "openai" {
		t.Fatalf("with empty order, active provider should be first; got %q", chain[0].ProviderID)
	}
}

func TestResolveProviderFallbackChainConfiguredProvidersAutoJoin(t *testing.T) {
	store := newMemoryProviderStore()
	configureAPIKeyProvider(store, "openrouter", "openrouter_api_key", "sk-or", "_config_model_openrouter", "deepseek/deepseek-r1")
	configureAPIKeyProvider(store, "openai", "openai_api_key", "sk-oa", "_config_model_openai", "gpt-5.5")

	// Order lists only openrouter, but openai is configured: it auto-joins the
	// chain as a tail fallback. The order only controls priority.
	chain, err := ResolveProviderFallbackChain(store, []string{"openrouter"})
	if err != nil {
		t.Fatalf("ResolveProviderFallbackChain() error = %v", err)
	}
	if len(chain) != 2 || chain[0].ProviderID != "openrouter" {
		t.Fatalf("chain = %v, want openrouter first then openai as fallback", chain)
	}
	var hasOpenAI bool
	for _, cfg := range chain {
		if cfg.ProviderID == "openai" {
			hasOpenAI = true
		}
	}
	if !hasOpenAI {
		t.Fatal("configured openai should auto-join the chain as a fallback")
	}
}

func TestResolveProviderFallbackChainErrorsWhenNoneConfigured(t *testing.T) {
	store := newMemoryProviderStore()
	_, err := ResolveProviderFallbackChain(store, []string{"openrouter", "openai"})
	if err == nil {
		t.Fatal("ResolveProviderFallbackChain() error = nil, want no-provider error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no provider is configured") {
		t.Fatalf("error = %q, want no-provider message", err.Error())
	}
}

func TestDeleteCredentialOfActiveProviderHandsOver(t *testing.T) {
	store := newMemoryProviderStore()
	store.values["openrouter_api_key"] = "sk-or-1"
	store.values["openai_api_key"] = "sk-2"
	store.values[activeProviderSecret] = "openai"

	result := DeleteProviderCredential(store, []string{"openai", "openrouter"}, "openai")
	if !result.Success {
		t.Fatalf("DeleteProviderCredential() = %+v", result)
	}
	// The next configured provider takes over; the active slot never goes
	// silently empty (that broke session restore and compaction while chats
	// kept working via fallback).
	if !result.Activated || result.ProviderID != "openrouter" {
		t.Fatalf("result = %+v, want openrouter activated", result)
	}
	if active := store.values[activeProviderSecret]; active != "openrouter" {
		t.Fatalf("active provider = %q, want openrouter", active)
	}

	// With no other configured provider, the slot clears with a warning.
	result = DeleteProviderCredential(store, []string{"openai", "openrouter"}, "openrouter")
	if !result.Success || result.Warning == "" {
		t.Fatalf("DeleteProviderCredential() = %+v, want success with warning", result)
	}
	if _, exists := store.values[activeProviderSecret]; exists {
		t.Fatalf("active slot should be empty after the last credential is deleted")
	}
}
