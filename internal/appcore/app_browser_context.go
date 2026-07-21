package appcore

import (
	"context"
	"time"

	"aw/internal/application"
	"aw/internal/domain"
)

const browserSnapshotTimeout = 1500 * time.Millisecond

func (a *App) browserSnapshotInput(ctx context.Context) application.BrowserSnapshotInput {
	if ctx == nil {
		ctx = context.Background()
	}
	snapshotCtx, cancel := context.WithTimeout(ctx, browserSnapshotTimeout)
	defer cancel()

	input := application.BrowserSnapshotInput{
		RefreshedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	for _, item := range []struct {
		id   string
		name string
	}{
		{id: domain.BrowserModuleEdge, name: "Microsoft Edge"},
		{id: domain.BrowserModuleChrome, name: "Google Chrome"},
	} {
		status, err := application.BrowserStatus(snapshotCtx, a.browser, item.id)
		if err != nil {
			continue
		}
		if !status.Running {
			continue
		}
		browser := application.BrowserSnapshotBrowser{
			ID:     item.id,
			Name:   item.name,
			Status: status,
		}
		tabs, err := application.BrowserTabs(snapshotCtx, a.browser, item.id)
		if err != nil {
			browser.Error = err.Error()
		} else {
			browser.Tabs = tabs
		}
		input.Browsers = append(input.Browsers, browser)
	}
	return input
}
