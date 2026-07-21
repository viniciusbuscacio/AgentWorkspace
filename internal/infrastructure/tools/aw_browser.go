package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/infrastructure/externalsafe"
)

// BrowserFuncs are the injected Agent Browser operations (the composition
// root wires them to application use cases over the CDP manager).
type BrowserFuncs struct {
	Start   func(ctx context.Context, id string, opts domain.BrowserStartOptions) (any, error)
	Stop    func(ctx context.Context, id string) (any, error)
	Status  func(ctx context.Context, id string) (any, error)
	Tabs    func(ctx context.Context, id string) (any, error)
	Command func(ctx context.Context, id string, command string, params map[string]any) (any, error)
}

// browserModuleIDs maps the optional "browser" arg to module ids.
var browserModuleIDs = map[string]string{
	"chrome":         "browser-chrome",
	"edge":           "browser-edge",
	"browser-chrome": "browser-chrome",
	"browser-edge":   "browser-edge",
}

const browserTabCacheTTL = 2 * time.Minute

// resolveBrowserID picks the target browser module. Only added modules are
// eligible (the Apps toggle is the fence; hidden-but-added still counts).
// Connection state comes from the browser backend (CDP status), not from the
// sidebar: with no explicit browser, use the single added browser for action
// flows, then fall back to the single running browser.
func (w *workspace) resolveBrowserID(ctx context.Context, args map[string]any) (string, error) {
	return w.resolveBrowserIDWithMode(ctx, args, false)
}

// resolveBrowserStatusID is for read-only connection questions. It prefers
// the single actually-running browser over the module list, so "are you
// connected to my browser?" reflects CDP truth rather than sidebar state.
func (w *workspace) resolveBrowserStatusID(ctx context.Context, args map[string]any) (string, error) {
	return w.resolveBrowserIDWithMode(ctx, args, true)
}

func (w *workspace) resolveBrowserIDWithMode(ctx context.Context, args map[string]any, preferRunning bool) (string, error) {
	requested, _, err := awStringArg(args, "browser")
	if err != nil {
		return "", err
	}
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" {
		id, ok := browserModuleIDs[requested]
		if !ok {
			return "", fmt.Errorf("unknown browser %q (use chrome or edge)", requested)
		}
		// The module toggle is the fence: an explicit browser choice must not
		// reach a browser whose module the user has not added.
		if !w.moduleAdded(id) {
			return "", fmt.Errorf("the %s browser module is not in the workspace — add it in Apps (module.add %q) first", requested, id)
		}
		return id, nil
	}
	if id, ok, err := w.browserIDForTab(ctx, args); ok || err != nil {
		return id, err
	}
	if preferRunning {
		if id, ok, err := w.singleRunningBrowserID(ctx); ok || err != nil {
			return id, err
		}
	}
	added := make([]string, 0, 2)
	for _, id := range []string{"browser-chrome", "browser-edge"} {
		if w.moduleAdded(id) {
			added = append(added, id)
		}
	}
	switch len(added) {
	case 1:
		return added[0], nil
	case 0:
		if id, ok, err := w.singleRunningBrowserID(ctx); ok || err != nil {
			return id, err
		}
		return "", fmt.Errorf("no browser module is in the workspace")
	default:
		return "", fmt.Errorf("both browser modules are added — pass browser: chrome or edge")
	}
}

