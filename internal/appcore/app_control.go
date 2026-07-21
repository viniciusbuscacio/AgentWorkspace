package appcore

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
	"aw/internal/infrastructure/appconfig"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// appControl adapts the App (interface/composition root) to the
// tools.AppControl port consumed by the aw action dispatcher. UI-affecting
// operations are delivered to the React frontend as ui:* Wails events; the
// frontend applies them and reports the resulting state back through
// ReportUIState.
type appControl struct {
	app *App
}

func (c appControl) SetTheme(theme string) error {
	c.app.emitUIEvent("ui:set-theme", map[string]any{"theme": theme})
	return nil
}

// SetCustomTheme creates/updates a custom theme via the Wails bindings, then
// emits ui:set-theme so the UI applies it immediately.
func (c appControl) SetCustomTheme(name string, tokens map[string]string) (string, error) {
	// Build the interface{} map the Wails binding expects.
	tokenMap := make(map[string]interface{}, len(tokens))
	for k, v := range tokens {
		tokenMap[k] = v
	}
	// Derive the id from the name (same slug logic as the frontend).
	id := customThemeSlug(name)
	result := c.app.SaveCustomTheme(id, tokenMap)
	if !result.Success {
		return "", errors.New(result.Error)
	}
	if err := c.app.SaveActiveTheme(id); !err.Success {
		return "", errors.New(err.Error)
	}
	c.app.emitUIEvent("ui:set-theme", map[string]any{"theme": id})
	return id, nil
}

// DeleteCustomTheme removes a custom theme via the Wails bindings.
func (c appControl) DeleteCustomTheme(id string) error {
	result := c.app.DeleteCustomTheme(id)
	if !result.Success {
		return errors.New(result.Error)
	}
	return nil
}

func (c appControl) SetWallpaper(id string) error {
	// App.SetWallpaper already emits ui:set-wallpaper; no second emit here.
	if result := c.app.SetWallpaper(id); !result.Success {
		return errors.New(result.Error)
	}
	return nil
}

func (c appControl) UploadWallpaper(path string) (string, error) {
	result := c.app.UploadWallpaperFromPath(path)
	if !result.Success {
		return "", errors.New(result.Error)
	}
	return result.ID, nil
}

func (c appControl) SetFont(family string, size int) error {
	c.app.emitUIEvent("ui:set-font", map[string]any{"family": family, "size": size})
	return nil
}

func (c appControl) SetZoomPercent(percent int) error {
	if result := c.app.SetAppZoomPercent(percent); !result.Success {
		return errors.New(result.Error)
	}
	c.app.emitUIEvent("ui:set-zoom", map[string]any{"percent": percent})
	return nil
}

func (c appControl) Navigate(view string, chatID string) error {
	c.app.emitUIEvent("ui:navigate", map[string]any{"view": view, "chatId": chatID})
	return nil
}

func (c appControl) AppState(_ context.Context) (any, error) {
	mcpStatus := c.app.GetMcpServerStatus()
	restStatus := c.app.GetRestServerStatus()
	return map[string]any{
		"ui":              c.app.reportedUIState(),
		"wallpaper":       c.app.GetWallpaper(),
		"wallpaperGlass":  c.app.GetWallpaperGlass(),
		"zoomPercent":     c.app.GetAppZoomPercent(),
		"autoLockMinutes": c.app.GetAutoLockMinutes(),
		"vault":           c.app.VaultStatus(),
		"provider":        c.app.GetProviderStatus(),
		"selfDev":         application.LoadSelfDevConfig(appconfig.Store{}).Enabled,
		"servers": map[string]any{
			"mcp":  map[string]any{"running": mcpStatus.Running, "autostart": mcpStatus.Autostart, "port": mcpStatus.Port},
			"rest": map[string]any{"running": restStatus.Running, "autostart": restStatus.Autostart, "port": restStatus.Port},
		},
	}, nil
}

func (c appControl) LockVault() error {
	if result := c.app.LockVault(); !result.Success {
		return errors.New(result.Error)
	}
	// Reuse the auto-lock event so the frontend drops back to the vault gate.
	c.app.emitChatEvent("vault:auto-locked", map[string]any{"reason": "agent"})
	return nil
}

