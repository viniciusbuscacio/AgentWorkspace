package dto

import (
	"aw/internal/domain"
)

// SkillView is one row of the Skills management panel. Mirrors
// application.SkillView; the interface layer maps between them.
type SkillView struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Origin          string `json:"origin"`
	Enabled         bool   `json:"enabled"`
	Customized      bool   `json:"customized"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Deleted         bool   `json:"deleted"`
	FileCount       int    `json:"fileCount"`
}

type SkillsResult struct {
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	Skills  []SkillView `json:"skills"`
}

type SkillDetailResult struct {
	Success bool          `json:"success"`
	Error   string        `json:"error,omitempty"`
	Skill   *domain.Skill `json:"skill,omitempty"`
}

type SkillResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// SkillSkip is one skill that a batch folder import did not import, with reason.
type SkillSkip struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type ImportSummary struct {
	Imported []string    `json:"imported"`
	Skipped  []SkillSkip `json:"skipped"`
}

type ImportSkillsResult struct {
	Success bool          `json:"success"`
	Error   string        `json:"error,omitempty"`
	Summary ImportSummary `json:"summary"`
}

// --- Agent Instructions (Settings → Agent Instructions) ---

// InstructionsListResult carries the editable instruction documents (AGENTS.md,
// USER.md) with derived status to the Agent Instructions page.
type InstructionsListResult struct {
	Success   bool                         `json:"success"`
	Error     string                       `json:"error,omitempty"`
	Documents []domain.InstructionDocument `json:"documents"`
}

// InstructionReadResult carries one instruction document with its content.
type InstructionReadResult struct {
	Success  bool                               `json:"success"`
	Error    string                             `json:"error,omitempty"`
	Document *domain.InstructionDocumentContent `json:"document,omitempty"`
}

// InstructionSaveResult reports a save/reset of an instruction document and the
// honest runtime-refresh status (including dev-override masking).
type InstructionSaveResult struct {
	Success           bool                        `json:"success"`
	Error             string                      `json:"error,omitempty"`
	ContextRefreshed  bool                        `json:"contextRefreshed"`
	EffectiveChanged  bool                        `json:"effectiveChanged"`
	RestartRequired   bool                        `json:"restartRequired"`
	DevOverrideActive bool                        `json:"devOverrideActive"`
	Warning           string                      `json:"warning,omitempty"`
	Document          *domain.InstructionDocument `json:"document,omitempty"`
}

// EffectiveInstructionsResult carries the composed (scrubbed) effective
// instruction block and its source breakdown to the UI.
type EffectiveInstructionsResult struct {
	Success     bool                          `json:"success"`
	Error       string                        `json:"error,omitempty"`
	Effective   *domain.EffectiveInstructions `json:"effective,omitempty"`
	GeneratedAt string                        `json:"generatedAt,omitempty"`
}

// InstructionSourcesResult carries the source inventory to the Sources section.
type InstructionSourcesResult struct {
	Success bool                       `json:"success"`
	Error   string                     `json:"error,omitempty"`
	Sources []domain.InstructionSource `json:"sources"`
}

// --- MCP Client (Block B) ---

// McpConnectionsResult carries the sanitized connection list to the UI.
type McpConnectionsResult struct {
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Connections []domain.McpConnection `json:"connections"`
}

// McpConnectionResult reports one connection operation (sanitized).
type McpConnectionResult struct {
	Success    bool                  `json:"success"`
	Error      string                `json:"error,omitempty"`
	Connection *domain.McpConnection `json:"connection,omitempty"`
}

// McpConnectionTestResult reports a connection-test outcome to the UI.
type McpConnectionTestResult struct {
	Success bool                            `json:"success"`
	Error   string                          `json:"error,omitempty"`
	Result  *domain.McpConnectionTestResult `json:"result,omitempty"`
}

// McpToolView is the UI-facing remote tool (name + description only; the raw
// input schema stays in the agent-facing aw action result).
type McpToolView struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// McpToolsResult carries a connection's remote tools to the UI, with the
// untrusted-content reminder surfaced in the UI copy (not here).
type McpToolsResult struct {
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
	ConnectionID string        `json:"connectionId"`
	Tools        []McpToolView `json:"tools"`
	Truncated    bool          `json:"truncated"`
}

type VaultStatusResponse struct {
	Exists         bool                `json:"exists"`
	Unlocked       bool                `json:"unlocked"`
	VaultDir       string              `json:"vaultDir"`
	HasRecovery    bool                `json:"hasRecovery"`
	CurrentProfile *domain.ProfileInfo `json:"currentProfile"`
}

type OperationResult struct {
	Success        bool                `json:"success"`
	Error          string              `json:"error,omitempty"`
	RecoveryKey    string              `json:"recoveryKey,omitempty"`
	NewRecoveryKey string              `json:"newRecoveryKey,omitempty"`
	Status         VaultStatusResponse `json:"status"`
	Chats          []domain.Chat       `json:"chats,omitempty"`
	Profile        *domain.ProfileInfo `json:"profile,omitempty"`
	Valid          bool                `json:"valid,omitempty"`
}

type FolderDialogResult struct {
	Canceled bool   `json:"canceled"`
	Path     string `json:"path,omitempty"`
}

type SecretResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Value   string `json:"value,omitempty"`
	Exists  bool   `json:"exists"`
}

type PromptDebugModeResponse struct {
	Enabled bool `json:"enabled"`
}

type SubagentModeResponse struct {
	Mode string `json:"mode"`
}

type PromptDebugSnapshot = domain.PromptDebugSnapshot

type ChatSendResult struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	// RunID identifies this turn's stream; the same id rides on every
	// chat:start/delta/done/error event so the frontend can drop late chunks
	// from stopped or stale runs (AW2's runId).
	RunID            string            `json:"runId,omitempty"`
	UserMessage      *domain.Message   `json:"userMessage,omitempty"`
	AssistantMessage *domain.Message   `json:"assistantMessage,omitempty"`
	Reply            domain.AgentReply `json:"reply"`
}

type ChatOperationResult struct {
	Success bool                `json:"success"`
	Error   string              `json:"error,omitempty"`
	Status  VaultStatusResponse `json:"status"`
	Chats   []domain.Chat       `json:"chats,omitempty"`
	Chat    *domain.Chat        `json:"chat,omitempty"`
}

type AudioTranscriptionResult struct {
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
	Text     string `json:"text,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type VoiceCaptureResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type ChatSessionInfo struct {
	Provider            string             `json:"provider"`
	Model               string             `json:"model"`
	Ready               bool               `json:"ready"`
	PlanMode            bool               `json:"planMode"`
	Streaming           bool               `json:"streaming"`
	PartialReply        string             `json:"partialReply,omitempty"`
	CompactReady        bool               `json:"compactReady"`
	GoalReady           bool               `json:"goalReady"`
	MessageCount        int                `json:"messageCount"`
	TokenUsage          *domain.TokenUsage `json:"tokenUsage,omitempty"`
	LastUsage           *domain.TokenUsage `json:"lastUsage,omitempty"`
	ContextWindow       int                `json:"contextWindow,omitempty"`
	ContextUsedPercent  int                `json:"contextUsedPercent,omitempty"`
	CompactionThreshold int                `json:"compactAtPercent,omitempty"`
	Error               string             `json:"error,omitempty"`
}

