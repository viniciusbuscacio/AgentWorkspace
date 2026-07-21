package application

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"aw/internal/domain"
)

type fakeLogStore struct {
	entries []domain.LogEntry
	fail    bool
}

func (f *fakeLogStore) InsertLog(entry domain.LogEntry) (domain.LogEntry, error) {
	if f.fail {
		return domain.LogEntry{}, fmt.Errorf("store down")
	}
	f.entries = append(f.entries, entry)
	return entry, nil
}
func (f *fakeLogStore) ListLogs(domain.LogQuery) ([]domain.LogEntry, error) { return f.entries, nil }
func (f *fakeLogStore) ListLogDates() ([]domain.LogDateCount, error)        { return nil, nil }
func (f *fakeLogStore) DeleteLogsOlderThan(int) (int, error)                { return 0, nil }
func (f *fakeLogStore) DeleteAllLogs() (int, error)                         { return 0, nil }

func TestEmitEventMapsSeverityAndStoresStructuredFields(t *testing.T) {
	store := &fakeLogStore{}
	ctx := domain.WithExternalTaintScope(context.Background(), "run-1")
	EmitEvent(ctx, store, LogEvent{
		Event:      "chat.run.completed",
		Severity:   "info",
		Source:     "chat",
		Message:    "chat run completed",
		Status:     "ok",
		Attributes: map[string]any{"tokens.input": 1234, "model": "gpt-x"},
	})
	if len(store.entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(store.entries))
	}
	e := store.entries[0]
	if e.Event != "chat.run.completed" || e.Severity != "info" || e.Level != 6 || e.Status != "ok" {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if e.TraceID != "run-1" {
		t.Fatalf("trace id should default from ctx scope, got %q", e.TraceID)
	}
	ctxWithSession := domain.WithChatSessionScope(ctx, "chat-1")
	store.entries = nil
	EmitEvent(ctxWithSession, store, LogEvent{Event: "tool.call.completed"})
	if store.entries[0].SessionID != "chat-1" {
		t.Fatalf("session id should default from ctx scope, got %q", store.entries[0].SessionID)
	}
	if !strings.Contains(e.AttributesJSON, "tokens.input") {
		t.Fatalf("attributes not stored: %q", e.AttributesJSON)
	}
}

func TestEmitEventSeverityLevels(t *testing.T) {
	cases := map[string]int{"fatal": 2, "error": 3, "warn": 4, "warning": 4, "info": 6, "debug": 7, "trace": 7, "": 6, "bogus": 6}
	for sev, wantLevel := range cases {
		store := &fakeLogStore{}
		EmitEvent(context.Background(), store, LogEvent{Event: "x.y.z", Severity: sev})
		if store.entries[0].Level != wantLevel {
			t.Fatalf("severity %q -> level %d, want %d", sev, store.entries[0].Level, wantLevel)
		}
	}
}

func TestEmitEventScrubsSecretsInAttributes(t *testing.T) {
	store := &fakeLogStore{}
	EmitEvent(context.Background(), store, LogEvent{
		Event:      "tool.call.completed",
		Attributes: map[string]any{"note": "key sk-ABCDEFGHIJKLMNOP1234 leaked"},
	})
	if strings.Contains(store.entries[0].AttributesJSON, "sk-ABCDEFGHIJKLMNOP1234") {
		t.Fatalf("secret survived scrubbing: %q", store.entries[0].AttributesJSON)
	}
}

func TestEmitEventNeverFailsCaller(_ *testing.T) {
	// nil store and a failing store must both be no-ops, never panic.
	EmitEvent(context.Background(), nil, LogEvent{Event: "x.y.z"})
	EmitEvent(context.Background(), &fakeLogStore{fail: true}, LogEvent{Event: "x.y.z"})
}

func TestEmitEventDropsLogsRecursion(t *testing.T) {
	store := &fakeLogStore{}
	EmitEvent(context.Background(), store, LogEvent{Event: "logs.list.requested"})
	if len(store.entries) != 0 {
		t.Fatalf("logs.* events must not be logged (recursion), got %+v", store.entries)
	}
}
