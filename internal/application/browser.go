package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// browserCommands are the page-level automation commands the Agent Browser
// exposes, mirroring the ui.* action set.
var browserCommands = map[string]bool{
	"navigate":   true,
	"new_tab":    true,
	"snapshot":   true,
	"click":      true,
	"fill":       true,
	"screenshot": true,
	"cdp":        true,
	"close_tab":  true,
	// Internal guard probes. RunBrowserCommand lowercases commands before
	// dispatch, while the browser core historically used camelCase names.
	"navigationtarget": true,
	"currenturl":       true,
}

// StartBrowser launches (or attaches to) the managed agent browser instance —
// always the isolated aw profile; the user's own browser is never touched.
func StartBrowser(ctx context.Context, b ports.BrowserAutomation, id string, opts domain.BrowserStartOptions) (domain.BrowserStatus, error) {
	if b == nil {
		return domain.BrowserStatus{}, errors.New("browser automation is not available")
	}
	return b.Start(ctx, strings.TrimSpace(id), opts)
}

// StopBrowser terminates the managed browser instance.
func StopBrowser(ctx context.Context, b ports.BrowserAutomation, id string) (domain.BrowserStatus, error) {
	if b == nil {
		return domain.BrowserStatus{}, errors.New("browser automation is not available")
	}
	return b.Stop(ctx, strings.TrimSpace(id))
}

// BrowserStatus reports the managed browser instance state.
func BrowserStatus(ctx context.Context, b ports.BrowserAutomation, id string) (domain.BrowserStatus, error) {
	if b == nil {
		return domain.BrowserStatus{}, errors.New("browser automation is not available")
	}
	return b.Status(ctx, strings.TrimSpace(id))
}

// BrowserTabs lists the open pages of the managed browser instance.
func BrowserTabs(ctx context.Context, b ports.BrowserAutomation, id string) ([]domain.BrowserTab, error) {
	if b == nil {
		return nil, errors.New("browser automation is not available")
	}
	return b.Tabs(ctx, strings.TrimSpace(id))
}

// RunBrowserCommand validates and runs one page automation command.
func RunBrowserCommand(ctx context.Context, b ports.BrowserAutomation, id string, command string, params map[string]any) (any, error) {
	if b == nil {
		return nil, errors.New("browser automation is not available")
	}
	command = strings.ToLower(strings.TrimSpace(command))
	if !browserCommands[command] {
		return nil, fmt.Errorf("unknown browser command %q (navigate, new_tab, snapshot, click, fill, screenshot, cdp, close_tab)", command)
	}
	if params == nil {
		params = map[string]any{}
	}
	return b.Command(ctx, strings.TrimSpace(id), command, params)
}
