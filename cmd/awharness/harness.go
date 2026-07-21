package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/domain/ports"
	"aw/internal/infrastructure/agent"
	"aw/internal/infrastructure/appconfig"
	"aw/internal/infrastructure/browser"
	"aw/internal/infrastructure/externaltaint"
	"aw/internal/infrastructure/profile"
	"aw/internal/infrastructure/restserver"
	"aw/internal/infrastructure/sandbox"
	"aw/internal/infrastructure/subagent"
	"aw/internal/infrastructure/tools"
	vaultpkg "aw/internal/infrastructure/vault"
)

const DefaultAddr = "127.0.0.1:9309"
const DefaultToken = "dev-token"

type Config struct {
	DataDir    string
	Workspace  string
	Addr       string
	Token      string
	Password   string
	Profile    string
	Browser    appconfig.BrowserConfig
	AddedMods  []string
	SelfDev    bool
	GwsBaseDir string
	// Chat enables chat mode: the harness unlocks the user's REAL vault (to
	// inherit the configured provider + API key), builds a model runtime, and
	// exposes POST /api/chat for a full model-driven chat turn.
	Chat bool
	// VaultPassword unlocks the REAL vault in chat mode. It comes from
	// AW_VAULT_PASSWORD (preferred) or the -password flag.
	VaultPassword string
}

type Server struct {
	REST             *restserver.Server
	vault            *vaultpkg.Vault
	Token            string
	DataDir          string
	Workspace        string
	VaultDir         string
	CurrentProfileID string
}

