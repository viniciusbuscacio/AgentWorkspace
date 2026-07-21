package application

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

const (
	DefaultProviderID           = "github-copilot"
	activeProviderSecret        = "_config_provider"
	customProvidersSecret       = "_config_custom_providers"
	hiddenBuiltinsSecret        = "_config_hidden_builtin_providers"
	disabledProvidersSecret     = "_config_disabled_providers"
	builtinCustomOpenAIID       = "custom-openai"
	customProviderIDPrefix      = "custom-"
	noProviderConfiguredMessage = "No provider is configured yet. Open Settings > LLM Providers to add and activate a provider."
)

// disabledProviderIDs loads the user's per-provider off switches (a JSON array
// of provider ids). Missing/corrupt data means "everything enabled".
func disabledProviderIDs(store ports.ProviderSecretStore) map[string]bool {
	disabled := map[string]bool{}
	value, exists, err := store.GetSecret(disabledProvidersSecret)
	if err != nil || !exists || strings.TrimSpace(value) == "" {
		return disabled
	}
	var ids []string
	if err := json.Unmarshal([]byte(value), &ids); err != nil {
		return disabled
	}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			disabled[id] = true
		}
	}
	return disabled
}

func saveDisabledProviderIDs(store ports.ProviderSecretStore, disabled map[string]bool) error {
	if len(disabled) == 0 {
		return store.DeleteSecret(disabledProvidersSecret)
	}
	ids := make([]string, 0, len(disabled))
	for id := range disabled {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	payload, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return store.SetSecret(disabledProvidersSecret, string(payload))
}

type ProviderDefinition = domain.ProviderDefinition
type ProviderInfo = domain.ProviderInfo
type ProviderStatus = domain.ProviderStatus
type ProviderSaveConfigInput = domain.ProviderSaveConfigInput
type ProviderOperationResult = domain.ProviderOperationResult
type ProviderRuntimeConfig = domain.ProviderRuntimeConfig

var providerDefinitions = []domain.ProviderDefinition{
	{
		ID:                  "github-copilot",
		Name:                "GitHub Copilot",
		AuthType:            "oauth-device-code",
		ModelSecretKey:      "_config_model_copilot",
		CredentialSecretKey: "github_copilot_auth_json",
		DefaultModel:        "claude-opus-4.8",
		Models: []string{
			"claude-sonnet-4.5", "claude-sonnet-4.6", "claude-sonnet-5",
			"claude-haiku-4.5",
			"claude-opus-4.5", "claude-opus-4.6", "claude-opus-4.7", "claude-opus-4.8", "claude-opus-4.8-fast",
			"claude-fable-5",
			"gpt-4.1", "gpt-5-mini", "gpt-5.2", "gpt-5.3-codex",
			"gpt-5.4", "gpt-5.4-mini", "gpt-5.5",
			"gemini-2.5-pro", "gemini-3-flash-preview", "gemini-3.1-pro-preview", "gemini-3.5-flash",
			"kimi-k2.7-code",
		},
		AuthDescription: "Sign in with your GitHub account (device flow). Requires an active Copilot subscription.",
	},
	{
		ID:                "openrouter",
		Name:              "OpenRouter",
		AuthType:          "api-key",
		ModelSecretKey:    "_config_model_openrouter",
		APIKeySecretKey:   "openrouter_api_key",
		BaseURL:           "https://openrouter.ai/api/v1",
		APIKeyPlaceholder: "sk-or-...",
		DefaultModel:      "z-ai/glm-5.2",
		AllowCustomModel:  true,
		// Static fallback catalog, curated from the live /models endpoint on
		// 2026-07-09. The Refresh button (and opening the provider editor)
		// replaces this with the live list, so it only needs to stay roughly
		// current, not complete.
		Models: []string{
			"z-ai/glm-5.2",
			"z-ai/glm-5",
			"anthropic/claude-fable-5",
			"anthropic/claude-sonnet-5",
			"anthropic/claude-sonnet-4.6",
			"anthropic/claude-opus-4.8",
			"anthropic/claude-haiku-4.5",
			"openai/gpt-5.5",
			"openai/gpt-5.4",
			"openai/gpt-5.4-mini",
			"openai/gpt-5.3-codex",
			"google/gemini-3.1-pro-preview",
			"google/gemini-3.5-flash",
			"x-ai/grok-4.5",
			"deepseek/deepseek-v4-pro",
			"deepseek/deepseek-v4-flash",
			"deepseek/deepseek-r1",
			"qwen/qwen3-max",
			"moonshotai/kimi-k2.7-code",
			"moonshotai/kimi-k2.6",
			"minimax/minimax-m3",
			"meta-llama/llama-4-maverick",
			"mistralai/mistral-large-2512",
			"meta-llama/llama-3.3-70b-instruct:free",
			"openai/gpt-oss-120b:free",
			"qwen/qwen3-coder:free",
		},
	},
	{
		ID:                "fireworks",
		Name:              "Fireworks",
		AuthType:          "api-key",
		ModelSecretKey:    "_config_model_fireworks",
		APIKeySecretKey:   "fireworks_api_key",
		BaseURL:           "https://api.fireworks.ai/inference/v1",
		APIKeyPlaceholder: "fw_...",
		// Serverless catalog default: cheapest strong pick per the user's
		// 2026-07-09 pricing survey. Custom LoRA deployments use the full
		// accounts/<acct>/models/<m>#accounts/<acct>/deployments/<id> string,
		// hence AllowCustomModel. The live /models refresh corrects this
		// static list whenever the key is configured.
		DefaultModel:     "accounts/fireworks/models/deepseek-v4-flash",
		AllowCustomModel: true,
		Models: []string{
			"accounts/fireworks/models/deepseek-v4-flash",
			"accounts/fireworks/models/deepseek-v4-pro",
			"accounts/fireworks/models/glm-5p2",
			"accounts/fireworks/models/kimi-k2p7-code",
			"accounts/fireworks/models/kimi-k2p6",
			"accounts/fireworks/models/minimax-m3",
			"accounts/fireworks/models/gpt-oss-120b",
			"accounts/fireworks/models/llama4-maverick-instruct-basic",
		},
	},
	{
		ID:                "openai",
		Name:              "OpenAI API Key",
		AuthType:          "api-key",
		ModelSecretKey:    "_config_model_openai",
		APIKeySecretKey:   "openai_api_key",
		BaseURL:           "https://api.openai.com/v1",
		APIKeyPlaceholder: "sk-...",
		DefaultModel:      "gpt-5.5",
		Models:            []string{"gpt-5.5", "gpt-5.5-pro", "gpt-5.4", "gpt-5.4-mini", "gpt-5.4-pro", "gpt-5.3-codex", "gpt-5.2-codex", "gpt-5.2", "gpt-5.1-codex-max", "gpt-5.1-codex-mini", "gpt-5.1"},
	},
	{
		ID:                  "openai-codex",
		Name:                "OpenAI Subscription",
		AuthType:            "oauth-browser",
		ModelSecretKey:      "_config_model_openai_codex",
		CredentialSecretKey: "openai_codex_auth_json",
		DefaultModel:        "gpt-5.5",
		Models:              []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex", "gpt-5.2-codex", "gpt-5.2", "gpt-5.1-codex-max", "gpt-5.1-codex-mini", "gpt-5.1"},
		AuthDescription:     "Paste OpenAI Codex OAuth/token JSON to keep it inside the encrypted vault.",
	},
	{
		ID:                "azure-openai",
		Name:              "Azure OpenAI",
		AuthType:          "api-key",
		ModelSecretKey:    "_config_model_azure_openai",
		APIKeySecretKey:   "azure_openai_api_key",
		APIKeyPlaceholder: "Azure OpenAI API key",
		DefaultModel:      "gpt-4.1",
		Models:            []string{"gpt-4.1", "gpt-4.1-mini", "gpt-4o", "gpt-4o-mini", "o3", "o4-mini"},
	},
	{
		ID:                "nvidia",
		Name:              "NVIDIA NIM",
		AuthType:          "api-key",
		ModelSecretKey:    "_config_model_nvidia",
		APIKeySecretKey:   "nvidia_api_key",
		BaseURL:           "https://integrate.api.nvidia.com/v1",
		APIKeyPlaceholder: "nvapi-...",
		DefaultModel:      "nvidia/llama-3.1-nemotron-70b-instruct",
		Models:            []string{"nvidia/llama-3.1-nemotron-70b-instruct", "meta/llama-3.1-405b-instruct", "mistralai/mixtral-8x22b-instruct-v0.1"},
	},
	{
		ID:                 "custom-openai",
		Name:               "Custom OpenAI-compatible",
		AuthType:           "api-key",
		ModelSecretKey:     "_config_model_custom_openai",
		APIKeySecretKey:    "custom_openai_api_key",
		BaseURLSecretKey:   "_config_base_url_custom_openai",
		APIKeyPlaceholder:  "API key",
		BaseURLPlaceholder: "https://your-provider.example/v1",
		DefaultModel:       "model-id",
		Models:             []string{"model-id"},
		AllowCustomModel:   true,
	},
}

func ProviderDefinitions() []domain.ProviderDefinition {
	out := make([]domain.ProviderDefinition, len(providerDefinitions))
	copy(out, providerDefinitions)
	return out
}

// AllProviderDefinitions returns the built-in provider definitions plus any
// user-created custom OpenAI-compatible providers persisted in the vault.
func AllProviderDefinitions(store ports.ProviderSecretStore) []domain.ProviderDefinition {
	return allProviderDefinitions(store)
}

// customProviderEntry is the persisted identity of a user-created provider.
// Credentials, base URL and model live in derived per-id secret keys (see
// customProviderDefinition), exactly like the built-in custom-openai slot.
type customProviderEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func loadCustomProviderEntries(store ports.ProviderSecretStore) ([]customProviderEntry, error) {
	value, exists, err := store.GetSecret(customProvidersSecret)
	if err != nil {
		return nil, err
	}
	if !exists || strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var entries []customProviderEntry
	if err := json.Unmarshal([]byte(value), &entries); err != nil {
		return nil, fmt.Errorf("decode custom providers: %w", err)
	}
	cleaned := make([]customProviderEntry, 0, len(entries))
	for _, entry := range entries {
		entry.ID = strings.TrimSpace(entry.ID)
		entry.Name = strings.TrimSpace(entry.Name)
		if entry.ID == "" {
			continue
		}
		cleaned = append(cleaned, entry)
	}
	return cleaned, nil
}

func saveCustomProviderEntries(store ports.ProviderSecretStore, entries []customProviderEntry) error {
	if len(entries) == 0 {
		return store.DeleteSecret(customProvidersSecret)
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return store.SetSecret(customProvidersSecret, string(encoded))
}

// customProviderDefinition builds an on-the-fly api-key provider definition for
// a dynamic custom provider. Secret keys are derived from the id so every
// instance gets its own isolated key/base-URL/model storage.
func customProviderDefinition(entry customProviderEntry) domain.ProviderDefinition {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		name = "Custom OpenAI-compatible"
	}
	return domain.ProviderDefinition{
		ID:                 entry.ID,
		Name:               name,
		AuthType:           "api-key",
		ModelSecretKey:     "_config_model_" + entry.ID,
		APIKeySecretKey:    "custom_openai_api_key_" + entry.ID,
		BaseURLSecretKey:   "_config_base_url_" + entry.ID,
		APIKeyPlaceholder:  "API key",
		BaseURLPlaceholder: "https://your-provider.example/v1",
		DefaultModel:       "model-id",
		Models:             []string{"model-id"},
		AllowCustomModel:   true,
	}
}

func customProviderDefinitions(store ports.ProviderSecretStore) []domain.ProviderDefinition {
	entries, err := loadCustomProviderEntries(store)
	if err != nil {
		return nil
	}
	defs := make([]domain.ProviderDefinition, 0, len(entries))
	for _, entry := range entries {
		defs = append(defs, customProviderDefinition(entry))
	}
	return defs
}

// allProviderDefinitions merges the static built-ins with the dynamic custom
// providers, dropping any built-in the user has hidden (deleted). The static
// slice is never mutated.
func allProviderDefinitions(store ports.ProviderSecretStore) []domain.ProviderDefinition {
	var hidden map[string]bool
	if store != nil {
		hidden = hiddenBuiltinSet(store)
	}
	defs := make([]domain.ProviderDefinition, 0, len(providerDefinitions)+2)
	for _, def := range providerDefinitions {
		if hidden[def.ID] {
			continue
		}
		defs = append(defs, def)
	}
	if store != nil {
		defs = append(defs, customProviderDefinitions(store)...)
	}
	return defs
}

// isHideableBuiltinID reports whether a built-in provider may be removed from
// the list (currently only the original "Custom OpenAI-compatible" slot).
func isHideableBuiltinID(id string) bool {
	return id == builtinCustomOpenAIID
}

// builtinDefinitionByID looks an id up in the static built-in slice only.
func builtinDefinitionByID(id string) (domain.ProviderDefinition, bool) {
	for _, def := range providerDefinitions {
		if def.ID == id {
			return def, true
		}
	}
	return domain.ProviderDefinition{}, false
}

func loadHiddenBuiltins(store ports.ProviderSecretStore) []string {
	value, exists, err := store.GetSecret(hiddenBuiltinsSecret)
	if err != nil || !exists || strings.TrimSpace(value) == "" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(value), &ids); err != nil {
		return nil
	}
	return ids
}

