package appcore

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/domain/ports"
	"aw/internal/dto"
	"aw/internal/infrastructure/agent"
	"aw/internal/infrastructure/appconfig"
	"aw/internal/infrastructure/attachmentsafe"
	"aw/internal/infrastructure/autolock"
	browserpkg "aw/internal/infrastructure/browser"
	"aw/internal/infrastructure/diagnostics"
	"aw/internal/infrastructure/externaltaint"
	"aw/internal/infrastructure/mcpclient"
	"aw/internal/infrastructure/mcpserver"
	"aw/internal/infrastructure/oauth"
	obsidianpkg "aw/internal/infrastructure/obsidian"
	"aw/internal/infrastructure/pip"
	"aw/internal/infrastructure/profile"
	"aw/internal/infrastructure/providers"
	"aw/internal/infrastructure/restserver"
	"aw/internal/infrastructure/sandbox"
	"aw/internal/infrastructure/selfdev"
	"aw/internal/infrastructure/servertls"
	"aw/internal/infrastructure/tools"
	vaultpkg "aw/internal/infrastructure/vault"
	"aw/internal/infrastructure/webserver"
	"aw/skills"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx              context.Context
	agent            *agent.Runtime
	agentErr         error
	vault            *vaultpkg.Vault
	profileManager   *profile.Manager
	currentProfileID string
	chatRunner       application.ChatRunCoordinator
	planModesMu      sync.Mutex
	planModes        map[string]bool
	chatUsage        *application.ChatUsageTracker
	spawnRecords     *spawnRecorder
	confirmations    application.ConfirmationCoordinator
	workspaceRoot    string
	autoLock         ports.AutoLockPolicy
	// writeBackFailures counts consecutive working-copy snapshot failures
	// (vaultWriteBackLoop goroutine only).
	writeBackFailures int
	// workspace is the per-vault UI/workspace state store (workspace.json in
	// the current vault's master folder — follows profile switches).
	workspace      appconfig.WorkspaceStore
	partialMu      sync.Mutex
	partialReplies map[string]*strings.Builder
	pipMu          sync.Mutex
	pipServer      *pip.Server
	pipHub         *pip.Hub
	mcpMu          sync.Mutex
	mcpServer      *mcpserver.Server
	mcpErr         string
	// mcpFirewallBlocked records that a wanted start was refused because no
	// firewall rule permits the service; a later rule change or interface
	// arrival retries the start. Any stop clears the want. Guarded by mcpMu.
	mcpFirewallBlocked bool
	// mcpRetrying marks a background auto-start retry loop in flight (bind
	// failures during a restart handoff). Guarded by mcpMu.
	mcpRetrying bool
	restMu      sync.Mutex
	restServer  *restserver.Server
	restErr     string
	// restFirewallBlocked mirrors mcpFirewallBlocked for the REST server.
	// Guarded by restMu.
	restFirewallBlocked bool
	// restRetrying mirrors mcpRetrying for the REST server. Guarded by restMu.
	restRetrying    bool
	webMu           sync.Mutex
	webServer       *webserver.Server
	webErr          string
	webHub          *webserver.Hub
	webSessionEpoch int64
	// webAssetsFS is the embedded frontend served in web mode, injected by the
	// host (the GUI shell's embed, or awd's). The package itself cannot embed it
	// (go:embed is relative to the embedding file's dir), so it is passed in.
	webAssetsFS      fs.FS
	awDispatch       *tools.Dispatcher
	browser          ports.BrowserAutomation
	voiceCapture     ports.VoiceCapture
	voiceTranscriber ports.VoiceTranscriber
	uiStateMu        sync.Mutex
	uiState          map[string]any
	uiCmdMu          sync.Mutex
	uiCommands       map[string]chan uiCommandResult
	pendingLogMu     sync.Mutex
	pendingLogEvents []application.LogEvent
	notifier         ports.Notifier
	// cooldown is the in-memory circuit breaker for the LLM fallback chain.
	cooldown *application.ProviderCooldown
	// taintStore is the shared per-turn external-content taint, wired into both
	// the tools layer and the attachment path so attachment content gates tool
	// actions in the same turn.
	taintStore *externaltaint.Store
	// tlsMaterial is the shared server-TLS certificate store (on-disk bundle per
	// mode). It is the only component that touches the private key; the servers
	// receive a ready *tls.Config and everyone else a sanitized status.
	tlsMaterial ports.TLSMaterialStore
}

type VaultStatusResponse = dto.VaultStatusResponse
type OperationResult = dto.OperationResult
type FolderDialogResult = dto.FolderDialogResult
type SecretResult = dto.SecretResult
type ChatSendResult = dto.ChatSendResult
type ChatOperationResult = dto.ChatOperationResult
type AudioTranscriptionResult = dto.AudioTranscriptionResult
type VoiceCaptureResult = dto.VoiceCaptureResult
type ChatSessionInfo = dto.ChatSessionInfo
type PlanModeResult = dto.PlanModeResult
type ChatModelResult = dto.ChatModelResult

const (
	autoRenameTimeout = 20 * time.Second
)

// NewApp creates a new App application struct
func NewApp() *App {
	runtime, err := agent.NewRuntime()
	config := appconfig.Load()
	profileManager := profile.NewManager(profile.DefaultBaseDir())
	profileBootstrap := application.BootstrapProfiles(profileManager, application.ProfileBootstrapInput{
		ConfigVaultDir:  config.VaultDir,
		DefaultVaultDir: vaultpkg.DefaultDir(),
	})
	app := &App{
		agent:            runtime,
		agentErr:         err,
		vault:            vaultpkg.New(profileBootstrap.VaultDir),
		profileManager:   profileManager,
		currentProfileID: profileBootstrap.CurrentProfileID,
		planModes:        map[string]bool{},
		chatUsage:        application.NewChatUsageTracker(),
		spawnRecords:     newSpawnRecorder(),
		workspaceRoot:    filepath.Join(appconfig.BaseDir(), "workspace"),
		autoLock:         autolock.New(appconfig.Store{}.LoadAutoLockMinutes()),
		pipHub:           pip.NewHub(),
		webHub:           webserver.NewHub(),
		browser: browserpkg.NewManager(appconfig.BaseDir(), browserpkg.Overrides{
			ChromePath: config.Browser.ChromePath,
			EdgePath:   config.Browser.EdgePath,
			ChromePort: config.Browser.ChromePort,
			EdgePort:   config.Browser.EdgePort,
		}),
		voiceCapture:     agent.NewFfmpegVoiceCapture("."),
		voiceTranscriber: agent.NewVoiceTranscriber(),
		cooldown:         application.NewProviderCooldown(time.Duration(appconfig.Store{}.LoadProviderCooldownMinutes()) * time.Minute),
		taintStore:       externaltaint.NewStore(),
		tlsMaterial:      servertls.New(appconfig.BaseDir()),
	}
	// The bootstrap profile's working copy must be wired before the first
	// unlock — same rule applyCurrentProfileID enforces on every later switch.
	application.ConfigureVaultWorkDir(app.vault, profileBootstrap.CurrentProfileID)
	// Per-vault workspace state follows whichever vault is selected.
	app.workspace = appconfig.WorkspaceStore{VaultDir: func() string { return application.VaultDirOf(app.vault) }}
	// One notification system, in-app only (user decision 2026-07-09): the
	// Notifier port emits an app:notify event that the shell shows as the
	// standard toast, instead of a macOS notification.
	app.notifier = inAppNotifier{app: app}
	toolOpts, instructionExtra, toolErr := app.toolOptions(config)
	if toolErr == nil {
		// The dispatcher feeds the MCP and REST servers the same aw registry
		// the agent uses. External clients authenticate with a bearer token and
		// run headless, so they auto-approve mutating actions instead of routing
		// through the in-app human confirmation prompt (which would hang a
		// caller with no one at the aw window). The in-app agent below keeps
		// its confirmation flow.
		dispatcherOpts := toolOpts
		dispatcherOpts.AutoApprove = true
		dispatcherOpts.Surface = tools.SurfaceExternal
		// External callers keep the surface they always had: shell over
		// MCP/REST only in self-dev. The in-app agent's always-on shell
		// (AllowShell in every chat) is a chat capability, not an API one.
		dispatcherOpts.AllowShell = toolOpts.SelfManage && toolOpts.AllowShell
		if dispatcher, err := tools.NewDispatcher(dispatcherOpts); err == nil {
			app.awDispatch = dispatcher
		}
	}
	if runtime != nil {
		runtime.SetPromptDebugSnapshotSink(app.emitPromptDebugSnapshot)
		if toolErr != nil {
			app.agentErr = toolErr
		} else {
			toolSet, toolErr := tools.New(toolOpts)
			if toolErr != nil {
				app.agentErr = toolErr
			} else {
				runtime.SetTools(toolSet)
				runtime.SetInstructionExtra(instructionExtra)
			}
		}
	}
	return app
}

// attachmentReader builds the safe attachment reader (isolated OCR + pure-Go
// PDF + externalsafe). Returns nil when no runtime is available, so chat falls
// back to attachment metadata only. The bound TranscribeImage is dependency
// wiring (a value), not a direct adapter call.
func (a *App) attachmentReader() ports.SafeAttachmentReader {
	if a.agent == nil {
		return nil
	}
	return attachmentsafe.New(a.agent.TranscribeImage, a.agent.DistillExternalContent)
}

// obsidianContextBlock renders the Obsidian always-read notes for the agent
// context (empty while the module is off or unset).
func (a *App) obsidianContextBlock() string {
	return application.ObsidianContextBlock(a.workspace, domain.ModuleCatalog(), appconfig.Store{}, obsidianpkg.New())
}

