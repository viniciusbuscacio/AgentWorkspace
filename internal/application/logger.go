package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// LogEvent is the OTel-lite structured event the app emits (see
// docs/plans/logs-observability-v2-spec.md). Severity drives the syslog numeric
// level; attributes are scrubbed and stored as JSON.
type LogEvent struct {
	Event        string // stable dotted name, e.g. "chat.run.completed"
	Severity     string // trace|debug|info|warn|error|fatal (default info)
	Source       string // app|chat|tool|browser|security|vault|provider|module (default app)
	Message      string
	ModuleID     string
	ModuleType   string
	SessionID    string
	TraceID      string // run/turn id; defaults to the external-taint scope on ctx
	SpanID       string
	ParentSpanID string
	DurationMs   int64
	Status       string // ok|error|blocked|canceled|denied|timeout
	ErrorName    string
	ErrorMessage string
	ErrorStack   string
	Attributes   map[string]any
}

func HashForLog(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

// severityToLevel maps an OTel severity to the existing syslog numeric level
// (kept for UI compatibility) and a canonical level name.
func severityToLevel(severity string) (int, string) {
	switch normalizeSeverity(severity) {
	case "fatal":
		return 2, "Critical"
	case "error":
		return 3, "Error"
	case "warn":
		return 4, "Warning"
	case "debug", "trace":
		return 7, "Debug"
	default: // info
		return 6, "Info"
	}
}

func normalizeSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "trace", "debug", "info", "warn", "error", "fatal":
		return strings.ToLower(strings.TrimSpace(severity))
	case "warning":
		return "warn"
	default:
		return "info"
	}
}

// EmitEvent writes one structured observability event. It is best-effort:
// attributes are scrubbed before write and any store error is swallowed, so a
// logging failure never breaks the caller's operation. `logs.*` events are
// dropped to prevent recursive logging.
func EmitEvent(ctx context.Context, store ports.LogStore, ev LogEvent) {
	if store == nil || strings.HasPrefix(ev.Event, "logs.") {
		return
	}
	severity := normalizeSeverity(ev.Severity)
	level, levelName := severityToLevel(severity)

	traceID := strings.TrimSpace(ev.TraceID)
	if traceID == "" {
		traceID = domain.ExternalTaintScope(ctx)
	}
	source := strings.TrimSpace(ev.Source)
	if source == "" {
		source = "app"
	}
	sessionID := strings.TrimSpace(ev.SessionID)
	if sessionID == "" {
		sessionID = domain.ChatSessionScope(ctx)
	}

	attributesJSON := ""
	if len(ev.Attributes) > 0 {
		if data, err := json.Marshal(ev.Attributes); err == nil {
			attributesJSON = string(data)
		}
	}

	// WriteLog scrubs message/error/context/attributes and swallows store errors.
	WriteLog(store, domain.LogEntry{
		Event:          strings.TrimSpace(ev.Event),
		Severity:       severity,
		Level:          level,
		LevelName:      levelName,
		Source:         source,
		ModuleID:       ev.ModuleID,
		ModuleType:     ev.ModuleType,
		SessionID:      sessionID,
		Message:        ev.Message,
		TraceID:        traceID,
		SpanID:         ev.SpanID,
		ParentSpanID:   ev.ParentSpanID,
		DurationMs:     ev.DurationMs,
		Status:         ev.Status,
		ErrorName:      ev.ErrorName,
		ErrorMessage:   ev.ErrorMessage,
		ErrorStack:     ev.ErrorStack,
		AttributesJSON: attributesJSON,
	})
}