func Start(ctx context.Context, cfg Config) (*Server, error) {
	if strings.TrimSpace(cfg.DataDir) == "" {
		if !cfg.Chat {
			cfg.DataDir = filepath.Join(os.TempDir(), "aw-harness-data")
		}
	}
	if strings.TrimSpace(cfg.DataDir) != "" {
		if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
			return nil, err
		}
		if err := os.Setenv("aw_DATA_DIR", cfg.DataDir); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(cfg.Workspace) == "" {
		if strings.TrimSpace(cfg.DataDir) != "" {
			cfg.Workspace = filepath.Join(cfg.DataDir, "workspace")
		} else {
			cfg.Workspace = filepath.Join(os.TempDir(), "aw-harness-chat-workspace")
		}
	}
	if err := os.MkdirAll(cfg.Workspace, 0o755); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = DefaultAddr
	}
	if strings.TrimSpace(cfg.Token) == "" {
		cfg.Token = DefaultToken
	}
	if strings.TrimSpace(cfg.Password) == "" {
		cfg.Password = application.DefaultDevVaultPassword
	}
	if strings.TrimSpace(cfg.Profile) == "" {
		cfg.Profile = application.DefaultDevVaultProfileName
	}
	pm := profile.NewManager(profile.DefaultBaseDir())
	var vault *vaultpkg.Vault
	var currentProfileID string
	if cfg.Chat {
		// Chat mode unlocks the user's REAL vault so the harness inherits the
		// already-configured provider + API key (mirrors NewApp).
		v, profileID, err := unlockRealVault(pm, cfg.VaultPassword)
		if err != nil {
			return nil, err
		}
		vault = v
		currentProfileID = profileID
	} else {
		v := vaultpkg.New(vaultpkg.DefaultDir())
		unlocked, err := application.EnsureDevVaultUnlocked(v, pm, application.DevVaultUnlockInput{
			ProfileName: cfg.Profile,
			Password:    cfg.Password,
		})
		if err != nil {
			return nil, err
		}
		vault = v
		currentProfileID = unlocked.CurrentProfileID
	}
	browserManager := browser.NewManager(appconfig.BaseDir(), browser.Overrides{
		ChromePath: cfg.Browser.ChromePath,
		EdgePath:   cfg.Browser.EdgePath,
		ChromePort: cfg.Browser.ChromePort,
		EdgePort:   cfg.Browser.EdgePort,
	})
	addedMods := cfg.AddedMods
	if len(addedMods) == 0 {
		addedMods = []string{"logs", domain.BrowserModuleChrome, domain.BrowserModuleEdge}
	}
	taintStore := externaltaint.NewStore()
	opts := tools.Options{
		Root:              cfg.Workspace,
		AutoApprove:       true,
		Surface:           tools.SurfaceExternal,
		TaintStore:        taintStore,
		GwsAccountsDir:    firstNonEmpty(cfg.GwsBaseDir, filepath.Join(appconfig.BaseDir(), "gws-accounts")),
		AddedModulesFn:    func() []string { return append([]string(nil), addedMods...) },
		SandboxPolicyFn:   sandboxPolicyFn(vault, cfg.Workspace, cfg.SelfDev),
		LogActionFn:       logActionFn(vault),
		LogExternalFn:     logExternalFn(vault),
		Browser:           browserFuncs(browserManager, vault),
		Logs:              &tools.LogsFuncs{List: func(_ context.Context, query domain.LogQuery) (any, error) { return application.ListLogs(vault, query) }},
		WebRead:           webReadFuncs(vault),
		ExternalDistillFn: nil,
	}

	restCfg := restserver.Config{Addr: cfg.Addr, Token: cfg.Token, Version: "harness"}
	var runtime *agent.Runtime
	if cfg.Chat {
		// Build the model runtime and wire the generic spawn runner so a full
		// chat turn can fan out parallel workers. The dispatcher (POST /api/aw)
		// shares the same opts/vault so logs/tools/chat share state.
		rt, err := agent.NewRuntime()
		if err != nil {
			return nil, fmt.Errorf("build chat runtime: %w", err)
		}
		if rt == nil {
			return nil, fmt.Errorf("chat runtime is not available")
		}
		runtime = rt
		opts.SpawnFn = subagent.NewSpawnFn(subagent.Deps{
			Runtime:     runtime,
			ModelConfig: harnessModelConfigFn(vault),
			Taint:       taintStore,
		}, func() tools.Options { return opts })
		toolSet, err := tools.New(opts)
		if err != nil {
			return nil, err
		}
		runtime.SetTools(toolSet)
		restCfg.ChatBackend = chatBackend{vault: vault, runtime: runtime, aliases: &chatAliasStore{aliases: map[string]string{}}}
	}

	dispatcher, err := tools.NewDispatcher(opts)
	if err != nil {
		return nil, err
	}
	rest, err := restserver.NewServer(restBackend{dispatcher: dispatcher}, restCfg)
	if err != nil {
		return nil, err
	}
	application.WriteLog(vault, domain.LogEntry{
		Level:     6,
		Source:    "harness",
		Event:     "harness.started",
		Severity:  "info",
		Status:    "ok",
		Message:   "harness started",
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	_ = ctx
	return &Server{
		REST:             rest,
		vault:            vault,
		Token:            cfg.Token,
		DataDir:          cfg.DataDir,
		Workspace:        cfg.Workspace,
		VaultDir:         vault.Dir(),
		CurrentProfileID: currentProfileID,
	}, nil
}

// harnessModelConfigFn resolves the active provider runtime config from the
// unlocked vault into the model config the isolated subagent runs with.
func harnessModelConfigFn(vault *vaultpkg.Vault) func() (domain.ModelConfig, error) {
	return func() (domain.ModelConfig, error) {
		cfg, err := application.ResolveProviderRuntimeConfig(vault)
		if err != nil {
			return domain.ModelConfig{}, err
		}
		return application.ModelConfigFromProviderRuntimeConfig(cfg), nil
	}
}

// unlockRealVault unlocks the user's real vault the same way NewApp does, so the
// harness inherits the configured provider + API key. It fails clearly when the
// password is missing, the unlock fails, or no provider is configured.
func unlockRealVault(pm *profile.Manager, password string) (*vaultpkg.Vault, string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return nil, "", fmt.Errorf("chat mode requires a vault password: set AW_VAULT_PASSWORD or pass -password")
	}
	config := appconfig.Load()
	bootstrap := application.BootstrapProfiles(pm, application.ProfileBootstrapInput{
		ConfigVaultDir:  config.VaultDir,
		DefaultVaultDir: vaultpkg.DefaultDir(),
	})
	vault := vaultpkg.New(bootstrap.VaultDir)
	if err := application.UnlockVaultForProfile(vault, pm, bootstrap.CurrentProfileID, password); err != nil {
		return nil, "", fmt.Errorf("unlock real vault: %w", err)
	}
	if _, err := application.ResolveProviderRuntimeConfig(vault); err != nil {
		_ = vault.Lock()
		return nil, "", fmt.Errorf("real vault has no usable provider configured: %w", err)
	}
	return vault, bootstrap.CurrentProfileID, nil
}

