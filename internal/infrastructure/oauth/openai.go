// Package oauth adapts external provider OAuth servers to aw. It owns the
// OpenAI Subscription (Codex) browser PKCE flow end to end — a localhost
// callback server, the authorize URL, the code-for-token exchange and the
// stored credential payload — so the composition root never has to embed
// HTTP/socket details. It implements ports.ProviderBrowserAuthenticator.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	openAIClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	openAIAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	openAITokenURL     = "https://auth.openai.com/oauth/token"
	openAIScopes       = "openid profile email offline_access api.connectors.read api.connectors.invoke"
	callbackPath       = "/auth/callback"
	successHTML        = "<html><body style=\"font-family:system-ui;text-align:center;padding:48px\"><h2>aw authentication complete</h2><p>Token saved and OpenAI Subscription activated. You can close this tab and return to aw.</p></body></html>"
)

// OpenAI/Codex Hydra validates redirect_uri against an allow-list. Keep these
// ports in sync with the Codex CLI login server (1455 primary, 1457 fallback);
// random localhost ports are rejected with authorize_hydra_invalid_request.
var openAICallbackPorts = []int{1455, 1457}

// OpenAIAuthenticator implements ports.ProviderBrowserAuthenticator for the
// OpenAI Subscription (Codex) browser OAuth flow.
type OpenAIAuthenticator struct {
	openBrowser  func(authURL string)
	httpClient   *http.Client
	authorizeURL string
	tokenURL     string
	ports        []int
}

// NewOpenAIAuthenticator builds the authenticator. openBrowser opens the
// system browser at the authorize URL (injected by the composition root so
// this package stays free of any UI framework).
func NewOpenAIAuthenticator(openBrowser func(authURL string)) *OpenAIAuthenticator {
	return &OpenAIAuthenticator{
		openBrowser:  openBrowser,
		httpClient:   http.DefaultClient,
		authorizeURL: openAIAuthorizeURL,
		tokenURL:     openAITokenURL,
		ports:        openAICallbackPorts,
	}
}

type callbackResult struct {
	credential string
	err        error
}

// Authenticate runs the full browser PKCE round trip and returns the provider
// credential JSON.
func (a *OpenAIAuthenticator) Authenticate(ctx context.Context) (string, error) {
	state, err := randomBase64URL(32)
	if err != nil {
		return "", err
	}
	verifier, err := randomBase64URL(64)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	listener, port, err := a.listen()
	if err != nil {
		return "", err
	}
	defer listener.Close()

	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, callbackPath)
	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		credential, err := a.handleCallback(ctx, r, state, verifier, redirectURI)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			resultCh <- callbackResult{err: err}
			return
		}
		_, _ = io.WriteString(w, successHTML)
		resultCh <- callbackResult{credential: credential}
	})
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			resultCh <- callbackResult{err: fmt.Errorf("OAuth callback server failed: %w", err)}
		}
	}()
	defer func() { _ = server.Shutdown(context.Background()) }()

	if a.openBrowser != nil {
		a.openBrowser(a.buildAuthorizeURL(redirectURI, state, challenge))
	}

	select {
	case res := <-resultCh:
		return res.credential, res.err
	case <-ctx.Done():
		return "", fmt.Errorf("authentication timed out before OpenAI returned a token")
	}
}

// handleCallback validates the OAuth callback and exchanges the code for a
// credential. It is pure of HTTP responses so it stays unit-testable.
func (a *OpenAIAuthenticator) handleCallback(ctx context.Context, r *http.Request, state, verifier, redirectURI string) (string, error) {
	query := r.URL.Query()
	if got := query.Get("state"); got != state {
		return "", fmt.Errorf("OAuth callback state did not match")
	}
	if oauthErr := strings.TrimSpace(query.Get("error")); oauthErr != "" {
		desc := strings.TrimSpace(query.Get("error_description"))
		return "", fmt.Errorf("OAuth error: %s %s", oauthErr, strings.TrimSpace(desc))
	}
	code := strings.TrimSpace(query.Get("code"))
	if code == "" {
		return "", fmt.Errorf("OAuth callback did not include an authorization code")
	}
	return a.exchange(ctx, code, verifier, redirectURI)
}

func (a *OpenAIAuthenticator) listen() (net.Listener, int, error) {
	for _, port := range a.ports {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return listener, port, nil
		}
	}
	return nil, 0, fmt.Errorf("could not start local OpenAI OAuth callback server on ports %v", a.ports)
}

func (a *OpenAIAuthenticator) buildAuthorizeURL(redirectURI, state, challenge string) string {
	values := url.Values{}
	values.Set("response_type", "code")
	values.Set("client_id", openAIClientID)
	values.Set("redirect_uri", redirectURI)
	values.Set("scope", openAIScopes)
	values.Set("code_challenge", challenge)
	values.Set("code_challenge_method", "S256")
	values.Set("id_token_add_organizations", "true")
	values.Set("codex_cli_simplified_flow", "true")
	values.Set("state", state)
	values.Set("originator", "codex_cli_rs")
	return a.authorizeURL + "?" + values.Encode()
}

func (a *OpenAIAuthenticator) exchange(ctx context.Context, code, verifier, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", openAIClientID)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OAuth token exchange transport failure: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("OAuth token exchange returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return buildCredential(raw)
}

// buildCredential wraps the raw token response in the stored credential
// payload (matching the Codex CLI auth.json shape).
func buildCredential(raw []byte) (string, error) {
	var token map[string]any
	if err := json.Unmarshal(raw, &token); err != nil {
		return "", fmt.Errorf("OAuth token response was not valid JSON")
	}
	if !nonEmptyString(token, "access_token") || !nonEmptyString(token, "refresh_token") {
		return "", fmt.Errorf("OAuth token response did not include access and refresh tokens")
	}
	payload := map[string]any{
		"auth_mode":    "chatgpt",
		"tokens":       token,
		"last_refresh": time.Now().UTC().Format(time.RFC3339),
	}
	credential, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("could not encode OAuth credential")
	}
	return string(credential), nil
}

// nonEmptyString reports whether m[key] is a present, non-blank string. A
// missing key yields a nil value, which earlier code stringified to "<nil>"
// and wrongly accepted — this checks presence and type properly.
func nonEmptyString(m map[string]any, key string) bool {
	value, ok := m[key]
	if !ok || value == nil {
		return false
	}
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

func randomBase64URL(bytesLen int) (string, error) {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
