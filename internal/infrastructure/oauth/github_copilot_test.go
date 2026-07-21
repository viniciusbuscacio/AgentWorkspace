package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGitHubCopilotAuthenticateHappyPath(t *testing.T) {
	var tokenCalls int
	deviceSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("client_id") == "" {
			t.Errorf("device code request missing client_id")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "dev-123",
			"user_code":        "WXYZ-1234",
			"verification_uri": "https://github.com/login/device",
			"expires_in":       900,
			"interval":         1,
		})
	}))
	defer deviceSrv.Close()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tokenCalls++
		w.Header().Set("Content-Type", "application/json")
		if tokenCalls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "authorization_pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "gho_test123", "token_type": "bearer"})
	}))
	defer tokenSrv.Close()

	var gotUserCode, gotURL, browserURL string
	auth := NewGitHubCopilotAuthenticator(
		func(uri string) { browserURL = uri },
		func(userCode, uri string) { gotUserCode, gotURL = userCode, uri },
	)
	auth.deviceCodeURL = deviceSrv.URL
	auth.tokenURL = tokenSrv.URL
	auth.httpClient = deviceSrv.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	credential, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if gotUserCode != "WXYZ-1234" || gotURL != "https://github.com/login/device" {
		t.Errorf("onUserCode got (%q,%q)", gotUserCode, gotURL)
	}
	if browserURL != "https://github.com/login/device" {
		t.Errorf("openBrowser got %q", browserURL)
	}
	if tokenCalls < 2 {
		t.Errorf("expected polling to retry pending, calls=%d", tokenCalls)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(credential), &env); err != nil {
		t.Fatalf("credential JSON: %v", err)
	}
	if env["auth_mode"] != "github-copilot" || env["github_token"] != "gho_test123" {
		t.Errorf("credential envelope wrong: %s", credential)
	}
}

func TestGitHubCopilotAuthenticateDenied(t *testing.T) {
	deviceSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dev-1", "user_code": "AAAA-1111",
			"verification_uri": "https://github.com/login/device", "expires_in": 900, "interval": 1,
		})
	}))
	defer deviceSrv.Close()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "access_denied"})
	}))
	defer tokenSrv.Close()

	auth := NewGitHubCopilotAuthenticator(func(string) {}, func(string, string) {})
	auth.deviceCodeURL = deviceSrv.URL
	auth.tokenURL = tokenSrv.URL
	auth.httpClient = deviceSrv.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := auth.Authenticate(ctx); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected denied error, got %v", err)
	}
}
