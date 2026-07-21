package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"aw/internal/domain"
)

// GitHub Copilot exposes an OpenAI-compatible chat completions API, but auth is
// a two-step dance: a long-lived GitHub OAuth token (gho_...) is exchanged at
// copilot_internal/v2/token for a short-lived (~30 min) Copilot bearer token.
// The Copilot token also encodes the per-account proxy endpoint (proxy-ep=...),
// so the real API base URL is only known after that exchange. Finally, some
// models (Claude, Gemini, Grok) must be explicitly enabled on the account via a
// /models/{id}/policy call before they can be used. This adapter wraps the
// shared OpenAICompatibleModel with a token manager that performs the exchange,
// derives the base URL, enables models, and refreshes the token on demand.

const (
	defaultGitHubCopilotBaseURL = "https://api.individual.githubcopilot.com"
	gitHubCopilotTokenURL       = "https://api.github.com/copilot_internal/v2/token"
	gitHubCopilotEditorVersion  = "vscode/1.107.0"
	gitHubCopilotPluginVersion  = "copilot-chat/0.35.0"
	gitHubCopilotIntegrationID  = "vscode-chat"
	gitHubCopilotUserAgent      = "GitHubCopilotChat/0.35.0"
)

var copilotProxyEndpointPattern = regexp.MustCompile(`proxy-ep=([^;]+)`)

// GitHubCopilotModelIDs is the canonical set of model ids served through GitHub
// Copilot (mirrors pi-ai's generated catalog). Used both as the UI model list
// source of truth and as the set enabled via policy on first sign-in.
var GitHubCopilotModelIDs = []string{
	"claude-sonnet-4.5",
	"claude-sonnet-4.6",
	"claude-sonnet-5",
	"claude-haiku-4.5",
	"claude-opus-4.5",
	"claude-opus-4.6",
	"claude-opus-4.7",
	"claude-opus-4.8",
	"claude-opus-4.8-fast",
	"claude-fable-5",
	"gpt-4.1",
	"gpt-5-mini",
	"gpt-5.2",
	"gpt-5.3-codex",
	"gpt-5.4",
	"gpt-5.4-mini",
	"gpt-5.5",
	"gemini-2.5-pro",
	"gemini-3-flash-preview",
	"gemini-3.1-pro-preview",
	"gemini-3.5-flash",
	"kimi-k2.7-code",
}

type GitHubCopilotConfig struct {
	ProviderID        string
	Model             string
	Credential        string
	BaseURL           string
	HTTPClient        *http.Client
	CredentialUpdater func(string) error
	// EnableModelIDs are enabled on the account via the policy endpoint on the
	// first token exchange (best-effort). Claude/Gemini/Grok require this.
	EnableModelIDs []string
}

// gitHubCopilotCredential is the stored envelope. GitHubToken is the long-lived
// OAuth token; the Copilot token is cached opportunistically so a fresh launch
// can reuse a still-valid token instead of exchanging again immediately.
type gitHubCopilotCredential struct {
	AuthMode         string `json:"auth_mode"`
	GitHubToken      string `json:"github_token"`
	CopilotToken     string `json:"copilot_token,omitempty"`
	CopilotExpiresAt int64  `json:"copilot_expires_at,omitempty"`
	LastRefresh      string `json:"last_refresh,omitempty"`
}

