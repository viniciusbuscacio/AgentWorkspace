package domain

type ProviderDefinition struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	AuthType            string   `json:"authType"`
	ModelSecretKey      string   `json:"-"`
	APIKeySecretKey     string   `json:"-"`
	CredentialSecretKey string   `json:"-"`
	BaseURL             string   `json:"baseUrl,omitempty"`
	BaseURLSecretKey    string   `json:"-"`
	APIKeyPlaceholder   string   `json:"apiKeyPlaceholder,omitempty"`
	BaseURLPlaceholder  string   `json:"baseUrlPlaceholder,omitempty"`
	DefaultModel        string   `json:"defaultModel"`
	Models              []string `json:"models"`
	AuthDescription     string   `json:"authDescription,omitempty"`
	AllowCustomModel    bool     `json:"allowCustomModel,omitempty"`
}

type ProviderInfo struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	AuthType           string   `json:"authType"`
	Status             string   `json:"status"`
	Model              string   `json:"model,omitempty"`
	Connected          bool     `json:"connected"`
	BaseURL            string   `json:"baseUrl,omitempty"`
	APIKeyPlaceholder  string   `json:"apiKeyPlaceholder,omitempty"`
	BaseURLPlaceholder string   `json:"baseUrlPlaceholder,omitempty"`
	DefaultModel       string   `json:"defaultModel"`
	Models             []string `json:"models"`
	AuthDescription    string   `json:"authDescription,omitempty"`
	AllowCustomModel   bool     `json:"allowCustomModel,omitempty"`
	RequiresBaseURL    bool     `json:"requiresBaseUrl,omitempty"`
	// Enabled is the user's per-provider on/off switch. A disabled provider is
	// never used: not as the active provider and not in the fallback chain.
	// Which enabled provider actually runs the chat follows the priority order.
	Enabled bool `json:"enabled"`
	// Custom marks a user-created, dynamic OpenAI-compatible provider (as
	// opposed to the built-in definitions). Only custom providers can be
	// renamed from the Settings UI.
	Custom bool `json:"custom,omitempty"`
	// Deletable marks a provider whose slot can be removed from the list
	// entirely (dynamic custom providers and the hideable built-in
	// "Custom OpenAI-compatible" slot).
	Deletable bool `json:"deletable,omitempty"`
}

type ProviderStatus struct {
	Active    string         `json:"active"`
	Providers []ProviderInfo `json:"providers"`
	Error     string         `json:"error,omitempty"`
}

type ProviderSaveConfigInput struct {
	Provider       string `json:"provider"`
	Model          string `json:"model,omitempty"`
	APIKey         string `json:"apiKey,omitempty"`
	Credential     string `json:"credential,omitempty"`
	BaseURL        string `json:"baseUrl,omitempty"`
	SetActive      bool   `json:"setActive"`
	SkipValidation bool   `json:"skipValidation,omitempty"`
}

type ProviderOperationResult struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	Warning   string `json:"warning,omitempty"`
	Saved     bool   `json:"saved,omitempty"`
	Activated bool   `json:"activated,omitempty"`
	Canceled  bool   `json:"canceled,omitempty"`
	// ProviderID carries the id of a newly created custom provider so the UI
	// can open it for configuration right after creation.
	ProviderID string `json:"providerId,omitempty"`
}

// ProviderTestResult is the result of a one-shot provider test call.
// Latency is a human-readable duration string (e.g. "342ms").
// No key material is included in any field.
type ProviderTestResult struct {
	Success bool   `json:"success"`
	Latency string `json:"latency,omitempty"`
	Model   string `json:"model,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ProviderBalanceResult carries credit/usage data for providers that expose it
// (currently only OpenRouter). Available=false means the row should be hidden.
// No key material is included in any field.
type ProviderBalanceResult struct {
	Available bool   `json:"available"`
	Used      string `json:"used,omitempty"`
	Limit     string `json:"limit,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ProviderModelsResult carries the model ids a provider can currently serve.
// Source is "live" (fetched from the provider's models endpoint just now),
// "cache" (the last successfully fetched list, used when the endpoint is
// unavailable) or "static" (the built-in catalog fallback). Error explains a
// failed live fetch, if one was attempted. No key material in any field.
type ProviderModelsResult struct {
	Models []string `json:"models"`
	Source string   `json:"source"`
	Error  string   `json:"error,omitempty"`
}

const (
	ProviderModelsSourceLive   = "live"
	ProviderModelsSourceCache  = "cache"
	ProviderModelsSourceStatic = "static"
)

type ProviderRuntimeConfig struct {
	ProviderID        string             `json:"providerId"`
	ProviderName      string             `json:"providerName"`
	AuthType          string             `json:"authType"`
	Model             string             `json:"model"`
	APIKey            string             `json:"-"`
	Credential        string             `json:"-"`
	BaseURL           string             `json:"baseUrl,omitempty"`
	CredentialUpdater func(string) error `json:"-"`
}
