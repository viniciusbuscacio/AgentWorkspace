package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
)

// fakeControl records AppControl calls so tests can assert routing and args.
type fakeControl struct {
	calls         []string
	theme         string
	wallpaper     string
	family        string
	size          int
	percent       int
	view          string
	chatID        string
	title         string
	archived      bool
	open          bool
	moduleID      string
	views         []string
	serverName    string
	serverEnabled bool
}

func (f *fakeControl) record(call string) { f.calls = append(f.calls, call) }

func (f *fakeControl) SetTheme(theme string) error {
	f.record("SetTheme")
	f.theme = theme
	return nil
}

func (f *fakeControl) SetCustomTheme(name string, _ map[string]string) (string, error) {
	f.record("SetCustomTheme")
	f.title = name
	id := "custom:" + strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	return id, nil
}

func (f *fakeControl) DeleteCustomTheme(id string) error {
	f.record("DeleteCustomTheme")
	f.moduleID = id
	return nil
}

func (f *fakeControl) SetWallpaper(id string) error {
	f.record("SetWallpaper")
	f.wallpaper = id
	return nil
}

func (f *fakeControl) UploadWallpaper(path string) (string, error) {
	f.record("UploadWallpaper")
	f.wallpaper = "custom:" + path
	return f.wallpaper, nil
}

func (f *fakeControl) SetFont(family string, size int) error {
	f.record("SetFont")
	f.family, f.size = family, size
	return nil
}

func (f *fakeControl) SetZoomPercent(percent int) error {
	f.record("SetZoomPercent")
	f.percent = percent
	return nil
}

func (f *fakeControl) Navigate(view string, chatID string) error {
	f.record("Navigate")
	f.view, f.chatID = view, chatID
	return nil
}

func (f *fakeControl) AppState(_ context.Context) (any, error) {
	f.record("AppState")
	return map[string]any{"ui": map[string]any{"theme": "midnight"}}, nil
}

func (f *fakeControl) LockVault() error {
	f.record("LockVault")
	return nil
}

func (f *fakeControl) SetAutoLockMinutes(minutes int) error {
	f.record("SetAutoLockMinutes")
	f.percent = minutes
	return nil
}

func (f *fakeControl) ProviderStatus() (any, error) {
	f.record("ProviderStatus")
	return map[string]any{"provider": "openai"}, nil
}

func (f *fakeControl) SwitchProvider(provider string, model string) (any, error) {
	f.record("SwitchProvider")
	f.title = provider + "/" + model
	return map[string]any{"success": true}, nil
}

func (f *fakeControl) SetProviderEnabled(provider string, enabled bool) (any, error) {
	f.record("SetProviderEnabled")
	f.title = provider + "/" + strconv.FormatBool(enabled)
	return map[string]any{"success": true}, nil
}

func (f *fakeControl) SaveProviderConfig(provider, model, _, _, _ string, activate bool) (any, error) {
	f.record("SaveProviderConfig")
	f.title = provider + "/" + model
	return map[string]any{"success": true, "saved": true, "activated": activate}, nil
}

func (f *fakeControl) DeleteProviderCredential(provider string) (any, error) {
	f.record("DeleteProviderCredential")
	f.title = provider
	return map[string]any{"success": true}, nil
}

func (f *fakeControl) CreateCustomProvider(name string) (any, error) {
	f.record("CreateCustomProvider")
	f.title = name
	return map[string]any{"success": true, "providerId": "custom-test"}, nil
}

func (f *fakeControl) RenameCustomProvider(provider, name string) (any, error) {
	f.record("RenameCustomProvider")
	f.title = provider + "/" + name
	return map[string]any{"success": true}, nil
}

func (f *fakeControl) DeleteCustomProvider(provider string) (any, error) {
	f.record("DeleteCustomProvider")
	f.title = provider
	return map[string]any{"success": true}, nil
}

func (f *fakeControl) SetProviderOrder(order []string) (any, error) {
	f.record("SetProviderOrder")
	f.views = order
	return map[string]any{"success": true, "order": order}, nil
}