// NewGitHubCopilotModel builds the Copilot adapter. It returns the shared
// OpenAICompatibleModel configured with a dynamic bearer and a dynamic base URL
// (both supplied by the token manager) plus the editor headers Copilot requires.
func NewGitHubCopilotModel(cfg GitHubCopilotConfig) (*OpenAICompatibleModel, error) {
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("model is required")
	}
	credential, err := parseGitHubCopilotCredential(cfg.Credential)
	if err != nil {
		return nil, err
	}
	client := cfg.HTTPClient
	if client == nil {
		client = defaultOpenAIHTTPClient()
	}
	fallbackBaseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if fallbackBaseURL == "" {
		fallbackBaseURL = defaultGitHubCopilotBaseURL
	}
	enableIDs := cfg.EnableModelIDs
	if len(enableIDs) == 0 {
		enableIDs = GitHubCopilotModelIDs
	}
	manager := &copilotTokenManager{
		credential:        credential,
		client:            client,
		tokenURL:          gitHubCopilotTokenURL,
		credentialUpdater: cfg.CredentialUpdater,
		fallbackBaseURL:   fallbackBaseURL,
		enableModelIDs:    enableIDs,
	}
	return NewOpenAICompatibleModel(OpenAICompatibleConfig{
		ProviderID:           strings.TrimSpace(cfg.ProviderID),
		Model:                strings.TrimSpace(cfg.Model),
		BaseURL:              fallbackBaseURL,
		HTTPClient:           client,
		AuthToken:            manager.Token,
		BaseURLProvider:      manager.BaseURL,
		UseResponsesEndpoint: manager.ModelUsesResponsesEndpoint,
		ExtraHeaders: map[string]string{
			"Editor-Version":         gitHubCopilotEditorVersion,
			"Editor-Plugin-Version":  gitHubCopilotPluginVersion,
			"Copilot-Integration-Id": gitHubCopilotIntegrationID,
			"User-Agent":             gitHubCopilotUserAgent,
		},
	})
}

func parseGitHubCopilotCredential(raw string) (gitHubCopilotCredential, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return gitHubCopilotCredential{}, fmt.Errorf("GitHub Copilot credential is required")
	}
	var credential gitHubCopilotCredential
	if err := json.Unmarshal([]byte(raw), &credential); err != nil {
		return gitHubCopilotCredential{}, fmt.Errorf("decode GitHub Copilot credential: %w", err)
	}
	if strings.TrimSpace(credential.GitHubToken) == "" {
		return gitHubCopilotCredential{}, fmt.Errorf("GitHub Copilot credential does not include a GitHub token")
	}
	return credential, nil
}

// copilotBaseURLFromToken extracts proxy-ep from the Copilot token and converts
// proxy.<host> into the API base URL https://api.<host>.
func copilotBaseURLFromToken(token string) string {
	match := copilotProxyEndpointPattern.FindStringSubmatch(token)
	if len(match) != 2 {
		return ""
	}
	host := strings.TrimSpace(match[1])
	if host == "" {
		return ""
	}
	host = strings.TrimPrefix(host, "proxy.")
	return "https://api." + host
}

// copilotTokenManager caches the short-lived Copilot bearer and refreshes it
// from the GitHub token when it is missing or about to expire. It also tracks
// the derived API base URL and enables models once after the first exchange.
type copilotTokenManager struct {
	mu                sync.Mutex
	credential        gitHubCopilotCredential
	client            *http.Client
	tokenURL          string
	credentialUpdater func(string) error
	fallbackBaseURL   string
	baseURL           string
	enableModelIDs    []string
	modelsEnabled     bool
	// modelEndpoints caches each catalog model's supported_endpoints for the
	// process lifetime, so per-request endpoint routing costs one /models
	// fetch total. nil until the first successful fetch.
	modelEndpoints map[string][]string
}

// Token returns a valid Copilot bearer, exchanging the GitHub token when the
// cached one is within 60s of expiry (or absent).
func (m *copilotTokenManager) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().Unix()
	if strings.TrimSpace(m.credential.CopilotToken) != "" && m.credential.CopilotExpiresAt-60 > now {
		if m.baseURL == "" {
			m.baseURL = copilotBaseURLFromToken(m.credential.CopilotToken)
		}
		return m.credential.CopilotToken, nil
	}
	return m.refreshLocked(ctx)
}

// BaseURL ensures a token exists (so proxy-ep is known) and returns the derived
// API base URL, falling back to the configured default.
func (m *copilotTokenManager) BaseURL(ctx context.Context) (string, error) {
	if _, err := m.Token(ctx); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.baseURL != "" {
		return m.baseURL, nil
	}
	return m.fallbackBaseURL, nil
}

