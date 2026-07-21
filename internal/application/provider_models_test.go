package application

import (
	"context"
	"errors"
	"testing"

	"aw/internal/domain"
)

type fakeModelCatalog struct {
	models []string
	err    error
	calls  int
	cfg    domain.ProviderRuntimeConfig
}

func (c *fakeModelCatalog) ListModels(_ context.Context, cfg domain.ProviderRuntimeConfig) ([]string, error) {
	c.calls++
	c.cfg = cfg
	return c.models, c.err
}

func configuredCopilotStore() *memoryProviderStore {
	store := newMemoryProviderStore()
	store.values["github_copilot_auth_json"] = `{"auth_mode":"github-copilot","github_token":"gho_x"}`
	return store
}

func TestListProviderModelsUnknownProvider(t *testing.T) {
	result := ListProviderModels(context.Background(), newMemoryProviderStore(), &fakeModelCatalog{}, "nope")
	if result.Error == "" || result.Source != domain.ProviderModelsSourceStatic {
		t.Errorf("expected static error result for unknown provider, got %+v", result)
	}
}

func TestListProviderModelsFallsBackToStaticWhenNotConfigured(t *testing.T) {
	catalog := &fakeModelCatalog{models: []string{"live-1"}}
	result := ListProviderModels(context.Background(), newMemoryProviderStore(), catalog, "github-copilot")
	if result.Source != domain.ProviderModelsSourceStatic || len(result.Models) == 0 {
		t.Errorf("expected static fallback, got %+v", result)
	}
	if catalog.calls != 0 {
		t.Errorf("catalog must not be called for an unconfigured provider (calls=%d)", catalog.calls)
	}
}

func TestListProviderModelsLiveAndCached(t *testing.T) {
	store := configuredCopilotStore()
	catalog := &fakeModelCatalog{models: []string{"live-a", "live-b"}}
	result := ListProviderModels(context.Background(), store, catalog, "github-copilot")
	if result.Source != domain.ProviderModelsSourceLive || result.Error != "" {
		t.Fatalf("expected live result, got %+v", result)
	}
	if len(result.Models) != 2 || result.Models[0] != "live-a" {
		t.Errorf("models = %v", result.Models)
	}
	if catalog.cfg.ProviderID != "github-copilot" {
		t.Errorf("catalog received cfg %+v", catalog.cfg)
	}

	// The endpoint goes away -> the previously fetched list is served from cache.
	catalog.models, catalog.err = nil, errors.New("endpoint down")
	result = ListProviderModels(context.Background(), store, catalog, "github-copilot")
	if result.Source != domain.ProviderModelsSourceCache {
		t.Fatalf("expected cache fallback, got %+v", result)
	}
	if len(result.Models) != 2 || result.Models[0] != "live-a" {
		t.Errorf("cached models = %v", result.Models)
	}
	if result.Error != "endpoint down" {
		t.Errorf("error = %q", result.Error)
	}
}

func TestListProviderModelsFallsBackToStaticOnFetchErrorWithoutCache(t *testing.T) {
	store := configuredCopilotStore()
	catalog := &fakeModelCatalog{err: errors.New("boom")}
	result := ListProviderModels(context.Background(), store, catalog, "github-copilot")
	if result.Source != domain.ProviderModelsSourceStatic || len(result.Models) == 0 {
		t.Errorf("expected static fallback with models, got %+v", result)
	}
	if result.Error != "boom" {
		t.Errorf("error = %q", result.Error)
	}
}

func TestListProviderModelsIgnoresCorruptCache(t *testing.T) {
	store := configuredCopilotStore()
	store.values[providerModelsCachePrefix+"github-copilot"] = "not json"
	catalog := &fakeModelCatalog{err: errors.New("down")}
	result := ListProviderModels(context.Background(), store, catalog, "github-copilot")
	if result.Source != domain.ProviderModelsSourceStatic || len(result.Models) == 0 {
		t.Errorf("expected static fallback on corrupt cache, got %+v", result)
	}
}