func (a *App) toolOptions(config appconfig.Config) (tools.Options, string, error) {
	opts := tools.Options{
		Root:    a.workspaceRoot,
		Confirm: a.confirmToolAction,
		// Always-on app control: the aw tool can drive the running app (theme,
		// navigation, chats, providers) in every chat, not just self-dev.
		Control: appControl{app: a},
	}
	// Always-on chat memory: the recall tools work in every chat, not just
	// self-dev. They delegate to application use cases against the vault.
	opts.ChatSearchFn = func(_ context.Context, query string, limit int) (any, error) {
		return application.SearchChatHistory(a.vault, query, limit)
	}
	opts.ChatHistoryFn = func(_ context.Context, sessionID string) (any, error) {
		return application.GetSessionHistory(a.vault, sessionID)
	}
	opts.ChatCatalogFn = func(_ context.Context) (any, error) {
		return application.GetSessionCatalog(a.vault, "", 0)
	}
	// Always-on user memory: the agent appends lines to the living document
	// about the user; the document is injected into future prompts. The
	// memory:changed event keeps an open Settings → Memory panel in sync so
	// its Save doesn't clobber what the agent just recorded.
	opts.UserMemoryRecordFn = func(_ context.Context, key, category, content string) (any, error) {
		_, err := application.AppendUserMemoryLine(a.vault, key, category, content)
		if err != nil {
			return nil, err
		}
		a.emitChatEvent("memory:changed", map[string]any{})
		return map[string]any{"remembered": true, "key": application.NormalizeUserFactKey(key)}, nil
	}
	opts.UserMemoryForgetFn = func(_ context.Context, key string) (any, error) {
		removed, err := application.RemoveUserMemoryLine(a.vault, key)
		if err != nil {
			return nil, err
		}
		a.emitChatEvent("memory:changed", map[string]any{})
		return map[string]any{"forgotten": removed > 0, "removedLines": removed, "key": application.NormalizeUserFactKey(key)}, nil
	}
	// Progressive disclosure: the system prompt advertises only skill triggers;
	// these let the agent list skills and pull a skill's body/files on demand.
	opts.SkillCatalogFn = func(_ context.Context) (any, error) {
		seeds, _ := skills.BundledSkills()
		return application.SkillCatalog(a.vault, a.skillsPromptInput(), seeds), nil
	}
	opts.SkillReadFn = func(_ context.Context, id, path string) (any, error) {
		content, err := application.ReadSkillFile(a.vault, a.skillsPromptInput(), id, path)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(path) == "" {
			path = domain.SkillMarkdownPath
		}
		result := map[string]any{"id": id, "path": path, "content": content}
		// Staleness travels with the body too: the gws guidance sends the agent
		// straight to skill.read, so it may never see skill.list's flags.
		seeds, _ := skills.BundledSkills()
		for _, s := range application.SkillCatalog(a.vault, a.skillsPromptInput(), seeds) {
			if s.ID == id && s.UpdateAvailable {
				result["updateAvailable"] = true
				result["notice"] = s.Guidance
			}
		}
		return result, nil
	}
	// Skill write actions (create/save/import/delete/...). Inside the unlocked
	// vault the agent manages its own skills with no extra confirmation.
	opts.SkillManage = a.skillManageFuncs()
	opts.LogActionFn = func(ctx context.Context, event tools.ActionLogEvent) {
		severity := "info"
		if event.Status == "error" {
			severity = "error"
		} else if event.Status == "blocked" || event.Status == "denied" || event.Status == "timeout" {
			severity = "warn"
		} else if event.Event == "tool.call.started" {
			severity = "debug"
		}
		errorText := ""
		if event.Err != nil {
			errorText = event.Err.Error()
		}
		attrs := map[string]any{
			"tool.name":   "aw",
			"action":      event.Action,
			"output.size": event.OutputSize,
		}
		for key, value := range event.Attributes {
			attrs[key] = value
		}
		a.emitLogEvent(ctx, application.LogEvent{
			Event:        event.Event,
			Severity:     severity,
			Source:       "tool",
			Message:      strings.ReplaceAll(event.Event, ".", " "),
			Status:       event.Status,
			DurationMs:   event.DurationMs,
			ErrorMessage: errorText,
			Attributes:   attrs,
		})
	}
	opts.LogExternalFn = func(ctx context.Context, event tools.ExternalLogEvent) {
		severity := "info"
		if event.Err != nil {
			severity = "warn"
		}
		if event.Suspicious || event.RiskLevel == domain.ExternalRiskHigh {
			severity = "warn"
		}
		errorText := ""
		if event.Err != nil {
			errorText = event.Err.Error()
		}
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
		a.emitLogEvent(ctx, application.LogEvent{
			Event:        event.Event,
			Severity:     severity,
			Source:       "security",
			Message:      strings.ReplaceAll(event.Event, ".", " "),
			Status:       event.Status,
			DurationMs:   event.DurationMs,
			ErrorMessage: errorText,
			Attributes:   attrs,
		})
	}
	// Visual OCR: wire the isolated, capability-less transcription so
	// visual.read_safe can convert images to untrusted text using the active
	// model. Injected as a bound method (dependency wiring, not a direct call).
	if a.agent != nil {
		opts.VisualExtractFn = a.agent.TranscribeActiveImage
		opts.ExternalDistillFn = a.agent.DistillActiveExternalContent
	}
	// Share the per-turn taint store so attachment content (recorded by the chat
	// path) gates tool actions in the same turn.
	opts.TaintStore = a.taintStore
	// Workspace modules: consulted per aw dispatch, so add/remove toggles a
	// module's action group without restarting the app.
	opts.AddedModulesFn = func() []string {
		ids, err := application.AddedModuleIDs(a.workspace, domain.ModuleCatalog())
		if err != nil {
			return nil
		}
		return ids
	}
	// moduleDataNotifier announces an agent-side data mutation so an open
	// module view can refresh live (the UI refreshes itself after its own
	// calls, but has no other way to learn the agent changed the vault behind
	// it). Wraps a use-case result: emits only on success.
	moduleDataNotifier := func(event string) func(any, error) (any, error) {
		return func(result any, err error) (any, error) {
			if err == nil {
				a.emitChatEvent(event, map[string]any{})
			}
			return result, err
		}
	}
	notesChanged := moduleDataNotifier("notes:changed")
	tasksChanged := moduleDataNotifier("tasks:changed")
	// Notes module: vault-backed operations behind injected funcs; the
	// dispatcher registers notes.* only while the module is added.
	opts.Notes = &tools.NotesFuncs{
		List: func(_ context.Context, includeArchived bool) (any, error) {
			return application.ListNotes(a.vault, includeArchived)
		},
		Get: func(_ context.Context, id string) (any, error) {
			return application.GetNote(a.vault, id)
		},
		Create: func(_ context.Context, title, content string, inPrompt bool) (any, error) {
			return notesChanged(application.CreateNote(a.vault, title, content, inPrompt))
		},
		Update: func(_ context.Context, id string, title, content *string, pinned, archived, inPrompt *bool) (any, error) {
			return notesChanged(application.UpdateNote(a.vault, id, title, content, pinned, archived, inPrompt))
		},
		Delete: func(_ context.Context, id string) (any, error) {
			return notesChanged(application.DeleteNote(a.vault, id))
		},
	}
	// Obsidian module: vault-folder-jailed file operations. Config is read
	// per call so Settings changes (folder, Write toggle) apply instantly;
	// write/append fail closed while the Write toggle is off.
	opts.Obsidian = &tools.ObsidianFuncs{
		List: func(_ context.Context, folder string) (any, error) {
			return application.ObsidianList(appconfig.Store{}, obsidianpkg.New(), folder)
		},
		Search: func(_ context.Context, query string, limit int) (any, error) {
			return application.ObsidianSearch(appconfig.Store{}, obsidianpkg.New(), query, limit)
		},
		Read: func(_ context.Context, path string) (string, error) {
			return application.ObsidianRead(appconfig.Store{}, obsidianpkg.New(), path)
		},
		Write: func(_ context.Context, path, content string) error {
			return application.ObsidianWrite(appconfig.Store{}, obsidianpkg.New(), path, content)
		},
		Append: func(_ context.Context, path, content string) error {
			return application.ObsidianAppend(appconfig.Store{}, obsidianpkg.New(), path, content)
		},
		Delete: func(_ context.Context, path string) error {
			return application.ObsidianDelete(appconfig.Store{}, obsidianpkg.New(), path)
		},
	}
	// Tasks module: same injected-funcs pattern as Notes.
	opts.Tasks = &tools.TasksFuncs{
		List: func(_ context.Context) (any, error) {
			return application.ListTasksItems(a.vault)
		},
		Get: func(_ context.Context, id string) (any, error) {
			return application.GetTasksItem(a.vault, id)
		},
		Add: func(_ context.Context, title, body, status string) (any, error) {
			return tasksChanged(application.CreateTasksItem(a.vault, title, body, status))
		},
		Update: func(_ context.Context, id, title string, body *string, status string, position *int) (any, error) {
			return tasksChanged(application.UpdateTasksItem(a.vault, id, title, body, status, position))
		},
		Delete: func(_ context.Context, id string) (any, error) {
			return tasksChanged(application.DeleteTasksItem(a.vault, id))
		},
	}
	// Passwords module: shared-with-the-agent by design (the user's private
	// credentials live in their own manager). List is metadata-only; Get is
	// the single value-revealing path and is taint-gated at the handler.
	opts.Passwords = &tools.PasswordsFuncs{
		List: func(_ context.Context) (any, error) {
			return application.ListPasswordSummaries(a.vault)
		},
		Get: func(_ context.Context, idOrName string) (any, error) {
			return application.GetPassword(a.vault, idOrName)
		},
	}
	opts.Logs = &tools.LogsFuncs{
		List: func(_ context.Context, query domain.LogQuery) (any, error) {
			return application.ListLogs(a.vault, query)
		},
	}
	opts.WebRead = &tools.WebReadFuncs{
		Save: func(ctx context.Context, input tools.WebReadSaveInput) (any, error) {
			view, err := application.SaveWebObservation(a.vault, application.SaveWebObservationInput{
				ChatID:  input.ChatID,
				RunID:   input.RunID,
				Browser: input.Browser,
				TabID:   input.TabID,
				SiteKey: input.SiteKey,
				URL:     input.URL,
				Title:   input.Title,
				Payload: input.Payload,
			})
			if err == nil {
				a.emitLogEvent(ctx, application.LogEvent{
					Event:     "web_read.observation.saved",
					Severity:  "info",
					Source:    "tool",
					SessionID: view.ChatID,
					TraceID:   view.RunID,
					Message:   "web read observation saved",
					Status:    "ok",
					Attributes: map[string]any{
						"observation.id": view.ID,
						"browser":        view.Browser,
						"site_key.hash":  application.HashForLog(view.SiteKey),
						"url.hash":       application.HashForLog(view.URL),
						"title.hash":     application.HashForLog(view.Title),
					},
				})
			}
			return view, err
		},
		Latest: func(ctx context.Context, chatID, browser, siteKey, rawURL string) (any, bool, error) {
			view, ok, err := application.LatestWebObservation(a.vault, chatID, browser, siteKey, rawURL)
			status := "miss"
			attrs := map[string]any{
				"browser":       strings.TrimSpace(browser),
				"site_key.hash": application.HashForLog(siteKey),
				"url.hash":      application.HashForLog(rawURL),
			}
			if ok {
				status = "hit"
				attrs["observation.id"] = view.ID
				attrs["browser"] = view.Browser
				attrs["site_key.hash"] = application.HashForLog(view.SiteKey)
			}
			a.emitLogEvent(ctx, application.LogEvent{
				Event:      "web_read.observation.latest",
				Severity:   "info",
				Source:     "tool",
				SessionID:  chatID,
				TraceID:    domain.ExternalTaintScope(ctx),
				Message:    "web read observation lookup",
				Status:     status,
				Attributes: attrs,
			})
			return view, ok, err
		},
	}
	// Native system diagnostics (product capability, not self-dev): one read-only
	// OS probe behind the application service, which owns defaults, timeouts,
	// caps, redaction, and the Permissions policy gate. diagnostics.capabilities
	// answers in every mode; detailed actions are blocked in block_all.
	diagProbe := diagnostics.New()
	opts.Diagnostics = &tools.DiagnosticsFuncs{
		Capabilities: func(ctx context.Context, mode domain.SandboxMode) (any, error) {
			return application.DiagnosticsCapabilities(ctx, diagProbe, mode), nil
		},
		Summary: func(ctx context.Context, mode domain.SandboxMode, o domain.DiagnosticsSummaryOptions) (any, error) {
			return application.DiagnosticsSummary(ctx, diagProbe, mode, o)
		},
		Report: func(ctx context.Context, mode domain.SandboxMode, o domain.DiagnosticsReportOptions) (any, error) {
			return application.DiagnosticsReport(ctx, diagProbe, mode, o)
		},
		Sensors: func(ctx context.Context, mode domain.SandboxMode, o domain.DiagnosticsSensorsOptions) (any, error) {
			return application.DiagnosticsSensors(ctx, diagProbe, mode, o)
		},
		Storage: func(ctx context.Context, mode domain.SandboxMode, o domain.DiagnosticsStorageOptions) (any, error) {
			return application.DiagnosticsStorage(ctx, diagProbe, mode, o)
		},
		Processes: func(ctx context.Context, mode domain.SandboxMode, o domain.DiagnosticsProcessesOptions) (any, error) {
			return application.DiagnosticsProcesses(ctx, diagProbe, mode, o)
		},
		Devices: func(ctx context.Context, mode domain.SandboxMode, o domain.DiagnosticsDevicesOptions) (any, error) {
			return application.DiagnosticsDevices(ctx, diagProbe, mode, o)
		},
		Logs: func(ctx context.Context, mode domain.SandboxMode, o domain.DiagnosticsLogsOptions) (any, error) {
			return application.DiagnosticsLogs(ctx, diagProbe, mode, o)
		},
		LogsSummary: func(ctx context.Context, mode domain.SandboxMode, o domain.DiagnosticsLogsSummaryOptions) (any, error) {
			return application.DiagnosticsLogsSummary(ctx, diagProbe, mode, o)
		},
	}
	// Agent Instructions (product capability): vault-backed instruction documents
	// (AGENTS.md, USER.md) behind the application use cases. Mutations refresh the
	// runtime context live, using the same composer the runtime injects.
	opts.Instructions = a.instructionFuncs()
	// MCP Client module: connect AW to external MCP servers as a client. The
	// vault is both the connection store and the credential store; the SDK adapter
	// is the per-operation runtime. Actions register only while the module is
	// added (module-scoped fencing in awRegistry).
	mcpRuntime := mcpclient.New("agent-workspace")
	opts.McpConnections = &tools.McpConnectionFuncs{
		List: func(_ context.Context) (any, error) {
			return application.ListMcpConnectionsUC(a.vault)
		},
		Get: func(_ context.Context, id string) (any, error) {
			return application.GetMcpConnectionUC(a.vault, id)
		},
		Add: func(_ context.Context, in domain.McpConnectionInput, token string) (any, error) {
			return application.AddMcpConnectionUC(a.vault, a.vault, in, token)
		},
		Update: func(_ context.Context, id string, patch domain.McpConnectionPatch, token *string, clearToken bool) (any, error) {
			return application.UpdateMcpConnectionUC(a.vault, a.vault, id, patch, token, clearToken)
		},
		Remove: func(_ context.Context, id string) (any, error) {
			return application.RemoveMcpConnectionUC(a.vault, a.vault, id)
		},
		SetEnabled: func(_ context.Context, id string, enabled bool) (any, error) {
			return application.SetMcpConnectionEnabledUC(a.vault, id, enabled)
		},
		Test: func(ctx context.Context, id string) (any, error) {
			return application.TestMcpConnectionUC(ctx, a.vault, a.vault, mcpRuntime, id)
		},
		ListTools: func(ctx context.Context, id string) (domain.McpToolListResult, error) {
			return application.ListMcpToolsUC(ctx, a.vault, a.vault, mcpRuntime, id)
		},
		CallTool: func(ctx context.Context, id, tool string, arguments map[string]any) (domain.McpToolCallResult, error) {
			return application.CallMcpToolUC(ctx, a.vault, a.vault, mcpRuntime, id, tool, arguments)
		},
	}
	// Agent Browser modules: the CDP manager behind injected funcs; the
	// dispatcher registers browser.* only while a browser module is added.
	opts.Browser = &tools.BrowserFuncs{
		Start: func(ctx context.Context, id string, startOpts domain.BrowserStartOptions) (any, error) {
			return application.StartBrowser(ctx, a.browser, id, startOpts)
		},
		Stop: func(ctx context.Context, id string) (any, error) {
			return application.StopBrowser(ctx, a.browser, id)
		},
		Status: func(ctx context.Context, id string) (any, error) {
			return application.BrowserStatus(ctx, a.browser, id)
		},
		Tabs: func(ctx context.Context, id string) (any, error) {
			return application.BrowserTabs(ctx, a.browser, id)
		},
		Command: func(ctx context.Context, id string, command string, params map[string]any) (any, error) {
			return application.RunBrowserCommand(ctx, a.browser, id, command, params)
		},
	}
	// Always-on raw UI automation: ui.* actions drive the frontend via a
	// ui:command round-trip, available in every chat and to external clients.
	opts.UIAutomationFn = a.uiAutomation
	// Tool-captured images (the agent's own screenshot) reach the user through
	// the inline-image chat side-channel instead of the model's tool result.
	opts.InlineImageFn = a.emitInlineChatImage
	// Google Workspace multi-account storage: isolated gws config dirs +
	// registry under the app config dir.
	opts.GwsAccountsDir = filepath.Join(appconfig.BaseDir(), "gws-accounts")
	if a.agent != nil {
		opts.SpawnFn = a.newSpawnFn(func() tools.Options { return opts })
	}
	// The agent workspace authorizes through the Permissions sandbox (block_all/
	// permit_list/deny_list/permit_all), enforced by SandboxPolicyFn on every
	// dispatch. The per-action "tool:confirm" popup is redundant with that fence
	// and would stall any headless/automated run, so the workspace auto-approves:
	// whatever Permissions allows, the agent does — no second prompt.
	opts.AutoApprove = true
	if !config.SelfDev.Enabled {
		// The main agent gets shell in every chat — the Permissions sandbox is
		// the fence on each call (block_all refuses shell outright), so tool
		// absence must not be a second, cruder fence: it only produced dead
		// ends like "nobody can install the gcloud CLI".
		opts.AllowShell = true
		opts.SandboxPolicyFn = a.sandboxPolicyFn(opts.Root, false)
		return opts, "", nil
	}

	repoRoot, err := selfdev.ResolveRepoRoot(config.SelfDev.RepoRoot)
	if err != nil {
		return opts, "", err
	}
	opts.Root = repoRoot
	opts.AllowShell = config.SelfDev.EffectiveAllowShell()
	opts.SelfManage = true
	opts.StateFn = a.selfDevState
	// Self-dev is the sandbox in permit_all with the repo as workspace (it
	// replaced the old Unconfined bypass) — the built-in denies hold there too.
	opts.SandboxPolicyFn = a.sandboxPolicyFn(repoRoot, true)
	return opts, application.SelfDevInstruction(repoRoot, opts.AllowShell, opts.SelfManage), nil
}

