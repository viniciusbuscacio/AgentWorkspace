package ports

import (
	"context"

	"aw/internal/domain"
)

type ProviderSecretStore interface {
	SetSecret(name string, value string) error
	GetSecret(name string) (string, bool, error)
	HasSecret(name string) (bool, error)
	DeleteSecret(name string) error
}

type ProviderCredentialValidator interface {
	ValidateAPIKeyProvider(ctx context.Context, def domain.ProviderDefinition, apiKey string, model string, baseURL string) error
}

// ProviderBrowserAuthenticator runs a provider's interactive browser OAuth
// flow and returns the credential JSON to persist in the vault. The OAuth
// mechanics (PKCE, callback server, token exchange) are an adapter to the
// provider's external auth server, implemented in infrastructure.
type ProviderBrowserAuthenticator interface {
	Authenticate(ctx context.Context) (string, error)
}

// ProviderModelCatalog fetches the model ids a configured provider can
// actually serve right now from the provider's live models endpoint. The HTTP
// and token-exchange mechanics live in infrastructure. No method returns key
// material.
type ProviderModelCatalog interface {
	ListModels(ctx context.Context, cfg domain.ProviderRuntimeConfig) ([]string, error)
}

// ProviderProbe performs outbound network probes against a provider's HTTP
// endpoints. The HTTP mechanics live in infrastructure; the application layer
// depends only on this port so the interface layer never issues provider I/O
// directly. No method returns key material.
type ProviderProbe interface {
	// TestConnection runs a one-shot reachability probe against an API-key
	// provider's stored runtime config. OAuth providers are validated through
	// the runtime path in the use case, not here.
	TestConnection(ctx context.Context, cfg domain.ProviderRuntimeConfig) domain.ProviderTestResult
	// FetchBalance returns credit/usage for providers that expose it (currently
	// only OpenRouter). Available=false means the caller should hide the row.
	FetchBalance(ctx context.Context, cfg domain.ProviderRuntimeConfig) domain.ProviderBalanceResult
}
