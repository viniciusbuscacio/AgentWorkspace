//go:build devunlock

package appcore

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/infrastructure/appconfig"
)

// devLog appends a line to a debug log inside the dev data dir. Wails discards
// stdout, so file logging is the only way to see what the auto-unlock did.
func devLog(format string, args ...any) {
	dir := strings.TrimSpace(os.Getenv("aw_DATA_DIR"))
	if dir == "" {
		dir = os.TempDir()
	}
	line := time.Now().Format("15:04:05.000") + " " + fmt.Sprintf(format, args...) + "\n"
	f, err := os.OpenFile(filepath.Join(dir, "dev-unlock.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

// devUnlockProfileName is the dedicated throwaway profile the dev binary
// bootstraps and unlocks. It is kept separate from any real profile so the dev
// auto-unlock never touches a production vault.
const devUnlockProfileName = "1234"

// devRestPort is the dev instance's REST port — deliberately NOT production's
// default 9301, so a forgotten dev instance never steals the real app's port.
const devRestPort = 9311

// devAutoUnlock is compiled ONLY into the dev binary (build tag `devunlock`).
// The release build links the no-op stub in dev_unlock_stub.go, so production
// can never auto-unlock a vault.
//
// When aw_DEV_UNLOCK is set, it bootstraps a throwaway dev vault/profile named
// "1234" and unlocks it on launch, so the REST and browser surfaces come up
// without a human at the window — closing the build→relaunch→test loop. The
// password comes from aw_DEV_VAULT_PASSWORD, then the aw_DEV_UNLOCK value
// itself, falling back to "1234". For full isolation from production state run
// the dev binary with aw_DATA_DIR pointed at a throwaway directory.
func (a *App) devAutoUnlock() {
	raw, ok := os.LookupEnv("aw_DEV_UNLOCK")
	if !ok {
		return
	}
	devLog("devAutoUnlock invoked (aw_DEV_UNLOCK=%q)", raw)
	password := strings.TrimSpace(os.Getenv("aw_DEV_VAULT_PASSWORD"))
	if password == "" {
		password = strings.TrimSpace(raw)
	}
	if password == "" || password == "1" || strings.EqualFold(password, "true") {
		password = "1234"
	}
	go a.runDevAutoUnlock(password)
}

func (a *App) runDevAutoUnlock(password string) {
	devLog("runDevAutoUnlock start (currentProfileID=%q)", a.currentProfileID)
	result, err := application.EnsureDevVaultUnlocked(a.vault, a.profileManager, application.DevVaultUnlockInput{
		ProfileName:      devUnlockProfileName,
		Password:         password,
		CurrentProfileID: a.currentProfileID,
	})
	if err != nil {
		devLog("dev vault unlock failed: %v", err)
		return
	}
	a.currentProfileID = result.CurrentProfileID
	devLog("dev vault profile id=%q name=%q vaultDir=%q created=%v", result.Profile.ID, result.Profile.Name, result.Profile.VaultDir, result.Created)
	res := a.UnlockVault(password)
	if !res.Success {
		devLog("post-unlock startup failed: %s", res.Error)
		return
	}
	devLog("vault %q unlocked OK", devUnlockProfileName)
	a.devSeedPermitAllSandbox()
	a.devEnableRestServer()
	a.emitChatEvent("vault:dev-unlocked", map[string]any{"profile": devUnlockProfileName})
}

// devEnableRestServer turns the REST surface on for the dev instance so an
// automated harness can drive the app without a human in Settings. The token
// comes from aw_DEV_REST_TOKEN (fallback "dev-1234" — loopback only, throwaway
// vault). The Agent Firewall still applies: the harness pre-seeds a loopback
// PERMIT rule in the throwaway data dir's config.json.
func (a *App) devEnableRestServer() {
	// The Agent Firewall denies all by default, so a fresh throwaway data dir
	// would block the REST listener (ErrNoPermittedInterface — startRestServer
	// returns nil but binds nothing). Seed a single loopback PERMIT for REST so
	// the harness can reach 127.0.0.1:9311 without a human in Settings. Loopback
	// only, throwaway vault — never widened to a routable interface.
	if err := a.devSeedLoopbackRestRule(); err != nil {
		devLog("dev firewall seed failed: %v", err)
	}
	token := strings.TrimSpace(os.Getenv("aw_DEV_REST_TOKEN"))
	if token == "" {
		token = "dev-1234"
	}
	if err := application.RestServerSettings.SetToken(a.vault, token); err != nil {
		devLog("dev REST token pin failed: %v", err)
		return
	}
	// Pin the dev REST port away from production's default (9301): a running
	// dev instance must never hold the port the user's real app tries to bind.
	// Override with aw_DEV_REST_PORT.
	port := devRestPort
	if raw := strings.TrimSpace(os.Getenv("aw_DEV_REST_PORT")); raw != "" {
		if parsed, perr := strconv.Atoi(raw); perr == nil {
			port = parsed
		}
	}
	if err := application.RestServerSettings.SetPort(a.vault, port); err != nil {
		devLog("dev REST port pin failed: %v", err)
		return
	}
	if err := application.RestServerSettings.SetAutostart(a.vault, true); err != nil {
		devLog("dev REST autostart failed: %v", err)
		return
	}
	if err := a.startRestServer(); err != nil {
		devLog("dev REST start failed: %v", err)
		return
	}
	devLog("dev REST server up (token from %s)", map[bool]string{true: "aw_DEV_REST_TOKEN", false: "fallback"}[os.Getenv("aw_DEV_REST_TOKEN") != ""])
}

// devSeedPermitAllSandbox loosens the throwaway dev vault to permit_all so the
// harness can exercise real file operations across the whole filesystem (the
// default permit_list blocks everything outside the workspace, which makes
// automated file / shell testing impossible). Dev-only, throwaway
// vault — production keeps whatever the user set.
func (a *App) devSeedPermitAllSandbox() {
	if _, err := application.SetSandboxConfig(a.vault, domain.SandboxConfig{Mode: domain.SandboxPermitAll}); err != nil {
		devLog("dev sandbox permit_all seed failed: %v", err)
		return
	}
	devLog("dev sandbox set to permit_all")
}

// devSeedLoopbackRestRule ensures the persisted firewall policy contains a
// loopback PERMIT for the REST service, without touching any rule the user
// already has. Idempotent — a no-op once the rule is present.
func (a *App) devSeedLoopbackRestRule() error {
	store := appconfig.Store{}
	rules := application.LoadFirewallRules(store)
	want := domain.FirewallRule{
		Action:    domain.FirewallActionPermit,
		Service:   domain.FirewallServiceREST,
		Interface: domain.FirewallIfaceLoopback,
		Origin:    domain.FirewallOriginAny,
	}
	for _, r := range rules {
		if r == want {
			return nil
		}
	}
	return application.SaveFirewallRules(store, append([]domain.FirewallRule{want}, rules...))
}
