package appcore

import (
	"time"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
	"aw/internal/infrastructure/appconfig"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// BrowserStart connects the agent's browser: it launches the isolated
// aw-profile instance (or attaches to one already running) — the user's own
// browser is never touched.
func (a *App) BrowserStart(id string) dto.BrowserStatusResult {
	a.recordActivity()
	started := time.Now()
	status, err := application.StartBrowser(a.contextOrBackground(), a.browser, id, domain.BrowserStartOptions{})
	if err != nil {
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:        "browser.connection_failed",
			Severity:     "error",
			Source:       "browser",
			ModuleID:     id,
			ModuleType:   "browser",
			Message:      "browser failed to connect",
			Status:       "error",
			DurationMs:   time.Since(started).Milliseconds(),
			ErrorMessage: err.Error(),
		})
		return browserStatusResult(status, err)
	}
	event := "browser.connected"
	message := "browser connected"
	if !status.Running {
		event = "browser.no_target"
		message = "browser did not come up"
	}
	a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
		Event:      event,
		Severity:   "info",
		Source:     "browser",
		ModuleID:   id,
		ModuleType: "browser",
		Message:    message,
		Status:     "ok",
		DurationMs: time.Since(started).Milliseconds(),
		Attributes: browserLogAttributes(status),
	})
	return browserStatusResult(status, nil)
}

// BrowserStop terminates the managed browser.
func (a *App) BrowserStop(id string) dto.BrowserStatusResult {
	a.recordActivity()
	status, err := application.StopBrowser(a.contextOrBackground(), a.browser, id)
	if err != nil {
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:        "browser.disconnect_failed",
			Severity:     "error",
			Source:       "browser",
			ModuleID:     id,
			ModuleType:   "browser",
			Message:      "browser failed to disconnect",
			Status:       "error",
			ErrorMessage: err.Error(),
		})
		return browserStatusResult(status, err)
	}
	a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
		Event:      "browser.disconnected",
		Severity:   "info",
		Source:     "browser",
		ModuleID:   id,
		ModuleType: "browser",
		Message:    "browser disconnected",
		Status:     "ok",
		Attributes: browserLogAttributes(status),
	})
	return browserStatusResult(status, nil)
}

// BrowserStatus reports the managed browser state for the module view.
func (a *App) BrowserStatus(id string) dto.BrowserStatusResult {
	result := browserStatusResult(application.BrowserStatus(a.contextOrBackground(), a.browser, id))
	result.Autostart = application.BrowserAutostart(a.vault, id)
	return result
}

func (a *App) GetBrowserExecutable(id string) dto.BrowserExecutableResult {
	result, err := application.BrowserExecutable(a.contextOrBackground(), a.browser, appconfig.Store{}, id)
	return browserExecutableResult(result, err)
}

func (a *App) SetBrowserExecutable(id string, path string) dto.BrowserExecutableResult {
	result, err := application.SaveBrowserExecutable(a.contextOrBackground(), a.browser, appconfig.Store{}, id, path)
	return browserExecutableResult(result, err)
}

func (a *App) PickBrowserExecutable(id string) dto.BrowserExecutableResult {
	if a.ctx == nil {
		return dto.BrowserExecutableResult{Error: "the app window is not available for the file picker"}
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: application.BrowserExecutableDialogTitle(id),
		Filters: []wailsruntime.FileFilter{{
			DisplayName: "Executable",
			Pattern:     "*.exe",
		}},
	})
	if err != nil {
		return dto.BrowserExecutableResult{Error: err.Error()}
	}
	if path == "" {
		return dto.BrowserExecutableResult{Canceled: true}
	}
	return dto.BrowserExecutableResult{Success: true, Path: path}
}