func saveHiddenBuiltins(store ports.ProviderSecretStore, ids []string) error {
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		cleaned = append(cleaned, id)
	}
	if len(cleaned) == 0 {
		return store.DeleteSecret(hiddenBuiltinsSecret)
	}
	encoded, err := json.Marshal(cleaned)
	if err != nil {
		return err
	}
	return store.SetSecret(hiddenBuiltinsSecret, string(encoded))
}

func hiddenBuiltinSet(store ports.ProviderSecretStore) map[string]bool {
	set := map[string]bool{}
	for _, id := range loadHiddenBuiltins(store) {
		set[id] = true
	}
	return set
}

// customProviderIDs returns the set of dynamic custom provider ids so callers
// can flag them in the UI (only these are renamable / deletable).
func customProviderIDs(store ports.ProviderSecretStore) map[string]bool {
	ids := map[string]bool{}
	for _, def := range customProviderDefinitions(store) {
		ids[def.ID] = true
	}
	return ids
}

// newCustomProviderID generates a collision-free id with the custom- prefix.
func newCustomProviderID(store ports.ProviderSecretStore, entries []customProviderEntry) (string, error) {
	taken := map[string]bool{}
	for _, def := range allProviderDefinitions(store) {
		taken[def.ID] = true
	}
	for _, entry := range entries {
		taken[entry.ID] = true
	}
	for attempt := 0; attempt < 20; attempt++ {
		buf := make([]byte, 5)
		if _, err := cryptorand.Read(buf); err != nil {
			return "", err
		}
		id := customProviderIDPrefix + hex.EncodeToString(buf)
		if !taken[id] {
			return id, nil
		}
	}
	return "", errors.New("could not allocate a unique custom provider id")
}