// sandboxPromptInput assembles what the "### Filesystem Access" prompt block
// needs, mirroring the enforcement policy: self-dev renders as permit_all
// with the repo as workspace (Decision 9).
func (a *App) sandboxPromptInput() application.SandboxPromptInput {
	input := application.SandboxPromptInput{WorkspaceRoot: a.workspaceRoot}
	if a.vault != nil {
		input.Store = a.vault
	}
	config := appconfig.Load()
	if config.SelfDev.Enabled {
		if repoRoot, err := selfdev.ResolveRepoRoot(config.SelfDev.RepoRoot); err == nil {
			input.SelfDev = true
			input.WorkspaceRoot = repoRoot
		}
	}
	return input
}

// sandboxPolicyFn builds the per-dispatch permissions policy: the vault
// config (fail-closed to the permit_list default while locked), the agent's
// workspace root, and the paths the fence always protects — the aw data dir
// (config.json lives there) and the active vault dir. Reading per dispatch
// means a Permissions save takes effect without a restart.
func (a *App) sandboxPolicyFn(workspaceRoot string, selfDev bool) func() sandbox.Policy {
	return func() sandbox.Policy {
		var store application.SandboxRuntimeStore
		if a.vault != nil {
			store = a.vault
		}
		runtime := application.LoadSandboxRuntime(store)
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

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.queueLogEvent(application.LogEvent{
		Event:    "app.lifecycle.started",
		Severity: "info",
		Source:   "app",
		Message:  "app started",
		Status:   "ok",
		Attributes: map[string]any{
			"workspace_root.hash": application.HashForLog(a.workspaceRoot),
		},
	})
	a.devAutoUnlock()
	go a.autoLockLoop(ctx)
	go a.vaultWriteBackLoop(ctx)
	go a.firewallRebindLoop(ctx)
	// The web server has a lock-independent lifecycle: it comes up here when
	// enabled (config.json, readable pre-unlock) and serves /login even while
	// the vault is locked, so a remote browser can unlock it.
	go a.startWebServerIfEnabled()
}

// StartHeadless runs the non-Wails part of startup for the awd daemon: the
// auto-lock loop and the web server, without setting a.ctx. Leaving a.ctx nil is
// deliberate — it is the gate that routes events to the web/PiP hubs instead of
// the (absent) Wails runtime, and keeps native-dialog methods (already on the
// web denylist) from touching a nil runtime. The vault starts locked; the user
// unlocks and runs any setup through the web login screen.
func (a *App) StartHeadless(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	a.queueLogEvent(application.LogEvent{
		Event:    "app.lifecycle.started",
		Severity: "info",
		Source:   "app",
		Message:  "headless daemon started",
		Status:   "ok",
		Attributes: map[string]any{
			"workspace_root.hash": application.HashForLog(a.workspaceRoot),
		},
	})
	go a.autoLockLoop(ctx)
	go a.vaultWriteBackLoop(ctx)
	go a.firewallRebindLoop(ctx)
	go a.startWebServerIfEnabled()
}

// onDomReady runs once the frontend DOM is loaded and the window is on screen.
// This is the reliable moment to zoom the window on macOS: WindowStartState is
// ignored there, and a startup timer races the window becoming visible. Native
// zoom fills the screen while respecting the menu bar and dock.
func (a *App) OnDomReady(ctx context.Context) {
	if !wailsruntime.WindowIsMaximised(ctx) {
		wailsruntime.WindowMaximise(ctx)
	}
	a.queueLogEvent(application.LogEvent{
		Event:    "app.lifecycle.ready",
		Severity: "info",
		Source:   "app",
		Message:  "app ready",
		Status:   "ok",
	})
}

// autoLockLoop periodically locks the vault after the configured inactivity
// timeout and notifies the UI via the "vault:auto-locked" event.
func (a *App) autoLockLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if application.ShouldLockVaultIfInactive(a.autoLock, a.vault, now) {
				a.emitLogEvent(ctx, application.LogEvent{
					Event:    "vault.locked",
					Severity: "info",
					Source:   "vault",
					Message:  "vault locked",
					Status:   "ok",
					Attributes: map[string]any{
						"reason": "inactivity",
					},
				})
			}
			locked, err := application.LockVaultIfInactive(a.autoLock, a.vault, now)
			if err == nil && locked {
				a.stopMcpServer(ctx)
				a.stopRestServer(ctx)
				// The web server keeps running (so the browser can re-unlock),
				// but every existing web session must die on autolock.
				a.InvalidateWebSessions("inactivity")
				a.emitChatEvent("vault:auto-locked", map[string]any{"reason": "inactivity"})
			}
		}
	}
}

// recordActivity resets the auto-lock inactivity window.
func (a *App) recordActivity() {
	application.RecordAutoLockActivity(a.autoLock, time.Now())
}

// RecordActivity is called by the frontend on user interaction to keep the
// vault unlocked while the app is in active use.
func (a *App) RecordActivity() {
	a.recordActivity()
}

// GetAutoLockMinutes returns the current inactivity timeout in minutes.
func (a *App) GetAutoLockMinutes() int {
	return application.GetAutoLockMinutes(a.autoLock)
}

// SetAutoLockMinutes updates and persists the inactivity timeout in minutes.
func (a *App) SetAutoLockMinutes(minutes int) OperationResult {
	err := application.SetAutoLockMinutes(a.autoLock, appconfig.Store{}, minutes)
	a.recordActivity()
	return a.basicOperationResult(err)
}

// GetDesktopNotificationsEnabled returns the persisted notifications preference.
func (a *App) GetDesktopNotificationsEnabled() bool {
	return appconfig.Store{}.LoadDesktopNotificationsEnabled()
}

// SetDesktopNotificationsEnabled persists the notifications toggle.
func (a *App) SetDesktopNotificationsEnabled(enabled bool) OperationResult {
	return a.basicOperationResult(appconfig.Store{}.SaveDesktopNotificationsEnabled(enabled))
}

// NotifyDesktop fires a transient desktop notification through the OS
// adapter. The preference is checked inside the use case; callers never need
// to gate on it. Body is capped and scrubbed before dispatch.
func (a *App) NotifyDesktop(title, body string) OperationResult {
	enabled := appconfig.Store{}.LoadDesktopNotificationsEnabled()
	err := application.NotifyUser(a.notifier, enabled, title, body)
	return a.basicOperationResult(err)
}

// GetAppZoomPercent returns the persisted app-wide zoom percent.
func (a *App) GetAppZoomPercent() int {
	return application.NewAppZoomPreferences(appconfig.Store{}).Get()
}

// SetAppZoomPercent persists the app-wide zoom percent for future launches.
func (a *App) SetAppZoomPercent(percent int) OperationResult {
	if _, err := application.NewAppZoomPreferences(appconfig.Store{}).Set(percent); err != nil {
		return OperationResult{Success: false, Error: err.Error()}
	}
	return OperationResult{Success: true}
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) SendMessage(chatID string, text string) (agent.Reply, error) {
	result := a.SendChatMessage(chatID, text, nil)
	if !result.Success {
		return agent.Reply{}, errors.New(result.Error)
	}
	return result.Reply, nil
}

func (a *App) SendChatMessage(chatID string, text string, attachments []domain.Attachment) ChatSendResult {
	return a.runChatMessage(chatID, text, attachments, false)
}

// StreamChatMessage behaves like SendChatMessage but streams the assistant reply
// token-by-token to the frontend via Wails events ("chat:delta") as it is
// generated. The final persisted assistant message is still returned in the
// result when the turn completes.
func (a *App) StreamChatMessage(chatID string, text string, attachments []domain.Attachment) ChatSendResult {
	return a.runChatMessage(chatID, text, attachments, true)
}

func (a *App) TranscribeAudio(_ string, mimeType string, dataURI string) AudioTranscriptionResult {
	if a.vault == nil {
		return AudioTranscriptionResult{Success: false, Error: "vault is not available"}
	}
	data, err := decodeAudioDataURI(dataURI)
	if err != nil {
		return AudioTranscriptionResult{Success: false, Error: err.Error()}
	}
	if len(data) > 25*1024*1024 {
		return AudioTranscriptionResult{Success: false, Error: "audio is too large for transcription (max 25 MB)"}
	}
	result, err := application.TranscribeAndCleanVoice(a.contextOrBackground(), a.voiceTranscriptionDeps(), data, mimeType, ".")
	if err != nil {
		return AudioTranscriptionResult{Success: false, Error: err.Error()}
	}
	return AudioTranscriptionResult{
		Success:  true,
		Text:     result.Text,
		Provider: "local-whisper",
		Model:    result.Model,
	}
}

