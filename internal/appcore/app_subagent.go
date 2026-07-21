package appcore

import (
	"context"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/domain/ports"
	"aw/internal/infrastructure/subagent"
	"aw/internal/infrastructure/tools"
)

// subagentDeps wires the app's runtime, provider resolution (from the vault),
// taint store, logging, and the chat-UI spawn broadcaster into the shared
// spawn runner. All worker behavior (prompt, allowlist, taint) lives in
// internal/infrastructure/subagent and is shared with the awharness test
// harness; this only injects the app's collaborators.
func (a *App) subagentDeps() subagent.Deps {
	return subagent.Deps{
		Runtime:     a.agent,
		ModelConfig: subagentModelConfigFn(a.vault),
		Taint:       a.taintStore,
		Log: func(c context.Context, ev subagent.LogEvent) {
			a.emitLogEvent(c, subagentLogEvent(ev))
		},
		NotifySpawn: a.emitSpawnStatus,
	}
}

// newSpawnFn builds the tools.SpawnFn behind system.spawn's generic multi-task
// form: it runs up to MaxConcurrentSpawn disposable subagents in parallel and
// broadcasts their lifecycle to the chat UI. It adapts the tool-facing task/result
// types to the subagent package's.
func (a *App) newSpawnFn(base func() tools.Options) func(ctx context.Context, tasks []tools.SpawnTaskInput, timeoutMs int) ([]tools.SpawnTaskResult, error) {
	return subagent.NewSpawnFn(a.subagentDeps(), base)
}

// emitSpawnStatus broadcasts a generic-spawn lifecycle status to the chat UI as a
// chat:subagent event, correlated to the current chat + run via the context
// scopes so the frontend can attach the card to the right assistant turn.
func (a *App) emitSpawnStatus(ctx context.Context, status subagent.SpawnStatus) {
	runID := domain.ExternalTaintScope(ctx)
	// Also collected per run so the persisted reply can embed the results at
	// its ::spawn marker (the card survives reload/restart).
	a.spawnRecords.observe(runID, status)
	payload := map[string]any{
		"chatId": domain.ChatSessionScope(ctx),
		"runId":  runID,
		"phase":  status.Phase,
	}
	if len(status.Tasks) > 0 {
		payload["tasks"] = status.Tasks
	}
	if status.TaskID != "" {
		payload["taskId"] = status.TaskID
	}
	if status.Result != nil {
		payload["result"] = status.Result
	}
	if status.Results != nil {
		payload["results"] = status.Results
	}
	a.emitChatEvent("chat:subagent", payload)
}

// subagentLogEvent maps the infrastructure subagent log payload onto the
// application LogEvent the app emits.
func subagentLogEvent(ev subagent.LogEvent) application.LogEvent {
	return application.LogEvent{
		Event:        ev.Event,
		Severity:     ev.Severity,
		Source:       ev.Source,
		Message:      ev.Message,
		Status:       ev.Status,
		DurationMs:   ev.DurationMs,
		ErrorMessage: ev.ErrorMessage,
		Attributes:   ev.Attributes,
	}
}

// subagentModelConfigFn resolves the active provider runtime config from a
// provider secret store into the model config the isolated subagent runs with.
func subagentModelConfigFn(store ports.ProviderSecretStore) func() (domain.ModelConfig, error) {
	return func() (domain.ModelConfig, error) {
		cfg, err := application.ResolveProviderRuntimeConfig(store)
		if err != nil {
			return domain.ModelConfig{}, err
		}
		return application.ModelConfigFromProviderRuntimeConfig(cfg), nil
	}
}