func NormalizeProvider(id string) string {
	id = strings.TrimSpace(id)
	for _, def := range providerDefinitions {
		if def.ID == id {
			return id
		}
	}
	return DefaultProviderID
}

// normalizeProviderID is the store-aware variant used internally so a dynamic
// custom provider can be the active/normalized provider.
func normalizeProviderID(store ports.ProviderSecretStore, id string) string {
	id = strings.TrimSpace(id)
	if _, ok := providerDefinitionByID(store, id); ok {
		return id
	}
	return DefaultProviderID
}

func GetProviderStatus(store ports.ProviderSecretStore) domain.ProviderStatus {
	active := DefaultProviderID
	if value, exists, err := store.GetSecret(activeProviderSecret); err == nil && exists {
		active = normalizeProviderID(store, value)
	} else if err != nil {
		return domain.ProviderStatus{Active: active, Providers: []domain.ProviderInfo{}, Error: err.Error()}
	}

	defs := allProviderDefinitions(store)
	customIDs := customProviderIDs(store)
	disabled := disabledProviderIDs(store)
	providers := make([]domain.ProviderInfo, 0, len(defs))
	for _, def := range defs {
		connected, err := providerConnected(store, def)
		if err != nil {
			return domain.ProviderStatus{Active: active, Providers: providers, Error: err.Error()}
		}
		// The generic built-in "Custom OpenAI-compatible" slot stays hidden until
		// configured: a fresh vault shows no empty custom slot (the user adds
		// custom providers via "Add custom provider"). A vault that already
		// configured it keeps it visible.
		if def.ID == builtinCustomOpenAIID && !connected {
			continue
		}
		model, err := providerModel(store, def, connected)
		if err != nil {
			return domain.ProviderStatus{Active: active, Providers: providers, Error: err.Error()}
		}
		baseURL, err := providerBaseURL(store, def)
		if err != nil {
			return domain.ProviderStatus{Active: active, Providers: providers, Error: err.Error()}
		}
		status := "not-configured"
		if connected && active == def.ID && !disabled[def.ID] {
			status = "active"
		} else if connected {
			status = "configured"
		}
		providers = append(providers, domain.ProviderInfo{
			ID:                 def.ID,
			Name:               def.Name,
			AuthType:           def.AuthType,
			Status:             status,
			Model:              model,
			Connected:          connected,
			BaseURL:            baseURL,
			APIKeyPlaceholder:  def.APIKeyPlaceholder,
			BaseURLPlaceholder: def.BaseURLPlaceholder,
			DefaultModel:       def.DefaultModel,
			Models:             append([]string(nil), def.Models...),
			AuthDescription:    def.AuthDescription,
			AllowCustomModel:   def.AllowCustomModel,
			RequiresBaseURL:    def.BaseURLSecretKey != "",
			Custom:             customIDs[def.ID],
			Deletable:          customIDs[def.ID] || isHideableBuiltinID(def.ID),
			// A provider that was never configured starts disabled: only a
			// connected (configured) provider is enabled by default, and only
			// then can the user toggle it off. The UI already greys the toggle
			// for unconfigured providers; this keeps its state honest (off).
			Enabled: connected && !disabled[def.ID],
		})
	}
	return domain.ProviderStatus{Active: active, Providers: providers}
}

