package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseGitHubCopilotCredential(t *testing.T) {
	if _, err := parseGitHubCopilotCredential(""); err == nil {
		t.Errorf("expected error on empty credential")
	}
	if _, err := parseGitHubCopilotCredential(`{"auth_mode":"github-copilot"}`); err == nil {
		t.Errorf("expected error when github_token is missing")
	}
	cred, err := parseGitHubCopilotCredential(`{"auth_mode":"github-copilot","github_token":"gho_x"}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cred.GitHubToken != "gho_x" {
		t.Errorf("github token = %q", cred.GitHubToken)
	}
}

func TestCopilotTokenManagerRefreshesAndCaches(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "token gho_x" {
			t.Errorf("token exchange auth header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "tid=abc;exp=123",
			"expires_at": time.Now().Add(30 * time.Minute).Unix(),
		})
	}))
	defer srv.Close()

	var persisted string
	mgr := &copilotTokenManager{
		credential:        gitHubCopilotCredential{AuthMode: "github-copilot", GitHubToken: "gho_x"},
		client:            srv.Client(),
		tokenURL:          srv.URL,
		credentialUpdater: func(next string) error { persisted = next; return nil },
	}

	ctx := context.Background()
	tok, err := mgr.Token(ctx)
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok != "tid=abc;exp=123" {
		t.Errorf("token = %q", tok)
	}
	// Second call must hit the cache, not the server.
	if _, err := mgr.Token(ctx); err != nil {
		t.Fatalf("Token (cached): %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 exchange call (cache hit on 2nd), got %d", calls)
	}
	if !strings.Contains(persisted, "copilot_token") {
		t.Errorf("credential was not persisted with refreshed token: %s", persisted)
	}
}

func TestCopilotTokenManagerExpiredRefetches(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "fresh",
			"expires_at": time.Now().Add(30 * time.Minute).Unix(),
		})
	}))
	defer srv.Close()

	mgr := &copilotTokenManager{
		// Cached token already expired -> must refetch.
		credential: gitHubCopilotCredential{
			GitHubToken: "gho_x", CopilotToken: "stale", CopilotExpiresAt: time.Now().Add(-time.Minute).Unix(),
		},
		client:   srv.Client(),
		tokenURL: srv.URL,
	}
	tok, err := mgr.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok != "fresh" || calls != 1 {
		t.Errorf("expected refetch to 'fresh' (calls=1), got %q calls=%d", tok, calls)
	}
}

func TestCopilotBaseURLFromToken(t *testing.T) {
	cases := map[string]string{
		"tid=abc;exp=123;proxy-ep=proxy.individual.githubcopilot.com;sku=x": "https://api.individual.githubcopilot.com",
		"proxy-ep=proxy.business.githubcopilot.com":                         "https://api.business.githubcopilot.com",
		"tid=abc;exp=123": "",
	}
	for token, want := range cases {
		if got := copilotBaseURLFromToken(token); got != want {
			t.Errorf("copilotBaseURLFromToken(%q) = %q, want %q", token, got, want)
		}
	}
}

func TestCopilotTokenManagerDerivesBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "tid=abc;proxy-ep=proxy.individual.githubcopilot.com;exp=1",
			"expires_at": time.Now().Add(30 * time.Minute).Unix(),
		})
	}))
	defer srv.Close()
	mgr := &copilotTokenManager{
		credential:      gitHubCopilotCredential{GitHubToken: "gho_x"},
		client:          srv.Client(),
		tokenURL:        srv.URL,
		fallbackBaseURL: defaultGitHubCopilotBaseURL,
	}
	base, err := mgr.BaseURL(context.Background())
	if err != nil {
		t.Fatalf("BaseURL: %v", err)
	}
	if base != "https://api.individual.githubcopilot.com" {
		t.Errorf("derived base url = %q", base)
	}
}

func TestNewGitHubCopilotModelBuilds(t *testing.T) {
	m, err := NewGitHubCopilotModel(GitHubCopilotConfig{
		Model:      "claude-sonnet-4",
		Credential: `{"auth_mode":"github-copilot","github_token":"gho_x"}`,
	})
	if err != nil {
		t.Fatalf("NewGitHubCopilotModel: %v", err)
	}
	if m.Name() != "claude-sonnet-4" {
		t.Errorf("model name = %q", m.Name())
	}
	if m.authToken == nil {
		t.Errorf("expected dynamic auth token provider to be wired")
	}
	if m.extraHeaders["Copilot-Integration-Id"] != gitHubCopilotIntegrationID {
		t.Errorf("missing Copilot integration header")
	}
}