func ChatSessionInfoFromDomain(info domain.ChatSessionInfo) ChatSessionInfo {
	return ChatSessionInfo{
		Provider:            info.Provider,
		Model:               info.Model,
		Ready:               info.Ready,
		PlanMode:            info.PlanMode,
		Streaming:           info.Streaming,
		PartialReply:        info.PartialReply,
		CompactReady:        info.CompactReady,
		GoalReady:           info.GoalReady,
		MessageCount:        info.MessageCount,
		TokenUsage:          info.TokenUsage,
		LastUsage:           info.LastUsage,
		ContextWindow:       info.ContextWindow,
		ContextUsedPercent:  info.ContextUsedPercent,
		CompactionThreshold: info.CompactionThreshold,
		Error:               info.Error,
	}
}

// APIServerStatus reports one local API server (MCP, REST) to the settings UI.
// ServerIndicatorEntry is one network server's live listening state.
type ServerIndicatorEntry struct {
	Running bool `json:"running"`
	Port    int  `json:"port"`
}

// ServerIndicator aggregates the three network servers for the sidebar
// exposure dot and the Settings › Servers hub. PiP is excluded (local window
// plumbing, not network exposure).
type ServerIndicator struct {
	Rest ServerIndicatorEntry `json:"rest"`
	Mcp  ServerIndicatorEntry `json:"mcp"`
	Web  ServerIndicatorEntry `json:"web"`
}