// normalizeOpenAICompatibleBaseURL cleans a user-entered OpenAI-compatible
// base URL so the runtime can safely append "/chat/completions". It trims
// whitespace and trailing slashes and, if the user pasted the full endpoint
// (e.g. https://chat.maritaca.ai/api/chat/completions), strips that suffix so
// the request does not become .../chat/completions/chat/completions (404).
func normalizeOpenAICompatibleBaseURL(raw string) string {
	url := strings.TrimSpace(raw)
	if url == "" {
		return ""
	}
	url = strings.TrimRight(url, "/")
	const suffix = "/chat/completions"
	if len(url) >= len(suffix) && strings.EqualFold(url[len(url)-len(suffix):], suffix) {
		url = strings.TrimRight(url[:len(url)-len(suffix)], "/")
	}
	return url
}

func SaveProviderConfig(ctx context.Context, store ports.ProviderSecretStore, validator ports.ProviderCredentialValidator, input domain.ProviderSaveConfigInput) domain.ProviderOperationResult {
	def, ok := providerDefinitionByID(store, input.Provider)
	if !ok {
		return domain.ProviderOperationResult{Success: false, Error: fmt.Sprintf("unsupported provider: %s", input.Provider)}
	}

	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = def.DefaultModel
	}
	if model != "" {
		if err := store.SetSecret(def.ModelSecretKey, model); err != nil {
			return domain.ProviderOperationResult{Success: false, Error: err.Error()}
		}
	}

	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey != "" && def.APIKeySecretKey != "" {
		if err := store.SetSecret(def.APIKeySecretKey, apiKey); err != nil {
			return domain.ProviderOperationResult{Success: false, Error: err.Error()}
		}
	}

	credential := strings.TrimSpace(input.Credential)
	if credential != "" && def.CredentialSecretKey != "" {
		if err := store.SetSecret(def.CredentialSecretKey, credential); err != nil {
			return domain.ProviderOperationResult{Success: false, Error: err.Error()}
		}
	}

	baseURL := normalizeOpenAICompatibleBaseURL(input.BaseURL)
	if baseURL != "" && def.BaseURLSecretKey != "" {
		if err := store.SetSecret(def.BaseURLSecretKey, baseURL); err != nil {
			return domain.ProviderOperationResult{Success: false, Error: err.Error()}
		}
	}

	if !input.SetActive {
		return domain.ProviderOperationResult{Success: true, Saved: true}
	}

	if err := activateProvider(ctx, store, validator, def, model, apiKey, baseURL, input.SkipValidation); err != nil {
		return domain.ProviderOperationResult{
			Success: true,
			Saved:   true,
			Warning: fmt.Sprintf("%s Configuration was saved, but this provider was not activated.", err.Error()),
		}
	}

	return domain.ProviderOperationResult{Success: true, Saved: true, Activated: true}
}

func SwitchProvider(ctx context.Context, store ports.ProviderSecretStore, validator ports.ProviderCredentialValidator, provider string, model string) domain.ProviderOperationResult {
	def, ok := providerDefinitionByID(store, provider)
	if !ok {
		return domain.ProviderOperationResult{Success: false, Error: fmt.Sprintf("unsupported provider: %s", provider)}
	}
	targetModel := strings.TrimSpace(model)
	if targetModel == "" {
		var err error
		targetModel, err = providerModel(store, def, true)
		if err != nil {
			return domain.ProviderOperationResult{Success: false, Error: err.Error()}
		}
	}

	if err := activateProvider(ctx, store, validator, def, targetModel, "", "", false); err != nil {
		return domain.ProviderOperationResult{Success: false, Error: fmt.Sprintf("%s Provider was not activated.", err.Error())}
	}
	if targetModel != "" {
		if err := store.SetSecret(def.ModelSecretKey, targetModel); err != nil {
			return domain.ProviderOperationResult{Success: false, Error: err.Error()}
		}
	}
	return domain.ProviderOperationResult{Success: true, Activated: true}
}

// SyncActiveProviderToOrder aligns the active provider (the global default)
// with the user-defined priority list: #1 IS the default, so the first
// configured+enabled provider of the order becomes active. Returns the id it
// activated, or "" when the effective head is already active or nothing in the
// order is usable (then the current active stays). A locked vault no-ops
// naturally: providerIsConnected reads vault secrets, so nothing qualifies.
func SyncActiveProviderToOrder(store ports.ProviderSecretStore, order []string) (string, error) {
	disabled := disabledProviderIDs(store)
	for _, id := range order {
		id = strings.TrimSpace(id)
		if id == "" || disabled[id] {
			continue
		}
		def, ok := providerDefinitionByID(store, id)
		if !ok || !providerIsConnected(store, def) {
			continue
		}
		active, hasActive, err := store.GetSecret(activeProviderSecret)
		if err == nil && hasActive && normalizeProviderID(store, active) == def.ID {
			return "", nil
		}
		if err := store.SetSecret(activeProviderSecret, def.ID); err != nil {
			return "", err
		}
		return def.ID, nil
	}
	return "", nil
}

// MoveProviderToOrderHead puts provider at #1 of the persisted priority list.
// Activation and #1 are the same lever seen from two places: switching the
// active provider must also make it the head of the order, or the numbered
// list would contradict what new chats actually use.
func MoveProviderToOrderHead(store ports.ProviderFallbackStore, provider string) error {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return nil
	}
	order, err := store.LoadProviderFallbackOrder()
	if err != nil {
		order = nil
	}
	if len(order) > 0 && order[0] == provider {
		return nil
	}
	next := make([]string, 0, len(order)+1)
	next = append(next, provider)
	for _, id := range order {
		if id != provider {
			next = append(next, id)
		}
	}
	return store.SaveProviderFallbackOrder(next)
}

