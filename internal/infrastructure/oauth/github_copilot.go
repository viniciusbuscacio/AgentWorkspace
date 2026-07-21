package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHub Copilot signs in through GitHub's OAuth device flow: aw requests a
// device + user code, shows the user the code and opens the verification page,
// then polls until the user authorizes. The resulting GitHub token is stored;
// the short-lived Copilot token is derived from it at runtime by the agent
// adapter. This implements ports.ProviderBrowserAuthenticator.

const (
	// Public client id used by the GitHub Copilot editor plugins for the
	// device flow. Copilot entitlement is checked at token-exchange time.
	gitHubCopilotClientID    = "Iv1.b507a08c87ecfe98"
	gitHubDeviceCodeURL      = "https://github.com/login/device/code"
	gitHubDeviceTokenURL     = "https://github.com/login/oauth/access_token"
	gitHubDeviceGrantType    = "urn:ietf:params:oauth:grant-type:device_code"
	gitHubCopilotDeviceScope = "read:user"
)

// GitHubCopilotAuthenticator runs the GitHub device flow for Copilot.
type GitHubCopilotAuthenticator struct {
	httpClient    *http.Client
	openBrowser   func(verificationURI string)
	onUserCode    func(userCode, verificationURI string)
	clientID      string
	scope         string
	deviceCodeURL string
	tokenURL      string
}

// NewGitHubCopilotAuthenticator builds the authenticator. openBrowser opens the
// system browser at the verification URI and onUserCode surfaces the user code
// to the UI (copy to clipboard / notify); both are injected by the composition
// root so this package stays free of any UI framework.
func NewGitHubCopilotAuthenticator(openBrowser func(string), onUserCode func(userCode, verificationURI string)) *GitHubCopilotAuthenticator {
	return &GitHubCopilotAuthenticator{
		// A per-request timeout: http.DefaultClient has none, so a stalled device
		// code request would hang the whole flow until the 10-min run context
		// expires — the user sees "Waiting..." with no code, no browser, no error.
		// This surfaces a network failure promptly instead. Each poll request is
		// quick, so the same cap is fine for token polling.
		httpClient:    &http.Client{Timeout: 25 * time.Second},
		openBrowser:   openBrowser,
		onUserCode:    onUserCode,
		clientID:      gitHubCopilotClientID,
		scope:         gitHubCopilotDeviceScope,
		deviceCodeURL: gitHubDeviceCodeURL,
		tokenURL:      gitHubDeviceTokenURL,
	}
}

type gitHubDeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type gitHubDeviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// Authenticate runs the full device flow and returns the stored credential JSON
// (the GitHub OAuth token wrapped in aw's Copilot credential envelope).
func (a *GitHubCopilotAuthenticator) Authenticate(ctx context.Context) (string, error) {
	device, err := a.requestDeviceCode(ctx)
	if err != nil {
		return "", err
	}
	if a.onUserCode != nil {
		a.onUserCode(device.UserCode, device.VerificationURI)
	}
	if a.openBrowser != nil {
		a.openBrowser(device.VerificationURI)
	}
	return a.pollForToken(ctx, device)
}

func (a *GitHubCopilotAuthenticator) requestDeviceCode(ctx context.Context) (gitHubDeviceCodeResponse, error) {
	form := url.Values{}
	form.Set("client_id", a.clientID)
	form.Set("scope", a.scope)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.deviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return gitHubDeviceCodeResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return gitHubDeviceCodeResponse{}, fmt.Errorf("GitHub device code request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return gitHubDeviceCodeResponse{}, fmt.Errorf("GitHub device code request returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var device gitHubDeviceCodeResponse
	if err := json.Unmarshal(raw, &device); err != nil {
		return gitHubDeviceCodeResponse{}, fmt.Errorf("GitHub device code response was not valid JSON")
	}
	if strings.TrimSpace(device.DeviceCode) == "" || strings.TrimSpace(device.UserCode) == "" {
		return gitHubDeviceCodeResponse{}, fmt.Errorf("GitHub device code response was incomplete")
	}
	if strings.TrimSpace(device.VerificationURI) == "" {
		device.VerificationURI = "https://github.com/login/device"
	}
	return device, nil
}

func (a *GitHubCopilotAuthenticator) pollForToken(ctx context.Context, device gitHubDeviceCodeResponse) (string, error) {
	interval := device.Interval
	if interval < 1 {
		interval = 5
	}
	deadline := time.Now().Add(time.Duration(maxInt(device.ExpiresIn, 300)) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("GitHub Copilot sign-in timed out before authorization")
		case <-time.After(time.Duration(interval) * time.Second):
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("GitHub Copilot sign-in expired; please try again")
		}
		token, retry, slowDown, err := a.exchangeDeviceCode(ctx, device.DeviceCode)
		if err != nil {
			return "", err
		}
		if slowDown {
			interval += 5
		}
		if retry {
			continue
		}
		return buildGitHubCopilotCredential(token)
	}
}

// exchangeDeviceCode polls the token endpoint once. retry=true means keep
// waiting (authorization_pending / slow_down); slowDown asks to back off.
func (a *GitHubCopilotAuthenticator) exchangeDeviceCode(ctx context.Context, deviceCode string) (string, bool, bool, error) {
	form := url.Values{}
	form.Set("client_id", a.clientID)
	form.Set("device_code", deviceCode)
	form.Set("grant_type", gitHubDeviceGrantType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", false, false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", false, false, fmt.Errorf("GitHub device token request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var token gitHubDeviceTokenResponse
	if err := json.Unmarshal(raw, &token); err != nil {
		return "", false, false, fmt.Errorf("GitHub device token response was not valid JSON")
	}
	switch strings.TrimSpace(token.Error) {
	case "":
		if strings.TrimSpace(token.AccessToken) == "" {
			return "", false, false, fmt.Errorf("GitHub device token response did not include an access token")
		}
		return token.AccessToken, false, false, nil
	case "authorization_pending":
		return "", true, false, nil
	case "slow_down":
		return "", true, true, nil
	case "expired_token":
		return "", false, false, fmt.Errorf("GitHub Copilot sign-in code expired; please try again")
	case "access_denied":
		return "", false, false, fmt.Errorf("GitHub Copilot sign-in was denied")
	default:
		desc := strings.TrimSpace(token.ErrorDesc)
		if desc == "" {
			desc = token.Error
		}
		return "", false, false, fmt.Errorf("GitHub Copilot sign-in failed: %s", desc)
	}
}

// buildGitHubCopilotCredential wraps the GitHub OAuth token in aw's stored
// Copilot credential envelope (matching the agent adapter's parser).
func buildGitHubCopilotCredential(githubToken string) (string, error) {
	payload := map[string]any{
		"auth_mode":    "github-copilot",
		"github_token": githubToken,
		"last_refresh": time.Now().UTC().Format(time.RFC3339),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("could not encode GitHub Copilot credential")
	}
	return string(encoded), nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