// voiceTranscriptionDeps assembles the ports the voice transcription use case
// needs. The agent runtime is passed as a true nil interface when absent so the
// best-effort cleanup pass is skipped instead of dereferencing a typed nil.
func (a *App) voiceTranscriptionDeps() application.VoiceTranscriptionDeps {
	deps := application.VoiceTranscriptionDeps{
		Transcriber: a.voiceTranscriber,
		Store:       a.vault,
		Memory:      a.vault,
	}
	if a.agent != nil {
		deps.Runtime = a.agent
	}
	return deps
}

// StartVoiceCapture records the default microphone natively via ffmpeg.
// The macOS WKWebView has no getUserMedia (wails:// is not a secure context),
// so the composer falls back to this path in the packaged app.
func (a *App) StartVoiceCapture() VoiceCaptureResult {
	if err := application.StartVoiceCapture(a.voiceCapture); err != nil {
		return VoiceCaptureResult{Success: false, Error: err.Error()}
	}
	return VoiceCaptureResult{Success: true}
}

func (a *App) StopVoiceCapture() AudioTranscriptionResult {
	data, err := application.StopVoiceCapture(a.voiceCapture)
	if err != nil {
		return AudioTranscriptionResult{Success: false, Error: err.Error()}
	}
	result, err := application.TranscribeAndCleanVoice(a.contextOrBackground(), a.voiceTranscriptionDeps(), data, "audio/wav", ".")
	if err != nil {
		return AudioTranscriptionResult{Success: false, Error: err.Error()}
	}
	return AudioTranscriptionResult{
		Success:  true,
		Text:     result.Text,
		Provider: "local-whisper",
		Model:    result.Model,
	}
}

func (a *App) CancelVoiceCapture() VoiceCaptureResult {
	if err := application.CancelVoiceCapture(a.voiceCapture); err != nil {
		return VoiceCaptureResult{Success: false, Error: err.Error()}
	}
	return VoiceCaptureResult{Success: true}
}

func decodeAudioDataURI(dataURI string) ([]byte, error) {
	value := strings.TrimSpace(dataURI)
	if value == "" {
		return nil, fmt.Errorf("audio is empty")
	}
	comma := strings.Index(value, ",")
	if comma < 0 {
		return nil, fmt.Errorf("invalid audio data URI")
	}
	meta := value[:comma]
	payload := value[comma+1:]
	if !strings.Contains(strings.ToLower(meta), ";base64") {
		decoded, err := url.QueryUnescape(payload)
		if err != nil {
			return nil, fmt.Errorf("invalid audio data URI: %w", err)
		}
		return []byte(decoded), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 audio data: %w", err)
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("audio is empty")
	}
	return decoded, nil
}

// effectiveChatRuntimeConfig resolves the provider config the way the turn
// will actually run: per-chat override first, then the workspace default,
// then the head of the fallback chain. The bare active-provider secret can be
// missing (e.g. the active provider's credential was deleted) while the chat
// still runs fine via fallback — session restore and run telemetry must
// resolve the same provider the turn does, not fail on the empty slot.
func (a *App) effectiveChatRuntimeConfig(chatID string) domain.ProviderRuntimeConfig {
	overrideProvider, overrideModel, _ := application.GetChatModelOverride(a.vault, chatID)
	if cfg, err := application.ResolveEffectiveProviderRuntimeConfig(a.vault, overrideProvider, overrideModel); err == nil {
		return cfg
	}
	if chain, err := application.ResolveChatFallbackChain(a.vault, a.providerFallbackOrder(), overrideProvider, overrideModel); err == nil && len(chain) > 0 {
		return chain[0]
	}
	return domain.ProviderRuntimeConfig{}
}

func (a *App) runChatMessage(chatID string, text string, attachments []domain.Attachment, stream bool) ChatSendResult {
	runStarted := time.Now()
	if a.agentErr != nil {
		return ChatSendResult{Success: false, Error: a.agentErr.Error(), SessionID: chatID}
	}
	if a.agent == nil {
		return ChatSendResult{Success: false, Error: "agent runtime is not available", SessionID: chatID}
	}
	runCtx, cancel := context.WithCancel(a.contextOrBackground())
	runCtx = domain.WithChatSessionScope(runCtx, chatID)
	runToken, err := a.chatRunner.BeginRun(chatID, cancel)
	if err != nil {
		cancel()
		return ChatSendResult{Success: false, Error: err.Error(), SessionID: chatID}
	}
	defer func() {
		cancel()
		a.chatRunner.EndRun(chatID, runToken)
		a.chatRunner.ClearStreamingRun(runToken)
	}()
	a.chatRunner.SetStreamingRun(chatID, runToken)
	a.recordActivity()

	// Every chat event carries chatId AND a per-run id (AW2's runId): the
	// frontend routes by chatId and drops late chunks from stopped/stale runs.
	runID, err := newConfirmationID()
	if err != nil {
		return ChatSendResult{Success: false, Error: err.Error(), SessionID: chatID}
	}
	// The run's spawn record was either consumed by DecorateReply on persist or
	// belongs to a reply that never got persisted (stopped/failed run).
	defer a.spawnRecords.forget(runID)
	userMessage, err := application.PersistChatUserMessage(a.vault, chatID, text, attachments)
	if err != nil {
		return ChatSendResult{Success: false, Error: err.Error(), SessionID: chatID, RunID: runID}
	}
	chatProviderOverride, chatModelOverride, _ := application.GetChatModelOverride(a.vault, chatID)
	startEmitted := false
	emitStart := func(chatID string) {
		if startEmitted {
			return
		}
		startEmitted = true
		a.setPartialReply(chatID, "")
		a.emitChatEvent("chat:start", map[string]any{"chatId": chatID, "runId": runID})
	}
	if stream {
		emitStart(chatID)
	}

	application.RefreshAgentContext(a.agent, a.vault, a.vault, a.vault, a.workspace, domain.ModuleCatalog(), appconfig.Store{}, a.sandboxPromptInput(), a.browserSnapshotInput(runCtx), a.obsidianContextBlock(), chatID)
	a.logPromptSkills(runCtx, chatID, runID)

	runtimeConfig := a.effectiveChatRuntimeConfig(chatID)
	a.emitLogEvent(runCtx, application.LogEvent{
		Event:     "chat.run.started",
		Severity:  "info",
		Source:    "chat",
		SessionID: chatID,
		TraceID:   runID,
		Message:   "chat run started",
		Status:    "ok",
		Attributes: map[string]any{
			"provider.id":      runtimeConfig.ProviderID,
			"provider.name":    runtimeConfig.ProviderName,
			"model":            runtimeConfig.Model,
			"stream":           stream,
			"input.chars":      len(strings.TrimSpace(text)),
			"attachment.count": len(attachments),
		},
	})
	a.emitLogEvent(runCtx, application.LogEvent{
		Event:     "model.call.started",
		Severity:  "debug",
		Source:    "provider",
		SessionID: chatID,
		TraceID:   runID,
		Message:   "model call started",
		Status:    "ok",
		Attributes: map[string]any{
			"provider.id":   runtimeConfig.ProviderID,
			"provider.name": runtimeConfig.ProviderName,
			"model":         runtimeConfig.Model,
			"stream":        stream,
		},
	})
	// After an app restart the runtime's session is cold while the vault still
	// holds the conversation the UI shows: re-seed the tail (30 msgs / 24k
	// chars, plus any compaction summary) so the model remembers. Best-effort:
	// a failed restore runs the turn cold rather than failing it.
	if seeded, restoreErr := application.EnsureChatSessionRestored(runCtx, a.vault, a.agent, application.ModelConfigFromProviderRuntimeConfig(runtimeConfig), chatID); restoreErr == nil && seeded > 0 {
		a.emitLogEvent(runCtx, application.LogEvent{
			Event: "chat.session.restored", Severity: "info", Source: "chat",
			SessionID: chatID, TraceID: runID,
			Message: "chat session restored from vault", Status: "ok",
			Attributes: map[string]any{"messages": seeded},
		})
	} else if restoreErr != nil {
		a.emitLogEvent(runCtx, application.LogEvent{
			Event: "chat.session.restore_failed", Severity: "warn", Source: "chat",
			SessionID: chatID, TraceID: runID,
			Message: "chat session restore failed; running cold", Status: "error",
			ErrorMessage: restoreErr.Error(),
		})
	}
	if a.shouldAutoCompactBeforeSend(chatID) {
		if compactResult, compactErr := a.autoCompactChatForRetry(runCtx, chatID, runID); compactErr == nil {
			a.emitChatEvent("chat:compacted", map[string]any{"chatId": chatID, "runId": runID, "messageCount": len(compactResult.Messages), "auto": true, "reason": "threshold"})
			application.RefreshAgentContext(a.agent, a.vault, a.vault, a.vault, a.workspace, domain.ModuleCatalog(), appconfig.Store{}, a.sandboxPromptInput(), a.browserSnapshotInput(runCtx), a.obsidianContextBlock(), chatID)
		}
	}

	input := application.RunChatMessageInput{
		ChatID:                  chatID,
		Text:                    text,
		Attachments:             attachments,
		Stream:                  stream,
		ExistingUserMessage:     &userMessage,
		LLMTurnStore:            a.vault,
		AttachmentReader:        a.attachmentReader(),
		AttachmentTaintRecorder: a.taintStore,
		TurnScope:               runID,
		FallbackOrder:           a.providerFallbackOrder(),
		ProviderOverride:        chatProviderOverride,
		ModelOverride:           chatModelOverride,
		Cooldown:                a.cooldown,
		DecorateReply:           a.spawnRecords.decorate,
		OnStart: func(chatID string) {
			emitStart(chatID)
		},
		OnDelta: func(chatID string, seq int, delta string) {
			a.appendPartialReply(chatID, delta)
			a.emitChatEvent("chat:delta", map[string]any{"chatId": chatID, "runId": runID, "seq": seq, "delta": delta})
		},
		OnDone: func(chatID string, messageID string) {
			// Unregister the run BEFORE announcing completion so any observer that
			// reacts to chat:done by re-reading the session info (ChatModule.loadChat)
			// sees IsRunning=false and does not re-arm the "thinking" bubble. Without
			// this, a run driven from the PiP window or `aw chat.send` — where the
			// main window only observes the event and never awaits the send — leaves
			// the spinner bouncing forever. The deferred EndRun stays as a
			// token-guarded safety net (a second EndRun with the same token no-ops).
			a.chatRunner.EndRun(chatID, runToken)
			a.clearPartialReply(chatID)
			a.emitChatEvent("chat:done", map[string]any{"chatId": chatID, "runId": runID, "messageId": messageID})
		},
		OnFallback: func(from, to domain.ProviderRuntimeConfig, reason error) {
			a.handleProviderFallback(chatID, runID, from, to, reason)
		},
	}

	result, err := application.RunChatMessage(runCtx, a.vault, a.vault, a.agent, input)
	if err != nil && domain.IsContextWindowError(err) && strings.TrimSpace(result.UserMessage.ID) != "" {
		compactResult, compactErr := a.autoCompactChatForRetry(runCtx, chatID, runID)
		if compactErr == nil {
			application.RefreshAgentContext(a.agent, a.vault, a.vault, a.vault, a.workspace, domain.ModuleCatalog(), appconfig.Store{}, a.sandboxPromptInput(), a.browserSnapshotInput(runCtx), a.obsidianContextBlock(), chatID)
			retryInput := input
			retryInput.ExistingUserMessage = compactedRetryUserMessage(compactResult.Messages, result.UserMessage)
			result, err = application.RunChatMessage(runCtx, a.vault, a.vault, a.agent, retryInput)
			if err == nil {
				a.emitChatEvent("chat:compacted", map[string]any{"chatId": chatID, "runId": runID, "messageCount": len(compactResult.Messages), "auto": true})
			}
		}
	}
	if err != nil {
		errorText := err.Error()
		status := "error"
		event := "chat.run.failed"
		severity := "error"
		if errors.Is(err, context.Canceled) {
			errorText = "Stopped"
			status = "canceled"
			event = "chat.run.canceled"
			severity = "info"
		}
		a.logChatMessageSent(runCtx, chatID, runID, result.UserMessage, text, attachments)
		a.emitLogEvent(runCtx, application.LogEvent{
			Event:        "model.call.failed",
			Severity:     severity,
			Source:       "provider",
			SessionID:    chatID,
			TraceID:      runID,
			Message:      "model call failed",
			Status:       status,
			DurationMs:   time.Since(runStarted).Milliseconds(),
			ErrorMessage: errorText,
			Attributes: map[string]any{
				"provider.id":   runtimeConfig.ProviderID,
				"provider.name": runtimeConfig.ProviderName,
				"model":         runtimeConfig.Model,
			},
		})
		a.emitLogEvent(runCtx, application.LogEvent{
			Event:        event,
			Severity:     severity,
			Source:       "chat",
			SessionID:    chatID,
			TraceID:      runID,
			Message:      strings.TrimPrefix(event, "chat."),
			Status:       status,
			DurationMs:   time.Since(runStarted).Milliseconds(),
			ErrorMessage: errorText,
			Attributes: map[string]any{
				"provider.id":      runtimeConfig.ProviderID,
				"provider.name":    runtimeConfig.ProviderName,
				"model":            runtimeConfig.Model,
				"attachment.count": len(attachments),
			},
		})
		// Mirror the OnDone success path: unregister the run BEFORE announcing the
		// error so an observer that flushes its queue on chat:error (chat-send-state
		// onRunEnded) does not race BeginRun against a still-registered run and hit
		// ErrChatAlreadyRunning. The deferred EndRun stays as a token-guarded net.
		a.chatRunner.EndRun(chatID, runToken)
		a.clearPartialReply(chatID)
		a.emitChatEvent("chat:error", map[string]any{"chatId": chatID, "runId": runID, "error": errorText})
		return ChatSendResult{Success: false, Error: errorText, SessionID: chatID, RunID: runID, UserMessage: chatSendMessagePtr(result.UserMessage)}
	}
	a.logChatMessageSent(runCtx, chatID, runID, result.UserMessage, text, attachments)
	a.emitLogEvent(runCtx, application.LogEvent{
		Event:      "model.call.completed",
		Severity:   "info",
		Source:     "provider",
		SessionID:  chatID,
		TraceID:    runID,
		Message:    "model call completed",
		Status:     "ok",
		DurationMs: time.Since(runStarted).Milliseconds(),
		Attributes: mergeLogAttributes(map[string]any{
			"provider.id":    result.ModelConfig.ProviderID,
			"provider.name":  result.ModelConfig.ProviderName,
			"model":          result.ModelConfig.Model,
			"reply.model":    result.Reply.Model,
			"reply.provider": result.Reply.Provider,
		}, tokenUsageLogAttributes(result.Reply.TokenUsage)),
	})
	a.emitLogEvent(runCtx, application.LogEvent{
		Event:      "chat.run.completed",
		Severity:   "info",
		Source:     "chat",
		SessionID:  chatID,
		TraceID:    runID,
		Message:    "chat run completed",
		Status:     "ok",
		DurationMs: time.Since(runStarted).Milliseconds(),
		Attributes: mergeLogAttributes(map[string]any{
			"provider.id":        result.ModelConfig.ProviderID,
			"provider.name":      result.ModelConfig.ProviderName,
			"model":              result.ModelConfig.Model,
			"attachment.count":   len(attachments),
			"turn_record.errors": len(result.TurnRecordErrors),
		}, tokenUsageLogAttributes(result.Reply.TokenUsage)),
	})
	a.rememberChatUsage(chatID, result.Reply.TokenUsage)
	a.scheduleAutoRenameChat(chatID, result.UserMessage.Content, result.Reply.Text, result.ModelConfig)
	return ChatSendResult{
		Success:          true,
		SessionID:        chatID,
		RunID:            runID,
		UserMessage:      chatSendMessagePtr(result.UserMessage),
		AssistantMessage: chatSendMessagePtr(result.AssistantMessage),
		Reply:            result.Reply,
	}
}