func (f *fakeControl) MoveProviderOrder(provider string, up bool) (any, error) {
	f.record("MoveProviderOrder")
	f.title = provider
	f.archived = up // reuse a field to record the direction in tests
	return map[string]any{"success": true, "order": []string{provider}}, nil
}

func (f *fakeControl) StartProviderAuth(provider, model string) (any, error) {
	f.record("StartProviderAuth")
	f.title = provider + "/" + model
	return map[string]any{"started": true, "message": "auth started"}, nil
}

func (f *fakeControl) ProviderBalance(provider string) (any, error) {
	f.record("ProviderBalance")
	f.title = provider
	return map[string]any{"available": true, "used": "$0.05", "limit": "$10.00"}, nil
}

func (f *fakeControl) ListModules() (any, error) {
	f.record("ListModules")
	return []map[string]any{{"id": "chat", "added": true}}, nil
}

func (f *fakeControl) AddModule(id string) (any, error) {
	f.record("AddModule")
	f.moduleID = id
	return map[string]any{"success": true}, nil
}

func (f *fakeControl) RemoveModule(id string) (any, error) {
	f.record("RemoveModule")
	f.moduleID = id
	return map[string]any{"success": true}, nil
}

func (f *fakeControl) NavigableViews() []string {
	f.record("NavigableViews")
	if f.views != nil {
		return append([]string{"home", "settings"}, f.views...)
	}
	return []string{"home", "settings", "chat"}
}

func (f *fakeControl) ListChats() (any, error) {
	f.record("ListChats")
	return []map[string]any{{"id": "c1"}}, nil
}

func (f *fakeControl) CreateChat(title string, open bool) (any, error) {
	f.record("CreateChat")
	f.title, f.open = title, open
	return map[string]any{"id": "c2", "title": title}, nil
}

func (f *fakeControl) SendChatMessage(chatID string, text string) error {
	f.record("SendChatMessage")
	f.chatID, f.title = chatID, text
	return nil
}

func (f *fakeControl) StopChat(chatID string) error {
	f.record("StopChat")
	f.chatID = chatID
	return nil
}

func (f *fakeControl) RenameChat(chatID string, title string) error {
	f.record("RenameChat")
	f.chatID, f.title = chatID, title
	return nil
}

func (f *fakeControl) SetChatArchived(chatID string, archived bool) error {
	f.record("SetChatArchived")
	f.chatID, f.archived = chatID, archived
	return nil
}

func (f *fakeControl) DeleteChat(chatID string) error {
	f.record("DeleteChat")
	f.chatID = chatID
	return nil
}

func (f *fakeControl) ClearChat(chatID string) error {
	f.record("ClearChat")
	f.chatID = chatID
	return nil
}

func (f *fakeControl) NewChatSession(chatID string) error {
	f.record("NewChatSession")
	f.chatID = chatID
	return nil
}

func (f *fakeControl) CompactChat(chatID string) error {
	f.record("CompactChat")
	f.chatID = chatID
	return nil
}

func (f *fakeControl) ChatMessages(chatID string, limit int) (any, error) {
	f.record("ChatMessages")
	f.chatID, f.size = chatID, limit
	return []map[string]any{}, nil
}

func (f *fakeControl) Notify(title, _ string) error {
	f.record("Notify")
	f.title = title
	return nil
}

func (f *fakeControl) SetServerEnabled(server string, enabled bool) (any, error) {
	f.record("SetServerEnabled")
	f.serverName = server
	f.serverEnabled = enabled
	return map[string]any{"server": server, "running": enabled, "autostart": enabled}, nil
}

func (f *fakeControl) TestProvider(provider string) (any, error) {
	f.record("TestProvider")
	f.title = provider
	return map[string]any{"success": true, "latency": "12ms", "model": "gpt-4o"}, nil
}

func (f *fakeControl) SetWallpaperGlass(percent int) error {
	f.record("SetWallpaperGlass")
	f.percent = percent
	return nil
}

