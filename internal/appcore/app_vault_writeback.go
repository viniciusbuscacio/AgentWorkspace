package appcore

import (
	"context"
	"time"

	"aw/internal/application"
	"aw/internal/domain/ports"
	"aw/internal/infrastructure/appconfig"
)

// writeBackFailureNoticeThreshold: snapshot failures are retried quietly every
// tick; only this many consecutive failures surface one notification
// (owner decision, spec Q17). The counter resets on the first success.
const writeBackFailureNoticeThreshold = 20

// vaultWriteBackLoop runs the working-copy write-back (spec:
// docs/specs/vault-working-copy.md): while the vault is unlocked, each
// 1-minute tick snapshots the live working copy over the master, skipping
// when nothing changed. When the master changed outside this session (another
// machine's snapshot arrived via sync), write-back freezes and the loop
// resyncs at the first idle moment — an invisible catch-up, no password
// prompt. Master manda: local changes since the last snapshot are discarded.
func (a *App) vaultWriteBackLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.vaultWriteBackTick(ctx)
		}
	}
}

func (a *App) vaultWriteBackTick(ctx context.Context) {
	if a.vault == nil {
		return
	}
	result := application.VaultWriteBackTick(a.vault)
	switch {
	case result.MasterChanged:
		a.vaultResyncWhenIdle(ctx)
	case result.Err != nil:
		a.writeBackFailures++
		if a.writeBackFailures == writeBackFailureNoticeThreshold {
			enabled := application.DesktopNotificationsEnabled(appconfig.Store{})
			_ = application.NotifyUser(a.notifier, enabled, "Vault sync falling behind",
				"Saving the vault to its folder keeps failing; your data is safe in the local working copy. Retrying every minute.")
		}
	default:
		a.writeBackFailures = 0
	}
}

// vaultResyncWhenIdle adopts a master updated by another machine as soon as
// no chat run is active — never severing an in-flight turn. After the swap
// the in-memory ADK sessions are stale by definition, so they are dropped and
// re-seed from the vault on each chat's next turn (the same path an app
// restart uses); the UI reloads through chat:refresh.
func (a *App) vaultResyncWhenIdle(ctx context.Context) {
	if a.chatRunner.ActiveCount() > 0 {
		return // next tick retries; snapshots stay frozen meanwhile
	}
	var sessions ports.ChatSessionResetter
	if a.agent != nil {
		sessions = a.agent
	}
	if err := application.ResyncVaultFromMaster(a.vault, sessions); err != nil {
		a.emitLogEvent(ctx, application.LogEvent{
			Event: "vault.resync.failed", Severity: "warn", Source: "vault",
			Message: "vault resync from changed master failed", Status: "error",
			ErrorMessage: err.Error(),
		})
		return
	}
	a.writeBackFailures = 0
	a.emitLogEvent(ctx, application.LogEvent{
		Event: "vault.resync.completed", Severity: "info", Source: "vault",
		Message: "vault resynced from master updated elsewhere", Status: "ok",
	})
	a.emitChatEvent("chat:refresh", map[string]any{})
	a.emitChatEvent("vault:resynced", map[string]any{})
}