// SetProviderEnabled flips the user's per-provider on/off switch. Disabling
// the provider that is currently active hands activity over to the highest
// priority enabled+configured provider (per the fallback order); enabling a
// provider only makes it eligible again — it does not steal activity, except
// when nothing is active yet.
func SetProviderEnabled(store ports.ProviderSecretStore, order []string, provider string, enabled bool) domain.ProviderOperationResult {
	def, ok := providerDefinitionByID(store, provider)
	if !ok {
		return domain.ProviderOperationResult{Success: false, Error: fmt.Sprintf("unsupported provider: %s", provider)}
	}
	disabled := disabledProviderIDs(store)
	if enabled == !disabled[def.ID] {
		return domain.ProviderOperationResult{Success: true}
	}
	if enabled {
		delete(disabled, def.ID)
	} else {
		disabled[def.ID] = true
	}
	if err := saveDisabledProviderIDs(store, disabled); err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}

	active, hasActive, err := store.GetSecret(activeProviderSecret)
	if err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}

	if enabled {
		// Nothing active yet and this provider is usable: it becomes active.
		if (!hasActive || strings.TrimSpace(active) == "") && providerIsConnected(store, def) {
			if err := store.SetSecret(activeProviderSecret, def.ID); err != nil {
				return domain.ProviderOperationResult{Success: false, Error: err.Error()}
			}
			return domain.ProviderOperationResult{Success: true, Activated: true}
		}
		return domain.ProviderOperationResult{Success: true}
	}

	// Disabled the active provider: hand over to the next enabled+configured
	// one by priority; with none left, no provider stays active.
	if hasActive && normalizeProviderID(store, active) == def.ID {
		return handOverActiveProvider(store, order, def)
	}
	return domain.ProviderOperationResult{Success: true}
}

// handOverActiveProvider re-elects the active provider after def stopped being
// usable (disabled, or its credential was deleted): the highest-priority
// enabled+connected provider takes over; with none left the active slot is
// cleared with a warning. Leaving the slot silently empty is not an option —
// everything that resolves the workspace default (session restore, compaction,
// run telemetry) breaks quietly while chats keep working via fallback.
func handOverActiveProvider(store ports.ProviderSecretStore, order []string, def domain.ProviderDefinition) domain.ProviderOperationResult {
	for _, id := range fallbackCandidateIDs(store, order) {
		next, ok := providerDefinitionByID(store, id)
		if !ok || next.ID == def.ID || !providerIsConnected(store, next) {
			continue
		}
		if err := store.SetSecret(activeProviderSecret, next.ID); err != nil {
			return domain.ProviderOperationResult{Success: false, Error: err.Error()}
		}
		return domain.ProviderOperationResult{Success: true, Activated: true, ProviderID: next.ID}
	}
	if err := store.DeleteSecret(activeProviderSecret); err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	return domain.ProviderOperationResult{Success: true, Warning: "No enabled provider remains active. Enable a configured provider to keep the chat working."}
}

// providerIsConnected is providerConnected with errors treated as "no".
func providerIsConnected(store ports.ProviderSecretStore, def domain.ProviderDefinition) bool {
	connected, err := providerConnected(store, def)
	return err == nil && connected
}

func DeleteProviderCredential(store ports.ProviderSecretStore, order []string, provider string) domain.ProviderOperationResult {
	def, ok := providerDefinitionByID(store, provider)
	if !ok {
		return domain.ProviderOperationResult{Success: false, Error: fmt.Sprintf("unsupported provider: %s", provider)}
	}
	if def.APIKeySecretKey != "" {
		if err := store.DeleteSecret(def.APIKeySecretKey); err != nil {
			return domain.ProviderOperationResult{Success: false, Error: err.Error()}
		}
	}
	if def.CredentialSecretKey != "" {
		if err := store.DeleteSecret(def.CredentialSecretKey); err != nil {
			return domain.ProviderOperationResult{Success: false, Error: err.Error()}
		}
	}
	// Deleting the active provider's credential must hand activity over, not
	// leave the workspace with no default (that broke session restore and
	// compaction silently while chats kept working via fallback).
	if active, exists, err := store.GetSecret(activeProviderSecret); err == nil && exists && normalizeProviderID(store, active) == def.ID {
		return handOverActiveProvider(store, order, def)
	} else if err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	return domain.ProviderOperationResult{Success: true}
}

func ResolveProviderRuntimeConfig(store ports.ProviderSecretStore) (domain.ProviderRuntimeConfig, error) {
	value, exists, err := store.GetSecret(activeProviderSecret)
	if err != nil {
		return domain.ProviderRuntimeConfig{}, err
	}
	if !exists || strings.TrimSpace(value) == "" {
		return domain.ProviderRuntimeConfig{}, errors.New(noProviderConfiguredMessage)
	}
	active := normalizeProviderID(store, value)
	def, ok := providerDefinitionByID(store, active)
	if !ok {
		return domain.ProviderRuntimeConfig{}, fmt.Errorf("unsupported provider: %s", active)
	}
	model, err := providerModel(store, def, true)
	if err != nil {
		return domain.ProviderRuntimeConfig{}, err
	}
	if strings.TrimSpace(model) == "" {
		model = def.DefaultModel
	}
	baseURL, err := providerBaseURL(store, def)
	if err != nil {
		return domain.ProviderRuntimeConfig{}, err
	}

	config := domain.ProviderRuntimeConfig{
		ProviderID:   def.ID,
		ProviderName: def.Name,
		AuthType:     def.AuthType,
		Model:        model,
		BaseURL:      baseURL,
	}

	if def.AuthType == "api-key" {
		if def.APIKeySecretKey == "" {
			return domain.ProviderRuntimeConfig{}, fmt.Errorf("%s has no API key secret configured", def.Name)
		}
		apiKey, exists, err := store.GetSecret(def.APIKeySecretKey)
		if err != nil {
			return domain.ProviderRuntimeConfig{}, err
		}
		if !exists || strings.TrimSpace(apiKey) == "" {
			return domain.ProviderRuntimeConfig{}, fmt.Errorf("%s API key is not configured", def.Name)
		}
		if strings.TrimSpace(config.BaseURL) == "" {
			return domain.ProviderRuntimeConfig{}, fmt.Errorf("%s base URL is not configured", def.Name)
		}
		config.APIKey = apiKey
		return config, nil
	}

	if def.CredentialSecretKey == "" {
		return domain.ProviderRuntimeConfig{}, fmt.Errorf("%s credential storage is not configured", def.Name)
	}
	credential, exists, err := store.GetSecret(def.CredentialSecretKey)
	if err != nil {
		return domain.ProviderRuntimeConfig{}, err
	}
	if !exists || strings.TrimSpace(credential) == "" {
		return domain.ProviderRuntimeConfig{}, fmt.Errorf("%s credential is not configured", def.Name)
	}
	config.Credential = credential
	config.CredentialUpdater = func(next string) error {
		return store.SetSecret(def.CredentialSecretKey, next)
	}
	return config, nil
}

