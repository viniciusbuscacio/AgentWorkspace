package appcore

import (
	"context"

	"aw/internal/application"
)

func (a *App) emitLogEvent(ctx context.Context, ev application.LogEvent) {
	if a.vault == nil {
		// Headless / pre-unlock: there is no vault to log into. Guard here
		// because a nil *Vault carried in the ports.LogStore interface is a
		// typed-nil, so EmitEvent's `store == nil` check would miss it and
		// Vault.InsertLog would panic on the nil receiver.
		return
	}
	if ctx == nil {
		ctx = a.contextOrBackground()
	}
	application.EmitEvent(ctx, a.vault, ev)
}

func (a *App) queueLogEvent(ev application.LogEvent) {
	a.pendingLogMu.Lock()
	a.pendingLogEvents = append(a.pendingLogEvents, ev)
	a.pendingLogMu.Unlock()
}

func (a *App) flushQueuedLogEvents(ctx context.Context) {
	a.pendingLogMu.Lock()
	events := append([]application.LogEvent(nil), a.pendingLogEvents...)
	a.pendingLogEvents = nil
	a.pendingLogMu.Unlock()
	for _, ev := range events {
		a.emitLogEvent(ctx, ev)
	}
}