// SetBrowserAutostart persists whether this Agent Browser starts automatically
// when the vault unlocks. It does not start or stop a running browser; it only
// records the preference, then returns the refreshed status.
func (a *App) SetBrowserAutostart(id string, enabled bool) dto.BrowserStatusResult {
	if err := application.SetBrowserAutostart(a.vault, id, enabled); err != nil {
		return dto.BrowserStatusResult{Error: err.Error()}
	}
	return a.BrowserStatus(id)
}

// startEnabledBrowsers starts the agent browser for every added Agent Browser
// module whose auto-start toggle is on, mirroring the MCP/REST auto-start
// after a vault unlock. Off by default — when on, unlocking the vault opens
// the agent's isolated browser window.
func (a *App) startEnabledBrowsers() {
	ids, err := application.AddedModuleIDs(a.workspace, domain.ModuleCatalog())
	if err != nil {
		return
	}
	for _, id := range ids {
		if id != domain.BrowserModuleChrome && id != domain.BrowserModuleEdge {
			continue
		}
		if !application.BrowserAutostart(a.vault, id) {
			continue
		}
		status, err := application.StartBrowser(a.contextOrBackground(), a.browser, id, domain.BrowserStartOptions{})
		if err != nil {
			a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
				Event:        "browser.connection_failed",
				Severity:     "error",
				Source:       "browser",
				ModuleID:     id,
				ModuleType:   "browser",
				Message:      "browser failed to connect",
				Status:       "error",
				ErrorMessage: err.Error(),
				Attributes: map[string]any{
					"autostart": true,
				},
			})
			continue
		}
		if !status.Running {
			continue
		}
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:      "browser.connected",
			Severity:   "info",
			Source:     "browser",
			ModuleID:   id,
			ModuleType: "browser",
			Message:    "browser connected",
			Status:     "ok",
			Attributes: mergeLogAttributes(browserLogAttributes(status), map[string]any{"autostart": true}),
		})
	}
}

// BrowserTabs lists the open tabs for the module view.
func (a *App) BrowserTabs(id string) dto.BrowserTabsResult {
	tabs, err := application.BrowserTabs(a.contextOrBackground(), a.browser, id)
	if err != nil {
		return dto.BrowserTabsResult{Error: err.Error()}
	}
	return dto.BrowserTabsResult{Success: true, Tabs: tabs}
}

// BrowserScreenshot captures the current page as a PNG data URI.
func (a *App) BrowserScreenshot(id string) dto.BrowserScreenshotResult {
	a.recordActivity()
	result, err := application.RunBrowserCommand(a.contextOrBackground(), a.browser, id, "screenshot", nil)
	if err != nil {
		return dto.BrowserScreenshotResult{Error: err.Error()}
	}
	payload, _ := result.(map[string]any)
	dataURI, _ := payload["dataUri"].(string)
	return dto.BrowserScreenshotResult{Success: true, DataURI: dataURI}
}

func browserStatusResult(status domain.BrowserStatus, err error) dto.BrowserStatusResult {
	if err != nil {
		return dto.BrowserStatusResult{Error: err.Error()}
	}
	return dto.BrowserStatusResult{Success: true, Status: &status}
}

func browserExecutableResult(settings application.BrowserExecutableSettings, err error) dto.BrowserExecutableResult {
	if err != nil {
		return dto.BrowserExecutableResult{Error: err.Error()}
	}
	return dto.BrowserExecutableResult{
		Success:     true,
		Path:        settings.Path,
		DefaultPath: settings.DefaultPath,
		Custom:      settings.Custom,
	}
}

func browserLogAttributes(status domain.BrowserStatus) map[string]any {
	return map[string]any{
		"browser.id":          status.ID,
		"browser.running":     status.Running,
		"browser.headless":    status.Headless,
		"browser.port":        status.Port,
		"browser.profile_dir": browserProfileKindForLog(status.ProfileDir),
		"browser.binary.hash": application.HashForLog(status.Binary),
	}
}

func browserProfileKindForLog(profileDir string) string {
	if profileDir == "" {
		return ""
	}
	return "aw"
}

func mergeLogAttributes(base map[string]any, extra map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}
