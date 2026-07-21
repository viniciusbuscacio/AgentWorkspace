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

// ModelCatalog implements ports.ProviderModelCatalog. It fetches the model ids
// a configured provider can actually serve from the provider's live models
// endpoint: the OpenAI-compatible GET /models for api-key providers (OpenAI,
// OpenRouter, custom endpoints) and the token-exchanged variant for GitHub
// Copilot. Provider catalogs change server-side (models are retired and added
// between releases), so the static list in the provider definition is only a
// fallback.
type ModelCatalog struct {
	Client *http.Client
	// tokenURL overrides the Copilot token-exchange endpoint in tests.
	tokenURL string
}

func NewModelCatalog() *ModelCatalog {
	return &ModelCatalog{}
}

func (c *ModelCatalog) ListModels(ctx context.Context, cfg domain.ProviderRuntimeConfig) ([]string, error) {
	switch cfg.AuthType {
	case "oauth-device-code":
		return c.listGitHubCopilotModels(ctx, cfg)
	case "api-key":
		return c.listOpenAICompatibleModels(ctx, cfg)
	default:
		return nil, fmt.Errorf("live model listing is not supported for %s", cfg.ProviderID)
	}
}

func (c *ModelCatalog) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return defaultOpenAIHTTPClient()
}

// listOpenAICompatibleModels fetches GET {base}/models with the provider API
// key — the standard OpenAI-compatible catalog endpoint.
func (c *ModelCatalog) listOpenAICompatibleModels(ctx context.Context, cfg domain.ProviderRuntimeConfig) ([]string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("%s has no base URL to list models from", cfg.ProviderID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if key := strings.TrimSpace(cfg.APIKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s models request transport failure: %w", cfg.ProviderID, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, domain.NewProviderHTTPError(resp.StatusCode, resp.Status, fmt.Sprintf("%s models request: %s", cfg.ProviderID, strings.TrimSpace(string(raw))))
	}
	return parseOpenAICompatibleModelIDs(cfg.ProviderID, raw)
}

// nonChatModelPrefixes filters ids that OpenAI-style /models endpoints list but
// that can never serve the chat completions path aw uses (audio, image,
// embedding and moderation models). Unknown ids are kept: wrongly hiding a
// usable model is worse than showing an exotic one.
var nonChatModelPrefixes = []string{
	"whisper", "tts", "dall-e", "text-embedding", "text-moderation",
	"omni-moderation", "davinci", "babbage", "sora",
}

func parseOpenAICompatibleModelIDs(providerID string, raw []byte) ([]string, error) {
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%s models response is not valid JSON", providerID)
	}
	var ids []string
	seen := map[string]bool{}
	for _, model := range payload.Data {
		id := strings.TrimSpace(model.ID)
		if id == "" || seen[id] || isNonChatModelID(id) {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%s models response contained no chat models", providerID)
	}
	return ids, nil
}

func isNonChatModelID(id string) bool {
	lower := strings.ToLower(id)
	// Namespaced ids (e.g. openai/whisper-1 on aggregators) filter on the bare name.
	if slash := strings.LastIndex(lower, "/"); slash >= 0 {
		lower = lower[slash+1:]
	}
	for _, prefix := range nonChatModelPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}