func (w *workspace) browserIDForTab(ctx context.Context, args map[string]any) (string, bool, error) {
	tab, _ := args["tab"].(string)
	tab = strings.TrimSpace(tab)
	if tab == "" || w.browser == nil || w.browser.Tabs == nil {
		return "", false, nil
	}
	if id, ok := w.cachedBrowserIDForTab(tab); ok {
		return id, true, nil
	}
	matches := make([]string, 0, 1)
	for _, id := range []string{domain.BrowserModuleChrome, domain.BrowserModuleEdge} {
		result, err := w.browser.Tabs(ctx, id)
		if err != nil {
			continue
		}
		if browserTabsContainID(result, tab) {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 0:
		return "", false, nil
	case 1:
		w.rememberBrowserTab(tab, matches[0])
		return matches[0], true, nil
	default:
		return "", false, fmt.Errorf("tab %q exists in multiple browser modules — pass browser: chrome or edge", tab)
	}
}

func (w *workspace) cachedBrowserIDForTab(tabID string) (string, bool) {
	w.browserTabMu.Lock()
	defer w.browserTabMu.Unlock()
	if w.browserTabCache == nil {
		return "", false
	}
	entry, ok := w.browserTabCache[tabID]
	if !ok {
		return "", false
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(w.browserTabCache, tabID)
		return "", false
	}
	return entry.BrowserID, true
}

func (w *workspace) rememberBrowserTab(tabID string, browserID string) {
	if tabID == "" || browserID == "" {
		return
	}
	w.browserTabMu.Lock()
	defer w.browserTabMu.Unlock()
	if w.browserTabCache == nil {
		w.browserTabCache = map[string]browserTabCacheEntry{}
	}
	w.browserTabCache[tabID] = browserTabCacheEntry{BrowserID: browserID, ExpiresAt: time.Now().Add(browserTabCacheTTL)}
}

func (w *workspace) rememberBrowserTabs(browserID string, result any) {
	for _, tabID := range browserTabIDs(result) {
		w.rememberBrowserTab(tabID, browserID)
	}
}

func browserTabIDs(result any) []string {
	out := []string{}
	switch tabs := result.(type) {
	case []domain.BrowserTab:
		for _, tab := range tabs {
			if tab.ID != "" {
				out = append(out, tab.ID)
			}
		}
	case []map[string]any:
		for _, tab := range tabs {
			if id, _ := tab["id"].(string); id != "" {
				out = append(out, id)
			}
		}
	case []any:
		for _, raw := range tabs {
			if tab, ok := raw.(map[string]any); ok {
				if id, _ := tab["id"].(string); id != "" {
					out = append(out, id)
				}
			}
		}
	}
	return out
}

func browserTabsContainID(result any, tabID string) bool {
	switch tabs := result.(type) {
	case []domain.BrowserTab:
		for _, tab := range tabs {
			if tab.ID == tabID {
				return true
			}
		}
	case []map[string]any:
		for _, tab := range tabs {
			if id, _ := tab["id"].(string); id == tabID {
				return true
			}
		}
	case []any:
		for _, raw := range tabs {
			if tab, ok := raw.(map[string]any); ok {
				if id, _ := tab["id"].(string); id == tabID {
					return true
				}
			}
		}
	}
	return false
}

func (w *workspace) singleRunningBrowserID(ctx context.Context) (string, bool, error) {
	running := w.runningBrowserIDs(ctx)
	switch len(running) {
	case 1:
		return running[0], true, nil
	case 2:
		return "", false, fmt.Errorf("both browser modules are running — pass browser: chrome or edge")
	default:
		return "", false, nil
	}
}

func (w *workspace) runningBrowserIDs(ctx context.Context) []string {
	if w.browser == nil || w.browser.Status == nil {
		return nil
	}
	running := make([]string, 0, 2)
	for _, id := range []string{domain.BrowserModuleChrome, domain.BrowserModuleEdge} {
		status, err := w.browser.Status(ctx, id)
		if err != nil {
			continue
		}
		if browserResultRunning(status) {
			running = append(running, id)
		}
	}
	return running
}

func browserResultRunning(result any) bool {
	switch status := result.(type) {
	case domain.BrowserStatus:
		return status.Running
	case map[string]any:
		running, _ := status["running"].(bool)
		return running
	default:
		return false
	}
}

// registerBrowserActions adds the Agent Browser action group, mirroring the
// ui.* design (snapshot refs, click/fill by ref or selector). Browser
// connection checks intentionally use browser.status/BrowserFuncs, not the
// module list, because the browser process can be running independently of
// whether the sidebar module is currently added.
func registerBrowserActions(reg map[string]AwActionHandler, surface Surface) {
	browserCommand := func(command string, buildParams func(args map[string]any) (map[string]any, error)) AwActionHandler {
		return func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
			id, err := w.resolveBrowserID(ctx, args)
			if err != nil {
				return "", err
			}
			params, err := buildParams(args)
			if err != nil {
				return "", err
			}
			switch command {
			case "click", "fill":
				if err := w.requireExternalActionGuard(ctx, "browser."+command, domain.ExternalActionKind(command), args); err != nil {
					return "", err
				}
				if blocked, isBlocked := w.precheckBrowserTarget(ctx, id, params); isBlocked {
					return awJSON(blocked)
				}
			case "cdp":
				if err := w.requireBrowserCDPGuard(ctx, surface, args); err != nil {
					return "", err
				}
			}
			result, err := w.browser.Command(ctx, id, command, params)
			if err != nil {
				return "", err
			}
			if command == "click" || command == "fill" {
				if blocked, isBlocked := w.postcheckBrowserNavigation(ctx, id, args); isBlocked {
					return awJSON(blocked)
				}
			}
			if command == "snapshot" {
				return awJSON(w.processExternalContent(ctx, result, externalProcessOptions{
					SourceType: externalsafe.SourceWeb,
					Origin:     "browser.snapshot",
					Mode:       domain.ExternalContentModeDistill,
					MaxChars:   w.browserReadMaxChars(),
				}))
			}
			if command == "cdp" {
				// cdp is a low-level escape hatch (Runtime.evaluate, DOM.*) that can
				// pull raw page content past the snapshot sanitizer. Keep its result
				// intact for the caller but scan it for taint + attach metadata.
				return awJSON(w.processExternalContent(ctx, result, externalProcessOptions{
					SourceType: externalsafe.SourceWeb,
					Origin:     "browser.cdp",
					Mode:       domain.ExternalContentModeDistill,
					MaxChars:   w.browserReadMaxChars(),
				}))
			}
			if command == "screenshot" {
				return awJSON(map[string]any{
					"content":         browserScreenshotModelResult(result),
					"external_safety": externalVisualSafety(domain.ExternalSourceWeb, "browser.screenshot"),
				})
			}
			return awJSON(result)
		}
	}

	reg["browser.start"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := w.resolveBrowserID(ctx, args)
		if err != nil {
			return "", err
		}
		headless, err := awBoolArg(args, "headless", false)
		if err != nil {
			return "", err
		}
		rawProfile, _, err := awStringArg(args, "profile")
		if err != nil {
			return "", err
		}
		profile, err := domain.ParseBrowserProfile(rawProfile)
		if err != nil {
			return "", err
		}
		result, err := w.browser.Start(ctx, id, domain.BrowserStartOptions{
			Headless: headless,
			Profile:  profile,
		})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["browser.stop"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := w.resolveBrowserID(ctx, args)
		if err != nil {
			return "", err
		}
		result, err := w.browser.Stop(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["browser.status"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := w.resolveBrowserStatusID(ctx, args)
		if err != nil {
			return "", err
		}
		result, err := w.browser.Status(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["browser.tabs"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := w.resolveBrowserStatusID(ctx, args)
		if err != nil {
			return "", err
		}
		result, err := w.browser.Tabs(ctx, id)
		if err != nil {
			return "", err
		}
		w.rememberBrowserTabs(id, result)
		return awJSON(w.processExternalContent(ctx, result, externalProcessOptions{
			SourceType: externalsafe.SourceWeb,
			Origin:     "browser.tabs",
			Mode:       domain.ExternalContentModePreserveVerbatim,
			MaxChars:   externalsafe.DefaultMaxChars,
		}))
	}

	reg["browser.new_tab"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := w.resolveBrowserID(ctx, args)
		if err != nil {
			return "", err
		}
		target, _, err := awStringArg(args, "url")
		if err != nil {
			return "", err
		}
		params := map[string]any{}
		if strings.TrimSpace(target) != "" {
			if err := w.requireExternalActionGuard(ctx, "browser.new_tab", domain.ExternalActionNavigate, args); err != nil {
				return "", err
			}
			if blocked, isBlocked := newNavigationGuard(w.effectiveSandboxPolicy(ctx)).check(ctx, target); isBlocked {
				return awJSON(blocked)
			}
			params["url"] = target
		}
		result, err := w.browser.Command(ctx, id, "new_tab", params)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	// navigate is not routed through browserCommand: every browser navigation
	// path must pass the same guard (disk fence + internal destination block).
	reg["browser.navigate"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := w.resolveBrowserID(ctx, args)
		if err != nil {
			return "", err
		}
		target, err := awRequiredStringArg(args, "url")
		if err != nil {
			return "", err
		}
		if blocked, isBlocked := newNavigationGuard(w.effectiveSandboxPolicy(ctx)).check(ctx, target); isBlocked {
			return awJSON(blocked)
		}
		if err := w.requireExternalActionGuard(ctx, "browser.navigate", domain.ExternalActionNavigate, args); err != nil {
			return "", err
		}
		result, err := w.browser.Command(ctx, id, "navigate", withTabParam(args, map[string]any{"url": target}))
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["browser.snapshot"] = browserCommand("snapshot", func(args map[string]any) (map[string]any, error) {
		params := map[string]any{}
		if maxNodes, has, err := awIntArg(args, "max"); err != nil {
			return nil, err
		} else if has {
			params["max"] = float64(maxNodes)
		}
		return withTabParam(args, params), nil
	})

	reg["browser.click"] = browserCommand("click", func(args map[string]any) (map[string]any, error) {
		params, err := uiTargetParams(args)
		if err != nil {
			return nil, err
		}
		return withTabParam(args, params), nil
	})

	reg["browser.fill"] = browserCommand("fill", func(args map[string]any) (map[string]any, error) {
		params, err := uiTargetParams(args)
		if err != nil {
			return nil, err
		}
		value, _, err := awStringArg(args, "value")
		if err != nil {
			return nil, err
		}
		params["value"] = value
		return withTabParam(args, params), nil
	})

	reg["browser.screenshot"] = browserCommand("screenshot", func(args map[string]any) (map[string]any, error) {
		return withTabParam(args, map[string]any{}), nil
	})

	reg["browser.cdp"] = browserCommand("cdp", func(args map[string]any) (map[string]any, error) {
		method, err := awRequiredStringArg(args, "method")
		if err != nil {
			return nil, err
		}
		params := map[string]any{"method": method}
		if rawParams, ok := args["params"]; ok && rawParams != nil {
			paramMap, ok := rawParams.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("params must be an object")
			}
			params["params"] = paramMap
		}
		if target, has, err := awStringArg(args, "target"); err != nil {
			return nil, err
		} else if has {
			params["target"] = target
		}
		return withTabParam(args, params), nil
	})

	reg["browser.close_tab"] = browserCommand("close_tab", func(args map[string]any) (map[string]any, error) {
		params := map[string]any{}
		for _, key := range []string{"url", "title"} {
			if v, has, err := awStringArg(args, key); err != nil {
				return nil, err
			} else if has {
				params[key] = v
			}
		}
		return withTabParam(args, params), nil
	})

}

func (w *workspace) requireBrowserCDPGuard(ctx context.Context, _ Surface, args map[string]any) error {
	// Page-content reads (Runtime.evaluate over a read-shaped expression) are
	// data extraction, not execution: their result comes back sanitized,
	// enveloped and tainted. Only mutating CDP calls need the execute guard.
	readProbe := map[string]any{"method": argString(args["method"])}
	if rawParams, ok := args["params"].(map[string]any); ok {
		readProbe["params"] = rawParams
	}
	if isBrowserCDPRead(readProbe) {
		return nil
	}
	if err := w.requireExternalActionGuard(ctx, "browser.cdp", domain.ExternalActionExecute, args); err != nil {
		return err
	}
	approved, err := w.requireConfirmationStrict(ctx, ConfirmRequest{
		Tool:    "browser.cdp",
		Summary: "Browser CDP execution confirmation required\n\nbrowser.cdp can execute low-level commands in a logged-in browser session.",
		Args: map[string]any{
			"action_kind": string(domain.ExternalActionExecute),
			"method":      argString(args["method"]),
		},
	})
	if err != nil {
		return err
	}
	if !approved {
		return ErrConfirmationDenied
	}
	return nil
}

// browserReadMaxChars is the sanitizer cap for page reads. Reads happen
// directly in the main chat (the browser-read subagent is gone): news portals
// easily exceed the 4k default, so page reads get a larger window — still
// sanitized, enveloped and tainted like any external content.
func (w *workspace) browserReadMaxChars() int {
	return 16000
}

func isBrowserCDPRead(params map[string]any) bool {
	method := strings.TrimSpace(argString(params["method"]))
	if method != "Runtime.evaluate" {
		return false
	}
	rawParams, _ := params["params"].(map[string]any)
	expr := strings.ToLower(strings.TrimSpace(argString(rawParams["expression"])))
	if expr == "" {
		return true
	}
	for _, marker := range []string{
		".click(",
		"click()",
		"dispatchEvent(",
		".focus(",
		".blur(",
		".submit(",
		"setAttribute(",
		"removeAttribute(",
		"classList.",
		".value =",
		".checked =",
	} {
		if strings.Contains(expr, strings.ToLower(marker)) {
			return false
		}
	}
	return true
}

func browserScreenshotModelResult(result any) map[string]any {
	out := map[string]any{
		"captured": true,
		"note":     "Screenshot captured. The binary image payload is omitted from the tool result to avoid flooding the model context; use browser.snapshot or browser.cdp for text extraction.",
	}
	m, ok := result.(map[string]any)
	if !ok {
		return out
	}
	dataURI, _ := m["dataUri"].(string)
	if strings.TrimSpace(dataURI) != "" {
		out["image_payload_omitted"] = true
		out["data_uri_chars"] = len(dataURI)
	}
	for _, key := range []string{"width", "height"} {
		if value, ok := m[key]; ok {
			out[key] = value
		}
	}
	return out
}

// cutPrefixFold is strings.CutPrefix with an ASCII case-insensitive match.
func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return s, false
	}
	return s[len(prefix):], true
}

func withTabParam(args map[string]any, params map[string]any) map[string]any {
	if tab, ok := args["tab"].(string); ok && strings.TrimSpace(tab) != "" {
		params["tab"] = strings.TrimSpace(tab)
	}
	return params
}

func (w *workspace) precheckBrowserTarget(ctx context.Context, id string, params map[string]any) (map[string]any, bool) {
	result, err := w.browser.Command(ctx, id, "navigationTarget", params)
	if err != nil {
		blocked := blockedNavigation("", fmt.Sprintf("could not inspect browser target safely: %v", err))
		w.logBrowserNavigationBlocked(ctx, id, "precheck", blocked, w.effectiveSandboxPolicy(ctx).Config.Mode)
		return blocked, true
	}
	payload, _ := result.(map[string]any)
	target, _ := payload["url"].(string)
	if strings.TrimSpace(target) == "" {
		return nil, false
	}
	policy := w.effectiveSandboxPolicy(ctx)
	blocked, isBlocked := newNavigationGuard(policy).check(ctx, target)
	if isBlocked {
		w.logBrowserNavigationBlocked(ctx, id, "precheck", blocked, policy.Config.Mode)
	}
	return blocked, isBlocked
}

func (w *workspace) postcheckBrowserNavigation(ctx context.Context, id string, args map[string]any) (map[string]any, bool) {
	result, err := w.browser.Command(ctx, id, "currentURL", withTabParam(args, map[string]any{}))
	if err != nil {
		blocked := blockedNavigation("", fmt.Sprintf("could not verify browser destination safely: %v", err))
		w.logBrowserNavigationBlocked(ctx, id, "postcheck", blocked, w.effectiveSandboxPolicy(ctx).Config.Mode)
		return blocked, true
	}
	payload, _ := result.(map[string]any)
	target, _ := payload["url"].(string)
	if strings.TrimSpace(target) == "" {
		return nil, false
	}
	policy := w.effectiveSandboxPolicy(ctx)
	blocked, isBlocked := newNavigationGuard(policy).check(ctx, target)
	if isBlocked {
		blocked["reason"] = "navigation reached a blocked destination after the action: " + fmt.Sprint(blocked["reason"])
		w.logBrowserNavigationBlocked(ctx, id, "postcheck", blocked, policy.Config.Mode)
	}
	return blocked, isBlocked
}

func (w *workspace) logBrowserNavigationBlocked(ctx context.Context, id string, stage string, blocked map[string]any, mode domain.SandboxMode) {
	if blocked == nil {
		return
	}
	w.logExternal(ctx, ExternalLogEvent{
		Event:      "browser.navigation.blocked",
		SourceType: domain.ExternalSourceWeb,
		Origin:     "browser.navigation." + strings.TrimSpace(stage),
		Status:     "blocked",
		Attributes: map[string]any{
			"browser.id":    id,
			"stage":         stage,
			"target":        fmt.Sprint(blocked["target"]),
			"reason":        fmt.Sprint(blocked["reason"]),
			"sandbox.mode":  string(mode),
			"blocked.paths": blocked["blockedPaths"],
		},
	})
}
