package application

import (
	"context"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// AuthenticateProviderViaBrowser runs a provider's interactive browser OAuth
// flow through the given authenticator and persists the resulting credential.
// The OAuth mechanics live in infrastructure (an adapter to the provider's
// auth server); this use case only orchestrates authenticate -> save.
func AuthenticateProviderViaBrowser(
	ctx context.Context,
	authenticator ports.ProviderBrowserAuthenticator,
	store ports.ProviderSecretStore,
	validator ports.ProviderCredentialValidator,
	provider string,
	model string,
) domain.ProviderOperationResult {
	provider = strings.TrimSpace(provider)
	if provider != "openai-codex" && provider != "github-copilot" {
		return domain.ProviderOperationResult{Success: false, Error: "browser authentication is only available for OpenAI Subscription and GitHub Copilot right now"}
	}
	if authenticator == nil {
		return domain.ProviderOperationResult{Success: false, Error: "browser authentication is not available"}
	}
	credential, err := authenticator.Authenticate(ctx)
	if err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	return SaveProviderConfig(ctx, store, validator, domain.ProviderSaveConfigInput{
		Provider:   provider,
		Model:      model,
		Credential: credential,
		SetActive:  true,
	})
}
