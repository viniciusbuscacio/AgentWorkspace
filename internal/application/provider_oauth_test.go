package application

import (
	"context"
	"errors"
	"testing"
)

type fakeBrowserAuthenticator struct {
	credential string
	err        error
	called     bool
}

func (f *fakeBrowserAuthenticator) Authenticate(_ context.Context) (string, error) {
	f.called = true
	return f.credential, f.err
}

func TestAuthenticateProviderViaBrowserRejectsUnsupportedProvider(t *testing.T) {
	store := newFakeSecretStore()
	authn := &fakeBrowserAuthenticator{credential: "x"}
	result := AuthenticateProviderViaBrowser(context.Background(), authn, store, nil, "openrouter", "")
	if result.Success {
		t.Fatal("expected failure for an unsupported provider")
	}
	if authn.called {
		t.Fatal("authenticator must not run for an unsupported provider")
	}
}

func TestAuthenticateProviderViaBrowserPropagatesAuthError(t *testing.T) {
	store := newFakeSecretStore()
	authn := &fakeBrowserAuthenticator{err: errors.New("user closed the tab")}
	result := AuthenticateProviderViaBrowser(context.Background(), authn, store, nil, "openai-codex", "gpt-5.5")
	if result.Success {
		t.Fatal("expected failure when authentication fails")
	}
	if result.Error != "user closed the tab" {
		t.Fatalf("expected the auth error to surface, got %q", result.Error)
	}
}

func TestAuthenticateProviderViaBrowserSavesCredential(t *testing.T) {
	store := newFakeSecretStore()
	authn := &fakeBrowserAuthenticator{credential: `{"auth_mode":"chatgpt"}`}
	result := AuthenticateProviderViaBrowser(context.Background(), authn, store, nil, "openai-codex", "gpt-5.5")
	if !result.Success || !result.Activated {
		t.Fatalf("expected success and activation, got %+v", result)
	}
	if store.secrets["openai_codex_auth_json"] != `{"auth_mode":"chatgpt"}` {
		t.Fatal("expected the credential to be persisted to the vault")
	}
	if store.secrets["_config_provider"] != "openai-codex" {
		t.Fatal("expected openai-codex to be set active")
	}
	if store.secrets["_config_model_openai_codex"] != "gpt-5.5" {
		t.Fatal("expected the model to be persisted")
	}
}

func TestAuthenticateProviderViaBrowserRequiresAuthenticator(t *testing.T) {
	store := newFakeSecretStore()
	result := AuthenticateProviderViaBrowser(context.Background(), nil, store, nil, "openai-codex", "")
	if result.Success {
		t.Fatal("expected failure with a nil authenticator")
	}
}