type APIServerStatus struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	Autostart bool   `json:"autostart"`
	Running   bool   `json:"running"`
	Port      int    `json:"port"`
	URL       string `json:"url,omitempty"`
	// TLSEnabled reports whether HTTPS is on for this server (default false).
	TLSEnabled bool `json:"tlsEnabled"`
	// CoverageWarning is set when TLS is on but the shared certificate does not
	// cover this server's bind identity (decision #4: warn, do not block).
	CoverageWarning string `json:"coverageWarning,omitempty"`
}

// AgentFirewallState is the Agent Firewall settings view: the ordered ruleset,
// the local interfaces available to bind, and the governed service ids. The
// implicit final DENY ALL ALL is not included — the UI renders it as a fixed,
// non-editable row.
type AgentFirewallState struct {
	Success    bool                      `json:"success"`
	Error      string                    `json:"error,omitempty"`
	Rules      []domain.FirewallRule     `json:"rules"`
	Interfaces []domain.NetworkInterface `json:"interfaces"`
	Services   []string                  `json:"services"`
}

// ServerTLSStatus is the sanitized state of the shared TLS certificate. It
// carries NO private key, NO PEM material and NO filesystem path.
type ServerTLSStatus struct {
	Success           bool     `json:"success"`
	Error             string   `json:"error,omitempty"`
	Mode              string   `json:"mode"`
	HasCertificate    bool     `json:"hasCertificate"`
	Ready             bool     `json:"ready"`
	SelfSigned        bool     `json:"selfSigned"`
	Subject           string   `json:"subject,omitempty"`
	Issuer            string   `json:"issuer,omitempty"`
	NotBefore         string   `json:"notBefore,omitempty"`
	NotAfter          string   `json:"notAfter,omitempty"`
	FingerprintSHA256 string   `json:"fingerprintSha256,omitempty"`
	DNSNames          []string `json:"dnsNames,omitempty"`
	IPAddresses       []string `json:"ipAddresses,omitempty"`
	ExpiringSoon      bool     `json:"expiringSoon"`
	CoverageWarning   string   `json:"coverageWarning,omitempty"`
}

// WebServerStatus reports the web-access server to the settings UI. Unlike the
// MCP/REST servers it authenticates with the vault password (no bearer token),
// so it exposes bind mode, session TTL and the detected Tailscale IP instead.
type WebServerStatus struct {
	Success           bool     `json:"success"`
	Error             string   `json:"error,omitempty"`
	Enabled           bool     `json:"enabled"`
	Running           bool     `json:"running"`
	Port              int      `json:"port"`
	URL               string   `json:"url,omitempty"`
	BindMode          string   `json:"bindMode"`
	BindAddr          string   `json:"bindAddr,omitempty"`
	AllowedCIDRs      []string `json:"allowedCIDRs,omitempty"`
	SessionTTLMinutes int      `json:"sessionTTLMinutes"`
	TailscaleIP       string   `json:"tailscaleIp,omitempty"`
	TailscaleDetected bool     `json:"tailscaleDetected"`
	// TLSEnabled reports whether HTTPS is on for web access (default false).
	TLSEnabled bool `json:"tlsEnabled"`
	// CoverageWarning is set when TLS is on but the shared certificate does not
	// cover the web bind identity (decision #4: warn, do not block).
	CoverageWarning string `json:"coverageWarning,omitempty"`
}