func (f *fakeControl) HideModule(id string) (any, error) {
	f.record("HideModule")
	f.moduleID = id
	return map[string]any{"success": true, "id": id}, nil
}

func (f *fakeControl) ShowModule(id string) (any, error) {
	f.record("ShowModule")
	f.moduleID = id
	return map[string]any{"success": true, "id": id}, nil
}

func (f *fakeControl) MoveModule(id string, up bool) (any, error) {
	f.record("MoveModule")
	f.moduleID = id
	f.open = up
	return map[string]any{"success": true, "id": id, "up": up}, nil
}

func (f *fakeControl) HideAllModules() (any, error) {
	f.record("HideAllModules")
	return map[string]any{"success": true}, nil
}

func controlWorkspace(t *testing.T) (*workspace, *fakeControl) {
	t.Helper()
	control := &fakeControl{}
	return &workspace{root: t.TempDir(), control: control, autoApprove: true}, control
}

func TestAwActionsIncludesControlActions(t *testing.T) {
	ws, _ := controlWorkspace(t)
	result, err := ws.awDispatch(nil, awArgs{Action: "aw.actions"})
	if err != nil {
		t.Fatalf("aw.actions error = %v", err)
	}
	var actions []string
	if err := json.Unmarshal([]byte(result.Result), &actions); err != nil {
		t.Fatalf("unmarshal actions: %v\n%s", err, result.Result)
	}
	have := map[string]bool{}
	for _, action := range actions {
		have[action] = true
	}
	for _, want := range []string{
		"app.state", "app.theme.set", "app.font.set", "app.zoom.set", "app.navigate",
		"app.lock", "app.autolock.set", "app.server.set", "app.wallpaper.glass.set",
		"app.wallpaper.set", "app.wallpaper.upload",
		"app.desktop.show", "provider.status", "provider.switch", "provider.set_enabled", "provider.test",
		"provider.create", "provider.rename", "provider.delete",
		"provider.order.set", "provider.order.move", "provider.auth.start", "provider.balance",
		"module.hide", "module.show", "module.move",
		"chat.list", "chat.create", "chat.open", "chat.send", "chat.stop",
		"chat.rename", "chat.archive", "chat.delete", "chat.clear",
		"chat.session.new", "chat.compact", "chat.messages",
	} {
		if !have[want] {
			t.Fatalf("aw.actions missing %q in %v", want, actions)
		}
	}
	// Self-management actions stay hidden outside self-dev mode.
	for _, hidden := range []string{"fs.read", "fs.write", "shell.exec", "system.selfcode"} {
		if have[hidden] {
			t.Fatalf("aw.actions should hide %q when selfManage is off: %v", hidden, actions)
		}
	}
}

func TestAwProviderActionsDoNotIncludeSecretReaders(t *testing.T) {
	ws, _ := controlWorkspace(t)
	result, err := ws.awDispatch(nil, awArgs{Action: "aw.actions"})
	if err != nil {
		t.Fatalf("aw.actions error = %v", err)
	}
	var actions []string
	if err := json.Unmarshal([]byte(result.Result), &actions); err != nil {
		t.Fatalf("unmarshal actions: %v", err)
	}

	providerActions := map[string]bool{}
	for _, action := range actions {
		if strings.HasPrefix(action, "provider.") {
			providerActions[action] = true
		}
	}
	for _, allowed := range []string{
		"provider.status", "provider.switch", "provider.set_enabled", "provider.test", "provider.config.set", "provider.credential.delete",
		"provider.create", "provider.rename", "provider.delete",
		"provider.order.set", "provider.order.move", "provider.auth.start", "provider.balance",
	} {
		if !providerActions[allowed] {
			t.Fatalf("aw.actions missing allowed provider action %q", allowed)
		}
		delete(providerActions, allowed)
	}
	if len(providerActions) != 0 {
		t.Fatalf("unexpected provider actions registered: %v", providerActions)
	}
}

