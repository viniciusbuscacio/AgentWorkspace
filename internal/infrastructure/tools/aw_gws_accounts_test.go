package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gwsAccountsWorkspace records the CONFIG_DIR env passed to each gws call so we
// can assert that account selection routes to the right isolated dir.
func gwsAccountsWorkspace(t *testing.T, exec func(argv, env []string) (string, string, int, error)) *workspace {
	t.Helper()
	return &workspace{
		root:           t.TempDir(),
		control:        &fakeControl{},
		autoApprove:    true,
		gwsAccountsDir: t.TempDir(),
		gwsExecFn: func(_ context.Context, _ string, argv []string, env []string, _ time.Duration) (string, string, int, error) {
			return exec(argv, env)
		},
	}
}

func configDirFromEnv(env []string) string {
	for _, e := range env {
		if strings.HasPrefix(e, "GOOGLE_WORKSPACE_CLI_CONFIG_DIR=") {
			return strings.TrimPrefix(e, "GOOGLE_WORKSPACE_CLI_CONFIG_DIR=")
		}
	}
	return ""
}

func TestGwsNoAccountsUsesDefaultDir(t *testing.T) {
	var sawConfigDir string
	ws := gwsAccountsWorkspace(t, func(_, env []string) (string, string, int, error) {
		sawConfigDir = configDirFromEnv(env)
		return "{}", "", 0, nil
	})
	// Empty registry -> no CONFIG_DIR override (uses gws ~/.config/gws).
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.call", Args: `{"service":"drive","resource":"files","method":"list"}`}); err != nil {
		t.Fatalf("gws.call error = %v", err)
	}
	if sawConfigDir != "" {
		t.Fatalf("with no accounts CONFIG_DIR must be unset, got %q", sawConfigDir)
	}
}

func writeAccounts(t *testing.T, dir string, file gwsAccountsFile) {
	t.Helper()
	data, _ := json.MarshalIndent(file, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "accounts.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGwsAccountSelectionRoutesConfigDir(t *testing.T) {
	var sawConfigDir string
	ws := gwsAccountsWorkspace(t, func(_, env []string) (string, string, int, error) {
		sawConfigDir = configDirFromEnv(env)
		return "[]", "", 0, nil
	})
	writeAccounts(t, ws.gwsAccountsDir, gwsAccountsFile{Accounts: []gwsAccount{
		{ID: "personal", Email: "personal@example.com", Default: true},
		{ID: "lab", Email: "lab@example.org"},
	}})

	// Explicit account -> its dir.
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.gmail.inbox", Args: `{"account":"lab"}`}); err != nil {
		t.Fatalf("inbox error = %v", err)
	}
	if !strings.HasSuffix(sawConfigDir, filepath.Join("accounts", "lab")) {
		t.Fatalf("account=lab should route to lab dir, got %q", sawConfigDir)
	}

	// Selecting by email also works.
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.gmail.inbox", Args: `{"account":"lab@example.org"}`}); err != nil {
		t.Fatalf("inbox by email error = %v", err)
	}
	if !strings.HasSuffix(sawConfigDir, filepath.Join("accounts", "lab")) {
		t.Fatalf("email selection should route to lab dir, got %q", sawConfigDir)
	}

	// No account -> default account dir.
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.gmail.inbox", Args: `{}`}); err != nil {
		t.Fatalf("inbox default error = %v", err)
	}
	if !strings.HasSuffix(sawConfigDir, filepath.Join("accounts", "personal")) {
		t.Fatalf("default should route to personal dir, got %q", sawConfigDir)
	}
}

func TestGwsUnknownAccountErrors(t *testing.T) {
	ws := gwsAccountsWorkspace(t, func(_, _ []string) (string, string, int, error) { return "{}", "", 0, nil })
	writeAccounts(t, ws.gwsAccountsDir, gwsAccountsFile{Accounts: []gwsAccount{{ID: "personal", Default: true}}})
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.call", Args: `{"service":"drive","resource":"files","method":"list","account":"ghost"}`}); err == nil ||
		!strings.Contains(err.Error(), "unknown Google account") {
		t.Fatalf("expected unknown-account error, got %v", err)
	}
}

func TestGwsAccountsListReportsAuthState(t *testing.T) {
	ws := gwsAccountsWorkspace(t, func(argv, _ []string) (string, string, int, error) {
		if strings.Join(argv, " ") == "auth status" {
			return "ok", "", 0, nil
		}
		return "{}", "", 0, nil
	})
	writeAccounts(t, ws.gwsAccountsDir, gwsAccountsFile{Accounts: []gwsAccount{
		{ID: "personal", Email: "personal@example.com", Default: true},
	}})
	result, err := ws.awDispatch(nil, awArgs{Action: "gws.accounts"})
	if err != nil {
		t.Fatalf("accounts list error = %v", err)
	}
	for _, want := range []string{"personal", "personal@example.com", `"authenticated": true`, `"default": true`} {
		if !strings.Contains(result.Result, want) {
			t.Fatalf("accounts list missing %q: %s", want, result.Result)
		}
	}
}

