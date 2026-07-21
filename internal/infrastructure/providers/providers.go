package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aw/internal/domain"
)

type Validator struct {
	Client *http.Client
}

func NewValidator() Validator {
	return Validator{Client: &http.Client{Timeout: 15 * time.Second}}
}

func (v Validator) ValidateAPIKeyProvider(ctx context.Context, def domain.ProviderDefinition, apiKey string, model string, baseURL string) error {
	if def.AuthType != "api-key" {
		return nil
	}
	baseURL = strings.TrimSpace(firstNonEmpty(baseURL, def.BaseURL))
	if baseURL == "" {
		return fmt.Errorf("%s base URL is required for validation", def.Name)
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("api key is required for validation")
	}
	if model = strings.TrimSpace(model); model == "" {
		model = def.DefaultModel
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
	body, err := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "Reply with exactly: ok"}},
		"max_tokens": 5,
		"stream":     false,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if def.ID == "openrouter" {
		req.Header.Set("HTTP-Referer", "https://agent-workspace.local")
		req.Header.Set("X-Title", "Agent Workspace")
	}

	client := v.Client
	if client == nil {
		client = NewValidator().Client
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 240))
	text := strings.TrimSpace(string(raw))
	if text == "" {
		text = response.Status
	}
	return fmt.Errorf("%d: %s", response.StatusCode, text)
}

// Probe is the ports.ProviderProbe adapter: it issues the outbound HTTP probes
// for provider connection tests and balance lookups. The zero value is ready
// to use (each call builds its own short-timeout client); the optional client
// fields exist so tests can inject an httptest transport.
type Probe struct {
	TestClient    *http.Client
	BalanceClient *http.Client
}

// NewProbe returns a Probe with default clients (built per call).
func NewProbe() Probe { return Probe{} }

// TestConnection implements ports.ProviderProbe.
func (p Probe) TestConnection(ctx context.Context, cfg domain.ProviderRuntimeConfig) domain.ProviderTestResult {
	return TestProviderConnection(ctx, cfg, p.TestClient)
}

// FetchBalance implements ports.ProviderProbe. Only OpenRouter exposes a
// balance endpoint today; every other provider reports unavailable.
func (p Probe) FetchBalance(ctx context.Context, cfg domain.ProviderRuntimeConfig) domain.ProviderBalanceResult {
	if cfg.ProviderID != "openrouter" {
		return domain.ProviderBalanceResult{Available: false}
	}
	return FetchOpenRouterBalance(ctx, cfg.APIKey, p.BalanceClient)
}

// TestProviderConnection sends a minimal one-shot completion through the given
// runtime config (15s timeout already baked into the context). Returns latency
// and model echoed, or the provider's error verbatim. No key material is
// returned.
func TestProviderConnection(ctx context.Context, cfg domain.ProviderRuntimeConfig, httpClient *http.Client) domain.ProviderTestResult {
	if cfg.AuthType != "api-key" {
		return domain.ProviderTestResult{Success: false, Error: "test is only supported for API-key providers"}
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return domain.ProviderTestResult{Success: false, Error: "base URL is not configured"}
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return domain.ProviderTestResult{Success: false, Error: "API key is not configured"}
	}

	endpoint := baseURL + "/chat/completions"
	body, err := json.Marshal(map[string]any{
		"model":      cfg.Model,
		"messages":   []map[string]string{{"role": "user", "content": "Reply with OK"}},
		"max_tokens": 5,
		"stream":     false,
	})
	if err != nil {
		return domain.ProviderTestResult{Success: false, Error: err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return domain.ProviderTestResult{Success: false, Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if cfg.ProviderID == "openrouter" {
		req.Header.Set("HTTP-Referer", "https://agent-workspace.local")
		req.Header.Set("X-Title", "Agent Workspace")
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return domain.ProviderTestResult{Success: false, Error: err.Error()}
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		text := strings.TrimSpace(string(raw))
		if text == "" {
			text = resp.Status
		}
		return domain.ProviderTestResult{Success: false, Error: fmt.Sprintf("%d: %s", resp.StatusCode, text)}
	}

	// Extract the model field from the response if present; fall back to the
	// configured model. Key material is never placed in the result.
	var payload struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(raw, &payload)
	model := payload.Model
	if model == "" {
		model = cfg.Model
	}

	return domain.ProviderTestResult{
		Success: true,
		Latency: latencyString(latency),
		Model:   model,
	}
}

// openrouterCreditsURL and openrouterAuthKeyURL are package-level so tests can
// override them with a local httptest server.
var (
	openrouterCreditsURL = "https://openrouter.ai/api/v1/credits"
	openrouterAuthKeyURL = "https://openrouter.ai/api/v1/auth/key"
)

// FetchOpenRouterBalance fetches credit/usage from OpenRouter using the
// inference key already stored for the provider. Tries /api/v1/credits first,
// then falls back to /api/v1/auth/key. Returns Available=false on any error or
// non-200 response so the UI can hide the row cleanly.
func FetchOpenRouterBalance(ctx context.Context, apiKey string, httpClient *http.Client) domain.ProviderBalanceResult {
	if strings.TrimSpace(apiKey) == "" {
		return domain.ProviderBalanceResult{Available: false}
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	// Try /api/v1/credits first (preferred endpoint).
	if result, ok := fetchOpenRouterCredits(ctx, apiKey, httpClient); ok {
		return result
	}
	// Fall back to /api/v1/auth/key.
	return fetchOpenRouterAuthKey(ctx, apiKey, httpClient)
}

func fetchOpenRouterCredits(ctx context.Context, apiKey string, client *http.Client) (domain.ProviderBalanceResult, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openrouterCreditsURL, nil)
	if err != nil {
		return domain.ProviderBalanceResult{}, false
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return domain.ProviderBalanceResult{}, false
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	var payload struct {
		Data struct {
			TotalCredits float64 `json:"total_credits"`
			TotalUsage   float64 `json:"total_usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.ProviderBalanceResult{}, false
	}
	limit := payload.Data.TotalCredits
	used := payload.Data.TotalUsage
	return domain.ProviderBalanceResult{
		Available: true,
		Used:      fmt.Sprintf("$%.4f", used),
		Limit:     fmt.Sprintf("$%.2f", limit),
	}, true
}

func fetchOpenRouterAuthKey(ctx context.Context, apiKey string, client *http.Client) domain.ProviderBalanceResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openrouterAuthKeyURL, nil)
	if err != nil {
		return domain.ProviderBalanceResult{Available: false}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return domain.ProviderBalanceResult{Available: false}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	// /api/v1/auth/key returns { data: { usage: float, limit: float|null } }
	var payload struct {
		Data struct {
			Usage float64  `json:"usage"`
			Limit *float64 `json:"limit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.ProviderBalanceResult{Available: false}
	}
	used := payload.Data.Usage
	result := domain.ProviderBalanceResult{
		Available: true,
		Used:      fmt.Sprintf("$%.4f", used),
	}
	if payload.Data.Limit != nil {
		result.Limit = fmt.Sprintf("$%.2f", *payload.Data.Limit)
	}
	return result
}

func latencyString(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
