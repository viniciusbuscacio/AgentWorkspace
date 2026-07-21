package application

import (
	"context"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// providerTestTimeout and providerBalanceTimeout bound the provider probes so a
// hung endpoint cannot stall the caller. These are application policy, not
// transport detail, so they live with the use case rather than the adapter.
const (
	providerTestTimeout    = 30 * time.Second
	providerBalanceTimeout = 10 * time.Second
)

// TestProviderConnection validates a provider end to end using its stored (not
// active) config. API-key providers use the lightweight HTTP probe; OAuth
// providers (OpenAI Subscription, GitHub Copilot) need their full runtime
// adapter (token exchange, derived base URL, editor headers, model
// enablement), so they route through a real one-shot generation. No key
// material is ever placed in the result.
func TestProviderConnection(ctx context.Context, store ports.ProviderSecretStore, runtime ports.ChatTitleRuntime, probe ports.ProviderProbe, provider string) domain.ProviderTestResult {
	cfg, err := ResolveNamedProviderRuntimeConfig(store, provider)
	if err != nil {
		return domain.ProviderTestResult{Success: false, Error: err.Error()}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, providerTestTimeout)
	defer cancel()
	if cfg.AuthType != "api-key" {
		return TestProviderViaRuntime(ctx, runtime, cfg)
	}
	return probe.TestConnection(ctx, cfg)
}

// GetProviderBalance fetches credit/usage data for providers that expose such
// an endpoint (currently only OpenRouter). Available=false means the caller
// should hide the balance row. No key material is included in the result.
func GetProviderBalance(ctx context.Context, store ports.ProviderSecretStore, probe ports.ProviderProbe, provider string) domain.ProviderBalanceResult {
	if provider != "openrouter" {
		return domain.ProviderBalanceResult{Available: false}
	}
	cfg, err := ResolveNamedProviderRuntimeConfig(store, provider)
	if err != nil {
		return domain.ProviderBalanceResult{Available: false}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, providerBalanceTimeout)
	defer cancel()
	return probe.FetchBalance(ctx, cfg)
}
