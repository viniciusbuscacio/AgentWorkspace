package application

import (
	"context"
	"encoding/json"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

const (
	providerModelsTimeout     = 20 * time.Second
	providerModelsCachePrefix = "_config_models_cache_"
)

// ListProviderModels returns the models a provider can serve right now, in
// this priority order: (1) a fresh list from the provider's own models
// endpoint, which is also cached; (2) the cached list from the last successful
// fetch, when the endpoint is unavailable; (3) the static definition catalog,
// when nothing was ever fetched. The picker therefore always has options and
// they are as current as the provider allows.
func ListProviderModels(ctx context.Context, store ports.ProviderSecretStore, catalog ports.ProviderModelCatalog, provider string) domain.ProviderModelsResult {
	def, ok := providerDefinitionByID(store, provider)
	if !ok {
		return domain.ProviderModelsResult{Source: domain.ProviderModelsSourceStatic, Error: "unknown provider: " + provider}
	}
	static := domain.ProviderModelsResult{
		Models: append([]string(nil), def.Models...),
		Source: domain.ProviderModelsSourceStatic,
	}
	if catalog == nil {
		return static
	}
	cfg, err := ResolveNamedProviderRuntimeConfig(store, provider)
	if err != nil {
		// Not configured yet — nothing to fetch or to have cached.
		return static
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, providerModelsTimeout)
	defer cancel()
	models, err := catalog.ListModels(ctx, cfg)
	if err == nil && len(models) > 0 {
		writeCachedProviderModels(store, def.ID, models)
		return domain.ProviderModelsResult{Models: models, Source: domain.ProviderModelsSourceLive}
	}
	failure := "provider returned an empty model list"
	if err != nil {
		failure = err.Error()
	}
	if cached := readCachedProviderModels(store, def.ID); len(cached) > 0 {
		return domain.ProviderModelsResult{Models: cached, Source: domain.ProviderModelsSourceCache, Error: failure}
	}
	static.Error = failure
	return static
}

// writeCachedProviderModels persists the last successfully fetched list
// (best-effort: a write failure only means the next offline fallback is the
// static catalog instead).
func writeCachedProviderModels(store ports.ProviderSecretStore, providerID string, models []string) {
	payload, err := json.Marshal(models)
	if err != nil {
		return
	}
	_ = store.SetSecret(providerModelsCachePrefix+providerID, string(payload))
}

func readCachedProviderModels(store ports.ProviderSecretStore, providerID string) []string {
	raw, exists, err := store.GetSecret(providerModelsCachePrefix + providerID)
	if err != nil || !exists {
		return nil
	}
	var models []string
	if err := json.Unmarshal([]byte(raw), &models); err != nil {
		return nil
	}
	return models
}