// WebBindCandidate is one local IPv4 address the web server could bind to, found
// on an up network interface and classified by reachability so the manual-mode
// picker in Settings can warn proportionally (Kind: loopback | private |
// tailscale | public).
type WebBindCandidate struct {
	Iface string `json:"iface"`
	IP    string `json:"ip"`
	Kind  string `json:"kind"`
}

// WebBindInterfacesResult lists the detected bind candidates for the Web Access
// manual mode, so the settings UI can offer a network-interface picker instead
// of a free-text address field.
type WebBindInterfacesResult struct {
	Success    bool               `json:"success"`
	Error      string             `json:"error,omitempty"`
	Candidates []WebBindCandidate `json:"candidates"`
}

// UserMemoryDocResult carries the v2 living memory document to the Settings
// page. Replaces the v1 UserMemoryListResult / UserMemoryFactResult pair.
type UserMemoryDocResult struct {
	Success bool                  `json:"success"`
	Error   string                `json:"error,omitempty"`
	Doc     *domain.UserMemoryDoc `json:"doc,omitempty"`
}

// UserMemoryListResult carries the long-term user facts to the Settings page.
type UserMemoryListResult struct {
	Success bool                    `json:"success"`
	Error   string                  `json:"error,omitempty"`
	Facts   []domain.UserMemoryFact `json:"facts"`
}

// UserMemoryFactResult reports one saved (added or edited) user fact.
type UserMemoryFactResult struct {
	Success bool                   `json:"success"`
	Error   string                 `json:"error,omitempty"`
	Fact    *domain.UserMemoryFact `json:"fact,omitempty"`
}

