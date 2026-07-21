package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Multi-account Google Workspace support.
//
// gws has no native multi-account flag: it keys everything off a single
// GOOGLE_WORKSPACE_CLI_CONFIG_DIR (default ~/.config/gws). To support several
// Google accounts we give each one its own isolated config dir under
// gwsAccountsDir and select it per call via the `account` arg.
//
// Backward compatible: with zero registered accounts, runGws uses the native
// gws default dir, so an already-authenticated machine keeps working untouched.

const gwsAuthTimeout = 5 * time.Minute

type gwsAccount struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Label   string `json:"label,omitempty"`
	Default bool   `json:"default,omitempty"`
}

type gwsAccountsFile struct {
	Accounts []gwsAccount `json:"accounts"`
}

func registerGwsAccountActions(reg map[string]AwActionHandler) {
	reg["gws.accounts"] = gwsAccountsListAction
	reg["gws.accounts.add"] = gwsAccountsAddAction
	reg["gws.accounts.remove"] = gwsAccountsRemoveAction
	reg["gws.accounts.default"] = gwsAccountsDefaultAction
}

func gwsAccountArg(args map[string]any) string {
	if v, ok := args["account"]; ok && v != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
	return ""
}

func (w *workspace) gwsAccountsPath() string {
	return filepath.Join(w.gwsAccountsDir, "accounts.json")
}

func (w *workspace) loadGwsAccounts() (gwsAccountsFile, error) {
	var out gwsAccountsFile
	if strings.TrimSpace(w.gwsAccountsDir) == "" {
		return out, nil
	}
	data, err := os.ReadFile(w.gwsAccountsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	if len(data) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, fmt.Errorf("corrupt gws accounts registry: %w", err)
	}
	return out, nil
}

