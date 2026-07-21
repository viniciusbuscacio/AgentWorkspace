package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aw/internal/domain"
)

func TestValidatorValidateAPIKeyProviderSendsChatCompletionsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	validator := NewValidator()
	err := validator.ValidateAPIKeyProvider(context.Background(), domain.ProviderDefinition{
		ID:           "custom-openai",
		Name:         "Custom OpenAI-compatible",
		AuthType:     "api-key",
		DefaultModel: "demo-model",
	}, "secret", "demo-model", server.URL)
	if err != nil {
		t.Fatalf("ValidateAPIKeyProvider() error = %v", err)
	}
}

func TestValidatorValidateAPIKeyProviderReturnsStatusBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer server.Close()

	validator := NewValidator()
	err := validator.ValidateAPIKeyProvider(context.Background(), domain.ProviderDefinition{
		ID:           "custom-openai",
		Name:         "Custom OpenAI-compatible",
		AuthType:     "api-key",
		DefaultModel: "demo-model",
	}, "wrong", "demo-model", server.URL)
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "bad key") {
		t.Fatalf("ValidateAPIKeyProvider() error = %v, want status body", err)
	}
}

func TestTestProviderConnectionUsesNamedProviderConfig(t *testing.T) {
	var capturedAuth string
	var capturedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		capturedModel = body.Model
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"message":{"content":"OK"}}]}`))
	}))
	defer server.Close()

	cfg := domain.ProviderRuntimeConfig{
		ProviderID: "custom-openai",
		AuthType:   "api-key",
		Model:      "test-model",
		APIKey:     "sk-test",
		BaseURL:    server.URL,
	}
	result := TestProviderConnection(context.Background(), cfg, nil)
	if !result.Success {
		t.Fatalf("TestProviderConnection() success = false, error = %q", result.Error)
	}
	if capturedAuth != "Bearer sk-test" {
		t.Fatalf("Authorization header = %q, want %q", capturedAuth, "Bearer sk-test")
	}
	if capturedModel != "test-model" {
		t.Fatalf("model in request = %q, want %q", capturedModel, "test-model")
	}
	if result.Model == "" {
		t.Fatalf("TestProviderConnection() model is empty")
	}
	if result.Latency == "" {
		t.Fatalf("TestProviderConnection() latency is empty")
	}
	// No key material in result
	if strings.Contains(result.Model+result.Latency+result.Error, "sk-test") {
		t.Fatalf("key material found in TestProviderConnection result")
	}
}

func TestTestProviderConnectionReturnsProviderErrorVerbatim(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key"}}`)) //nolint:errcheck
	}))
	defer server.Close()

	cfg := domain.ProviderRuntimeConfig{
		ProviderID: "openrouter",
		AuthType:   "api-key",
		Model:      "some/model",
		APIKey:     "bad-key",
		BaseURL:    server.URL,
	}
	result := TestProviderConnection(context.Background(), cfg, nil)
	if result.Success {
		t.Fatalf("TestProviderConnection() expected failure")
	}
	if !strings.Contains(result.Error, "401") {
		t.Fatalf("error %q does not contain status code", result.Error)
	}
}

func TestFetchOpenRouterBalanceCreditsEndpoint(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path == "/api/v1/credits" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"total_credits":10.0,"total_usage":2.5}}`)) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Patch the URL by using a custom client that redirects openrouter.ai to the test server.
	// Instead, call the internal helpers directly via the exported function with a custom Transport.
	origCreditsURL := openrouterCreditsURL
	origAuthKeyURL := openrouterAuthKeyURL
	openrouterCreditsURL = server.URL + "/api/v1/credits"
	openrouterAuthKeyURL = server.URL + "/api/v1/auth/key"
	defer func() {
		openrouterCreditsURL = origCreditsURL
		openrouterAuthKeyURL = origAuthKeyURL
	}()

	result := FetchOpenRouterBalance(context.Background(), "sk-or-test", nil)
	if !result.Available {
		t.Fatalf("FetchOpenRouterBalance() available = false")
	}
	if gotAuth != "Bearer sk-or-test" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer sk-or-test")
	}
	if result.Used == "" || result.Limit == "" {
		t.Fatalf("FetchOpenRouterBalance() used=%q limit=%q", result.Used, result.Limit)
	}
	// No key material in result
	if strings.Contains(result.Used+result.Limit, "sk-or-test") {
		t.Fatalf("key material found in balance result")
	}
}

func TestFetchOpenRouterBalanceHiddenOnNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	orig1, orig2 := openrouterCreditsURL, openrouterAuthKeyURL
	openrouterCreditsURL = server.URL + "/api/v1/credits"
	openrouterAuthKeyURL = server.URL + "/api/v1/auth/key"
	defer func() {
		openrouterCreditsURL = orig1
		openrouterAuthKeyURL = orig2
	}()

	result := FetchOpenRouterBalance(context.Background(), "bad-key", nil)
	if result.Available {
		t.Fatalf("FetchOpenRouterBalance() available = true on non-200, want false")
	}
}