// SandboxTestResult carries the result of a TestSandboxCommand dry-run to the
// Settings page test box. The allowed field and reason string mirror what the
// agent receives from the sandbox.test tool action, so the two cannot drift.
type SandboxTestResult struct {
	Allowed bool     `json:"allowed"`
	Reason  string   `json:"reason"`
	Paths   []string `json:"paths,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// SandboxSettings carries the Permissions configuration to the Settings page,
// including the built-in lists (shown as non-removable badges) and the
// resulting top-level folders visible to the agent.
type SandboxSettings struct {
	Mode           string   `json:"mode"`
	AllowedFolders []string `json:"allowedFolders"`
	BuiltinDenied  []string `json:"builtinDenied"`
	BuiltinAllowed []string `json:"builtinAllowed"`
	WorkspaceRoot  string   `json:"workspaceRoot"`
	SelfDev        bool     `json:"selfDev"`
	Roots          []string `json:"roots"`
}

// SandboxSettingsResult wraps the Permissions settings round-trip.
type SandboxSettingsResult struct {
	Success  bool             `json:"success"`
	Error    string           `json:"error,omitempty"`
	Canceled bool             `json:"canceled,omitempty"`
	Settings *SandboxSettings `json:"settings,omitempty"`
}

// ModuleInfo is one workspace-module catalog entry for the frontend.
type ModuleInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	Core        bool   `json:"core"`
	Fixed       bool   `json:"fixed"`
	ComingSoon  bool   `json:"comingSoon"`
	Added       bool   `json:"added"`
	Hidden      bool   `json:"hidden"`
	SidebarPos  int    `json:"sidebarPosition"`
}

// ModulesResult carries the module catalog (with added state) to the frontend.
type ModulesResult struct {
	Success bool         `json:"success"`
	Error   string       `json:"error,omitempty"`
	Modules []ModuleInfo `json:"modules"`
}

// NotesResult carries the Notes module list to the frontend.
type NotesResult struct {
	Success bool          `json:"success"`
	Error   string        `json:"error,omitempty"`
	Notes   []domain.Note `json:"notes"`
}

type PasswordsResult struct {
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Passwords []domain.PasswordEntry `json:"passwords"`
}

type PasswordResult struct {
	Success  bool                  `json:"success"`
	Error    string                `json:"error,omitempty"`
	Password *domain.PasswordEntry `json:"password,omitempty"`
}

type LogsResult struct {
	Success bool              `json:"success"`
	Error   string            `json:"error,omitempty"`
	Logs    []domain.LogEntry `json:"logs"`
}

type LogDatesResult struct {
	Success bool                  `json:"success"`
	Error   string                `json:"error,omitempty"`
	Dates   []domain.LogDateCount `json:"dates"`
}

type LogRetentionResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Deleted int    `json:"deleted"`
}

// NoteResult reports one note operation (create/get/update/delete).
type NoteResult struct {
	Success bool         `json:"success"`
	Error   string       `json:"error,omitempty"`
	Note    *domain.Note `json:"note,omitempty"`
}

// TasksResult carries the Tasks module list to the frontend.
type TasksResult struct {
	Success bool               `json:"success"`
	Error   string             `json:"error,omitempty"`
	Items   []domain.TasksItem `json:"items"`
}

// TasksItemResult reports one tasks-item operation.
type TasksItemResult struct {
	Success bool              `json:"success"`
	Error   string            `json:"error,omitempty"`
	Item    *domain.TasksItem `json:"item,omitempty"`
}

// TasksAttachmentResult reports one attachment operation (add/delete).
type TasksAttachmentResult struct {
	Success    bool                    `json:"success"`
	Error      string                  `json:"error,omitempty"`
	Attachment *domain.TasksAttachment `json:"attachment,omitempty"`
}

// TasksAttachmentDataResult carries one attachment's content as a data URI
// to the module view.
type TasksAttachmentDataResult struct {
	Success    bool                    `json:"success"`
	Error      string                  `json:"error,omitempty"`
	Attachment *domain.TasksAttachment `json:"attachment,omitempty"`
	DataURI    string                  `json:"dataUri,omitempty"`
}

// BrowserStatusResult reports one Agent Browser instance to the module view.
type BrowserStatusResult struct {
	Success   bool                  `json:"success"`
	Error     string                `json:"error,omitempty"`
	Status    *domain.BrowserStatus `json:"status,omitempty"`
	Autostart bool                  `json:"autostart"`
}

type BrowserExecutableResult struct {
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
	Canceled    bool   `json:"canceled,omitempty"`
	Path        string `json:"path,omitempty"`
	DefaultPath string `json:"defaultPath,omitempty"`
	Custom      bool   `json:"custom"`
}

// WallpaperUploadResult reports a user-uploaded wallpaper. ID is the new
// "custom:<file>" id (empty when the picker was canceled), and Custom is the
// full list of stored upload ids after the operation.
type WallpaperUploadResult struct {
	Success  bool     `json:"success"`
	Error    string   `json:"error,omitempty"`
	Canceled bool     `json:"canceled,omitempty"`
	ID       string   `json:"id,omitempty"`
	Custom   []string `json:"custom"`
}

// WallpaperImageResult carries one wallpaper as a base64 data URI for the
// frontend to render (used for user uploads, which are not bundled assets).
type WallpaperImageResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	DataURI string `json:"dataUri,omitempty"`
}

// BrowserTabsResult lists the open tabs of an Agent Browser instance.
type BrowserTabsResult struct {
	Success bool                `json:"success"`
	Error   string              `json:"error,omitempty"`
	Tabs    []domain.BrowserTab `json:"tabs"`
}

// BrowserScreenshotResult carries one page screenshot as a PNG data URI.
type BrowserScreenshotResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	DataURI string `json:"dataUri,omitempty"`
}

type PlanModeResult struct {
	Success bool   `json:"success"`
	Enabled bool   `json:"enabled"`
	Error   string `json:"error,omitempty"`
}

// ChatModelResult is the outcome of a per-chat /model change. On success it
// carries the provider/model now in effect so the frontend can show a system
// bubble; Cleared marks a revert to the global default.
type ChatModelResult struct {
	Success      bool   `json:"success"`
	Error        string `json:"error,omitempty"`
	Provider     string `json:"provider,omitempty"`
	ProviderName string `json:"providerName,omitempty"`
	Model        string `json:"model,omitempty"`
	Cleared      bool   `json:"cleared,omitempty"`
}

// ThemeSuggestionResult carries an AI-suggested theme token map back to the
// frontend. When Success is false the frontend falls back to localThemeFromPalette.
type ThemeSuggestionResult struct {
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
	Suggestion map[string]interface{} `json:"suggestion,omitempty"`
}

// MacosPermission is one row in the macOS TCC permission inventory.
type MacosPermission struct {
	// ID is a stable machine-readable key used by the probe API.
	ID string `json:"id"`
	// Name is the human-readable display label.
	Name string `json:"name"`
	// Why is one sentence describing what breaks without this permission.
	Why string `json:"why"`
	// SettingsPane is the System Settings breadcrumb (human-readable).
	SettingsPane string `json:"settingsPane"`
	// DeepLink is the x-apple.systempreferences: URL for the Open button.
	DeepLink string `json:"deepLink"`
	// HasProbe indicates whether a Test button is available for this row.
	HasProbe bool `json:"hasProbe"`
}

// MacosPermissionsResult holds the platform TCC permission inventory.
// Items is empty on non-macOS builds so the frontend hides the card.
type MacosPermissionsResult struct {
	Items []MacosPermission `json:"items"`
}

// MacosProbeResult is the outcome of a single-permission status probe.
type MacosProbeResult struct {
	// ID is the permission id that was probed.
	ID string `json:"id"`
	// Status is "granted", "denied", or "unknown".
	Status string `json:"status"`
	// Detail is a short human-readable message (may be empty).
	Detail string `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
}