func (c appControl) SetAutoLockMinutes(minutes int) error {
	if result := c.app.SetAutoLockMinutes(minutes); !result.Success {
		return errors.New(result.Error)
	}
	return nil
}

func (c appControl) ProviderStatus() (any, error) {
	return c.app.GetProviderStatus(), nil
}

func (c appControl) SwitchProvider(provider string, model string) (any, error) {
	return c.app.SwitchProvider(provider, model), nil
}

func (c appControl) SetProviderEnabled(provider string, enabled bool) (any, error) {
	return c.app.SetProviderEnabled(provider, enabled), nil
}

func (c appControl) SaveProviderConfig(provider, model, apiKey, credential, baseURL string, activate bool) (any, error) {
	// Reuse the App binding (no native dialog): persists model/secrets and,
	// when activate is set, makes this the active provider.
	result := c.app.SaveProviderConfig(application.ProviderSaveConfigInput{
		Provider:   provider,
		Model:      model,
		APIKey:     apiKey,
		Credential: credential,
		BaseURL:    baseURL,
		SetActive:  activate,
	})
	if !result.Success && result.Error != "" {
		return result, errors.New(result.Error)
	}
	return result, nil
}

func (c appControl) DeleteProviderCredential(provider string) (any, error) {
	// Call the application layer directly: the App.DeleteProviderCredential
	// binding pops a native confirmation dialog, which REST/MCP clients (token
	// holders) must not get. The agent manages its own credentials.
	result := application.DeleteProviderCredential(c.app.vault, c.app.providerFallbackOrder(), provider)
	if !result.Success && result.Error != "" {
		return result, errors.New(result.Error)
	}
	return result, nil
}

func (c appControl) CreateCustomProvider(name string) (any, error) {
	result := c.app.CreateCustomProvider(name)
	if !result.Success && result.Error != "" {
		return result, errors.New(result.Error)
	}
	return result, nil
}

func (c appControl) RenameCustomProvider(provider, name string) (any, error) {
	result := c.app.RenameCustomProvider(provider, name)
	if !result.Success && result.Error != "" {
		return result, errors.New(result.Error)
	}
	return result, nil
}

func (c appControl) DeleteCustomProvider(provider string) (any, error) {
	// No-dialog variant: the App.DeleteCustomProvider binding pops a native
	// confirmation dialog (denied to token holders). The *Confirmed variant also
	// drops the id from the fallback priority list.
	result := c.app.DeleteCustomProviderConfirmed(provider)
	if !result.Success && result.Error != "" {
		return result, errors.New(result.Error)
	}
	return result, nil
}

func (c appControl) SetProviderOrder(order []string) (any, error) {
	result := c.app.SetProviderFallbackOrder(order)
	if !result.Success && result.Error != "" {
		return result, errors.New(result.Error)
	}
	return map[string]any{"success": true, "order": c.app.GetProviderFallbackOrder()}, nil
}

func (c appControl) MoveProviderOrder(provider string, up bool) (any, error) {
	// The fallback priority list is the real failover order. A provider that is
	// not yet listed is appended first so it gains a position to move from.
	order := c.app.GetProviderFallbackOrder()
	idx := -1
	for i, id := range order {
		if id == provider {
			idx = i
			break
		}
	}
	if idx == -1 {
		order = append(order, provider)
		idx = len(order) - 1
	}
	swap := idx
	if up && idx > 0 {
		swap = idx - 1
	} else if !up && idx < len(order)-1 {
		swap = idx + 1
	}
	order[idx], order[swap] = order[swap], order[idx]
	result := c.app.SetProviderFallbackOrder(order)
	if !result.Success && result.Error != "" {
		return result, errors.New(result.Error)
	}
	return map[string]any{"success": true, "order": c.app.GetProviderFallbackOrder()}, nil
}

func (c appControl) StartProviderAuth(provider, model string) (any, error) {
	// Non-blocking: kicks off the browser/device flow and returns immediately
	// with the device code / verification URL for the agent to relay. The user
	// completes sign-in; the agent observes completion via provider.status.
	return c.app.StartProviderBrowserAuthAsync(provider, model), nil
}

func (c appControl) ProviderBalance(provider string) (any, error) {
	return c.app.GetProviderBalance(provider), nil
}

func (c appControl) ListModules() (any, error) {
	return c.app.ListModules().Modules, nil
}