func TestAwThemeSetValidatesAndRoutes(t *testing.T) {
	ws, control := controlWorkspace(t)
	// Unknown built-in names should error with the catalog hint.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.theme.set", Args: `{"theme":"neon"}`}); err == nil ||
		!strings.Contains(err.Error(), "midnight") {
		t.Fatalf("invalid theme error = %v, want theme catalog hint", err)
	}
	// Valid built-in themes are normalised to lowercase.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.theme.set", Args: `{"theme":"Ocean"}`}); err != nil {
		t.Fatalf("app.theme.set error = %v", err)
	}
	if control.theme != "ocean" {
		t.Fatalf("theme = %q, want ocean (normalised)", control.theme)
	}
	// Custom ids ("custom:slug") are pass-through — no catalog check.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.theme.set", Args: `{"theme":"custom:neon-night"}`}); err != nil {
		t.Fatalf("custom theme set error = %v", err)
	}
	if control.theme != "custom:neon-night" {
		t.Fatalf("theme = %q, want custom:neon-night", control.theme)
	}
}

func TestAwThemeCustomSet(t *testing.T) {
	ws, control := controlWorkspace(t)
	// Missing required token.
	_, err := ws.awDispatch(nil, awArgs{Action: "app.theme.custom.set", Args: `{"name":"Neon","background":"#111111","surface":"#222222","surfaceAlt":"#333333",` +
		`"text":"#eeeeee","mutedText":"#999999","border":"#444444"}`})
	if err == nil || !strings.Contains(err.Error(), "accent") {
		t.Fatalf("missing accent error = %v", err)
	}
	// Invalid hex color.
	_, err = ws.awDispatch(nil, awArgs{Action: "app.theme.custom.set", Args: `{"name":"N","background":"red","surface":"#222222","surfaceAlt":"#333333",` +
		`"text":"#eeeeee","mutedText":"#999999","border":"#444444","accent":"#5566ff"}`})
	if err == nil || !strings.Contains(err.Error(), "background") {
		t.Fatalf("invalid hex error = %v", err)
	}
	// Valid create — should return the custom id.
	res, err := ws.awDispatch(nil, awArgs{Action: "app.theme.custom.set", Args: `{"name":"Neon Night","background":"#111111","surface":"#222222","surfaceAlt":"#333333",` +
		`"text":"#eeeeee","mutedText":"#999999","border":"#444444","accent":"#5566ff"}`})
	if err != nil {
		t.Fatalf("theme custom set error = %v", err)
	}
	if !strings.Contains(res.Result, "custom:") {
		t.Fatalf("result = %q, want custom: id", res.Result)
	}
	if control.title != "Neon Night" {
		t.Fatalf("SetCustomTheme name = %q, want \"Neon Night\"", control.title)
	}
}

func TestAwThemeCustomDelete(t *testing.T) {
	ws, control := controlWorkspace(t)
	// id must start with "custom:".
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.theme.custom.delete", Args: `{"id":"midnight"}`}); err == nil {
		t.Fatalf("invalid id should error")
	}
	// Valid delete.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.theme.custom.delete", Args: `{"id":"custom:neon-night"}`}); err != nil {
		t.Fatalf("custom delete error = %v", err)
	}
	if control.moduleID != "custom:neon-night" {
		t.Fatalf("DeleteCustomTheme id = %q, want custom:neon-night", control.moduleID)
	}
}

func TestAwFontSetValidatesFamilyAndSize(t *testing.T) {
	ws, control := controlWorkspace(t)
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.font.set", Args: `{}`}); err == nil {
		t.Fatalf("empty font args should fail with usage hint")
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.font.set", Args: `{"size":40}`}); err == nil ||
		!strings.Contains(err.Error(), "out of range") {
		t.Fatalf("oversized font error = %v, want range error", err)
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.font.set", Args: `{"family":"jetbrains-mono","size":16}`}); err != nil {
		t.Fatalf("app.font.set error = %v", err)
	}
	if control.family != "jetbrains-mono" || control.size != 16 {
		t.Fatalf("font = %q/%d, want jetbrains-mono/16", control.family, control.size)
	}
}