func ModelConfigFromProviderRuntimeConfig(config domain.ProviderRuntimeConfig) domain.ModelConfig {
	return domain.ModelConfig(config)
}

// ResolveEffectiveProviderRuntimeConfig resolves the runtime config a chat
// should use: a per-chat override when set (that provider, with the override
// model if given), otherwise the global active provider. A broken override
// (provider no longer configured) transparently falls back to the global one.
func ResolveEffectiveProviderRuntimeConfig(store ports.ProviderSecretStore, overrideProvider, overrideModel string) (domain.ProviderRuntimeConfig, error) {
	overrideProvider = strings.TrimSpace(overrideProvider)
	if overrideProvider == "" {
		return ResolveProviderRuntimeConfig(store)
	}
	cfg, err := ResolveNamedProviderRuntimeConfig(store, overrideProvider)
	if err != nil {
		return ResolveProviderRuntimeConfig(store)
	}
	if m := strings.TrimSpace(overrideModel); m != "" {
		cfg.Model = m
	}
	return cfg, nil
}

// ResolveChatFallbackChain is ResolveProviderFallbackChain with an optional
// per-chat override placed at the head of the chain, so the chat's chosen
// provider/model is tried first and the normal fallback still follows.
func ResolveChatFallbackChain(store ports.ProviderSecretStore, order []string, overrideProvider, overrideModel string) ([]domain.ProviderRuntimeConfig, error) {
	chain, err := ResolveProviderFallbackChain(store, order)
	if err != nil {
		return nil, err
	}
	overrideProvider = strings.TrimSpace(overrideProvider)
	if overrideProvider == "" {
		return chain, nil
	}
	head, herr := ResolveNamedProviderRuntimeConfig(store, overrideProvider)
	if herr != nil {
		// Override no longer usable — keep the normal chain.
		return chain, nil
	}
	if m := strings.TrimSpace(overrideModel); m != "" {
		head.Model = m
	}
	out := []domain.ProviderRuntimeConfig{head}
	for _, cfg := range chain {
		if cfg.ProviderID == head.ProviderID {
			continue
		}
		out = append(out, cfg)
	}
	return out, nil
}

// GetChatModelOverride reads a chat's provider/model override (empty = global).
func GetChatModelOverride(store ports.ChatModelOverrideStore, chatID string) (string, string, error) {
	return store.ChatModelOverride(chatID)
}

// SetChatModelOverride persists a chat's provider/model override. Empty
// provider clears it (the chat reverts to the global active provider).
func SetChatModelOverride(store ports.ChatModelOverrideStore, chatID, provider, model string) error {
	return store.SetChatModelOverride(chatID, strings.TrimSpace(provider), strings.TrimSpace(model))
}

// ChatModelSelectionResult reports the effective per-chat model after a change,
// so the caller can surface a system message describing what will now be used.
type ChatModelSelectionResult struct {
	Provider     string
	ProviderName string
	Model        string
	// Cleared is true when the override was removed and the chat reverts to the
	// global active provider (Provider/Model then describe that global default).
	Cleared bool
}

// SetChatModel sets (or clears) a chat's per-chat provider/model override.
// provider may be a provider ID or its display name (case-insensitive). An empty
// provider clears the override and reports the global default that now applies.
// The target provider must be connected; a given model must be offered by that
// provider unless it permits custom models.
func SetChatModel(providerStore ports.ProviderSecretStore, overrideStore ports.ChatModelOverrideStore, chatID, provider, model string) (ChatModelSelectionResult, error) {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return ChatModelSelectionResult{}, errors.New("chat id is required")
	}
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)

	if provider == "" {
		if err := overrideStore.SetChatModelOverride(chatID, "", ""); err != nil {
			return ChatModelSelectionResult{}, err
		}
		cfg, err := ResolveProviderRuntimeConfig(providerStore)
		if err != nil {
			return ChatModelSelectionResult{Cleared: true}, nil
		}
		return ChatModelSelectionResult{Provider: cfg.ProviderID, ProviderName: cfg.ProviderName, Model: cfg.Model, Cleared: true}, nil
	}

	status := GetProviderStatus(providerStore)
	if status.Error != "" {
		return ChatModelSelectionResult{}, errors.New(status.Error)
	}
	var match *domain.ProviderInfo
	for i := range status.Providers {
		p := &status.Providers[i]
		if strings.EqualFold(p.ID, provider) || strings.EqualFold(p.Name, provider) {
			match = p
			break
		}
	}
	if match == nil {
		return ChatModelSelectionResult{}, fmt.Errorf("unknown provider: %s", provider)
	}
	if !match.Connected {
		return ChatModelSelectionResult{}, fmt.Errorf("%s is not configured — connect it in Settings › LLM Providers first", match.Name)
	}
	if model != "" && !match.AllowCustomModel {
		allowed := false
		for _, m := range match.Models {
			if strings.EqualFold(m, model) {
				model = m
				allowed = true
				break
			}
		}
		if !allowed {
			available := strings.Join(match.Models, ", ")
			if available == "" {
				return ChatModelSelectionResult{}, fmt.Errorf("%s does not offer model %q", match.Name, model)
			}
			return ChatModelSelectionResult{}, fmt.Errorf("%s does not offer model %q; available: %s", match.Name, model, available)
		}
	}
	if err := overrideStore.SetChatModelOverride(chatID, match.ID, model); err != nil {
		return ChatModelSelectionResult{}, err
	}
	effectiveModel := model
	if effectiveModel == "" {
		effectiveModel = strings.TrimSpace(match.Model)
		if effectiveModel == "" {
			effectiveModel = match.DefaultModel
		}
	}
	return ChatModelSelectionResult{Provider: match.ID, ProviderName: match.Name, Model: effectiveModel}, nil
}

