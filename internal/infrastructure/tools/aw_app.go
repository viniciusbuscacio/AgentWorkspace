package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// hexColorPattern matches exactly "#rrggbb" (6 hex digits).
var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// AppControl gives aw actions control over the running application: UI
// preferences, navigation, providers, chats and the vault. It is implemented
// in the interface layer (composition root) so this package stays decoupled
// from the concrete App type, mirroring pip.Backend.
type AppControl interface {
	SetTheme(theme string) error
	// SetCustomTheme creates or updates a custom theme in config.json and
	// applies it in the UI. name must be non-empty; tokens must include the 7
	// required hex-color keys. Returns the assigned id.
	SetCustomTheme(name string, tokens map[string]string) (string, error)
	// DeleteCustomTheme removes a custom theme from config.json.
	DeleteCustomTheme(id string) error
	SetWallpaper(id string) error
	// UploadWallpaper imports the image at an absolute path as a custom
	// wallpaper and selects it. Returns the new "custom:<file>" id.
	UploadWallpaper(path string) (string, error)
	SetFont(family string, size int) error
	SetZoomPercent(percent int) error
	Navigate(view string, chatID string) error
	AppState(ctx context.Context) (any, error)
	LockVault() error
	SetAutoLockMinutes(minutes int) error
	ProviderStatus() (any, error)
	SwitchProvider(provider string, model string) (any, error)
	// SetProviderEnabled flips a provider's on/off switch. Disabling the active
	// provider hands activity to the next enabled one by priority order.
	SetProviderEnabled(provider string, enabled bool) (any, error)
	// SaveProviderConfig persists a provider's model and/or secrets (API key,
	// vault credential, base URL). Empty secret fields are left unchanged.
	// activate also makes it the active provider. Used by the agent to manage
	// its own LLM config (e.g. a key the user pasted into the chat).
	SaveProviderConfig(provider, model, apiKey, credential, baseURL string, activate bool) (any, error)
	// DeleteProviderCredential removes a provider's stored secret material.
	DeleteProviderCredential(provider string) (any, error)
	// CreateCustomProvider adds a new custom OpenAI-compatible provider slot.
	CreateCustomProvider(name string) (any, error)
	// RenameCustomProvider changes a custom provider's display name.
	RenameCustomProvider(provider, name string) (any, error)
	// DeleteCustomProvider removes a custom provider slot. The agent path uses
	// the no-dialog variant (bearer token + unlocked vault is the authorization).
	DeleteCustomProvider(provider string) (any, error)
	// SetProviderOrder replaces the fallback priority list (index 0 = #1).
	SetProviderOrder(order []string) (any, error)
	// MoveProviderOrder shifts a provider one slot up/down in the fallback list.
	MoveProviderOrder(provider string, up bool) (any, error)
	// StartProviderAuth kicks off browser/device OAuth WITHOUT blocking; the
	// result carries any device code / verification URL to relay to the user,
	// who completes sign-in. Completion is observed via provider.status.
	StartProviderAuth(provider, model string) (any, error)
	// ProviderBalance reports the provider's balance summary (never key material).
	ProviderBalance(provider string) (any, error)

	// Workspace modules: the catalog with added state, add/remove, and the
	// navigable view ids (fixed surfaces + added modules) — all derived from
	// the module registry, never hardcoded.
	ListModules() (any, error)
	AddModule(id string) (any, error)
	RemoveModule(id string) (any, error)
	NavigableViews() []string

	ListChats() (any, error)
	CreateChat(title string, open bool) (any, error)
	SendChatMessage(chatID string, text string) error
	StopChat(chatID string) error
	RenameChat(chatID string, title string) error
	SetChatArchived(chatID string, archived bool) error
	DeleteChat(chatID string) error
	ClearChat(chatID string) error
	NewChatSession(chatID string) error
	CompactChat(chatID string) error
	ChatMessages(chatID string, limit int) (any, error)

	// Notify sends a transient native notification. Callers pass the raw
	// title/body; the use case layer caps and scrubs the body and checks the
	// preference before dispatching.
	Notify(title, body string) error

	// SetServerEnabled starts or stops the named API server (mcp|rest) and
	// persists the auto-start setting. Returns a status summary without key
	// material. Caller must pre-validate server name.
	SetServerEnabled(server string, enabled bool) (any, error)
	// TestProvider sends a one-shot completion through the named provider's
	// stored config. Returns latency/model/error, never key material. This
	// is a paid call — one per invocation, no retry.
	TestProvider(provider string) (any, error)
	// SetWallpaperGlass persists the glass-slider (0-100) and emits the
	// ui:set-wallpaper-glass event. Same validation as the module's slider.
	SetWallpaperGlass(percent int) error
	// HideModule closes a module's sidebar item without removing it.
	HideModule(id string) (any, error)
	// ShowModule reopens a hidden module in the sidebar.
	ShowModule(id string) (any, error)
	// MoveModule shifts a module one slot up or down in the sidebar order.
	MoveModule(id string, up bool) (any, error)
	// HideAllModules closes every non-core sidebar item in one write
	// (the "Show desktop" action). Modules remain added; actions are untouched.
	HideAllModules() (any, error)
}