func TestAwNavigateValidatesView(t *testing.T) {
	ws, control := controlWorkspace(t)
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.navigate", Args: `{"view":"dashboard"}`}); err == nil ||
		!strings.Contains(err.Error(), "views:") {
		t.Fatalf("invalid view error = %v, want views catalog hint", err)
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.navigate", Args: `{"view":"chat","chatId":"c1"}`}); err != nil {
		t.Fatalf("app.navigate error = %v", err)
	}
	if control.view != "chat" || control.chatID != "c1" {
		t.Fatalf("navigate = %q/%q, want chat/c1", control.view, control.chatID)
	}
}

func TestAwChatCreateDefaultsToOpen(t *testing.T) {
	ws, control := controlWorkspace(t)
	if _, err := ws.awDispatch(nil, awArgs{Action: "chat.create", Args: `{"title":"Plans"}`}); err != nil {
		t.Fatalf("chat.create error = %v", err)
	}
	if control.title != "Plans" || !control.open {
		t.Fatalf("create = %q/open=%v, want Plans/open=true", control.title, control.open)
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "chat.create", Args: `{"open":false}`}); err != nil {
		t.Fatalf("chat.create (open=false) error = %v", err)
	}
	if control.open {
		t.Fatalf("open = true, want false when explicitly disabled")
	}
}

// chat.delete is now the trash-bin verb: it ARCHIVES (recoverable), with NO
// confirmation, so the agent can "delete" freely without losing anything.
func TestAwChatDeleteArchivesWithoutConfirmation(t *testing.T) {
	control := &fakeControl{}
	ws := &workspace{root: t.TempDir(), control: control} // no confirm wired
	if _, err := ws.awDispatch(nil, awArgs{Action: "chat.delete", Args: `{"chatId":"c9"}`}); err != nil {
		t.Fatalf("chat.delete error = %v, want none (no confirmation)", err)
	}
	if !awContains(control.calls, "SetChatArchived") || awContains(control.calls, "DeleteChat") {
		t.Fatalf("chat.delete must archive, not delete; calls = %v", control.calls)
	}
	if control.chatID != "c9" || !control.archived {
		t.Fatalf("expected chat c9 archived, got id=%q archived=%v", control.chatID, control.archived)
	}
}

// chat.delete_permanent is the irreversible removal: it keeps the human
// confirmation and only then calls DeleteChat.
func TestAwChatDeletePermanentRequiresConfirmation(t *testing.T) {
	control := &fakeControl{}
	denied := &workspace{root: t.TempDir(), control: control}
	if _, err := denied.awDispatch(nil, awArgs{Action: "chat.delete_permanent", Args: `{"chatId":"c9"}`}); !errors.Is(err, ErrConfirmationDenied) {
		t.Fatalf("chat.delete_permanent without approval error = %v, want ErrConfirmationDenied", err)
	}
	if len(control.calls) != 0 {
		t.Fatalf("control calls = %v, want none when confirmation is denied", control.calls)
	}

	approved := &workspace{
		root:    t.TempDir(),
		control: control,
		confirm: func(_ context.Context, req ConfirmRequest) (bool, error) {
			if req.Tool != "aw" || !strings.Contains(req.Summary, "c9") {
				t.Fatalf("confirm request = %+v, want aw tool with chat id in summary", req)
			}
			return true, nil
		},
	}
	if _, err := approved.awDispatch(nil, awArgs{Action: "chat.delete_permanent", Args: `{"chatId":"c9"}`}); err != nil {
		t.Fatalf("chat.delete_permanent approved error = %v", err)
	}
	if !awContains(control.calls, "DeleteChat") || control.chatID != "c9" {
		t.Fatalf("permanent delete should call DeleteChat for c9; calls=%v id=%q", control.calls, control.chatID)
	}
}

func TestAwChatSendRoutesChatAndText(t *testing.T) {
	ws, control := controlWorkspace(t)
	result, err := ws.awDispatch(nil, awArgs{Action: "chat.send", Args: `{"chatId":"c1","text":"hello"}`})
	if err != nil {
		t.Fatalf("chat.send error = %v", err)
	}
	if control.chatID != "c1" || control.title != "hello" {
		t.Fatalf("send = %q/%q, want c1/hello", control.chatID, control.title)
	}
	if !strings.Contains(result.Result, "accepted") {
		t.Fatalf("chat.send result = %q, want accepted ack", result.Result)
	}
}