func (c appControl) AddModule(id string) (any, error) {
	result := c.app.AddModule(id)
	if !result.Success {
		return nil, errors.New(result.Error)
	}
	return map[string]any{"success": true, "id": id}, nil
}

func (c appControl) RemoveModule(id string) (any, error) {
	result := c.app.RemoveModule(id)
	if !result.Success {
		return nil, errors.New(result.Error)
	}
	return map[string]any{"success": true, "id": id}, nil
}

func (c appControl) NavigableViews() []string {
	views, err := application.NavigableViews(c.app.workspace, domain.ModuleCatalog())
	if err != nil {
		return []string{"home", "settings", "chat"}
	}
	return views
}

func (c appControl) ListChats() (any, error) {
	return c.app.ListChats()
}

func (c appControl) CreateChat(title string, open bool) (any, error) {
	chat, err := c.app.CreateChatWithTitle(title)
	if err != nil {
		return nil, err
	}
	c.app.emitChatEvent("chat:refresh", map[string]any{"chatId": chat.ID})
	if open {
		c.app.emitUIEvent("ui:navigate", map[string]any{"view": "chat", "chatId": chat.ID})
	}
	return chat, nil
}

func (c appControl) SendChatMessage(chatID string, text string) error {
	// Run asynchronously: a chat run can take minutes and the dispatcher must
	// answer immediately (mirrors the PiP server send path). Errors surface to
	// the UI through the chat:error event.
	go func() {
		if result := c.app.StreamChatMessage(chatID, text, nil); !result.Success {
			c.app.emitChatEvent("chat:error", map[string]any{"chatId": chatID, "error": result.Error})
		}
	}()
	return nil
}

func (c appControl) StopChat(chatID string) error {
	return chatOpError(c.app.StopChat(chatID))
}

func (c appControl) RenameChat(chatID string, title string) error {
	return chatOpError(c.app.RenameChat(chatID, title))
}

func (c appControl) SetChatArchived(chatID string, archived bool) error {
	if err := chatOpError(c.app.SetChatArchived(chatID, archived)); err != nil {
		return err
	}
	c.app.emitChatEvent("chat:refresh", map[string]any{"chatId": chatID})
	return nil
}

func (c appControl) DeleteChat(chatID string) error {
	if err := chatOpError(c.app.DeleteChat(chatID)); err != nil {
		return err
	}
	c.app.emitChatEvent("chat:refresh", map[string]any{"chatId": chatID})
	return nil
}

func (c appControl) ClearChat(chatID string) error {
	if err := chatOpError(c.app.ClearChat(chatID)); err != nil {
		return err
	}
	c.app.emitChatEvent("chat:refresh", map[string]any{"chatId": chatID})
	return nil
}

func (c appControl) NewChatSession(chatID string) error {
	if err := chatOpError(c.app.NewChatSession(chatID)); err != nil {
		return err
	}
	c.app.emitChatEvent("chat:refresh", map[string]any{"chatId": chatID})
	return nil
}

func (c appControl) CompactChat(chatID string) error {
	return chatOpError(c.app.CompactChat(chatID))
}

func (c appControl) ChatMessages(chatID string, limit int) (any, error) {
	return c.app.RecentMessages(chatID, limit)
}

func (c appControl) Notify(title, body string) error {
	enabled := c.app.GetDesktopNotificationsEnabled()
	return application.NotifyUser(c.app.notifier, enabled, title, body)
}

// SetServerEnabled starts or stops the named API server (mcp|rest) and
// persists the auto-start toggle. Reuses the exact start/stop methods the
// Settings pages call; Decision 1 in the coverage spec.
func (c appControl) SetServerEnabled(server string, enabled bool) (any, error) {
	switch server {
	case "mcp":
		if err := application.McpServerSettings.SetAutostart(c.app.vault, enabled); err != nil {
			return nil, err
		}
		if enabled {
			_ = c.app.startMcpServer()
		} else {
			c.app.stopMcpServer(c.app.contextOrBackground())
		}
		status := c.app.GetMcpServerStatus()
		result := map[string]any{
			"server":    "mcp",
			"running":   status.Running,
			"autostart": status.Autostart,
		}
		if !enabled {
			result["note"] = "MCP server stopped; external agents connected to it are now disconnected."
		}
		return result, nil
	case "rest":
		if err := application.RestServerSettings.SetAutostart(c.app.vault, enabled); err != nil {
			return nil, err
		}
		if enabled {
			_ = c.app.startRestServer()
		} else {
			c.app.stopRestServer(c.app.contextOrBackground())
		}
		status := c.app.GetRestServerStatus()
		return map[string]any{
			"server":    "rest",
			"running":   status.Running,
			"autostart": status.Autostart,
		}, nil
	default:
		return nil, fmt.Errorf("unknown server %q", server)
	}
}