// ResolveProviderFallbackChain builds the ordered list of usable provider
// runtime configs for the fallback chain. The user-defined order IS the
// priority: #1 is the primary (the Agent Workspace default), #2 the first
// fallback, and so on. The active provider is kept in step with #1 by
// SyncActiveProviderToOrder, so what the UI and /model report matches what
// actually runs. Providers that are not configured are silently skipped, and
// any remaining known providers are appended as last resorts so a configured
// provider missing from order is still reachable. Cooldown is NOT applied here
// — that is a live decision made by the failover loop; this returns the full
// static chain.
func ResolveProviderFallbackChain(store ports.ProviderSecretStore, order []string) ([]domain.ProviderRuntimeConfig, error) {
	ids := fallbackCandidateIDs(store, order)
	chain := make([]domain.ProviderRuntimeConfig, 0, len(ids))
	for _, id := range ids {
		cfg, err := ResolveNamedProviderRuntimeConfig(store, id)
		if err != nil {
			// Not configured / unusable provider — skip it from the chain.
			continue
		}
		chain = append(chain, cfg)
	}
	if len(chain) == 0 {
		return nil, errors.New(noProviderConfiguredMessage)
	}
	return chain, nil
}

// fallbackCandidateIDs produces the de-duplicated, ordered list of provider IDs
// to consider for the chain. Priority is: the user-defined order first (#1 is
// the primary), then the active provider, then every other known provider so
// that all configured accounts automatically participate in failover (the
// resolver later drops the ones without credentials).
//
// The tail "append everything" only runs when the user actually has a selection
// (an explicit order or an active provider). That preserves the contract that
// stray leftover credentials alone do not activate a provider — with no order
// and no active provider the chain stays empty and the caller errors out.
func fallbackCandidateIDs(store ports.ProviderSecretStore, order []string) []string {
	seen := make(map[string]bool)
	disabled := disabledProviderIDs(store)
	ids := make([]string, 0, len(providerDefinitions))
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] || disabled[id] {
			return
		}
		if _, ok := providerDefinitionByID(store, id); !ok {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	for _, id := range order {
		add(id)
	}
	active, hasActive, err := store.GetSecret(activeProviderSecret)
	if err == nil && hasActive {
		add(normalizeProviderID(store, active))
	}
	if hasActive || len(ids) > 0 {
		for _, def := range allProviderDefinitions(store) {
			add(def.ID)
		}
	}
	return ids
}

func activateProvider(ctx context.Context, store ports.ProviderSecretStore, validator ports.ProviderCredentialValidator, def domain.ProviderDefinition, model string, apiKey string, baseURL string, skipValidation bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	connected, err := providerConnected(store, def)
	if err != nil {
		return err
	}
	if def.AuthType == "api-key" {
		if apiKey == "" && def.APIKeySecretKey != "" {
			if value, exists, err := store.GetSecret(def.APIKeySecretKey); err != nil {
				return err
			} else if exists {
				apiKey = value
			}
		}
		if baseURL == "" && def.BaseURLSecretKey != "" {
			if value, exists, err := store.GetSecret(def.BaseURLSecretKey); err != nil {
				return err
			} else if exists {
				baseURL = value
			}
		}
		if !skipValidation {
			if validator == nil {
				return fmt.Errorf("provider validator is required for activation")
			}
			if err := validator.ValidateAPIKeyProvider(ctx, def, apiKey, model, baseURL); err != nil {
				return err
			}
		} else if strings.TrimSpace(apiKey) == "" {
			return fmt.Errorf("api key is required for activation")
		}
	} else if !connected {
		return fmt.Errorf("vault credential is required for activation")
	}
	if err := store.SetSecret(activeProviderSecret, def.ID); err != nil {
		return err
	}
	return nil
}

// CreateCustomProvider adds a new dynamic OpenAI-compatible provider with the
// given display name and returns its generated id in the result. The provider
// starts unconfigured (no key / base URL / model yet).
func CreateCustomProvider(store ports.ProviderSecretStore, name string) domain.ProviderOperationResult {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Custom OpenAI-compatible"
	}
	entries, err := loadCustomProviderEntries(store)
	if err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	id, err := newCustomProviderID(store, entries)
	if err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	entries = append(entries, customProviderEntry{ID: id, Name: name})
	if err := saveCustomProviderEntries(store, entries); err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	return domain.ProviderOperationResult{Success: true, Saved: true, ProviderID: id}
}

// RenameCustomProvider updates the display name of an existing custom provider.
func RenameCustomProvider(store ports.ProviderSecretStore, id string, name string) domain.ProviderOperationResult {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.ProviderOperationResult{Success: false, Error: "provider name is required"}
	}
	entries, err := loadCustomProviderEntries(store)
	if err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	found := false
	for i := range entries {
		if entries[i].ID == id {
			entries[i].Name = name
			found = true
			break
		}
	}
	if !found {
		return domain.ProviderOperationResult{Success: false, Error: fmt.Sprintf("unknown custom provider: %s", id)}
	}
	if err := saveCustomProviderEntries(store, entries); err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	return domain.ProviderOperationResult{Success: true, Saved: true, ProviderID: id}
}

