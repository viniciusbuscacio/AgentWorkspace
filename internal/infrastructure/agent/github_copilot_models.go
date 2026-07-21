package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"aw/internal/domain"
)

// copilotCatalogEntry is one model in Copilot's GET /models catalog. The
// fields drive both the Settings picker (id / picker flag / type) and the
// per-model endpoint routing (supported_endpoints).
type copilotCatalogEntry struct {
	ID                 string   `json:"id"`
	ModelPicker        bool     `json:"model_picker_enabled"`
	SupportedEndpoints []string `json:"supported_endpoints"`
	Capabilities       struct {
		Type string `json:"type"`
	} `json:"capabilities"`
}

// fetchCopilotCatalog fetches GET {base}/models with the editor headers the
// endpoint requires. Shared by the Settings model list and the runtime's
// endpoint routing.
func fetchCopilotCatalog(ctx context.Context, client *http.Client, baseURL string, token string) ([]copilotCatalogEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Editor-Version", gitHubCopilotEditorVersion)
	req.Header.Set("Editor-Plugin-Version", gitHubCopilotPluginVersion)
	req.Header.Set("Copilot-Integration-Id", gitHubCopilotIntegrationID)
	req.Header.Set("User-Agent", gitHubCopilotUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub Copilot models request transport failure: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, domain.NewProviderHTTPError(resp.StatusCode, resp.Status, fmt.Sprintf("GitHub Copilot models request: %s", strings.TrimSpace(string(raw))))
	}
	var payload struct {
		Data []copilotCatalogEntry `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("GitHub Copilot models response is not valid JSON")
	}
	return payload.Data, nil
}

// listGitHubCopilotModels lists the model ids for the Settings picker through
// the Copilot token exchange (the base URL is only known after it).
func (c *ModelCatalog) listGitHubCopilotModels(ctx context.Context, cfg domain.ProviderRuntimeConfig) ([]string, error) {
	credential, err := parseGitHubCopilotCredential(cfg.Credential)
	if err != nil {
		return nil, err
	}
	client := c.httpClient()
	tokenURL := c.tokenURL
	if tokenURL == "" {
		tokenURL = gitHubCopilotTokenURL
	}
	fallbackBaseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if fallbackBaseURL == "" {
		fallbackBaseURL = defaultGitHubCopilotBaseURL
	}
	manager := &copilotTokenManager{
		credential:        credential,
		client:            client,
		tokenURL:          tokenURL,
		credentialUpdater: cfg.CredentialUpdater,
		fallbackBaseURL:   fallbackBaseURL,
	}
	token, err := manager.Token(ctx)
	if err != nil {
		return nil, err
	}
	baseURL, err := manager.BaseURL(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := fetchCopilotCatalog(ctx, client, baseURL, token)
	if err != nil {
		return nil, err
	}
	return copilotModelIDsFromCatalog(entries)
}

// copilotModelIDsFromCatalog extracts chat-capable model ids, preferring the
// picker-enabled subset (what VS Code shows in its own model picker) when the
// catalog marks one. /responses-only models are included: the runtime routes
// each model to the endpoint the catalog advertises. Order follows the API
// response.
func copilotModelIDsFromCatalog(entries []copilotCatalogEntry) ([]string, error) {
	var all, picker []string
	seen := map[string]bool{}
	for _, model := range entries {
		id := strings.TrimSpace(model.ID)
		if id == "" || seen[id] {
			continue
		}
		if model.Capabilities.Type != "" && model.Capabilities.Type != "chat" {
			continue
		}
		seen[id] = true
		all = append(all, id)
		if model.ModelPicker {
			picker = append(picker, id)
		}
	}
	if len(picker) > 0 {
		return picker, nil
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("GitHub Copilot models response contained no chat models")
	}
	return all, nil
}

// supportsChatCompletions reports whether a catalog entry is served through
// the classic chat/completions endpoint. An absent list means the catalog
// predates the field, so the model is assumed to be chat/completions-served.
func supportsChatCompletions(endpoints []string) bool {
	if len(endpoints) == 0 {
		return true
	}
	return endpointListContains(endpoints, "chat/completions")
}

func endpointListContains(endpoints []string, needle string) bool {
	for _, endpoint := range endpoints {
		if strings.Contains(strings.ToLower(endpoint), needle) {
			return true
		}
	}
	return false
}