func chatSendMessagePtr(message domain.Message) *domain.Message {
	if strings.TrimSpace(message.ID) == "" {
		return nil
	}
	return &message
}

// logPromptSkills logs the EFFECTIVE skill catalog the agent received this turn
// (AW_SKILLS_DIR dev override or the enabled vault skills) so the log never lies
// about what was injected.
func (a *App) logPromptSkills(ctx context.Context, chatID, runID string) {
	list, source := application.EffectiveSkills(a.vault, a.skillsPromptInput())
	for _, skill := range list {
		name := strings.TrimSpace(skill.Name)
		if name == "" {
			continue
		}
		a.emitLogEvent(ctx, application.LogEvent{
			Event:     "skill.prompt.loaded",
			Severity:  "info",
			Source:    "agent",
			SessionID: chatID,
			TraceID:   runID,
			Message:   "skill loaded into prompt",
			Status:    "ok",
			Attributes: map[string]any{
				"skill.name":   name,
				"skill.source": source,
				"skill.count":  len(list),
			},
		})
	}
}

// skillsPromptInput resolves the AW_SKILLS_DIR dev override into the
// application-layer input. With no override it is the zero value, so the vault
// catalog is used.
func (a *App) skillsPromptInput() application.SkillsPromptInput {
	devSkills, agents, hasAgents, root, active := skills.DevCatalog()
	if !active {
		return application.SkillsPromptInput{}
	}
	return application.SkillsPromptInput{
		Active:       true,
		Skills:       devSkills,
		AgentsDoc:    agents,
		HasAgentsDoc: hasAgents,
		Source:       root,
	}
}

// bootstrapAndRefreshSkills seeds/updates the vault skills + runtime AGENTS.md
// (skipped when AW_SKILLS_DIR overrides the catalog) and pushes the effective
// set into the agent skills slot. Best-effort: failures are logged, never fatal
// to unlock.
func (a *App) bootstrapAndRefreshSkills(ctx context.Context) {
	if dir := skills.DevDir(); dir != "" {
		a.emitLogEvent(ctx, application.LogEvent{
			Event:      "skill.dev_override.active",
			Severity:   "info",
			Source:     "agent",
			Message:    "AW_SKILLS_DIR override active; vault skills ignored",
			Status:     "ok",
			Attributes: map[string]any{"skill.root.hash": application.HashForLog(dir)},
		})
	} else if seeds, err := skills.BundledSkills(); err == nil {
		agentsDoc, hasAgents, _ := skills.BundledAgentsDoc()
		if bErr := application.BootstrapSkills(a.vault, seeds, agentsDoc, hasAgents); bErr != nil {
			a.emitLogEvent(ctx, application.LogEvent{
				Event:        "skill.bootstrap.failed",
				Severity:     "warn",
				Source:       "agent",
				Message:      "skill seed bootstrap failed",
				Status:       "error",
				ErrorMessage: bErr.Error(),
			})
		}
		// Seed an empty USER.md so the Agent Instructions page always shows it.
		if uErr := application.EnsureUserDocument(a.vault); uErr != nil {
			a.emitLogEvent(ctx, application.LogEvent{
				Event:        "instructions.user_doc.bootstrap_failed",
				Severity:     "warn",
				Source:       "agent",
				Message:      "USER.md bootstrap failed",
				Status:       "error",
				ErrorMessage: uErr.Error(),
			})
		}
	}
	application.RefreshSkillsContext(a.agent, a.vault, a.skillsPromptInput())
}

func (a *App) logChatMessageSent(ctx context.Context, chatID, runID string, message domain.Message, originalText string, attachments []domain.Attachment) {
	if strings.TrimSpace(message.ID) == "" {
		return
	}
	a.emitLogEvent(ctx, application.LogEvent{
		Event:     "chat.message.sent",
		Severity:  "info",
		Source:    "chat",
		SessionID: chatID,
		TraceID:   runID,
		Message:   "chat message sent",
		Status:    "ok",
		Attributes: map[string]any{
			"message.id":       message.ID,
			"message.role":     message.Role,
			"message.chars":    len(strings.TrimSpace(originalText)),
			"attachment.count": len(attachments),
		},
	})
}

func tokenUsageLogAttributes(usage *domain.TokenUsage) map[string]any {
	if usage == nil {
		return map[string]any{}
	}
	return map[string]any{
		"tokens.input":  usage.Input,
		"tokens.output": usage.Output,
		"tokens.total":  usage.Total,
	}
}

// inAppNotifier implements ports.Notifier by emitting an app:notify event to
// every connected frontend (main window, PiP, web). The shell renders it with
// the same toast used for local notices, so the user sees exactly one
// notification style everywhere. It never touches the OS notification center.
type inAppNotifier struct{ app *App }

func (n inAppNotifier) Notify(title, body string) error {
	n.app.emitChatEvent("app:notify", map[string]any{"title": title, "body": body})
	return nil
}

func (a *App) emitChatEvent(name string, payload map[string]any) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, name, payload)
	}
	a.broadcastPipEvent(name, payload)
	a.broadcastWebEvent(name, payload)
}

// emitInlineChatImage shows a tool-captured image in the active chat as a
// system message, decoupled from the model's tool result. It returns false
// when no chat is streaming, so the caller can fall back to returning the raw
// image (e.g. the REST self-test path).
func (a *App) emitInlineChatImage(dataURI string, width, height int, caption string) bool {
	chatID := a.chatRunner.Streaming()
	if chatID == "" || dataURI == "" {
		return false
	}
	// A stable id lets the frontend dedupe: the event is handled both at the
	// always-mounted AppShell level (so it is captured even when the user has
	// navigated away from the chat) and inside the active ChatModule.
	id, err := newConfirmationID()
	if err != nil {
		id = fmt.Sprintf("inline-%d", time.Now().UnixNano())
	}
	a.emitChatEvent("chat:inline-image", map[string]any{
		"id":      id,
		"chatId":  chatID,
		"dataUri": dataURI,
		"width":   width,
		"height":  height,
		"caption": caption,
	})
	return true
}

// toolConfirmationTimeout bounds how long an in-app confirmation prompt waits
// for the user before the action is rejected, so a forgotten dialog cannot pin
// a tool call (and its goroutine) open forever.
const toolConfirmationTimeout = 3 * time.Minute

// confirmToolAction implements tools.ConfirmFunc: it emits a "tool:confirm"
// event to the UI and blocks until the user responds via ResolveToolConfirmation
// or the run is canceled.
func (a *App) confirmToolAction(ctx context.Context, req tools.ConfirmRequest) (bool, error) {
	// No a.ctx nil-guard: over web mode / headless awd the Wails runtime may be
	// absent, but the confirmation still goes out on the event hub (emitChatEvent
	// broadcasts to the web surface) and the browser answers via the normal
	// ResolveToolConfirmation path. Returning false here would silently deny
	// every tool action for a remote user.
	started := time.Now()
	id, err := newConfirmationID()
	if err != nil {
		return false, err
	}
	ch := a.confirmations.Register(id)
	defer a.confirmations.Unregister(id)
	a.emitLogEvent(ctx, application.LogEvent{
		Event:    "tool.confirmation.requested",
		Severity: "info",
		Source:   "tool",
		SpanID:   id,
		Message:  "tool confirmation requested",
		Status:   "ok",
		Attributes: map[string]any{
			"tool.name":     req.Tool,
			"summary":       req.Summary,
			"module.id":     req.ModuleID,
			"module.name":   req.ModuleName,
			"module.policy": req.ModulePolicy,
		},
	})

	a.emitChatEvent("tool:confirm", map[string]any{
		"id":            id,
		"tool":          req.Tool,
		"summary":       req.Summary,
		"args":          req.Args,
		"moduleId":      req.ModuleID,
		"moduleName":    req.ModuleName,
		"modulePolicy":  req.ModulePolicy,
		"module_id":     req.ModuleID,
		"module_name":   req.ModuleName,
		"module_policy": req.ModulePolicy,
	})
	select {
	case approved := <-ch:
		status := "denied"
		event := "tool.confirmation.denied"
		severity := "warn"
		if approved {
			status = "ok"
			event = "tool.confirmation.approved"
			severity = "info"
		}
		a.emitLogEvent(ctx, application.LogEvent{
			Event:      event,
			Severity:   severity,
			Source:     "tool",
			SpanID:     id,
			Message:    strings.ReplaceAll(event, ".", " "),
			Status:     status,
			DurationMs: time.Since(started).Milliseconds(),
			Attributes: map[string]any{
				"tool.name":     req.Tool,
				"module.id":     req.ModuleID,
				"module.name":   req.ModuleName,
				"module.policy": req.ModulePolicy,
			},
		})
		return approved, nil
	case <-time.After(toolConfirmationTimeout):
		err := fmt.Errorf("confirmation timed out after %s", toolConfirmationTimeout)
		a.emitLogEvent(ctx, application.LogEvent{
			Event:        "tool.confirmation.timeout",
			Severity:     "warn",
			Source:       "tool",
			SpanID:       id,
			Message:      "tool confirmation timed out",
			Status:       "timeout",
			DurationMs:   time.Since(started).Milliseconds(),
			ErrorMessage: err.Error(),
			Attributes: map[string]any{
				"tool.name":     req.Tool,
				"timeout_ms":    toolConfirmationTimeout.Milliseconds(),
				"module.id":     req.ModuleID,
				"module.name":   req.ModuleName,
				"module.policy": req.ModulePolicy,
			},
		})
		return false, err
	case <-ctx.Done():
		a.emitLogEvent(ctx, application.LogEvent{
			Event:        "tool.confirmation.denied",
			Severity:     "warn",
			Source:       "tool",
			SpanID:       id,
			Message:      "tool confirmation canceled",
			Status:       "canceled",
			DurationMs:   time.Since(started).Milliseconds(),
			ErrorMessage: ctx.Err().Error(),
			Attributes: map[string]any{
				"tool.name":     req.Tool,
				"module.id":     req.ModuleID,
				"module.name":   req.ModuleName,
				"module.policy": req.ModulePolicy,
			},
		})
		return false, ctx.Err()
	}
}