// chatBackend adapts the unlocked vault + runtime to restserver.ChatBackend.
// The *vault.Vault satisfies both the chat conversation store and provider
// secret store ports; *agent.Runtime satisfies ports.ChatRuntime.
type chatBackend struct {
	vault   *vaultpkg.Vault
	runtime *agent.Runtime
	aliases *chatAliasStore
}

type chatAliasStore struct {
	mu      sync.Mutex
	aliases map[string]string
}

func (b chatBackend) RunChat(ctx context.Context, chatID string, text string, _ bool) (restserver.ChatReply, error) {
	actualChatID, err := b.resolveChatID(chatID)
	if err != nil {
		return restserver.ChatReply{}, err
	}
	result, err := application.RunChatMessage(ctx, b.vault, b.vault, b.runtime, application.RunChatMessageInput{
		ChatID: actualChatID,
		Text:   text,
		Stream: false,
	})
	if err != nil {
		return restserver.ChatReply{}, err
	}
	return restserver.ChatReply{
		ChatID:             result.ChatID,
		Reply:              result.Reply.Text,
		AssistantMessageID: result.AssistantMessage.ID,
	}, nil
}

func (b chatBackend) resolveChatID(requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", fmt.Errorf("chatId is required")
	}
	if b.aliases != nil {
		b.aliases.mu.Lock()
		defer b.aliases.mu.Unlock()
	}
	chats, err := application.ListChats(b.vault)
	if err != nil {
		return "", err
	}
	for _, chat := range chats {
		if chat.ID == requested {
			return requested, nil
		}
	}
	if b.aliases != nil {
		if actual := strings.TrimSpace(b.aliases.aliases[requested]); actual != "" {
			for _, chat := range chats {
				if chat.ID == actual {
					return actual, nil
				}
			}
		}
	}
	chat, err := application.CreateChat(b.vault, "Harness "+requested)
	if err != nil {
		return "", err
	}
	if b.aliases != nil {
		b.aliases.aliases[requested] = chat.ID
	}
	return chat.ID, nil
}

func (s *Server) Shutdown(ctx context.Context) {
	if s != nil && s.REST != nil {
		s.REST.Shutdown(ctx)
	}
	if s != nil && s.vault != nil {
		_ = s.vault.Lock()
	}
}

type restBackend struct {
	dispatcher *tools.Dispatcher
}

func (b restBackend) CallAw(ctx context.Context, action string, argsJSON string) (string, error) {
	return b.dispatcher.Call(ctx, action, argsJSON)
}

func (b restBackend) AwDescription() string {
	return b.dispatcher.Description()
}

func sandboxPolicyFn(vault *vaultpkg.Vault, workspaceRoot string, selfDev bool) func() sandbox.Policy {
	return func() sandbox.Policy {
		runtime := application.LoadSandboxRuntime(vault)
		config := runtime.Config
		if selfDev {
			config = domain.SandboxConfig{Mode: domain.SandboxPermitAll}
		}
		return sandbox.Policy{
			Config:        config,
			WorkspaceRoot: workspaceRoot,
			DataDir:       appconfig.BaseDir(),
			Protected:     []string{runtime.VaultDir},
		}
	}
}

func browserFuncs(manager *browser.Manager, store ports.SecretStore) *tools.BrowserFuncs {
	_ = store // the restart-to-attach setting died with the personal profile mode
	return &tools.BrowserFuncs{
		Start: func(ctx context.Context, id string, opts domain.BrowserStartOptions) (any, error) {
			return application.StartBrowser(ctx, manager, id, opts)
		},
		Stop: func(ctx context.Context, id string) (any, error) {
			return application.StopBrowser(ctx, manager, id)
		},
		Status: func(ctx context.Context, id string) (any, error) {
			return application.BrowserStatus(ctx, manager, id)
		},
		Tabs: func(ctx context.Context, id string) (any, error) {
			return application.BrowserTabs(ctx, manager, id)
		},
		Command: func(ctx context.Context, id string, command string, params map[string]any) (any, error) {
			return application.RunBrowserCommand(ctx, manager, id, command, params)
		},
	}
}