func TestAwToolDescriptionListsEnabledGroups(t *testing.T) {
	controlOnly := awToolDescription(Options{Control: &fakeControl{}})
	if !strings.Contains(controlOnly, "app.theme.set") || strings.Contains(controlOnly, "system.selfcode") {
		t.Fatalf("control-only description wrong: %q", controlOnly)
	}
	full := awToolDescription(Options{Control: &fakeControl{}, SelfManage: true, AllowShell: true})
	for _, want := range []string{"app.theme.set", "system.selfcode", "shell.exec"} {
		if !strings.Contains(full, want) {
			t.Fatalf("full description missing %q: %q", want, full)
		}
	}
}

func TestAwNotifyRequiresBody(t *testing.T) {
	ws, _ := controlWorkspace(t)
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.notify", Args: `{"title":"aw"}`}); err == nil {
		t.Fatal("app.notify without body should fail")
	}
}

func TestAwNotifyRoutesToControl(t *testing.T) {
	ws, control := controlWorkspace(t)
	_, err := ws.awDispatch(nil, awArgs{Action: "app.notify", Args: `{"title":"aw","body":"Task done."}`})
	if err != nil {
		t.Fatalf("app.notify error = %v", err)
	}
	if control.title != "aw" {
		t.Fatalf("notify title = %q, want aw", control.title)
	}
	if !awContains(control.calls, "Notify") {
		t.Fatalf("Notify not called; calls = %v", control.calls)
	}
}

func TestAwActionsIncludesNotify(t *testing.T) {
	ws, _ := controlWorkspace(t)
	result, err := ws.awDispatch(nil, awArgs{Action: "aw.actions"})
	if err != nil {
		t.Fatalf("aw.actions error = %v", err)
	}
	if !strings.Contains(result.Result, "app.notify") {
		t.Fatalf("aw.actions missing app.notify: %s", result.Result)
	}
}

func TestAwServerSetValidatesAndRoutes(t *testing.T) {
	ws, control := controlWorkspace(t)
	// Missing server.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.server.set", Args: `{"enabled":true}`}); err == nil {
		t.Fatal("app.server.set without server should fail")
	}
	// Missing enabled.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.server.set", Args: `{"server":"mcp"}`}); err == nil {
		t.Fatal("app.server.set without enabled should fail")
	}
	// Unknown server name.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.server.set", Args: `{"server":"grpc","enabled":true}`}); err == nil ||
		!strings.Contains(err.Error(), "\"mcp\" or \"rest\"") {
		t.Fatalf("unknown server error = %v, want constraint hint", err)
	}
	// Valid MCP enable.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.server.set", Args: `{"server":"mcp","enabled":true}`}); err != nil {
		t.Fatalf("app.server.set mcp error = %v", err)
	}
	if control.serverName != "mcp" || !control.serverEnabled {
		t.Fatalf("SetServerEnabled = %q/%v, want mcp/true", control.serverName, control.serverEnabled)
	}
	// Valid REST disable.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.server.set", Args: `{"server":"REST","enabled":false}`}); err != nil {
		t.Fatalf("app.server.set rest error = %v", err)
	}
	if control.serverName != "rest" || control.serverEnabled {
		t.Fatalf("SetServerEnabled = %q/%v, want rest/false", control.serverName, control.serverEnabled)
	}
}