func (w *workspace) saveGwsAccounts(file gwsAccountsFile) error {
	if strings.TrimSpace(w.gwsAccountsDir) == "" {
		return fmt.Errorf("multi-account storage is not configured")
	}
	if err := os.MkdirAll(w.gwsAccountsDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(w.gwsAccountsPath(), data, 0o600)
}

// resolveGwsConfigDir maps an accountID to its gws config dir. Empty result
// means "use the gws native default dir". Rules:
//   - no registered accounts -> "" (backward compatible)
//   - accountID set          -> that account's dir (error if unknown)
//   - accountID empty        -> the default account's dir
func (w *workspace) resolveGwsConfigDir(accountID string) (string, error) {
	file, err := w.loadGwsAccounts()
	if err != nil {
		return "", err
	}
	if len(file.Accounts) == 0 {
		return "", nil
	}
	accountID = strings.TrimSpace(accountID)
	if accountID != "" {
		for _, a := range file.Accounts {
			if strings.EqualFold(a.ID, accountID) || strings.EqualFold(a.Email, accountID) {
				return w.gwsAccountConfigDir(a.ID), nil
			}
		}
		return "", fmt.Errorf("unknown Google account %q; use gws.accounts to list configured accounts", accountID)
	}
	for _, a := range file.Accounts {
		if a.Default {
			return w.gwsAccountConfigDir(a.ID), nil
		}
	}
	// No explicit default: fall back to the first registered account.
	return w.gwsAccountConfigDir(file.Accounts[0].ID), nil
}

func (w *workspace) gwsAccountConfigDir(id string) string {
	return filepath.Join(w.gwsAccountsDir, "accounts", id)
}

// gws.accounts: list configured accounts (id, email, label, default) plus the
// live auth state of each.
func gwsAccountsListAction(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
	if strings.TrimSpace(w.gwsAccountsDir) == "" {
		return awJSON(map[string]any{"accounts": []any{}, "note": "multi-account storage not configured; using the gws default account"})
	}
	file, err := w.loadGwsAccounts()
	if err != nil {
		return "", err
	}
	type accountStatus struct {
		gwsAccount
		Authenticated bool   `json:"authenticated"`
		StatusError   string `json:"status_error,omitempty"`
	}
	statuses := make([]accountStatus, 0, len(file.Accounts))
	for _, a := range file.Accounts {
		st := accountStatus{gwsAccount: a}
		run, rerr := w.runGws(ctx, []string{"auth", "status"}, a.ID)
		if rerr != nil {
			st.StatusError = rerr.Error()
		} else {
			st.Authenticated = run.ExitCode == 0
			if run.ExitCode != 0 {
				st.StatusError = strings.TrimSpace(run.Stderr)
			}
		}
		statuses = append(statuses, st)
	}
	return awJSON(map[string]any{"accounts": statuses})
}

// gws.accounts.add: register a new account and run the OAuth login flow in its
// isolated config dir. Args: id (required, short slug), label?, makeDefault?.
// Opens a browser; always confirmed.
func gwsAccountsAddAction(ctx context.Context, args map[string]any, w *workspace) (string, error) {
	if strings.TrimSpace(w.gwsAccountsDir) == "" {
		return "", fmt.Errorf("multi-account storage is not configured")
	}
	id, err := awRequiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	id = gwsSlug(id)
	if id == "" {
		return "", fmt.Errorf("id must contain letters or digits")
	}
	file, err := w.loadGwsAccounts()
	if err != nil {
		return "", err
	}
	for _, a := range file.Accounts {
		if strings.EqualFold(a.ID, id) {
			return "", fmt.Errorf("account %q already exists", id)
		}
	}

	if ok, cerr := gwsConfirm(ctx, w, "gws.accounts.add", "Add a Google account",
		fmt.Sprintf("Run OAuth login for new account %q (a browser window will open).", id)); cerr != nil || !ok {
		return gwsConfirmResult(ok, cerr)
	}

	if err := os.MkdirAll(w.gwsAccountConfigDir(id), 0o700); err != nil {
		return "", err
	}
	// The isolated config dir starts empty; seed it with the machine's OAuth
	// client (client_secret.json) so `auth login` has an app to authorize
	// against. The per-account token (credentials.enc) stays isolated here.
	if err := w.gwsSeedClientSecret(ctx, w.gwsAccountConfigDir(id)); err != nil {
		_ = os.RemoveAll(w.gwsAccountConfigDir(id))
		return "", err
	}
	run, err := w.runGwsTimeout(ctx, []string{"auth", "login"}, id, gwsAuthTimeout)
	if err != nil {
		return "", err
	}
	if run.ExitCode != 0 {
		_ = os.RemoveAll(w.gwsAccountConfigDir(id))
		return "", fmt.Errorf("gws auth login failed (exit %d): %s", run.ExitCode, strings.TrimSpace(run.Stderr))
	}

	email := gwsExtractEmail(ctx, w, id)
	makeDefault := boolArg(args, "makeDefault") || len(file.Accounts) == 0
	if makeDefault {
		for i := range file.Accounts {
			file.Accounts[i].Default = false
		}
	}
	label, _, _ := awStringArg(args, "label")
	file.Accounts = append(file.Accounts, gwsAccount{
		ID:      id,
		Email:   email,
		Label:   strings.TrimSpace(label),
		Default: makeDefault,
	})
	if err := w.saveGwsAccounts(file); err != nil {
		return "", err
	}
	return awJSON(map[string]any{"added": id, "email": email, "default": makeDefault})
}

// gws.accounts.remove: log out and delete an account. Args: id (required).
// Confirmed.
func gwsAccountsRemoveAction(ctx context.Context, args map[string]any, w *workspace) (string, error) {
	if strings.TrimSpace(w.gwsAccountsDir) == "" {
		return "", fmt.Errorf("multi-account storage is not configured")
	}
	id, err := awRequiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	id = strings.TrimSpace(id)
	file, err := w.loadGwsAccounts()
	if err != nil {
		return "", err
	}
	idx := -1
	for i, a := range file.Accounts {
		if strings.EqualFold(a.ID, id) || strings.EqualFold(a.Email, id) {
			idx = i
			id = a.ID
			break
		}
	}
	if idx < 0 {
		return "", fmt.Errorf("unknown account %q", id)
	}

	if ok, cerr := gwsConfirm(ctx, w, "gws.accounts.remove", "Remove a Google account",
		fmt.Sprintf("Log out and delete local credentials for %q.", id)); cerr != nil || !ok {
		return gwsConfirmResult(ok, cerr)
	}

	// Best-effort logout, then remove the dir and the registry entry.
	_, _ = w.runGws(ctx, []string{"auth", "logout"}, id)
	_ = os.RemoveAll(w.gwsAccountConfigDir(id))

	wasDefault := file.Accounts[idx].Default
	file.Accounts = append(file.Accounts[:idx], file.Accounts[idx+1:]...)
	if wasDefault && len(file.Accounts) > 0 {
		file.Accounts[0].Default = true
	}
	if err := w.saveGwsAccounts(file); err != nil {
		return "", err
	}
	return awJSON(map[string]any{"removed": id})
}

// gws.accounts.default: set the default account. Args: id (required).
func gwsAccountsDefaultAction(_ context.Context, args map[string]any, w *workspace) (string, error) {
	if strings.TrimSpace(w.gwsAccountsDir) == "" {
		return "", fmt.Errorf("multi-account storage is not configured")
	}
	id, err := awRequiredStringArg(args, "id")
	if err != nil {
		return "", err
	}
	id = strings.TrimSpace(id)
	file, err := w.loadGwsAccounts()
	if err != nil {
		return "", err
	}
	found := false
	for i := range file.Accounts {
		match := strings.EqualFold(file.Accounts[i].ID, id) || strings.EqualFold(file.Accounts[i].Email, id)
		file.Accounts[i].Default = match
		if match {
			found = true
			id = file.Accounts[i].ID
		}
	}
	if !found {
		return "", fmt.Errorf("unknown account %q", id)
	}
	if err := w.saveGwsAccounts(file); err != nil {
		return "", err
	}
	return awJSON(map[string]any{"default": id})
}

// gwsSeedClientSecret copies the machine's OAuth client (client_secret.json)
// from the default gws config into a per-account config dir. It asks gws where
// the client lives (cross-platform) via `auth status --format json` on the
// default account, so we never guess paths. If no client is configured yet
// (fresh machine), it returns a clear setup hint.
func (w *workspace) gwsSeedClientSecret(ctx context.Context, configDir string) error {
	run, err := w.runGws(ctx, []string{"auth", "status", "--format", "json"}, "")
	if err != nil {
		return err
	}
	var st struct {
		ClientConfig       string `json:"client_config"`
		ClientConfigExists bool   `json:"client_config_exists"`
	}
	if jerr := json.Unmarshal([]byte(run.Stdout), &st); jerr != nil {
		return fmt.Errorf("could not read gws auth status: %w", jerr)
	}
	if !st.ClientConfigExists || strings.TrimSpace(st.ClientConfig) == "" {
		return fmt.Errorf("no OAuth client configured on this machine; run `gws auth setup` once first — " +
			"it automates the GCP project + OAuth client and only needs the gcloud CLI. " +
			"With shell access you can drive it yourself after the user's OK " +
			"(see the google-workspace skill, section 'Authenticate'); do not just re-ask the user to confirm")
	}
	data, err := os.ReadFile(st.ClientConfig)
	if err != nil {
		return fmt.Errorf("could not read OAuth client %q: %w", st.ClientConfig, err)
	}
	return os.WriteFile(filepath.Join(configDir, "client_secret.json"), data, 0o600)
}

// gwsExtractEmail reads the account email from `gws auth status` after login.
func gwsExtractEmail(ctx context.Context, w *workspace, id string) string {
	run, err := w.runGws(ctx, []string{"auth", "status", "--format", "json"}, id)
	if err != nil || run.ExitCode != 0 {
		return ""
	}
	var probe map[string]any
	if json.Unmarshal([]byte(run.Stdout), &probe) == nil {
		for _, key := range []string{"email", "account", "user"} {
			if v, ok := probe[key].(string); ok && strings.Contains(v, "@") {
				return v
			}
		}
	}
	// Fallback: scan for an email-looking token.
	for _, field := range strings.Fields(run.Stdout) {
		token := strings.Trim(field, `",:{}`)
		if strings.Contains(token, "@") && strings.Contains(token, ".") {
			return token
		}
	}
	return ""
}

func gwsSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastHyphen := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		// Any other character (space, @, ., _, -, ...) collapses to one hyphen.
		if !lastHyphen {
			b.WriteRune('-')
			lastHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func boolArg(args map[string]any, key string) bool {
	b, _ := args[key].(bool)
	return b
}
