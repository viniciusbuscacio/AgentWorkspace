// Package subagent holds the generic parallel spawn runner (system.spawn)
// shared by the desktop app (composition root) and the awharness test harness.
// It lives in the infrastructure layer because it wires the tools registry and
// the model runtime; application-layer helpers (provider resolution, logging)
// are injected as function values so this package depends only on domain +
// infrastructure peers.
//
// The browser-read subagent that used to live here is gone: page reads happen
// directly in the main chat through the sanitized/enveloped/tainted browser.*
// actions — a second model in the read path added latency and an injectable
// hop without adding protection.
package subagent

import (
	"context"

	"aw/internal/domain"

	"google.golang.org/adk/tool"
)

// IsolatedRunner is the capability the spawn workers need from the agent
// runtime: run one isolated turn with a restricted tool set. *agent.Runtime
// satisfies it.
type IsolatedRunner interface {
	RunIsolated(ctx context.Context, cfg domain.ModelConfig, text string, instruction string, tools []tool.Tool, maxOutputTokens int32) (domain.AgentReply, error)
}

// TaintRecorder records the per-turn external-content taint produced by
// untrusted browser/web/email reads. *externaltaint.Store satisfies it.
type TaintRecorder interface {
	Record(scope string, safety domain.ExternalContentSafety)
}

// LogEvent is the structured log payload emitted for subagent lifecycle events.
// It mirrors application.LogEvent's fields; the caller's Log closure maps it
// onto the real logger so this package needs no application import.
type LogEvent struct {
	Event        string
	Severity     string
	Source       string
	Message      string
	Status       string
	DurationMs   int64
	ErrorMessage string
	Attributes   map[string]any
}

// Deps carries the explicit dependencies the spawn runner needs. It is shared
// by the desktop app and the test harness so both drive identical behavior.
type Deps struct {
	// Runtime runs the isolated worker turn. Required.
	Runtime IsolatedRunner
	// ModelConfig resolves the active provider runtime config (model + key)
	// for the isolated run. Required.
	ModelConfig func() (domain.ModelConfig, error)
	// Taint records the untrusted-content taint for the turn. Optional.
	Taint TaintRecorder
	// Log emits a lifecycle event. Optional (no-op when nil).
	Log func(ctx context.Context, ev LogEvent)
	// NotifySpawn broadcasts a generic-spawn lifecycle status to the chat UI
	// (start/task-done/end). Optional (no-op when nil).
	NotifySpawn func(ctx context.Context, status SpawnStatus)
}

func emitLog(ctx context.Context, deps Deps, ev LogEvent) {
	if deps.Log == nil {
		return
	}
	deps.Log(ctx, ev)
}