func TestAwWallpaperGlassSetValidatesAndRoutes(t *testing.T) {
	ws, control := controlWorkspace(t)
	// Missing percent.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.wallpaper.glass.set", Args: `{}`}); err == nil {
		t.Fatal("app.wallpaper.glass.set without percent should fail")
	}
	// Out of range.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.wallpaper.glass.set", Args: `{"percent":150}`}); err == nil ||
		!strings.Contains(err.Error(), "out of range") {
		t.Fatalf("out-of-range percent error = %v, want range error", err)
	}
	// Valid call.
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.wallpaper.glass.set", Args: `{"percent":40}`}); err != nil {
		t.Fatalf("app.wallpaper.glass.set error = %v", err)
	}
	if control.percent != 40 {
		t.Fatalf("SetWallpaperGlass percent = %d, want 40", control.percent)
	}
	if !awContains(control.calls, "SetWallpaperGlass") {
		t.Fatalf("SetWallpaperGlass not called; calls = %v", control.calls)
	}
}

func TestAwProviderTestRoutesToControl(t *testing.T) {
	ws, control := controlWorkspace(t)
	// Missing provider.
	if _, err := ws.awDispatch(nil, awArgs{Action: "provider.test", Args: `{}`}); err == nil {
		t.Fatal("provider.test without provider should fail")
	}
	// Valid call.
	res, err := ws.awDispatch(nil, awArgs{Action: "provider.test", Args: `{"provider":"openai"}`})
	if err != nil {
		t.Fatalf("provider.test error = %v", err)
	}
	if control.title != "openai" {
		t.Fatalf("TestProvider provider = %q, want openai", control.title)
	}
	if !strings.Contains(res.Result, "latency") {
		t.Fatalf("provider.test result = %q, want latency field", res.Result)
	}
	if !awContains(control.calls, "TestProvider") {
		t.Fatalf("TestProvider not called; calls = %v", control.calls)
	}
}

func TestAwProviderManagementActionsRoute(t *testing.T) {
	cases := []struct {
		action string
		args   string
		call   string
	}{
		{"provider.create", `{"name":"My Provider"}`, "CreateCustomProvider"},
		{"provider.rename", `{"provider":"custom-1","name":"New"}`, "RenameCustomProvider"},
		{"provider.delete", `{"provider":"custom-1"}`, "DeleteCustomProvider"},
		{"provider.order.set", `{"order":["a","b"]}`, "SetProviderOrder"},
		{"provider.order.move", `{"provider":"a","direction":"up"}`, "MoveProviderOrder"},
		{"provider.auth.start", `{"provider":"github-copilot"}`, "StartProviderAuth"},
		{"provider.balance", `{"provider":"openrouter"}`, "ProviderBalance"},
	}
	for _, tc := range cases {
		ws, control := controlWorkspace(t)
		if _, err := ws.awDispatch(nil, awArgs{Action: tc.action, Args: tc.args}); err != nil {
			t.Fatalf("%s error = %v", tc.action, err)
		}
		if !awContains(control.calls, tc.call) {
			t.Fatalf("%s did not call %s; calls = %v", tc.action, tc.call, control.calls)
		}
	}

	// Required-arg validation.
	ws, _ := controlWorkspace(t)
	for _, action := range []string{"provider.rename", "provider.delete", "provider.order.move", "provider.auth.start", "provider.balance"} {
		if _, err := ws.awDispatch(nil, awArgs{Action: action, Args: `{}`}); err == nil {
			t.Fatalf("%s without required args should fail", action)
		}
	}
	// Bad direction.
	if _, err := ws.awDispatch(nil, awArgs{Action: "provider.order.move", Args: `{"provider":"a","direction":"sideways"}`}); err == nil {
		t.Fatal("provider.order.move with bad direction should fail")
	}
	// order elements must be strings.
	if _, err := ws.awDispatch(nil, awArgs{Action: "provider.order.set", Args: `{"order":[1,2]}`}); err == nil {
		t.Fatal("provider.order.set with non-string order elements should fail")
	}
}

func TestAwDesktopShowRoutesToControl(t *testing.T) {
	ws, control := controlWorkspace(t)
	if _, err := ws.awDispatch(nil, awArgs{Action: "app.desktop.show"}); err != nil {
		t.Fatalf("app.desktop.show error = %v", err)
	}
	if !awContains(control.calls, "HideAllModules") {
		t.Fatalf("HideAllModules not called; calls = %v", control.calls)
	}
}
