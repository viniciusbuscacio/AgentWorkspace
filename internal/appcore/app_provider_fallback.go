package appcore

import (
	"fmt"
	"time"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/infrastructure/appconfig"
)

// providerFallbackOrder returns the user-defined provider priority list used to
// build the LLM fallback chain. Empty falls back to the active provider.
func (a *App) providerFallbackOrder() []string {
	return application.LoadProviderFallbackOrder(a.workspace)
}

// GetProviderFallbackOrder exposes the persisted fallback priority list to the
// Settings UI.
func (a *App) GetProviderFallbackOrder() []string {
	order := a.providerFallbackOrder()
	if order == nil {
		return []string{}
	}
	return order
}

// SetProviderFallbackOrder persists the priority list (index 0 = #1). The
// order IS the default order: #1 becomes the active provider (the Agent
// Workspace default), #2 the first fallback, and so on. Unconfigured/disabled
// entries are skipped when picking the new active.
func (a *App) SetProviderFallbackOrder(ids []string) OperationResult {
	if err := application.SaveProviderFallbackOrder(a.workspace, ids); err != nil {
		return a.basicOperationResult(err)
	}
	if _, err := application.SyncActiveProviderToOrder(a.vault, ids); err != nil {
		return a.basicOperationResult(err)
	}
	return a.basicOperationResult(nil)
}

// GetProviderCooldownMinutes returns the circuit-breaker bench time in minutes.
func (a *App) GetProviderCooldownMinutes() int {
	return application.GetProviderCooldownMinutes(a.workspace)
}

// SetProviderCooldownMinutes persists the bench time and updates the live
// in-memory breaker so the change takes effect without a restart.
func (a *App) SetProviderCooldownMinutes(minutes int) OperationResult {
	if err := application.SaveProviderCooldownMinutes(a.workspace, minutes); err != nil {
		return a.basicOperationResult(err)
	}
	if a.cooldown != nil {
		a.cooldown.SetDuration(time.Duration(minutes) * time.Minute)
	}
	return a.basicOperationResult(nil)
}

// GetBenchedProviders returns the provider IDs currently in cooldown, so the
// Settings UI can show which accounts are temporarily benched.
func (a *App) GetBenchedProviders() []string {
	if a.cooldown == nil {
		return []string{}
	}
	return a.cooldown.Benched()
}

// handleProviderFallback surfaces an automatic provider failover to the user:
// a chat system-message (so the timeline shows which account took over) and a
// transient desktop notification. The failed provider has already been benched
// by the circuit breaker; this is purely the user-facing signal.
func (a *App) handleProviderFallback(chatID, runID string, from, to domain.ProviderRuntimeConfig, reason error) {
	reasonText := ""
	if reason != nil {
		reasonText = reason.Error()
	}
	a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
		Event:        "provider.fallback.used",
		Severity:     "warn",
		Source:       "provider",
		SessionID:    chatID,
		TraceID:      runID,
		Message:      "provider fallback used",
		Status:       "ok",
		ErrorMessage: reasonText,
		Attributes: map[string]any{
			"from.provider.id":   from.ProviderID,
			"from.provider.name": from.ProviderName,
			"from.model":         from.Model,
			"to.provider.id":     to.ProviderID,
			"to.provider.name":   to.ProviderName,
			"to.model":           to.Model,
		},
	})

	a.emitChatEvent("chat:fallback", map[string]any{
		"chatId":       chatID,
		"runId":        runID,
		"fromProvider": from.ProviderID,
		"fromName":     from.ProviderName,
		"toProvider":   to.ProviderID,
		"toName":       to.ProviderName,
		"reason":       reasonText,
	})

	enabled := application.DesktopNotificationsEnabled(appconfig.Store{})
	_ = application.NotifyUser(a.notifier, enabled,
		fmt.Sprintf("%s unavailable", from.ProviderName),
		fmt.Sprintf("Switched to %s after %s failed.", to.ProviderName, from.ProviderName),
	)
}