// These catalogs must stay in sync with the frontend sources of truth:
// themes with ThemePage.tsx / theme/tokens.css, fonts with lib/app-font.ts.
// Views are NOT listed here — app.navigate asks AppControl.NavigableViews(),
// which derives from the module registry.
var (
	awThemes = []string{"midnight", "light", "espresso", "violet", "forest", "ocean", "rose"}
	// Bundled font ids; must match APP_FONT_OPTIONS in frontend/src/lib/app-font.ts.
	awFontFamilies = []string{
		"system",
		"inter", "roboto", "open-sans", "lato", "montserrat", "poppins", "nunito",
		"work-sans", "dm-sans", "manrope", "rubik", "arimo", "libre-franklin", "jost",
		"comic-neue",
		"merriweather", "lora", "playfair-display", "tinos", "gelasio", "libre-baskerville",
		"jetbrains-mono", "fira-code", "ibm-plex-mono", "cousine", "inconsolata",
	}
)

const (
	awFontSizeMin = 12
	awFontSizeMax = 22
	awZoomMin     = 50
	awZoomMax     = 200
)

func registerAppActions(reg map[string]AwActionHandler) {
	reg["app.state"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		state, err := w.control.AppState(ctx)
		if err != nil {
			return "", err
		}
		return awJSON(state)
	}

	reg["app.theme.set"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		theme, err := awRequiredStringArg(args, "theme")
		if err != nil {
			return "", err
		}
		theme = strings.TrimSpace(theme)
		// Custom ids ("custom:slug") are pass-through — the catalog drift
		// convention does not apply to user-defined themes.
		if !strings.HasPrefix(theme, "custom:") {
			theme = strings.ToLower(theme)
			if !awContains(awThemes, theme) {
				return "", fmt.Errorf("unknown theme %q (built-in themes: %s; custom themes start with \"custom:\")",
					theme, strings.Join(awThemes, ", "))
			}
		}
		if err := w.control.SetTheme(theme); err != nil {
			return "", err
		}
		return awOK(map[string]any{"theme": theme})
	}

	reg["app.theme.custom.set"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		name, err := awRequiredStringArg(args, "name")
		if err != nil {
			return "", err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return "", fmt.Errorf("name is required")
		}
		// Collect token keys.
		required := []string{"background", "surface", "surfaceAlt", "text", "mutedText", "border", "accent"}
		optional := []string{"sidebar", "sidebarHover", "sidebarActive", "sidebarActiveText",
			"chatSurface", "chatComposer", "userBubble", "userBubbleText", "assistantBubble", "codeSurface"}
		tokens := map[string]string{"name": name}
		hexRe := hexColorPattern
		for _, key := range required {
			v, hasV, argErr := awStringArg(args, key)
			if argErr != nil {
				return "", argErr
			}
			if !hasV {
				return "", fmt.Errorf("%s is required (#rrggbb hex)", key)
			}
			if !hexRe.MatchString(v) {
				return "", fmt.Errorf("%s must be a #rrggbb hex color, got %q", key, v)
			}
			tokens[key] = v
		}
		for _, key := range optional {
			v, hasV, argErr := awStringArg(args, key)
			if argErr != nil {
				return "", argErr
			}
			if hasV && hexRe.MatchString(v) {
				tokens[key] = v
			}
		}
		id, err := w.control.SetCustomTheme(name, tokens)
		if err != nil {
			return "", err
		}
		return awOK(map[string]any{"id": id, "name": name})
	}

	reg["app.theme.custom.delete"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		if !strings.HasPrefix(id, "custom:") {
			return "", fmt.Errorf("id must start with \"custom:\", got %q", id)
		}
		if err := w.control.DeleteCustomTheme(id); err != nil {
			return "", err
		}
		return awOK(map[string]any{"id": id, "deleted": true})
	}

	reg["app.wallpaper.set"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		id = strings.ToLower(strings.TrimSpace(id))
		if err := w.control.SetWallpaper(id); err != nil {
			return "", err
		}
		return awOK(map[string]any{"wallpaper": id})
	}

	reg["app.wallpaper.upload"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		path, err := awRequiredStringArg(args, "path")
		if err != nil {
			return "", err
		}
		id, err := w.control.UploadWallpaper(strings.TrimSpace(path))
		if err != nil {
			return "", err
		}
		return awOK(map[string]any{"wallpaper": id})
	}

	reg["app.font.set"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		family, hasFamily, err := awStringArg(args, "family")
		if err != nil {
			return "", err
		}
		size, hasSize, err := awIntArg(args, "size")
		if err != nil {
			return "", err
		}
		if !hasFamily && !hasSize {
			return "", fmt.Errorf("family and/or size is required (families: %s; size: %d-%d)",
				strings.Join(awFontFamilies, ", "), awFontSizeMin, awFontSizeMax)
		}
		family = strings.ToLower(strings.TrimSpace(family))
		if hasFamily && !awContains(awFontFamilies, family) {
			return "", fmt.Errorf("unknown font family %q (families: %s)", family, strings.Join(awFontFamilies, ", "))
		}
		if hasSize && (size < awFontSizeMin || size > awFontSizeMax) {
			return "", fmt.Errorf("size %d out of range (%d-%d)", size, awFontSizeMin, awFontSizeMax)
		}
		if err := w.control.SetFont(family, size); err != nil {
			return "", err
		}
		return awOK(map[string]any{"family": family, "size": size})
	}

	reg["app.zoom.set"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		percent, has, err := awIntArg(args, "percent")
		if err != nil {
			return "", err
		}
		if !has {
			return "", fmt.Errorf("percent is required (%d-%d)", awZoomMin, awZoomMax)
		}
		if percent < awZoomMin || percent > awZoomMax {
			return "", fmt.Errorf("percent %d out of range (%d-%d)", percent, awZoomMin, awZoomMax)
		}
		if err := w.control.SetZoomPercent(percent); err != nil {
			return "", err
		}
		return awOK(map[string]any{"percent": percent})
	}

	reg["app.navigate"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		view, err := awRequiredStringArg(args, "view")
		if err != nil {
			return "", err
		}
		view = strings.ToLower(strings.TrimSpace(view))
		views := w.control.NavigableViews()
		if !awContains(views, view) {
			return "", fmt.Errorf("unknown view %q (views: %s)", view, strings.Join(views, ", "))
		}
		chatID, _, err := awStringArg(args, "chatId")
		if err != nil {
			return "", err
		}
		if err := w.control.Navigate(view, strings.TrimSpace(chatID)); err != nil {
			return "", err
		}
		return awOK(map[string]any{"view": view})
	}

	reg["app.lock"] = func(_ context.Context, _ map[string]any, w *workspace) (string, error) {
		if err := w.control.LockVault(); err != nil {
			return "", err
		}
		return awOK(map[string]any{"locked": true})
	}

	reg["app.notify"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		title, _, err := awStringArg(args, "title")
		if err != nil {
			return "", err
		}
		body, hasBody, err := awStringArg(args, "body")
		if err != nil {
			return "", err
		}
		if !hasBody {
			return "", fmt.Errorf("body is required")
		}
		if err := w.control.Notify(title, body); err != nil {
			return "", err
		}
		return awOK(map[string]any{"sent": true})
	}

	reg["app.autolock.set"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		minutes, has, err := awIntArg(args, "minutes")
		if err != nil {
			return "", err
		}
		if !has || minutes < 0 {
			return "", fmt.Errorf("minutes is required (0 disables auto-lock)")
		}
		if err := w.control.SetAutoLockMinutes(minutes); err != nil {
			return "", err
		}
		return awOK(map[string]any{"minutes": minutes})
	}

	reg["app.server.set"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		server, err := awRequiredStringArg(args, "server")
		if err != nil {
			return "", err
		}
		server = strings.ToLower(strings.TrimSpace(server))
		if server != "mcp" && server != "rest" {
			return "", fmt.Errorf("server must be \"mcp\" or \"rest\"")
		}
		enabled, has, err := awOptionalBoolArg(args, "enabled")
		if err != nil {
			return "", err
		}
		if !has {
			return "", fmt.Errorf("enabled is required")
		}
		result, err := w.control.SetServerEnabled(server, enabled)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["app.wallpaper.glass.set"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		percent, has, err := awIntArg(args, "percent")
		if err != nil {
			return "", err
		}
		if !has {
			return "", fmt.Errorf("percent is required (0-100)")
		}
		if percent < 0 || percent > 100 {
			return "", fmt.Errorf("percent %d out of range (0-100)", percent)
		}
		if err := w.control.SetWallpaperGlass(percent); err != nil {
			return "", err
		}
		return awOK(map[string]any{"percent": percent})
	}

	reg["app.desktop.show"] = func(_ context.Context, _ map[string]any, w *workspace) (string, error) {
		result, err := w.control.HideAllModules()
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.status"] = func(_ context.Context, _ map[string]any, w *workspace) (string, error) {
		status, err := w.control.ProviderStatus()
		if err != nil {
			return "", err
		}
		return awJSON(status)
	}

	reg["provider.test"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		provider, err := awRequiredStringArg(args, "provider")
		if err != nil {
			return "", err
		}
		result, err := w.control.TestProvider(provider)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.switch"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		provider, err := awRequiredStringArg(args, "provider")
		if err != nil {
			return "", err
		}
		model, _, err := awStringArg(args, "model")
		if err != nil {
			return "", err
		}
		result, err := w.control.SwitchProvider(provider, model)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.set_enabled"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		provider, err := awRequiredStringArg(args, "provider")
		if err != nil {
			return "", err
		}
		enabled, err := awBoolArg(args, "enabled", true)
		if err != nil {
			return "", err
		}
		result, err := w.control.SetProviderEnabled(provider, enabled)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.config.set"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		provider, err := awRequiredStringArg(args, "provider")
		if err != nil {
			return "", err
		}
		model, _, err := awStringArg(args, "model")
		if err != nil {
			return "", err
		}
		apiKey, _, err := awStringArg(args, "apiKey")
		if err != nil {
			return "", err
		}
		credential, _, err := awStringArg(args, "credential")
		if err != nil {
			return "", err
		}
		baseURL, _, err := awStringArg(args, "baseUrl")
		if err != nil {
			return "", err
		}
		activate, err := awBoolArg(args, "activate", false)
		if err != nil {
			return "", err
		}
		result, err := w.control.SaveProviderConfig(provider, model, apiKey, credential, baseURL, activate)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.credential.delete"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		provider, err := awRequiredStringArg(args, "provider")
		if err != nil {
			return "", err
		}
		result, err := w.control.DeleteProviderCredential(provider)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.create"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		name, _, err := awStringArg(args, "name")
		if err != nil {
			return "", err
		}
		result, err := w.control.CreateCustomProvider(name)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.rename"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		provider, err := awRequiredStringArg(args, "provider")
		if err != nil {
			return "", err
		}
		name, err := awRequiredStringArg(args, "name")
		if err != nil {
			return "", err
		}
		result, err := w.control.RenameCustomProvider(provider, name)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.delete"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		provider, err := awRequiredStringArg(args, "provider")
		if err != nil {
			return "", err
		}
		result, err := w.control.DeleteCustomProvider(provider)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.order.set"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		order, err := awStringSliceArg(args, "order")
		if err != nil {
			return "", err
		}
		result, err := w.control.SetProviderOrder(order)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.order.move"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		provider, err := awRequiredStringArg(args, "provider")
		if err != nil {
			return "", err
		}
		direction, err := awRequiredStringArg(args, "direction")
		if err != nil {
			return "", err
		}
		var up bool
		switch strings.ToLower(strings.TrimSpace(direction)) {
		case "up":
			up = true
		case "down":
			up = false
		default:
			return "", fmt.Errorf(`direction must be "up" or "down"`)
		}
		result, err := w.control.MoveProviderOrder(provider, up)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.auth.start"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		provider, err := awRequiredStringArg(args, "provider")
		if err != nil {
			return "", err
		}
		model, _, err := awStringArg(args, "model")
		if err != nil {
			return "", err
		}
		result, err := w.control.StartProviderAuth(provider, model)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["provider.balance"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		provider, err := awRequiredStringArg(args, "provider")
		if err != nil {
			return "", err
		}
		result, err := w.control.ProviderBalance(provider)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}

func awOK(extra map[string]any) (string, error) {
	payload := map[string]any{"success": true}
	for key, value := range extra {
		payload[key] = value
	}
	return awJSON(payload)
}

func awContains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func awIntArg(args map[string]any, name string) (int, bool, error) {
	value, ok := args[name]
	if !ok || value == nil {
		return 0, false, nil
	}
	switch v := value.(type) {
	case float64:
		return int(v), true, nil
	case int:
		return v, true, nil
	default:
		return 0, true, fmt.Errorf("%s must be a number", name)
	}
}

func awBoolArg(args map[string]any, name string, fallback bool) (bool, error) {
	value, ok := args[name]
	if !ok || value == nil {
		return fallback, nil
	}
	parsed, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return parsed, nil
}

// awOptionalBoolArg reports presence alongside the value (like awStringArg),
// for partial updates where "absent" and "false" mean different things.
func awOptionalBoolArg(args map[string]any, name string) (bool, bool, error) {
	value, ok := args[name]
	if !ok || value == nil {
		return false, false, nil
	}
	parsed, ok := value.(bool)
	if !ok {
		return false, true, fmt.Errorf("%s must be a boolean", name)
	}
	return parsed, true, nil
}