// DeleteCustomProvider removes a custom provider entirely: its stored key, base
// URL and model secrets, the registry entry, and the active flag if it was the
// active provider. Removal from the fallback order is handled by the caller
// (it lives in a different store).
func DeleteCustomProvider(store ports.ProviderSecretStore, id string) domain.ProviderOperationResult {
	id = strings.TrimSpace(id)
	entries, err := loadCustomProviderEntries(store)
	if err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	remaining := make([]customProviderEntry, 0, len(entries))
	var removed *customProviderEntry
	for i := range entries {
		if entries[i].ID == id {
			entry := entries[i]
			removed = &entry
			continue
		}
		remaining = append(remaining, entries[i])
	}

	var def domain.ProviderDefinition
	hideBuiltin := false
	switch {
	case removed != nil:
		def = customProviderDefinition(*removed)
	case isHideableBuiltinID(id):
		builtin, ok := builtinDefinitionByID(id)
		if !ok || hiddenBuiltinSet(store)[id] {
			return domain.ProviderOperationResult{Success: false, Error: fmt.Sprintf("unknown provider: %s", id)}
		}
		def = builtin
		hideBuiltin = true
	default:
		return domain.ProviderOperationResult{Success: false, Error: fmt.Sprintf("unknown custom provider: %s", id)}
	}

	for _, key := range []string{def.APIKeySecretKey, def.BaseURLSecretKey, def.ModelSecretKey} {
		if key == "" {
			continue
		}
		if err := store.DeleteSecret(key); err != nil {
			return domain.ProviderOperationResult{Success: false, Error: err.Error()}
		}
	}
	if hideBuiltin {
		if err := saveHiddenBuiltins(store, append(loadHiddenBuiltins(store), id)); err != nil {
			return domain.ProviderOperationResult{Success: false, Error: err.Error()}
		}
	} else if err := saveCustomProviderEntries(store, remaining); err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	if active, exists, err := store.GetSecret(activeProviderSecret); err == nil && exists && active == id {
		_ = store.DeleteSecret(activeProviderSecret)
	} else if err != nil {
		return domain.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	return domain.ProviderOperationResult{Success: true}
}

func providerDefinitionByID(store ports.ProviderSecretStore, id string) (domain.ProviderDefinition, bool) {
	id = strings.TrimSpace(id)
	for _, def := range allProviderDefinitions(store) {
		if def.ID == id {
			return def, true
		}
	}
	return domain.ProviderDefinition{}, false
}

func providerConnected(store ports.ProviderSecretStore, def domain.ProviderDefinition) (bool, error) {
	key := def.APIKeySecretKey
	if key == "" {
		key = def.CredentialSecretKey
	}
	if key == "" {
		return false, nil
	}
	return store.HasSecret(key)
}

func providerModel(store ports.ProviderSecretStore, def domain.ProviderDefinition, connected bool) (string, error) {
	value, exists, err := store.GetSecret(def.ModelSecretKey)
	if err != nil {
		return "", err
	}
	if exists && strings.TrimSpace(value) != "" {
		return value, nil
	}
	if connected {
		return def.DefaultModel, nil
	}
	return "", nil
}

func providerBaseURL(store ports.ProviderSecretStore, def domain.ProviderDefinition) (string, error) {
	if def.BaseURLSecretKey == "" {
		return def.BaseURL, nil
	}
	value, exists, err := store.GetSecret(def.BaseURLSecretKey)
	if err != nil {
		return "", err
	}
	if exists && strings.TrimSpace(value) != "" {
		return normalizeOpenAICompatibleBaseURL(value), nil
	}
	return def.BaseURL, nil
}

// ResolveNamedProviderRuntimeConfig resolves a runtime config for any provider by
// ID (not necessarily the active one). Used by the Test and Balance endpoints.
func ResolveNamedProviderRuntimeConfig(store ports.ProviderSecretStore, providerID string) (domain.ProviderRuntimeConfig, error) {
	def, ok := providerDefinitionByID(store, providerID)
	if !ok {
		return domain.ProviderRuntimeConfig{}, fmt.Errorf("unsupported provider: %s", providerID)
	}
	connected, err := providerConnected(store, def)
	if err != nil {
		return domain.ProviderRuntimeConfig{}, err
	}
	model, err := providerModel(store, def, connected)
	if err != nil {
		return domain.ProviderRuntimeConfig{}, err
	}
	if strings.TrimSpace(model) == "" {
		model = def.DefaultModel
	}
	baseURL, err := providerBaseURL(store, def)
	if err != nil {
		return domain.ProviderRuntimeConfig{}, err
	}
	config := domain.ProviderRuntimeConfig{
		ProviderID:   def.ID,
		ProviderName: def.Name,
		AuthType:     def.AuthType,
		Model:        model,
		BaseURL:      baseURL,
	}
	if def.AuthType == "api-key" {
		if def.APIKeySecretKey == "" {
			return domain.ProviderRuntimeConfig{}, fmt.Errorf("%s has no API key secret configured", def.Name)
		}
		apiKey, exists, err := store.GetSecret(def.APIKeySecretKey)
		if err != nil {
			return domain.ProviderRuntimeConfig{}, err
		}
		if !exists || strings.TrimSpace(apiKey) == "" {
			return domain.ProviderRuntimeConfig{}, fmt.Errorf("%s API key is not configured", def.Name)
		}
		if strings.TrimSpace(config.BaseURL) == "" {
			return domain.ProviderRuntimeConfig{}, fmt.Errorf("%s base URL is not configured", def.Name)
		}
		config.APIKey = apiKey
		return config, nil
	}
	if def.CredentialSecretKey == "" {
		return domain.ProviderRuntimeConfig{}, fmt.Errorf("%s credential storage is not configured", def.Name)
	}
	credential, exists, err := store.GetSecret(def.CredentialSecretKey)
	if err != nil {
		return domain.ProviderRuntimeConfig{}, err
	}
	if !exists || strings.TrimSpace(credential) == "" {
		return domain.ProviderRuntimeConfig{}, fmt.Errorf("%s credential is not configured", def.Name)
	}
	config.Credential = credential
	config.CredentialUpdater = func(next string) error {
		return store.SetSecret(def.CredentialSecretKey, next)
	}
	return config, nil
}
