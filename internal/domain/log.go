package domain

import "time"

type LogEntry struct {
	ID           string `json:"id"`
	Timestamp    string `json:"timestamp"`
	Level        int    `json:"level"`
	LevelName    string `json:"levelName"`
	Source       string `json:"source"`
	ModuleID     string `json:"moduleId,omitempty"`
	ModuleType   string `json:"moduleType,omitempty"`
	SessionID    string `json:"sessionId,omitempty"`
	Message      string `json:"message"`
	ContextJSON  string `json:"contextJson,omitempty"`
	ErrorName    string `json:"errorName,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	ErrorStack   string `json:"errorStack,omitempty"`
	CreatedAt    string `json:"createdAt"`

	// Observability v2 (OTel-lite) fields. Additive; older rows leave them empty.
	Event          string `json:"event,omitempty"`          // stable dotted name, e.g. chat.run.completed
	Severity       string `json:"severity,omitempty"`       // trace|debug|info|warn|error|fatal
	TraceID        string `json:"traceId,omitempty"`        // one user turn / app operation (== run id)
	SpanID         string `json:"spanId,omitempty"`         // current sub-operation
	ParentSpanID   string `json:"parentSpanId,omitempty"`   // parent operation for the trace tree
	DurationMs     int64  `json:"durationMs,omitempty"`     // for completed operations
	Status         string `json:"status,omitempty"`         // ok|error|blocked|canceled|denied|timeout
	AttributesJSON string `json:"attributesJson,omitempty"` // scrubbed structured metadata
}

type LogDateCount struct {
	Date    string `json:"date"`
	Entries int    `json:"entries"`
}

type LogQuery struct {
	Date        string
	Level       *int
	Source      string
	Search      string
	EventPrefix string
	TraceID     string
	Status      string
	ModuleID    string
	SessionID   string
	Risk        string
	Limit       int
	Offset      int
}

func LogRetentionCutoff(days int) string {
	return LogRetentionCutoffAt(time.Now(), days)
}

func LogRetentionCutoffAt(now time.Time, days int) string {
	if days < 1 {
		days = 1
	}
	now = now.UTC()
	keepFrom := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(days - 1))
	return keepFrom.Format(time.RFC3339)
}