// CodesignTrustResult reports whether the app's code-signing certificate is
// trusted by macOS. When Supported is true, HasCertificate is false for ad-hoc
// builds; Trusted reflects the trust evaluation of the leaf certificate. An
// untrusted local certificate makes Keychain "Always Allow" grants expire on
// every access — the Security page offers a one-click fix.
type CodesignTrustResult struct {
	// Supported is true on macOS builds that can inspect the signature.
	Supported bool `json:"supported"`
	// HasCertificate is false when the app is ad-hoc signed (no identity).
	HasCertificate bool `json:"hasCertificate"`
	// Trusted is true when the signing certificate passes code-signing trust.
	Trusted bool `json:"trusted"`
	// CertificateName is the leaf certificate's common name (may be empty).
	CertificateName string `json:"certificateName,omitempty"`
}

// ObsidianSettings is the Obsidian module setup shown in the module page.
type ObsidianSettings struct {
	Enabled       bool     `json:"enabled"`
	VaultDir      string   `json:"vaultDir"`
	WriteEnabled  bool     `json:"writeEnabled"`
	DeleteEnabled bool     `json:"deleteEnabled"`
	AlwaysRead    []string `json:"alwaysRead"`
	// AlwaysReadChars is the total size the always-read notes inject into
	// every prompt; the page warns when it gets heavy.
	AlwaysReadChars int `json:"alwaysReadChars"`
}

// ObsidianFileDialogResult is the outcome of the always-read file picker.
type ObsidianFileDialogResult struct {
	// Path is the picked note, relative to the vault folder (slash-separated).
	Path     string `json:"path,omitempty"`
	Canceled bool   `json:"canceled,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ObsidianTestResult is the outcome of the module page's Test access button.
type ObsidianTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