// ResolveToolConfirmation is called by the frontend to approve or reject a
// pending tool confirmation request emitted via "tool:confirm".
func (a *App) ResolveToolConfirmation(id string, approved bool) OperationResult {
	if !a.confirmations.Resolve(id, approved) {
		return OperationResult{Success: false, Error: "unknown or expired confirmation"}
	}
	return OperationResult{Success: true}
}

func newConfirmationID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (a *App) VaultStatus() VaultStatusResponse {
	return a.vaultStatus()
}

func (a *App) SelectFolder() (FolderDialogResult, error) {
	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select folder for Agent Workspace vault",
	})
	if err != nil {
		return FolderDialogResult{Canceled: true}, err
	}
	if dir == "" {
		return FolderDialogResult{Canceled: true}, nil
	}
	return FolderDialogResult{Canceled: false, Path: dir}, nil
}

// dialogAffirmative reports whether a native QuestionDialog choice approves the
// action. On Windows, Wails' MessageDialog ignores the custom Buttons and maps a
// QuestionDialog to MB_YESNO, so it returns "Yes"/"No" instead of the custom
// label (internal/frontend/desktop/windows/dialog.go). Accept both the custom
// affirmative label and the Windows "Yes"; anything else (incl. "No"/"Cancel")
// is treated as declined. Mirrors the fallback already used in
// app_permissions.go.
func dialogAffirmative(choice, label string) bool {
	answer := strings.ToLower(strings.TrimSpace(choice))
	return answer == strings.ToLower(label) || answer == "yes"
}

func (a *App) ChooseVaultDir() (VaultStatusResponse, error) {
	// Native confirmation dialog (Decision 1 / Decision 3 of vault-page-ux-spec).
	// Must be outside the webview DOM so UI automation cannot self-approve.
	if a.ctx != nil {
		choice, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
			Type:          wailsruntime.QuestionDialog,
			Title:         "Change vault location?",
			Message:       "This switches which vault the app opens. Your current vault stays where it is.",
			Buttons:       []string{"Change location", "Cancel"},
			DefaultButton: "Cancel",
			CancelButton:  "Cancel",
		})
		if err != nil {
			return a.vaultStatus(), err
		}
		if !dialogAffirmative(choice, "Change location") {
			return a.vaultStatus(), nil
		}
	}
	folder, err := a.SelectFolder()
	if err != nil || folder.Canceled {
		return a.vaultStatus(), err
	}
	// A live SQLite vault inside a synced folder produces disk I/O errors and
	// eventually corruption — refuse instead of letting the user find out.
	if provider := application.SyncedPathProvider(folder.Path); provider != "" {
		return a.vaultStatus(), fmt.Errorf("this folder is synced by %s — the vault database cannot live in a synced folder; choose a local folder instead", provider)
	}
	if err := application.ChooseVaultDir(a.vault, appconfig.Store{}, folder.Path); err != nil {
		return a.vaultStatus(), err
	}
	return a.vaultStatus(), nil
}

func (a *App) ListProfiles() []domain.ProfileInfo {
	return application.ListProfiles(a.profileManager)
}

func (a *App) SelectProfile(id string) OperationResult {
	result, err := application.SelectProfile(a.profileManager, a.vault, appconfig.Store{}, a.currentProfileID, id)
	a.applyCurrentProfileID(result.CurrentProfileID)
	return a.profileOperationResult(result.Profile, err)
}

func (a *App) CreateProfileAtLocation(parentDir string, folderName string) OperationResult {
	result, err := application.CreateProfileAtLocation(a.profileManager, a.vault, appconfig.Store{}, a.currentProfileID, parentDir, folderName)
	a.applyCurrentProfileID(result.CurrentProfileID)
	return a.profileOperationResult(result.Profile, err)
}

func (a *App) ImportVaultProfile(vaultDir string) OperationResult {
	result, err := application.ImportVaultProfile(a.profileManager, a.vault, appconfig.Store{}, a.currentProfileID, vaultDir)
	a.applyCurrentProfileID(result.CurrentProfileID)
	return a.profileOperationResult(result.Profile, err)
}

// ClearRecentProfiles forgets every entry in the start screen's "Recent
// vaults" list. The master vault directories are never touched; the
// profiles' ephemeral working copies are deleted (spec Q6) — except the
// currently unlocked one, whose live DB Windows would refuse to remove.
func (a *App) ClearRecentProfiles() OperationResult {
	err := application.ForgetProfilesAndWorkDirs(a.profileManager, a.vault, a.currentProfileID)
	return a.profileOperationResult(domain.ProfileInfo{}, err)
}

func (a *App) CreateVault(password string) OperationResult {
	result, err := application.CreateVaultForProfile(a.vault, a.profileManager, a.currentProfileID, password)
	a.applyCurrentProfileID(result.CurrentProfileID)
	if err == nil {
		// A brand-new vault starts with a clean workspace instead of
		// inheriting the previous vault's sidebar/theme/wallpaper.
		application.InitCleanWorkspace(a.workspace)
		a.resetPromptDebugModeForNewAppSession()
		a.flushQueuedLogEvents(a.contextOrBackground())
		a.bootstrapAndRefreshSkills(a.contextOrBackground())
	}
	return a.vaultOperationResult(result.RecoveryKey, err)
}

func (a *App) UnlockVault(password string) OperationResult {
	started := time.Now()
	err := application.UnlockVaultForProfile(a.vault, a.profileManager, a.currentProfileID, password)
	if err == nil {
		a.recordActivity()
		a.touchIDAutoRepair(password)
		a.resetPromptDebugModeForNewAppSession()
		a.flushQueuedLogEvents(a.contextOrBackground())
		a.bootstrapAndRefreshSkills(a.contextOrBackground())
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:      "vault.unlocked",
			Severity:   "info",
			Source:     "vault",
			Message:    "vault unlocked",
			Status:     "ok",
			DurationMs: time.Since(started).Milliseconds(),
			Attributes: map[string]any{
				"profile_id.hash": application.HashForLog(a.currentProfileID),
			},
		})
		go a.startMcpServerIfEnabled()
		go a.startRestServerIfEnabled()
		go a.startEnabledBrowsers()
		// Check once-per-day condensation of the user-memory document.
		go func() {
			_, _ = application.MaybeCondenseUserMemory(a.contextOrBackground(), application.UserMemoryCondenseInput{
				DocStore:      a.vault,
				ProviderStore: a.vault,
				LogStore:      a.vault,
				SecretStore:   a.vault,
				Runtime:       a.agent,
			})
		}()
	} else {
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:        "vault.unlock_failed",
			Severity:     "warn",
			Source:       "vault",
			Message:      "vault unlock failed",
			Status:       "error",
			DurationMs:   time.Since(started).Milliseconds(),
			ErrorMessage: err.Error(),
		})
	}
	return a.vaultOperationResult("", err)
}

func (a *App) RecoverVault(recoveryKey string, newPassword string) OperationResult {
	newRecoveryKey, err := application.RecoverVaultForProfile(a.vault, a.profileManager, a.currentProfileID, recoveryKey, newPassword)
	if err == nil {
		a.resetPromptDebugModeForNewAppSession()
		a.flushQueuedLogEvents(a.contextOrBackground())
		a.bootstrapAndRefreshSkills(a.contextOrBackground())
		go a.startMcpServerIfEnabled()
		go a.startRestServerIfEnabled()
		go a.startEnabledBrowsers()
	}
	return a.vaultOperationResult(newRecoveryKey, err)
}

func (a *App) ChangePassword(currentPassword string, newPassword string) OperationResult {
	newRecoveryKey, err := application.ChangeVaultPassword(a.vault, currentPassword, newPassword)
	return a.vaultOperationResult(newRecoveryKey, err)
}

func (a *App) GenerateRecoveryKey() OperationResult {
	recoveryKey, err := application.GenerateVaultRecoveryKey(a.vault)
	return a.vaultOperationResult(recoveryKey, err)
}

func (a *App) VerifyRecoveryKey(recoveryKey string) OperationResult {
	valid, err := application.VerifyVaultRecoveryKey(a.vault, recoveryKey)
	return OperationResult{
		Success: err == nil,
		Error:   errorString(err),
		Valid:   valid,
		Status:  a.vaultStatus(),
	}
}

func (a *App) LockVault() OperationResult {
	a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
		Event:    "vault.locked",
		Severity: "info",
		Source:   "vault",
		Message:  "vault locked",
		Status:   "ok",
		Attributes: map[string]any{
			"reason": "user",
		},
	})
	err := application.LockVault(a.vault)
	if err == nil {
		a.stopMcpServer(a.ctx)
		a.stopRestServer(a.ctx)
		// The web server is intentionally NOT stopped here (a remote browser
		// must be able to re-unlock), but all live web sessions are invalidated.
		a.InvalidateWebSessions("user")
		// Drop sensitive skill text from the prompt while locked (no-op under a
		// dev override, where skills come from disk, not the vault).
		application.RefreshSkillsContext(a.agent, a.vault, a.skillsPromptInput())
		// A PiP window keeps decrypted chat text on screen after the vault
		// gate drops; tell it to close. Auto-lock and agent-lock paths reach
		// the PiP through emitChatEvent already.
		a.broadcastPipEvent("vault:auto-locked", map[string]any{"reason": "user"})
	}
	return a.vaultOperationResult("", err)
}

func (a *App) ListChats() ([]domain.Chat, error) {
	return application.ListChats(a.vault)
}

func (a *App) CreateChat() (domain.Chat, error) {
	return application.CreateChat(a.vault, "")
}

func (a *App) CreateChatWithTitle(title string) (domain.Chat, error) {
	chat, err := application.CreateChat(a.vault, title)
	if err == nil {
		// Callers outside the chat view (e.g. a module's "?" help button)
		// create chats too — broadcast so the sidebar list picks it up.
		a.emitChatEvent("chat:refresh", map[string]any{"chatId": chat.ID})
	}
	return chat, err
}

func (a *App) ListMessages(chatID string) ([]domain.Message, error) {
	return application.ListMessages(a.vault, chatID)
}

func (a *App) RecentMessages(chatID string, limit int) ([]domain.Message, error) {
	return application.RecentMessages(a.vault, chatID, limit)
}

// ListChatTurns returns the persisted LLM turns (raw request/response/usage) of a
// chat for the prompt-debug panel.
func (a *App) ListChatTurns(chatID string) ([]domain.LLMTurn, error) {
	return application.ListLLMTurns(a.vault, chatID)
}

func (a *App) ClearChat(chatID string) ChatOperationResult {
	return a.chatOperationResult(nil, application.ClearChat(a.vault, chatID))
}

func (a *App) NewChatSession(chatID string) ChatOperationResult {
	a.chatRunner.Stop(chatID)
	return a.chatOperationResult(nil, application.ClearChat(a.vault, chatID))
}

func (a *App) RenameChat(chatID string, title string) ChatOperationResult {
	chat, err := application.RenameChat(a.vault, chatID, title)
	if err == nil {
		_ = application.MarkChatManuallyRenamed(a.vault, chatID, chat.Title)
	}
	return a.chatOperationResult(&chat, err)
}

func (a *App) SetChatArchived(chatID string, archived bool) ChatOperationResult {
	chat, err := application.SetChatArchived(a.vault, chatID, archived)
	return a.chatOperationResult(&chat, err)
}

func (a *App) DeleteChat(chatID string) ChatOperationResult {
	a.chatRunner.Stop(chatID)
	return a.chatOperationResult(nil, application.DeleteChat(a.vault, chatID))
}

func (a *App) StopChat(chatID string) ChatOperationResult {
	a.chatRunner.Stop(chatID)
	a.emitChatEvent("chat:refresh", map[string]any{"chatId": chatID})
	return a.chatOperationResult(nil, nil)
}

