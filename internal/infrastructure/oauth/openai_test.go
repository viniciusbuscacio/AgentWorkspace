package oauth

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBuildCredentialWrapsTokenResponse(t *testing.T) {
	raw := []byte(`{"access_token":"acc","refresh_token":"ref","id_token":"idt"}`)
	credential, err := buildCredential(raw)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		AuthMode    string         `json:"auth_mode"`
		Tokens      map[string]any `json:"tokens"`
		LastRefresh string         `json:"last_refresh"`
	}
	if err := json.Unmarshal([]byte(credential), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.AuthMode != "chatgpt" {
		t.Fatalf("expected auth_mode chatgpt, got %q", payload.AuthMode)
	}
	if payload.Tokens["access_token"] != "acc" || payload.Tokens["refresh_token"] != "ref" {
		t.Fatalf("tokens not preserved: %v", payload.Tokens)
	}
	if _, err := time.Parse(time.RFC3339, payload.LastRefresh); err != nil {
		t.Fatalf("last_refresh is not RFC3339: %q", payload.LastRefresh)
	}
}

func TestBuildCredentialRejectsIncompleteToken(t *testing.T) {
	if _, err := buildCredential([]byte(`not json`)); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if _, err := buildCredential([]byte(`{"access_token":"acc"}`)); err == nil {
		t.Fatal("expected error when refresh_token is missing")
	}
}

func TestBuildAuthorizeURLCarriesPKCEParams(t *testing.T) {
	a := NewOpenAIAuthenticator(nil)
	raw := a.buildAuthorizeURL("http://localhost:1455/auth/callback", "state123", "challenge456")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	// The endpoint and Codex-specific params must match the Codex Hydra
	// allow-list, or OpenAI rejects the request with authorize_hydra_invalid_request.
	if endpoint := parsed.Scheme + "://" + parsed.Host + parsed.Path; endpoint != openAIAuthorizeURL {
		t.Fatalf("authorize endpoint = %q, want %q", endpoint, openAIAuthorizeURL)
	}
	q := parsed.Query()
	checks := map[string]string{
		"response_type":             "code",
		"client_id":                 openAIClientID,
		"redirect_uri":              "http://localhost:1455/auth/callback",
		"code_challenge":            "challenge456",
		"code_challenge_method":     "S256",
		"state":                     "state123",
		"originator":                "codex_cli_rs",
		"codex_cli_simplified_flow": "true",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Errorf("authorize URL %s = %q, want %q", key, got, want)
		}
	}
}

func TestExchangeReturnsCredential(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("code") != "the-code" || r.FormValue("code_verifier") != "the-verifier" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `{"access_token":"acc","refresh_token":"ref"}`)
	}))
	defer tokenServer.Close()

	a := NewOpenAIAuthenticator(nil)
	a.tokenURL = tokenServer.URL
	credential, err := a.exchange(context.Background(), "the-code", "the-verifier", "http://localhost:1455/auth/callback")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(credential, `"auth_mode":"chatgpt"`) {
		t.Fatalf("unexpected credential: %s", credential)
	}
}

func TestExchangePropagatesServerError(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid_grant", http.StatusBadRequest)
	}))
	defer tokenServer.Close()

	a := NewOpenAIAuthenticator(nil)
	a.tokenURL = tokenServer.URL
	if _, err := a.exchange(context.Background(), "code", "verifier", "uri"); err == nil {
		t.Fatal("expected an error for a non-2xx token response")
	}
}

// TestAuthenticateFullRoundTrip drives the whole browser flow: openBrowser
// stands in for the user's browser by calling back with the code, and a stub
// token server completes the exchange.
func TestAuthenticateFullRoundTrip(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"access_token":"acc","refresh_token":"ref"}`)
	}))
	defer tokenServer.Close()

	port := freePort(t)
	a := NewOpenAIAuthenticator(func(authURL string) {
		parsed, err := url.Parse(authURL)
		if err != nil {
			t.Error(err)
			return
		}
		state := parsed.Query().Get("state")
		redirect := parsed.Query().Get("redirect_uri")
		// Simulate the provider redirecting the browser back with the code.
		go func() {
			resp, err := http.Get(redirect + "?state=" + state + "&code=the-code")
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
	})
	a.tokenURL = tokenServer.URL
	a.ports = []int{port}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	credential, err := a.Authenticate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(credential, `"access_token":"acc"`) {
		t.Fatalf("unexpected credential: %s", credential)
	}
}

func TestAuthenticateRejectsMismatchedState(t *testing.T) {
	port := freePort(t)
	a := NewOpenAIAuthenticator(func(authURL string) {
		parsed, _ := url.Parse(authURL)
		redirect := parsed.Query().Get("redirect_uri")
		go func() {
			resp, err := http.Get(redirect + "?state=wrong&code=the-code")
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
	})
	a.ports = []int{port}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := a.Authenticate(ctx); err == nil {
		t.Fatal("expected an error when the callback state does not match")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("expected a TCP address")
	}
	return addr.Port
}