func TestGwsAccountsAddRegistersAndLogsIn(t *testing.T) {
	// A real client_secret.json the seed step will copy.
	clientPath := filepath.Join(t.TempDir(), "client_secret.json")
	if err := os.WriteFile(clientPath, []byte(`{"installed":{"client_id":"x"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	statusPayload, err := json.Marshal(map[string]any{
		"client_config":        clientPath,
		"client_config_exists": true,
		"email":                "new@corp.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	statusJSON := string(statusPayload)
	calls := [][]string{}
	ws := gwsAccountsWorkspace(t, func(argv, _ []string) (string, string, int, error) {
		calls = append(calls, argv)
		if strings.Contains(strings.Join(argv, " "), "auth status") {
			return statusJSON, "", 0, nil
		}
		return "logged in", "", 0, nil
	})
	result, err := ws.awDispatch(nil, awArgs{Action: "gws.accounts.add", Args: `{"id":"Work Account","label":"Job"}`})
	if err != nil {
		t.Fatalf("accounts.add error = %v", err)
	}
	// id slugified, login ran, email captured, first account becomes default.
	for _, want := range []string{`"added": "work-account"`, `"email": "new@corp.com"`, `"default": true`} {
		if !strings.Contains(result.Result, want) {
			t.Fatalf("add result missing %q: %s", want, result.Result)
		}
	}
	var sawLogin bool
	for _, c := range calls {
		if strings.Join(c, " ") == "auth login" {
			sawLogin = true
		}
	}
	if !sawLogin {
		t.Fatalf("expected an auth login call, got %v", calls)
	}
	// Client secret seeded into the account's isolated config dir.
	if _, statErr := os.Stat(filepath.Join(ws.gwsAccountConfigDir("work-account"), "client_secret.json")); statErr != nil {
		t.Fatalf("client_secret.json should be seeded: %v", statErr)
	}
	// Registry persisted.
	file, err := ws.loadGwsAccounts()
	if err != nil || len(file.Accounts) != 1 || file.Accounts[0].ID != "work-account" {
		t.Fatalf("registry not persisted: %+v err=%v", file, err)
	}
}

func TestGwsAccountsAddFailsWithoutOAuthClient(t *testing.T) {
	ws := gwsAccountsWorkspace(t, func(argv, _ []string) (string, string, int, error) {
		if strings.Contains(strings.Join(argv, " "), "auth status") {
			return `{"client_config_exists":false}`, "", 0, nil
		}
		t.Fatal("login must not run when no OAuth client is configured")
		return "", "", 0, nil
	})
	_, err := ws.awDispatch(nil, awArgs{Action: "gws.accounts.add", Args: `{"id":"x"}`})
	if err == nil || !strings.Contains(err.Error(), "gws auth setup") {
		t.Fatalf("expected setup hint error, got %v", err)
	}
}

func TestGwsAccountsAddDeniedWithoutApproval(t *testing.T) {
	ran := false
	ws := &workspace{
		root:           t.TempDir(),
		control:        &fakeControl{},
		gwsAccountsDir: t.TempDir(),
		confirm:        func(context.Context, ConfirmRequest) (bool, error) { return false, nil },
		gwsExecFn: func(_ context.Context, _ string, _ []string, _ []string, _ time.Duration) (string, string, int, error) {
			ran = true
			return "", "", 0, nil
		},
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.accounts.add", Args: `{"id":"x"}`}); err != ErrConfirmationDenied {
		t.Fatalf("expected denial, got %v", err)
	}
	if ran {
		t.Fatal("login must not run when add is denied")
	}
}

func TestGwsAccountsRemoveAndDefault(t *testing.T) {
	ws := gwsAccountsWorkspace(t, func(_, _ []string) (string, string, int, error) { return "{}", "", 0, nil })
	writeAccounts(t, ws.gwsAccountsDir, gwsAccountsFile{Accounts: []gwsAccount{
		{ID: "a", Email: "a@x.com", Default: true},
		{ID: "b", Email: "b@x.com"},
	}})

	// Switch default to b.
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.accounts.default", Args: `{"id":"b"}`}); err != nil {
		t.Fatalf("default error = %v", err)
	}
	file, _ := ws.loadGwsAccounts()
	for _, acc := range file.Accounts {
		if acc.ID == "b" && !acc.Default {
			t.Fatal("b should be default")
		}
		if acc.ID == "a" && acc.Default {
			t.Fatal("a should no longer be default")
		}
	}

	// Remove b -> a becomes default again.
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.accounts.remove", Args: `{"id":"b"}`}); err != nil {
		t.Fatalf("remove error = %v", err)
	}
	file, _ = ws.loadGwsAccounts()
	if len(file.Accounts) != 1 || file.Accounts[0].ID != "a" || !file.Accounts[0].Default {
		t.Fatalf("after remove, registry = %+v", file.Accounts)
	}
}

func TestGwsAccountActionsRegistered(t *testing.T) {
	ws := gwsAccountsWorkspace(t, func(_, _ []string) (string, string, int, error) { return "{}", "", 0, nil })
	listed, err := ws.awDispatch(nil, awArgs{Action: "aw.actions"})
	if err != nil {
		t.Fatalf("aw.actions error = %v", err)
	}
	for _, want := range []string{"gws.accounts", "gws.accounts.add", "gws.accounts.remove", "gws.accounts.default"} {
		if !strings.Contains(listed.Result, want) {
			t.Fatalf("aw.actions missing %q", want)
		}
	}
}