func (a *App) CompactChat(chatID string) ChatOperationResult {
	if a.agentErr != nil {
		return a.chatOperationResult(nil, a.agentErr)
	}
	runCtx, cancel := context.WithCancel(a.contextOrBackground())
	runID, _ := newConfirmationID()
	// Token-guarded so a slow compaction goroutine that outlives a Stop (which
	// frees the slot and lets a new send register its own run) cannot clear the
	// newer run on its way out.
	compactToken, err := a.chatRunner.BeginRun(chatID, cancel)
	if err != nil {
		cancel()
		return a.chatOperationResult(nil, err)
	}
	defer func() {
		cancel()
		a.chatRunner.EndRun(chatID, compactToken)
	}()
	a.recordActivity()
	started := time.Now()
	a.emitLogEvent(runCtx, application.LogEvent{
		Event:     "chat.compaction.started",
		Severity:  "info",
		Source:    "chat",
		SessionID: chatID,
		TraceID:   runID,
		Message:   "chat compaction started",
		Status:    "ok",
		Attributes: map[string]any{
			"auto": false,
		},
	})

	result, err := application.CompactChatHistory(runCtx, a.vault, a.vault, a.agent, application.CompactChatHistoryInput{ChatID: chatID})
	if err == nil {
		a.emitLogEvent(runCtx, application.LogEvent{
			Event:      "chat.compaction.completed",
			Severity:   "info",
			Source:     "chat",
			SessionID:  chatID,
			TraceID:    runID,
			Message:    "chat compaction completed",
			Status:     "ok",
			DurationMs: time.Since(started).Milliseconds(),
			Attributes: map[string]any{
				"auto":          false,
				"message.count": len(result.Messages),
			},
		})
		a.emitChatEvent("chat:compacted", map[string]any{"chatId": chatID, "messageCount": len(result.Messages)})
	} else {
		a.emitLogEvent(runCtx, application.LogEvent{
			Event:        "chat.compaction.failed",
			Severity:     "warn",
			Source:       "chat",
			SessionID:    chatID,
			TraceID:      runID,
			Message:      "chat compaction failed",
			Status:       "error",
			DurationMs:   time.Since(started).Milliseconds(),
			ErrorMessage: err.Error(),
			Attributes: map[string]any{
				"auto": false,
			},
		})
	}
	return a.chatOperationResult(nil, err)
}

func (a *App) shouldAutoCompactBeforeSend(chatID string) bool {
	usage := a.lastChatUsage(chatID)
	if usage == nil {
		return false
	}
	runtimeConfig, err := application.ResolveProviderRuntimeConfig(a.vault)
	if err != nil {
		return false
	}
	return application.ShouldAutoCompactForUsage((*domain.TokenUsage)(usage), runtimeConfig.ProviderID, runtimeConfig.Model)
}

func compactedRetryUserMessage(messages []domain.Message, original domain.Message) *domain.Message {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if strings.EqualFold(strings.TrimSpace(message.Role), "user") &&
			strings.TrimSpace(message.Content) == strings.TrimSpace(original.Content) {
			return &message
		}
	}
	if strings.TrimSpace(original.ID) == "" {
		return nil
	}
	return &original
}

func (a *App) autoCompactChatForRetry(ctx context.Context, chatID string, runID string) (application.CompactChatHistoryResult, error) {
	started := time.Now()
	a.emitLogEvent(ctx, application.LogEvent{
		Event:     "chat.compaction.started",
		Severity:  "info",
		Source:    "chat",
		SessionID: chatID,
		TraceID:   runID,
		Message:   "chat compaction started",
		Status:    "ok",
		Attributes: map[string]any{
			"auto": true,
		},
	})
	a.emitChatEvent("chat:compact-start", map[string]any{"chatId": chatID, "runId": runID, "auto": true})
	result, err := application.CompactChatHistory(ctx, a.vault, a.vault, a.agent, application.CompactChatHistoryInput{ChatID: chatID})
	if err != nil {
		a.emitLogEvent(ctx, application.LogEvent{
			Event:        "chat.compaction.failed",
			Severity:     "warn",
			Source:       "chat",
			SessionID:    chatID,
			TraceID:      runID,
			Message:      "chat compaction failed",
			Status:       "error",
			DurationMs:   time.Since(started).Milliseconds(),
			ErrorMessage: err.Error(),
			Attributes: map[string]any{
				"auto": true,
			},
		})
		a.emitChatEvent("chat:compact-error", map[string]any{"chatId": chatID, "runId": runID, "auto": true, "error": err.Error()})
		return result, err
	}
	a.emitLogEvent(ctx, application.LogEvent{
		Event:      "chat.compaction.completed",
		Severity:   "info",
		Source:     "chat",
		SessionID:  chatID,
		TraceID:    runID,
		Message:    "chat compaction completed",
		Status:     "ok",
		DurationMs: time.Since(started).Milliseconds(),
		Attributes: map[string]any{
			"auto":          true,
			"message.count": len(result.Messages),
		},
	})
	return result, nil
}

// Partial-reply buffer: the text streamed so far by each in-flight run,
// composition-root memory only. It reseeds the live bubble when a chat view
// remounts mid-run (module switch, PiP) — without it those tokens vanished
// until the run finished.
func (a *App) setPartialReply(chatID, text string) {
	a.partialMu.Lock()
	defer a.partialMu.Unlock()
	if a.partialReplies == nil {
		a.partialReplies = map[string]*strings.Builder{}
	}
	b := &strings.Builder{}
	b.WriteString(text)
	a.partialReplies[chatID] = b
}

func (a *App) appendPartialReply(chatID, delta string) {
	a.partialMu.Lock()
	defer a.partialMu.Unlock()
	if a.partialReplies == nil {
		a.partialReplies = map[string]*strings.Builder{}
	}
	if a.partialReplies[chatID] == nil {
		a.partialReplies[chatID] = &strings.Builder{}
	}
	a.partialReplies[chatID].WriteString(delta)
}

func (a *App) clearPartialReply(chatID string) {
	a.partialMu.Lock()
	defer a.partialMu.Unlock()
	delete(a.partialReplies, chatID)
}

func (a *App) partialReply(chatID string) string {
	a.partialMu.Lock()
	defer a.partialMu.Unlock()
	if b := a.partialReplies[chatID]; b != nil {
		return b.String()
	}
	return ""
}

func (a *App) GetChatSessionInfo(chatID string) ChatSessionInfo {
	overrideProvider, overrideModel, _ := application.GetChatModelOverride(a.vault, chatID)
	info := application.BuildChatSessionInfo(a.vault, a.vault, application.ChatSessionInfoInput{
		ChatID:           chatID,
		AgentErr:         a.agentErr,
		PlanMode:         a.GetPlanMode(chatID).Enabled,
		RunActive:        a.chatRunner.IsRunning(chatID),
		LastUsage:        a.lastChatUsage(chatID),
		SupportsRuntime:  agent.SupportsConfig,
		ProviderOverride: overrideProvider,
		ModelOverride:    overrideModel,
	})
	if info.Streaming {
		info.PartialReply = a.partialReply(chatID)
	}
	return dto.ChatSessionInfoFromDomain(info)
}

// SetChatModel applies a per-chat provider/model override (or clears it when
// provider is empty). provider accepts a provider ID or display name. The
// override only affects this chat; the global default set in Settings is
// unchanged.
func (a *App) SetChatModel(chatID string, provider string, model string) ChatModelResult {
	sel, err := application.SetChatModel(a.vault, a.vault, chatID, provider, model)
	if err != nil {
		return ChatModelResult{Success: false, Error: err.Error()}
	}
	return ChatModelResult{
		Success:      true,
		Provider:     sel.Provider,
		ProviderName: sel.ProviderName,
		Model:        sel.Model,
		Cleared:      sel.Cleared,
	}
}

func (a *App) scheduleAutoRenameChat(chatID string, userMessage string, assistantReply string, modelCfg agent.ModelConfig) {
	go a.maybeAutoRenameChat(chatID, userMessage, assistantReply, modelCfg)
	// Same deterministic post-turn hook: backfill the preferred-language
	// memory line from the user's own words (one-shot; no-op once present).
	go func() {
		if a.agent == nil {
			return
		}
		if application.MaybeDetectUserLanguage(a.contextOrBackground(), a.agent, modelCfg, a.vault, userMessage) {
			a.emitChatEvent("memory:changed", map[string]any{})
		}
	}()
}

func (a *App) maybeAutoRenameChat(chatID string, userMessage string, assistantReply string, modelCfg agent.ModelConfig) {
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), autoRenameTimeout)
	defer cancel()

	result, err := application.MaybeAutoRenameChat(ctx, a.vault, a.agent, application.AutoRenameChatInput{
		ChatID:         chatID,
		UserMessage:    userMessage,
		AssistantReply: assistantReply,
		ModelConfig:    modelCfg,
	})
	if err != nil {
		return
	}
	if result.Renamed {
		a.emitChatEvent("chat:renamed", map[string]any{"chatId": chatID, "title": result.Title})
	}
}

func (a *App) SetPlanMode(chatID string, enabled bool) PlanModeResult {
	a.planModesMu.Lock()
	defer a.planModesMu.Unlock()
	if strings.TrimSpace(chatID) == "" {
		return PlanModeResult{Success: false, Error: "chat id is required"}
	}
	a.planModes[chatID] = enabled
	return PlanModeResult{Success: true, Enabled: enabled}
}

func (a *App) rememberChatUsage(chatID string, usage *agent.TokenUsage) {
	if a.chatUsage == nil {
		a.chatUsage = application.NewChatUsageTracker()
	}
	a.chatUsage.Remember(chatID, usage)
}

func (a *App) lastChatUsage(chatID string) *agent.TokenUsage {
	if a.chatUsage == nil {
		return nil
	}
	return a.chatUsage.Last(chatID)
}

func (a *App) GetPlanMode(chatID string) PlanModeResult {
	a.planModesMu.Lock()
	defer a.planModesMu.Unlock()
	return PlanModeResult{Success: true, Enabled: a.planModes[chatID]}
}

func (a *App) isPlanModeEnabled(chatID string) bool {
	a.planModesMu.Lock()
	defer a.planModesMu.Unlock()
	return a.planModes[chatID]
}

func (a *App) SetSecret(name string, value string) OperationResult {
	err := application.SetSecret(a.vault, name, value)
	return a.vaultOperationResult("", err)
}

