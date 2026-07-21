package application

import (
	"context"
	"strings"
	"testing"
)

func TestCreateCustomProviderAppearsInStatus(t *testing.T) {
	store := newMemoryProviderStore()

	result := CreateCustomProvider(store, "Maritaca")
	if !result.Success || result.ProviderID == "" {
		t.Fatalf("create failed: %+v", result)
	}
	if !strings.HasPrefix(result.ProviderID, customProviderIDPrefix) {
		t.Fatalf("unexpected id %q", result.ProviderID)
	}

	status := GetProviderStatus(store)
	if status.Error != "" {
		t.Fatalf("status error: %s", status.Error)
	}
	var found *ProviderInfo
	for i := range status.Providers {
		if status.Providers[i].ID == result.ProviderID {
			found = &status.Providers[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("custom provider %q not in status", result.ProviderID)
	}
	if found.Name != "Maritaca" {
		t.Fatalf("name = %q, want Maritaca", found.Name)
	}
	if !found.Custom {
		t.Fatalf("expected Custom=true for dynamic provider")
	}
	if found.AuthType != "api-key" || !found.RequiresBaseURL || !found.AllowCustomModel {
		t.Fatalf("unexpected provider shape: %+v", found)
	}
	if found.Status != "not-configured" {
		t.Fatalf("new provider status = %q, want not-configured", found.Status)
	}
}

func TestTwoCustomProvidersAreIsolated(t *testing.T) {
	store := newMemoryProviderStore()
	a := CreateCustomProvider(store, "Maritaca")
	b := CreateCustomProvider(store, "Together")
	if !a.Success || !b.Success {
		t.Fatalf("create failed: %+v %+v", a, b)
	}
	if a.ProviderID == b.ProviderID {
		t.Fatalf("expected distinct ids, got %q twice", a.ProviderID)
	}

	validator := &fakeProviderValidator{}
	// Configure + activate provider A.
	saveA := SaveProviderConfig(context.Background(), store, validator, ProviderSaveConfigInput{
		Provider:  a.ProviderID,
		Model:     "sabia-4",
		APIKey:    "key-a",
		BaseURL:   "https://chat.maritaca.ai/api",
		SetActive: true,
	})
	if !saveA.Activated {
		t.Fatalf("activate A failed: %+v", saveA)
	}
	// Configure provider B (don't activate).
	saveB := SaveProviderConfig(context.Background(), store, validator, ProviderSaveConfigInput{
		Provider: b.ProviderID,
		Model:    "meta-llama-3",
		APIKey:   "key-b",
		BaseURL:  "https://api.together.xyz/v1",
	})
	if !saveB.Saved {
		t.Fatalf("save B failed: %+v", saveB)
	}

	cfgA, err := ResolveNamedProviderRuntimeConfig(store, a.ProviderID)
	if err != nil {
		t.Fatalf("resolve A: %v", err)
	}
	cfgB, err := ResolveNamedProviderRuntimeConfig(store, b.ProviderID)
	if err != nil {
		t.Fatalf("resolve B: %v", err)
	}
	if cfgA.APIKey != "key-a" || cfgA.BaseURL != "https://chat.maritaca.ai/api" || cfgA.Model != "sabia-4" {
		t.Fatalf("A config bled: %+v", cfgA)
	}
	if cfgB.APIKey != "key-b" || cfgB.BaseURL != "https://api.together.xyz/v1" || cfgB.Model != "meta-llama-3" {
		t.Fatalf("B config bled: %+v", cfgB)
	}

	// Active runtime config must resolve to A.
	active, err := ResolveProviderRuntimeConfig(store)
	if err != nil {
		t.Fatalf("resolve active: %v", err)
	}
	if active.ProviderID != a.ProviderID {
		t.Fatalf("active = %q, want %q", active.ProviderID, a.ProviderID)
	}
}

func TestRenameCustomProvider(t *testing.T) {
	store := newMemoryProviderStore()
	created := CreateCustomProvider(store, "Old")
	if res := RenameCustomProvider(store, created.ProviderID, "New Name"); !res.Success {
		t.Fatalf("rename failed: %+v", res)
	}
	if res := RenameCustomProvider(store, created.ProviderID, "  "); res.Success {
		t.Fatalf("expected blank rename to fail")
	}
	if res := RenameCustomProvider(store, "custom-doesnotexist", "X"); res.Success {
		t.Fatalf("expected rename of unknown id to fail")
	}
	status := GetProviderStatus(store)
	for _, p := range status.Providers {
		if p.ID == created.ProviderID && p.Name != "New Name" {
			t.Fatalf("name not updated: %q", p.Name)
		}
	}
}

func TestDeleteCustomProviderClearsSecretsAndActive(t *testing.T) {
	store := newMemoryProviderStore()
	created := CreateCustomProvider(store, "Maritaca")
	validator := &fakeProviderValidator{}
	SaveProviderConfig(context.Background(), store, validator, ProviderSaveConfigInput{
		Provider:  created.ProviderID,
		Model:     "sabia-4",
		APIKey:    "secret-key",
		BaseURL:   "https://chat.maritaca.ai/api",
		SetActive: true,
	})

	def := customProviderDefinition(customProviderEntry{ID: created.ProviderID, Name: "Maritaca"})

	if res := DeleteCustomProvider(store, created.ProviderID); !res.Success {
		t.Fatalf("delete failed: %+v", res)
	}

	for _, key := range []string{def.APIKeySecretKey, def.BaseURLSecretKey, def.ModelSecretKey} {
		if _, exists, _ := store.GetSecret(key); exists {
			t.Fatalf("secret %q survived delete", key)
		}
	}
	if _, exists, _ := store.GetSecret(activeProviderSecret); exists {
		t.Fatalf("active flag survived delete of active provider")
	}
	// Provider should be gone from status.
	for _, p := range GetProviderStatus(store).Providers {
		if p.ID == created.ProviderID {
			t.Fatalf("deleted provider still in status")
		}
	}
	// Deleting again fails cleanly.
	if res := DeleteCustomProvider(store, created.ProviderID); res.Success {
		t.Fatalf("expected second delete to fail")
	}
}

func TestNormalizeOpenAICompatibleBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://chat.maritaca.ai/api":                   "https://chat.maritaca.ai/api",
		"  https://chat.maritaca.ai/api  ":               "https://chat.maritaca.ai/api",
		"https://chat.maritaca.ai/api/":                  "https://chat.maritaca.ai/api",
		"https://chat.maritaca.ai/api///":                "https://chat.maritaca.ai/api",
		"https://chat.maritaca.ai/api/chat/completions":  "https://chat.maritaca.ai/api",
		"https://chat.maritaca.ai/api/chat/completions/": "https://chat.maritaca.ai/api",
		"https://chat.maritaca.ai/api/CHAT/Completions":  "https://chat.maritaca.ai/api",
		"https://api.openai.com/v1":                      "https://api.openai.com/v1",
		"https://api.openai.com/v1/chat/completions":     "https://api.openai.com/v1",
		"":    "",
		"   ": "",
	}
	for in, want := range cases {
		if got := normalizeOpenAICompatibleBaseURL(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSaveAndReadCustomProviderNormalizesPastedEndpoint(t *testing.T) {
	store := newMemoryProviderStore()
	created := CreateCustomProvider(store, "Maritaca")
	validator := &fakeProviderValidator{}
	// User pastes the full endpoint as the base URL.
	SaveProviderConfig(context.Background(), store, validator, ProviderSaveConfigInput{
		Provider:  created.ProviderID,
		Model:     "sabia-4",
		APIKey:    "key",
		BaseURL:   "https://chat.maritaca.ai/api/chat/completions",
		SetActive: true,
	})
	// Validator must have received the cleaned base URL (no doubled suffix).
	if validator.baseURL != "https://chat.maritaca.ai/api" {
		t.Fatalf("validator baseURL = %q, want cleaned", validator.baseURL)
	}
	cfg, err := ResolveNamedProviderRuntimeConfig(store, created.ProviderID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.BaseURL != "https://chat.maritaca.ai/api" {
		t.Fatalf("runtime baseURL = %q, want https://chat.maritaca.ai/api", cfg.BaseURL)
	}
}

func TestBuiltinCustomOpenAIHiddenUntilConfiguredThenDeletable(t *testing.T) {
	store := newMemoryProviderStore()
	// Fresh vault: the empty built-in "Custom OpenAI-compatible" slot is not
	// shown — the user adds custom providers via "Add custom provider".
	for _, p := range GetProviderStatus(store).Providers {
		if p.ID == "custom-openai" {
			t.Fatalf("empty custom-openai slot must be hidden on a fresh vault")
		}
	}
	// Once configured it appears; it is not renamable (not Custom) but is
	// Deletable (the legacy hideable slot).
	SaveProviderConfig(context.Background(), store, &fakeProviderValidator{}, ProviderSaveConfigInput{
		Provider: "custom-openai",
		Model:    "sabia-4",
		APIKey:   "secret-key",
		BaseURL:  "https://chat.maritaca.ai/api",
	})
	var found bool
	for _, p := range GetProviderStatus(store).Providers {
		if p.ID == "custom-openai" {
			found = true
			if p.Custom {
				t.Fatalf("built-in custom-openai must not be flagged Custom (not renamable)")
			}
			if !p.Deletable {
				t.Fatalf("built-in custom-openai must be Deletable (hideable slot)")
			}
		}
	}
	if !found {
		t.Fatalf("configured custom-openai should appear in status")
	}
}

func TestDeleteBuiltinCustomOpenAIHidesItAndClearsActive(t *testing.T) {
	store := newMemoryProviderStore()
	validator := &fakeProviderValidator{}
	SaveProviderConfig(context.Background(), store, validator, ProviderSaveConfigInput{
		Provider:  "custom-openai",
		Model:     "sabia-4",
		APIKey:    "secret-key",
		BaseURL:   "https://chat.maritaca.ai/api",
		SetActive: true,
	})

	if res := DeleteCustomProvider(store, "custom-openai"); !res.Success {
		t.Fatalf("delete built-in failed: %+v", res)
	}

	// Gone from status.
	for _, p := range GetProviderStatus(store).Providers {
		if p.ID == "custom-openai" {
			t.Fatalf("hidden built-in still in status")
		}
	}
	// Secrets cleared.
	if _, exists, _ := store.GetSecret("custom_openai_api_key"); exists {
		t.Fatalf("api key secret survived built-in delete")
	}
	// Active flag cleared (it was active).
	if _, exists, _ := store.GetSecret(activeProviderSecret); exists {
		t.Fatalf("active flag survived built-in delete")
	}
	// Not a fallback candidate anymore.
	for _, def := range allProviderDefinitions(store) {
		if def.ID == "custom-openai" {
			t.Fatalf("hidden built-in still a provider definition")
		}
	}
	// Deleting again fails cleanly.
	if res := DeleteCustomProvider(store, "custom-openai"); res.Success {
		t.Fatalf("expected second delete of hidden built-in to fail")
	}
}

func TestBuiltinNonCustomProviderNotDeletable(t *testing.T) {
	store := newMemoryProviderStore()
	for _, p := range GetProviderStatus(store).Providers {
		if p.ID == "openrouter" && p.Deletable {
			t.Fatalf("openrouter must not be Deletable")
		}
	}
	if res := DeleteCustomProvider(store, "openrouter"); res.Success {
		t.Fatalf("expected delete of non-custom built-in to fail")
	}
}
