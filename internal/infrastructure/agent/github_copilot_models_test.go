package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aw/internal/domain"
)

func decodeCatalogEntries(t *testing.T, raw string) []copilotCatalogEntry {
	t.Helper()
	var payload struct {
		Data []copilotCatalogEntry `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode catalog fixture: %v", err)
	}
	return payload.Data
}

func TestCopilotModelIDsFromCatalog(t *testing.T) {
	entries := decodeCatalogEntries(t, `{"data":[
		{"id":"gpt-5.4","model_picker_enabled":true,"capabilities":{"type":"chat"}},
		{"id":"claude-fable-5","model_picker_enabled":true,"capabilities":{"type":"chat"},"supported_endpoints":["/chat/completions","/responses"]},
		{"id":"gpt-5.4","model_picker_enabled":true,"capabilities":{"type":"chat"}},
		{"id":"gpt-5.5","model_picker_enabled":true,"capabilities":{"type":"chat"},"supported_endpoints":["/responses"]},
		{"id":"text-embedding-3-small","model_picker_enabled":false,"capabilities":{"type":"embeddings"}},
		{"id":"gpt-4o-2024-05-13","model_picker_enabled":false,"capabilities":{"type":"chat"}}
	]}`)
	ids, err := copilotModelIDsFromCatalog(entries)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	// Picker-enabled subset wins, deduped, embeddings excluded. gpt-5.5 is
	// /responses-only and stays listed: the runtime routes it to /responses.
	want := []string{"gpt-5.4", "claude-fable-5", "gpt-5.5"}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
}

func TestCopilotModelIDsFromCatalogFallsBackToAllChatModels(t *testing.T) {
	entries := decodeCatalogEntries(t, `{"data":[
		{"id":"a","capabilities":{"type":"chat"}},
		{"id":"b","capabilities":{"type":"chat"}}
	]}`)
	ids, err := copilotModelIDsFromCatalog(entries)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("ids = %v", ids)
	}
	if _, err := copilotModelIDsFromCatalog(nil); err == nil {
		t.Errorf("expected error on empty catalog")
	}
}

func TestSupportsChatCompletions(t *testing.T) {
	if !supportsChatCompletions(nil) {
		t.Errorf("absent endpoint list must mean chat/completions")
	}
	if !supportsChatCompletions([]string{"/chat/completions", "/responses"}) {
		t.Errorf("list with chat/completions must match")
	}
	if supportsChatCompletions([]string{"/responses"}) {
		t.Errorf("responses-only list must not match")
	}
}

func TestModelCatalogListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			if got := r.Header.Get("Authorization"); got != "Bearer copilot-tok" {
				t.Errorf("models auth header = %q", got)
			}
			if got := r.Header.Get("Copilot-Integration-Id"); got != gitHubCopilotIntegrationID {
				t.Errorf("integration id header = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "live-model", "model_picker_enabled": true, "capabilities": map[string]any{"type": "chat"}},
				},
			})
			return
		}
		// Token exchange endpoint (no proxy-ep -> base URL falls back to cfg.BaseURL).
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "copilot-tok",
			"expires_at": time.Now().Add(30 * time.Minute).Unix(),
		})
	}))
	defer srv.Close()

	catalog := &ModelCatalog{Client: srv.Client(), tokenURL: srv.URL}
	ids, err := catalog.ListModels(context.Background(), domain.ProviderRuntimeConfig{
		ProviderID: "github-copilot",
		AuthType:   "oauth-device-code",
		BaseURL:    srv.URL,
		Credential: `{"auth_mode":"github-copilot","github_token":"gho_x"}`,
	})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(ids) != 1 || ids[0] != "live-model" {
		t.Errorf("ids = %v", ids)
	}
}

func TestModelCatalogRejectsUnsupportedAuthType(t *testing.T) {
	catalog := NewModelCatalog()
	if _, err := catalog.ListModels(context.Background(), domain.ProviderRuntimeConfig{
		ProviderID: "openai-codex",
		AuthType:   "oauth-browser",
	}); err == nil {
		t.Errorf("expected error for oauth-browser provider")
	}
}

func TestModelCatalogListsOpenAICompatibleModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("auth header = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "gpt-5.5"},
				{"id": "gpt-5.5"},
				{"id": "whisper-1"},
				{"id": "text-embedding-3-small"},
				{"id": "anthropic/claude-sonnet-4.5"},
				{"id": "openai/tts-1"},
			},
		})
	}))
	defer srv.Close()

	catalog := &ModelCatalog{Client: srv.Client()}
	ids, err := catalog.ListModels(context.Background(), domain.ProviderRuntimeConfig{
		ProviderID: "openrouter",
		AuthType:   "api-key",
		BaseURL:    srv.URL,
		APIKey:     "sk-test",
	})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	// Deduped; audio/embedding models dropped (also behind aggregator prefixes).
	want := []string{"gpt-5.5", "anthropic/claude-sonnet-4.5"}
	if len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}

func TestModelCatalogAPIKeyRequiresBaseURL(t *testing.T) {
	catalog := NewModelCatalog()
	if _, err := catalog.ListModels(context.Background(), domain.ProviderRuntimeConfig{
		ProviderID: "openai",
		AuthType:   "api-key",
	}); err == nil {
		t.Errorf("expected error when base URL is missing")
	}
}

func TestCopilotTokenManagerModelUsesResponsesEndpoint(t *testing.T) {
	var catalogFetches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			catalogFetches++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "gpt-5.5", "supported_endpoints": []string{"/responses"}},
					{"id": "claude-fable-5", "supported_endpoints": []string{"/chat/completions", "/responses"}},
					{"id": "gpt-4.1"},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "copilot-tok",
			"expires_at": time.Now().Add(30 * time.Minute).Unix(),
		})
	}))
	defer srv.Close()

	mgr := &copilotTokenManager{
		credential:      gitHubCopilotCredential{GitHubToken: "gho_x"},
		client:          srv.Client(),
		tokenURL:        srv.URL,
		fallbackBaseURL: srv.URL,
	}
	ctx := context.Background()
	cases := map[string]bool{
		"gpt-5.5":        true,  // /responses-only
		"claude-fable-5": false, // both -> keep chat/completions
		"gpt-4.1":        false, // no endpoint list -> chat/completions
		"unknown-model":  false, // not in catalog -> chat/completions
	}
	for modelID, want := range cases {
		got, err := mgr.ModelUsesResponsesEndpoint(ctx, modelID)
		if err != nil {
			t.Fatalf("ModelUsesResponsesEndpoint(%s): %v", modelID, err)
		}
		if got != want {
			t.Errorf("ModelUsesResponsesEndpoint(%s) = %v, want %v", modelID, got, want)
		}
	}
	if catalogFetches != 1 {
		t.Errorf("catalog fetched %d times, want 1 (cached)", catalogFetches)
	}
}