// ModelUsesResponsesEndpoint reports whether the catalog serves modelID only
// through POST /responses (e.g. the newest OpenAI models). Unknown models,
// models predating supported_endpoints and any catalog-fetch failure all
// answer false, keeping the proven /chat/completions path.
func (m *copilotTokenManager) ModelUsesResponsesEndpoint(ctx context.Context, modelID string) (bool, error) {
	token, err := m.Token(ctx)
	if err != nil {
		return false, nil //nolint:nilerr // degrade to chat/completions rather than failing the turn
	}
	base, err := m.BaseURL(ctx)
	if err != nil {
		return false, nil //nolint:nilerr // degrade to chat/completions rather than failing the turn
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.modelEndpoints == nil {
		entries, err := fetchCopilotCatalog(ctx, m.client, base, token)
		if err != nil {
			return false, nil //nolint:nilerr // degrade to chat/completions rather than failing the turn
		}
		endpoints := make(map[string][]string, len(entries))
		for _, entry := range entries {
			if id := strings.TrimSpace(entry.ID); id != "" {
				endpoints[id] = entry.SupportedEndpoints
			}
		}
		m.modelEndpoints = endpoints
	}
	supported, known := m.modelEndpoints[strings.TrimSpace(modelID)]
	if !known {
		return false, nil
	}
	return !supportsChatCompletions(supported) && endpointListContains(supported, "responses"), nil
}

func (m *copilotTokenManager) refreshLocked(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.tokenURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+m.credential.GitHubToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Editor-Version", gitHubCopilotEditorVersion)
	req.Header.Set("Editor-Plugin-Version", gitHubCopilotPluginVersion)
	req.Header.Set("User-Agent", gitHubCopilotUserAgent)

	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GitHub Copilot token exchange transport failure: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", domain.NewProviderHTTPError(resp.StatusCode, resp.Status, "GitHub Copilot authorization failed; sign in again or check your Copilot subscription")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", domain.NewProviderHTTPError(resp.StatusCode, resp.Status, fmt.Sprintf("GitHub Copilot token exchange: %s", strings.TrimSpace(string(raw))))
	}
	var token struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &token); err != nil {
		return "", fmt.Errorf("GitHub Copilot token exchange returned invalid JSON")
	}
	if strings.TrimSpace(token.Token) == "" {
		return "", fmt.Errorf("GitHub Copilot token exchange did not include a token")
	}
	m.credential.CopilotToken = token.Token
	m.credential.CopilotExpiresAt = token.ExpiresAt
	m.credential.LastRefresh = time.Now().UTC().Format(time.RFC3339)
	if derived := copilotBaseURLFromToken(token.Token); derived != "" {
		m.baseURL = derived
	}
	if m.credentialUpdater != nil {
		if encoded, err := json.Marshal(m.credential); err == nil {
			_ = m.credentialUpdater(string(encoded))
		}
	}
	// Enable models on the account once per process (best-effort): Claude,
	// Gemini and Grok require explicit policy acceptance before first use.
	if !m.modelsEnabled && len(m.enableModelIDs) > 0 {
		m.modelsEnabled = true
		base := m.baseURL
		if base == "" {
			base = m.fallbackBaseURL
		}
		go enableCopilotModels(m.client, base, token.Token, m.enableModelIDs)
	}
	return token.Token, nil
}

// enableCopilotModels POSTs an "enabled" policy for each model id. Failures are
// ignored: a model may already be enabled or not require a policy at all.
func enableCopilotModels(client *http.Client, baseURL, token string, modelIDs []string) {
	for _, id := range modelIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		body := bytes.NewReader([]byte(`{"state":"enabled"}`))
		req, err := http.NewRequest(http.MethodPost, baseURL+"/models/"+id+"/policy", body)
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Editor-Version", gitHubCopilotEditorVersion)
		req.Header.Set("Editor-Plugin-Version", gitHubCopilotPluginVersion)
		req.Header.Set("Copilot-Integration-Id", gitHubCopilotIntegrationID)
		req.Header.Set("User-Agent", gitHubCopilotUserAgent)
		req.Header.Set("openai-intent", "chat-policy")
		req.Header.Set("x-interaction-type", "chat-policy")
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}
}