// TestProvider sends a one-shot completion through the named provider's stored
// config. Wraps the existing TestProviderConnection Wails binding; Decision 2.
func (c appControl) TestProvider(provider string) (any, error) {
	result := c.app.TestProviderConnection(provider)
	return result, nil
}

// SetWallpaperGlass persists the glass slider value and emits the UI event.
// Same validation as the Wallpaper module's slider; Decision 4.
func (c appControl) SetWallpaperGlass(percent int) error {
	if result := c.app.SetWallpaperGlass(percent); !result.Success {
		return errors.New(result.Error)
	}
	return nil
}

// HideModule closes a module's sidebar item. The module stays added; Decision 3.
func (c appControl) HideModule(id string) (any, error) {
	result := c.app.HideModule(id)
	if !result.Success {
		return nil, errors.New(result.Error)
	}
	return map[string]any{"success": true, "id": id}, nil
}

// ShowModule reopens a hidden module in the sidebar; Decision 3.
func (c appControl) ShowModule(id string) (any, error) {
	result := c.app.ShowModule(id)
	if !result.Success {
		return nil, errors.New(result.Error)
	}
	return map[string]any{"success": true, "id": id}, nil
}

// MoveModule shifts a module one slot up or down in the sidebar order; Decision 3.
func (c appControl) MoveModule(id string, up bool) (any, error) {
	result := c.app.MoveModule(id, up)
	if !result.Success {
		return nil, errors.New(result.Error)
	}
	return map[string]any{"success": true, "id": id, "up": up}, nil
}

// HideAllModules closes every non-core sidebar item in one write ("Show
// desktop"). Modules remain added; their actions are untouched; Decision 3.
func (c appControl) HideAllModules() (any, error) {
	result := c.app.HideAllModules()
	if !result.Success {
		return nil, errors.New(result.Error)
	}
	return map[string]any{"success": true}, nil
}

func chatOpError(result dto.ChatOperationResult) error {
	if result.Success {
		return nil
	}
	return errors.New(result.Error)
}

// emitUIEvent delivers a ui:* control event to the main window and the web
// surface (PiP windows manage their own appearance, so they are skipped). The
// web broadcast keeps a remote browser's navigation/theme/font in sync.
func (a *App) emitUIEvent(name string, payload map[string]any) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, name, payload)
	}
	a.broadcastWebEvent(name, payload)
}

// ReportUIState is called by the frontend whenever UI-managed state (theme,
// font, view, selected chat) changes, so aw actions like app.state can answer
// without a frontend round-trip.
func (a *App) ReportUIState(state map[string]any) {
	if state == nil {
		return
	}
	a.uiStateMu.Lock()
	defer a.uiStateMu.Unlock()
	if a.uiState == nil {
		a.uiState = map[string]any{}
	}
	for key, value := range state {
		a.uiState[key] = value
	}
}

func (a *App) reportedUIState() map[string]any {
	a.uiStateMu.Lock()
	defer a.uiStateMu.Unlock()
	snapshot := map[string]any{}
	for key, value := range a.uiState {
		snapshot[key] = value
	}
	return snapshot
}

// slugNonAlnum replaces sequences of non-alphanumeric runes with hyphens.
var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// customThemeSlug mirrors the frontend's customThemeIdForName: strips
// diacritics, lowercases, replaces non-alphanumeric runs with hyphens and caps
// the slug at 40 chars, then prepends "custom:".
func customThemeSlug(name string) string {
	// Decompose to NFD so we can strip combining marks.
	var buf strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == '-' {
			buf.WriteRune(unicode.ToLower(r))
		}
	}
	slug := slugNonAlnum.ReplaceAllString(buf.String(), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	if slug == "" {
		slug = "theme"
	}
	return "custom:" + slug
}