func webReadFuncs(vault *vaultpkg.Vault) *tools.WebReadFuncs {
	return &tools.WebReadFuncs{
		Save: func(_ context.Context, input tools.WebReadSaveInput) (any, error) {
			return application.SaveWebObservation(vault, application.SaveWebObservationInput{
				ChatID:  input.ChatID,
				RunID:   input.RunID,
				Browser: input.Browser,
				TabID:   input.TabID,
				SiteKey: input.SiteKey,
				URL:     input.URL,
				Title:   input.Title,
				Payload: input.Payload,
			})
		},
		Latest: func(_ context.Context, chatID, browser, siteKey, rawURL string) (any, bool, error) {
			return application.LatestWebObservation(vault, chatID, browser, siteKey, rawURL)
		},
	}
}

func logActionFn(vault *vaultpkg.Vault) func(context.Context, tools.ActionLogEvent) {
	return func(_ context.Context, event tools.ActionLogEvent) {
		if strings.HasPrefix(event.Action, "logs.") {
			return
		}
		attrs := map[string]any{
			"action":      event.Action,
			"output.size": event.OutputSize,
		}
		for key, value := range event.Attributes {
			attrs[key] = value
		}
		attrsJSON, _ := json.Marshal(attrs)
		level, severity := levelForStatus(event.Status, event.Event)
		errText := ""
		if event.Err != nil {
			errText = event.Err.Error()
		}
		application.WriteLog(vault, domain.LogEntry{
			Level:          level,
			Source:         "tool",
			Event:          event.Event,
			Severity:       severity,
			Status:         event.Status,
			DurationMs:     event.DurationMs,
			Message:        strings.ReplaceAll(event.Event, ".", " "),
			ErrorMessage:   errText,
			AttributesJSON: string(attrsJSON),
			CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
}

func logExternalFn(vault *vaultpkg.Vault) func(context.Context, tools.ExternalLogEvent) {
	return func(_ context.Context, event tools.ExternalLogEvent) {
		attrs := map[string]any{
			"channel":      event.SourceType,
			"origin":       event.Origin,
			"mode":         string(event.Mode),
			"input.chars":  event.InputChars,
			"output.chars": event.OutputChars,
			"risk":         event.RiskLevel,
			"suspicious":   event.Suspicious,
			"distilled":    event.Distilled,
			"warnings":     event.Warnings,
		}
		for key, value := range event.Attributes {
			attrs[key] = value
		}
		attrsJSON, _ := json.Marshal(attrs)
		level := 6
		severity := "info"
		if event.Err != nil || event.Suspicious || event.RiskLevel == domain.ExternalRiskHigh {
			level = 4
			severity = "warn"
		}
		errText := ""
		if event.Err != nil {
			errText = event.Err.Error()
		}
		application.WriteLog(vault, domain.LogEntry{
			Level:          level,
			Source:         "security",
			Event:          event.Event,
			Severity:       severity,
			Status:         event.Status,
			DurationMs:     event.DurationMs,
			Message:        strings.ReplaceAll(event.Event, ".", " "),
			ErrorMessage:   errText,
			AttributesJSON: string(attrsJSON),
			CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
}

func levelForStatus(status string, event string) (int, string) {
	if status == "error" {
		return 3, "error"
	}
	if status == "blocked" || status == "denied" || status == "timeout" {
		return 4, "warn"
	}
	if event == "tool.call.started" {
		return 7, "debug"
	}
	return 6, "info"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *Server) Info() map[string]any {
	return map[string]any{
		"url":              s.REST.URL(),
		"token":            s.Token,
		"dataDir":          s.DataDir,
		"workspace":        s.Workspace,
		"vaultDir":         s.VaultDir,
		"currentProfileId": s.CurrentProfileID,
	}
}

func (s *Server) String() string {
	return fmt.Sprintf("%s token=%s data=%s", s.REST.URL(), s.Token, s.DataDir)
}