func (a *App) GetSecret(name string) SecretResult {
	secret, err := application.GetSecret(a.vault, name)
	result := SecretResult{Success: err == nil, Value: secret.Value, Exists: secret.Exists}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func (a *App) HasSecret(name string) SecretResult {
	exists, err := application.HasSecret(a.vault, name)
	result := SecretResult{Success: err == nil, Exists: exists}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func (a *App) ListSecrets() ([]string, error) {
	return application.ListSecrets(a.vault)
}

func (a *App) DeleteSecret(name string) OperationResult {
	err := application.DeleteSecret(a.vault, name)
	return a.vaultOperationResult("", err)
}

func (a *App) GetProviderStatus() application.ProviderStatus {
	return application.GetProviderStatus(a.vault)
}

func (a *App) SaveProviderConfig(input application.ProviderSaveConfigInput) application.ProviderOperationResult {
	result := application.SaveProviderConfig(a.contextOrBackground(), a.vault, providers.NewValidator(), input)
	if result.Success && a.cooldown != nil {
		// Fresh credentials/config are a reason to retry: lift the breaker bench.
		a.cooldown.Clear(input.Provider)
	}
	return result
}

func (a *App) StartProviderBrowserAuth(provider string, model string) application.ProviderOperationResult {
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 10*time.Minute)
	defer cancel()
	var authenticator ports.ProviderBrowserAuthenticator
	if strings.TrimSpace(provider) == "github-copilot" {
		authenticator = oauth.NewGitHubCopilotAuthenticator(
			func(verificationURI string) {
				wailsruntime.BrowserOpenURL(a.contextOrBackground(), verificationURI)
			},
			func(userCode, verificationURI string) {
				a.surfaceDeviceCode(userCode, verificationURI)
			},
		)
	} else {
		authenticator = oauth.NewOpenAIAuthenticator(func(authURL string) {
			wailsruntime.BrowserOpenURL(a.contextOrBackground(), authURL)
		})
	}
	return application.AuthenticateProviderViaBrowser(ctx, authenticator, a.vault, providers.NewValidator(), provider, model)
}

// surfaceDeviceCode shows the GitHub device-flow user code to the user: it
// copies the code to the clipboard, emits a UI event so the Providers page can
// render it, and fires a desktop notification as a fallback.
func (a *App) surfaceDeviceCode(userCode, verificationURI string) {
	devicePayload := map[string]any{
		"userCode":        userCode,
		"verificationUri": verificationURI,
	}
	if a.ctx != nil {
		_ = wailsruntime.ClipboardSetText(a.ctx, userCode)
		wailsruntime.EventsEmit(a.ctx, "provider:device-code", devicePayload)
	}
	// Also surface the device code to a remote browser (GitHub Copilot
	// device-flow is supported in web v1: the user reads the code on screen).
	a.broadcastWebEvent("provider:device-code", devicePayload)
	enabled := appconfig.Store{}.LoadDesktopNotificationsEnabled()
	_ = application.NotifyUser(a.notifier, enabled, "GitHub Copilot sign-in", fmt.Sprintf("Enter code %s at %s (copied to clipboard).", userCode, verificationURI))
}

// StartProviderBrowserAuthAsync begins browser/device OAuth WITHOUT blocking, so
// an agent (aw action) can trigger sign-in and relay the instructions to the
// user. Unlike StartProviderBrowserAuth (which the UI awaits for up to 10 min),
// this launches the flow in the background and returns as soon as the first
// user-facing detail is available: the device code + verification URL (GitHub
// Copilot) or a "browser opened" acknowledgement (OpenAI). The user completes
// sign-in; completion is observed via provider.status (the connected flag).
func (a *App) StartProviderBrowserAuthAsync(provider string, model string) map[string]any {
	provider = strings.TrimSpace(provider)
	if provider != "github-copilot" && provider != "openai-codex" {
		return map[string]any{"started": false, "error": fmt.Sprintf("browser authentication is not supported for provider %q", provider)}
	}

	// Buffered so the goroutine never blocks if we already returned on timeout.
	started := make(chan map[string]any, 1)
	signal := func(info map[string]any) {
		select {
		case started <- info:
		default:
		}
	}

	go func() {
		ctx, cancel := context.WithTimeout(a.contextOrBackground(), 10*time.Minute)
		defer cancel()
		var authenticator ports.ProviderBrowserAuthenticator
		if provider == "github-copilot" {
			authenticator = oauth.NewGitHubCopilotAuthenticator(
				func(verificationURI string) {
					wailsruntime.BrowserOpenURL(a.contextOrBackground(), verificationURI)
				},
				func(userCode, verificationURI string) {
					a.surfaceDeviceCode(userCode, verificationURI)
					signal(map[string]any{
						"started":         true,
						"deviceCode":      userCode,
						"verificationUri": verificationURI,
						"message":         fmt.Sprintf("Tell the user to open %s and enter code %s to finish sign-in, then poll provider.status.", verificationURI, userCode),
					})
				},
			)
		} else {
			authenticator = oauth.NewOpenAIAuthenticator(func(authURL string) {
				wailsruntime.BrowserOpenURL(a.contextOrBackground(), authURL)
				signal(map[string]any{
					"started":         true,
					"verificationUri": authURL,
					"message":         "A browser was opened for the user to complete sign-in; then poll provider.status.",
				})
			})
		}
		// Blocks until the flow completes; the agent observes success via status.
		_ = application.AuthenticateProviderViaBrowser(ctx, authenticator, a.vault, providers.NewValidator(), provider, model)
	}()

	select {
	case info := <-started:
		return info
	case <-time.After(30 * time.Second):
		return map[string]any{
			"started": true,
			"message": "Authentication started; the user must complete it in the browser. Poll provider.status until connected.",
		}
	}
}

func (a *App) SwitchProvider(provider string, model string) application.ProviderOperationResult {
	result := application.SwitchProvider(a.contextOrBackground(), a.vault, providers.NewValidator(), provider, model)
	if result.Success {
		// Activation and #1 are the same lever: the newly active provider must
		// also head the priority list, or the numbered order would contradict
		// what new chats actually use.
		_ = application.MoveProviderToOrderHead(a.workspace, provider)
		// An explicit user choice overrides the breaker: retry this provider.
		if a.cooldown != nil {
			a.cooldown.Clear(provider)
		}
	}
	return result
}

// SetProviderEnabled flips a provider's on/off switch. Disabling the active
// provider hands activity to the next enabled one by priority order.
func (a *App) SetProviderEnabled(provider string, enabled bool) application.ProviderOperationResult {
	return application.SetProviderEnabled(a.vault, a.providerFallbackOrder(), provider, enabled)
}

func (a *App) DeleteProviderCredential(provider string) application.ProviderOperationResult {
	if a.ctx == nil {
		return application.ProviderOperationResult{Success: false, Error: "app window is not available for the confirmation dialog"}
	}
	providerName := provider
	for _, def := range application.AllProviderDefinitions(a.vault) {
		if def.ID == provider {
			providerName = def.Name
			break
		}
	}
	choice, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Remove credential?",
		Message:       fmt.Sprintf("Remove the stored credential for %s?\n\nThis deletes the saved key from the encrypted vault. The provider will need to be reconfigured.", providerName),
		Buttons:       []string{"Remove", "Cancel"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		return application.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	if !dialogAffirmative(choice, "Remove") {
		return application.ProviderOperationResult{Canceled: true}
	}
	return application.DeleteProviderCredential(a.vault, a.providerFallbackOrder(), provider)
}

// DeleteProviderCredentialConfirmed removes a stored provider credential
// without a native dialog. It is the web-mode counterpart of
// DeleteProviderCredential: the remote browser confirms with a React modal and
// then calls this no-dialog variant (the dialog version is on the web denylist).
func (a *App) DeleteProviderCredentialConfirmed(provider string) application.ProviderOperationResult {
	return application.DeleteProviderCredential(a.vault, a.providerFallbackOrder(), provider)
}

// CreateCustomProvider adds a new dynamic OpenAI-compatible provider slot and
// returns its generated id (in result.ProviderID) so the UI can open it.
func (a *App) CreateCustomProvider(name string) application.ProviderOperationResult {
	return application.CreateCustomProvider(a.vault, name)
}

// RenameCustomProvider updates the display name of a custom provider.
func (a *App) RenameCustomProvider(id string, name string) application.ProviderOperationResult {
	return application.RenameCustomProvider(a.vault, id, name)
}

// DeleteCustomProvider removes a custom provider entirely (credential, base
// URL, model, registry entry, active flag and fallback-order membership) after
// a native confirmation dialog.
func (a *App) DeleteCustomProvider(id string) application.ProviderOperationResult {
	if a.ctx == nil {
		return application.ProviderOperationResult{Success: false, Error: "app window is not available for the confirmation dialog"}
	}
	providerName := id
	for _, def := range application.AllProviderDefinitions(a.vault) {
		if def.ID == id {
			providerName = def.Name
			break
		}
	}
	choice, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Delete provider?",
		Message:       fmt.Sprintf("Delete the custom provider %q?\n\nThis removes its slot and deletes the saved key, base URL and model from the encrypted vault. This cannot be undone.", providerName),
		Buttons:       []string{"Delete", "Cancel"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		return application.ProviderOperationResult{Success: false, Error: err.Error()}
	}
	if !dialogAffirmative(choice, "Delete") {
		return application.ProviderOperationResult{Canceled: true}
	}
	result := application.DeleteCustomProvider(a.vault, id)
	if result.Success {
		a.dropProviderFromFallbackOrder(id)
	}
	return result
}

// DeleteCustomProviderConfirmed deletes a custom provider without a native
// dialog. Web-mode counterpart of DeleteCustomProvider: the remote browser
// confirms with a React modal, then calls this no-dialog variant.
func (a *App) DeleteCustomProviderConfirmed(id string) application.ProviderOperationResult {
	result := application.DeleteCustomProvider(a.vault, id)
	if result.Success {
		a.dropProviderFromFallbackOrder(id)
	}
	return result
}

// DiscardCustomProvider removes a custom provider without a confirmation
// dialog. It backs the "Add custom provider" → Cancel flow: a freshly created
// slot that was never saved should disappear instead of lingering.
func (a *App) DiscardCustomProvider(id string) application.ProviderOperationResult {
	result := application.DeleteCustomProvider(a.vault, id)
	if result.Success {
		a.dropProviderFromFallbackOrder(id)
	}
	return result
}

// dropProviderFromFallbackOrder removes an id from the persisted fallback
// priority list (a no-op when it is absent).
func (a *App) dropProviderFromFallbackOrder(id string) {
	order := application.LoadProviderFallbackOrder(a.workspace)
	filtered := make([]string, 0, len(order))
	changed := false
	for _, entry := range order {
		if entry == id {
			changed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if changed {
		_ = application.SaveProviderFallbackOrder(a.workspace, filtered)
	}
}

func (a *App) vaultStatus() VaultStatusResponse {
	return vaultStatusResponse(application.BuildVaultStatusView(a.vault, a.profileManager, a.currentProfileID))
}

// TestProviderConnection sends a minimal one-shot completion ("Reply with OK")
// through the named provider's stored config (not the active one). Returns
// latency and model echoed on success, or the provider's error verbatim.
// No key material is included in the result.
func (a *App) TestProviderConnection(provider string) domain.ProviderTestResult {
	a.recordActivity()
	return application.TestProviderConnection(a.contextOrBackground(), a.vault, a.agent, providers.NewProbe(), provider)
}

// ListProviderModels returns the models a provider can currently serve — live
// from the provider's own models endpoint when configured (GitHub Copilot),
// otherwise the static catalog from the provider definition.
func (a *App) ListProviderModels(provider string) domain.ProviderModelsResult {
	a.recordActivity()
	return application.ListProviderModels(a.contextOrBackground(), a.vault, agent.NewModelCatalog(), provider)
}

// GetProviderBalance fetches credit/usage data for providers that expose such
// an endpoint (currently only OpenRouter). Available=false means the caller
// should hide the balance row. No key material is included in the result.
func (a *App) GetProviderBalance(provider string) domain.ProviderBalanceResult {
	a.recordActivity()
	return application.GetProviderBalance(a.contextOrBackground(), a.vault, providers.NewProbe(), provider)
}

func (a *App) vaultOperationResult(recoveryKey string, err error) OperationResult {
	snapshot := application.BuildOperationSnapshot(a.vault, a.profileManager, a.currentProfileID)
	result := OperationResult{
		Success:        err == nil,
		RecoveryKey:    recoveryKey,
		NewRecoveryKey: recoveryKey,
		Status:         vaultStatusResponseFromSnapshot(snapshot),
		Chats:          snapshot.Chats,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func (a *App) basicOperationResult(err error) OperationResult {
	return a.vaultOperationResult("", err)
}

func (a *App) profileOperationResult(info domain.ProfileInfo, err error) OperationResult {
	snapshot := application.BuildOperationSnapshot(a.vault, a.profileManager, a.currentProfileID)
	result := OperationResult{
		Success: err == nil,
		Status:  vaultStatusResponseFromSnapshot(snapshot),
		Chats:   snapshot.Chats,
	}
	if info.ID != "" {
		result.Profile = &info
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func (a *App) chatOperationResult(chat *domain.Chat, err error) ChatOperationResult {
	snapshot := application.BuildOperationSnapshot(a.vault, a.profileManager, a.currentProfileID)
	result := ChatOperationResult{
		Success: err == nil,
		Status:  vaultStatusResponseFromSnapshot(snapshot),
		Chat:    chat,
		Chats:   snapshot.Chats,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func (a *App) contextOrBackground() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) applyCurrentProfileID(id string) {
	if id != "" {
		a.currentProfileID = id
		// Working-copy model: the live DB always runs in the profile's local
		// work dir, never in the (possibly cloud-synced) master folder.
		application.ConfigureVaultWorkDir(a.vault, id)
	}
}

func vaultStatusResponse(view application.VaultStatusView) VaultStatusResponse {
	return VaultStatusResponse{
		Exists:         view.Status.Exists,
		Unlocked:       view.Status.Unlocked,
		VaultDir:       view.Status.VaultDir,
		HasRecovery:    view.Status.HasRecovery,
		CurrentProfile: view.CurrentProfile,
	}
}

func vaultStatusResponseFromSnapshot(snapshot application.OperationSnapshot) VaultStatusResponse {
	return VaultStatusResponse{
		Exists:         snapshot.Status.Exists,
		Unlocked:       snapshot.Status.Unlocked,
		VaultDir:       snapshot.Status.VaultDir,
		HasRecovery:    snapshot.Status.HasRecovery,
		CurrentProfile: snapshot.CurrentProfile,
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
