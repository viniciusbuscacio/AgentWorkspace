package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"aw/internal/domain"
	"google.golang.org/adk/tool"
)

// AwActionHandler executes one action inside the multiplexed aw dispatcher.
// Handlers take a plain context.Context so the same registry can serve the ADK
// agent tool and transport-agnostic callers (e.g. a future MCP server).
type AwActionHandler func(ctx context.Context, args map[string]any, w *workspace) (string, error)

type awArgs struct {
	Action string `json:"action"`
	Args   string `json:"args,omitempty"`
}

type awResult struct {
	Action string `json:"action"`
	Result string `json:"result"`
}

// awDispatch is the thin ADK adapter for the multiplexed aw tool.
func (w *workspace) awDispatch(ctx tool.Context, in awArgs) (awResult, error) {
	base := contextOrBackground(ctx)
	if ctx != nil && domain.ExternalTaintScope(base) == "" {
		// No per-turn scope was set upstream (e.g. by the chat, so attachment
		// taint and tool taint share a scope). Fall back to the ADK invocation id
		// so tool reads still scope per turn without bleeding across turns/chats.
		base = domain.WithExternalTaintScope(base, ctx.InvocationID())
	}
	return w.dispatchAction(base, in)
}

// dispatchAction routes one aw action through the registry. It is the
// transport-agnostic entry point shared by the agent tool and external
// callers.
func (w *workspace) dispatchAction(ctx context.Context, in awArgs) (awResult, error) {
	action := strings.TrimSpace(in.Action)
	if action == "" {
		return awResult{}, fmt.Errorf("action is required (use aw.actions to list)")
	}
	registry := w.awRegistry()
	handler, ok := registry[action]
	if !ok {
		return awResult{}, fmt.Errorf("unknown action %q (use aw.actions to list)", action)
	}
	args := map[string]any{}
	if strings.TrimSpace(in.Args) != "" {
		if err := json.Unmarshal([]byte(in.Args), &args); err != nil {
			return awResult{}, fmt.Errorf("args must be a JSON object: %w", err)
		}
		if args == nil {
			args = map[string]any{}
		}
	}
	logAttrs := actionLogAttributes(action, args, len(in.Args))
	started := time.Now()
	w.logAction(ctx, ActionLogEvent{Action: action, Event: "tool.call.started", Status: "ok", Attributes: logAttrs})
	out, err := handler(ctx, args, w)
	if err != nil {
		status := "error"
		event := "tool.call.failed"
		if err == ErrConfirmationDenied {
			status = "denied"
			event = "tool.confirmation.denied"
		}
		w.logAction(ctx, ActionLogEvent{
			Action:     action,
			Event:      event,
			Status:     status,
			DurationMs: time.Since(started).Milliseconds(),
			Err:        err,
			Attributes: logAttrs,
		})
		return awResult{}, err
	}
	event := "tool.call.completed"
	status := "ok"
	if strings.Contains(out, `"blocked": true`) {
		event = "tool.call.blocked"
		status = "blocked"
	}
	w.logAction(ctx, ActionLogEvent{
		Action:     action,
		Event:      event,
		Status:     status,
		DurationMs: time.Since(started).Milliseconds(),
		OutputSize: len(out),
		Attributes: logAttrs,
	})
	return awResult{Action: action, Result: out}, nil
}

func (w *workspace) logAction(ctx context.Context, event ActionLogEvent) {
	if w.logActionFn == nil || strings.HasPrefix(event.Action, "logs.") {
		return
	}
	w.logActionFn(ctx, event)
}

func actionLogAttributes(action string, args map[string]any, rawBytes int) map[string]any {
	attrs := map[string]any{
		"arg.bytes": rawBytes,
		"arg.count": len(args),
		"arg.keys":  sortedArgKeys(args),
	}
	switch action {
	case "browser.snapshot":
		if value, ok := args["max"]; ok {
			attrs["browser.max"] = numericLogValue(value)
		}
		attrs["browser.tab_present"] = strings.TrimSpace(argString(args["tab"])) != ""
	case "browser.cdp":
		if method := strings.TrimSpace(argString(args["method"])); method != "" {
			attrs["cdp.method"] = method
		}
		if target := strings.TrimSpace(argString(args["target"])); target != "" {
			attrs["cdp.target"] = target
		}
		attrs["cdp.params_present"] = args["params"] != nil
	case "browser.navigate", "browser.new_tab":
		if host := safeURLHost(argString(args["url"])); host != "" {
			attrs["url.host"] = host
		}
	case "fs.read", "fs.write", "fs.edit", "document.read_safe", "visual.read_safe":
		if path := strings.TrimSpace(argString(args["path"])); path != "" {
			attrs["path.hash"] = hashForToolLog(path)
		}
	case "mcp.tools.call", "mcp.tools.list":
		// Safe metadata only: never the tool arguments (may carry secrets) or the
		// remote output. connection id and tool name are non-secret identifiers.
		if id := strings.TrimSpace(argString(args["connectionId"])); id != "" {
			attrs["mcp.connection_id"] = id
		}
		if tool := strings.TrimSpace(argString(args["tool"])); tool != "" {
			attrs["mcp.tool"] = tool
		}
	}
	return attrs
}

func sortedArgKeys(args map[string]any) []string {
	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if keys == nil {
		return []string{}
	}
	return keys
}

func numericLogValue(value any) any {
	switch n := value.(type) {
	case float64, int, int64:
		return n
	default:
		return nil
	}
}

func safeURLHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		return ""
	}
	afterScheme := raw[strings.Index(raw, "://")+3:]
	host := afterScheme
	if idx := strings.IndexAny(host, "/?#"); idx >= 0 {
		host = host[:idx]
	}
	return strings.TrimSpace(host)
}

func argString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func hashForToolLog(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func (w *workspace) awRegistry() map[string]AwActionHandler {
	reg := map[string]AwActionHandler{}
	// The "hands" — file, document, office, shell and git tools — are
	// sandbox-governed chat capabilities, not a self-dev privilege: if the
	// user set permit_all, it IS permit all; block_all/permit_list gate every
	// call. External MCP/REST dispatchers keep them off outside self-dev (the
	// composition root clears AllowShell there). Only system.* stays self-dev
	// (its selfcode map and state hooks only exist with the repo as root).
	hands := w.allowShell || w.selfManage
	if w.allowShell {
		registerShellActions(reg)
		registerGitActions(reg)
	}
	if hands {
		registerFsActions(reg)
		// Enforced "read" path for non-text files: convert to text and quarantine
		// it through externalsafe + taint. document.read_safe (PDF) is
		// deterministic; visual.read_safe (image OCR) needs an isolated vision
		// model, so it registers only when one is wired (fail-closed).
		reg["document.read_safe"] = documentReadSafeAction
		if w.visualExtractFn != nil {
			reg["visual.read_safe"] = visualReadSafeAction
		}
		// Office documents (.docx/.xlsx/.pptx) are ZIP+XML: fs.read/fs.write
		// would see raw archive bytes, so they get dedicated read/edit/create
		// actions that preserve formatting.
		registerOfficeActions(reg)
	}
	if w.selfManage {
		registerSystemActions(reg)
	}
	if w.spawnFn != nil || w.selfManage {
		registerSubagentActions(reg)
	}
	if w.control != nil {
		registerAppActions(reg)
		registerChatActions(reg)
		registerModuleActions(reg)
	}
	if w.uiFn != nil {
		registerUIActions(reg)
	}
	if w.userMemoryFn != nil || w.chatSearchFn != nil || w.chatHistoryFn != nil || w.chatCatalogFn != nil {
		registerMemoryActions(reg)
	}
	if w.skillCatalogFn != nil || w.skillReadFn != nil {
		registerSkillActions(reg)
	}
	if w.skillManage != nil {
		registerSkillManageActions(reg)
	}
	if w.webRead != nil {
		registerWebReadActions(reg)
	}
	// Always-on: the agent can inspect and dry-run the permissions fence in
	// every chat (sandbox.set_mode answers with a deterministic refusal).
	registerSandboxActions(reg)
	// Always-on: Google Workspace (gws CLI). The whole gws surface is exposed
	// through gws.call; reads of Gmail message bodies are fenced behind the
	// quarantine pipeline (gws.gmail.read_safe).
	registerGwsActions(reg)
	// Product capability (not self-dev): native system diagnostics. Registered
	// whenever the probe is wired; diagnostics.capabilities answers in every
	// mode and the detailed actions self-gate on the Permissions policy.
	if w.diagnostics != nil {
		registerDiagnosticsActions(reg)
	}
	// Product capability: Agent Instructions (Settings → Agent Instructions).
	// Registered whenever the vault-backed instruction funcs are wired.
	if w.instructions != nil {
		registerInstructionActions(reg)
	}
	// Module-scoped action groups: registered only while the module is added
	// (checked per dispatch, so add/remove takes effect without a restart).
	if w.passwords != nil && w.moduleAdded("passwords") {
		registerPasswordsActions(reg)
	}
	if w.notes != nil && w.moduleAdded("notes") {
		registerNotesActions(reg)
	}
	if w.tasks != nil && w.moduleAdded("tasks") {
		registerTasksActions(reg)
	}
	if w.obsidian != nil && w.moduleAdded("obsidian") {
		registerObsidianActions(reg)
	}
	// logs.list is read-only diagnostics and Logs is a Settings surface, not a
	// workspace module — the action registers whenever the port is wired.
	if w.logs != nil {
		registerLogsActions(reg)
	}
	if w.mcpConnections != nil && w.moduleAdded("mcp-client") {
		registerMcpConnectionActions(reg)
	}
	// Browser control is fenced like every other module: the browser.* and
	// gmail_web.* actions exist only while at least one browser module is
	// added ("Allow the Agent to control..." toggle in Apps). Hidden-but-added
	// still counts — hiding a sidebar card never removes capabilities.
	if w.browser != nil && (w.moduleAdded(domain.BrowserModuleChrome) || w.moduleAdded(domain.BrowserModuleEdge)) {
		registerGmailWebActions(reg)
		registerBrowserActions(reg, w.surface)
	}
	if len(w.allowedActions) > 0 {
		for action := range reg {
			if !w.allowedActions[action] {
				delete(reg, action)
			}
		}
	}
	reg["aw.actions"] = func(_ context.Context, _ map[string]any, _ *workspace) (string, error) {
		keys := make([]string, 0, len(reg))
		for key := range reg {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return awJSON(keys)
	}
	return reg
}

func awJSON(value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func awRequiredStringArg(args map[string]any, name string) (string, error) {
	value, ok, err := awStringArg(args, name)
	if err != nil {
		return "", err
	}
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func awStringArg(args map[string]any, name string) (string, bool, error) {
	value, ok := args[name]
	if !ok || value == nil {
		return "", false, nil
	}
	text, ok := value.(string)
	if !ok {
		return "", true, fmt.Errorf("%s must be a string", name)
	}
	return text, true, nil
}
