package vault

import (
	"testing"
	"time"

	"aw/internal/domain"
)

func TestDeleteLogsOlderThanUsesCalendarRetention(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha-logs"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1)
	for _, entry := range []domain.LogEntry{
		{ID: "log-yesterday", Timestamp: yesterday.Format(time.RFC3339), Level: 6, Source: "test", Message: "old"},
		{ID: "log-yesterday-offset", Timestamp: yesterday.Format("2006-01-02T15:04:05-03:00"), Level: 6, Source: "test", Message: "old offset"},
		{ID: "log-yesterday-nano", Timestamp: yesterday.Format(time.RFC3339Nano), Level: 6, Source: "test", Message: "old nano"},
		{ID: "log-yesterday-legacy", Timestamp: yesterday.Format("2006-01-02 15:04:05"), Level: 6, Source: "test", Message: "old legacy"},
		{ID: "log-today", Timestamp: today.Format(time.RFC3339), Level: 6, Source: "test", Message: "keep"},
	} {
		if _, err := v.InsertLog(entry); err != nil {
			t.Fatalf("InsertLog(%s) error = %v", entry.ID, err)
		}
	}

	deleted, err := v.DeleteLogsOlderThan(1)
	if err != nil {
		t.Fatalf("DeleteLogsOlderThan() error = %v", err)
	}
	if deleted != 4 {
		t.Fatalf("deleted = %d, want 4", deleted)
	}
	logs, err := v.ListLogs(domain.LogQuery{Limit: 10})
	if err != nil {
		t.Fatalf("ListLogs() error = %v", err)
	}
	if len(logs) != 1 || logs[0].ID != "log-today" {
		t.Fatalf("remaining logs = %+v, want only today's log", logs)
	}
}

func TestDeleteAllLogs(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha-logs"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()

	for _, entry := range []domain.LogEntry{
		{ID: "log-1", Timestamp: time.Now().UTC().Format(time.RFC3339), Level: 6, Source: "test", Message: "one"},
		{ID: "log-2", Timestamp: time.Now().UTC().Format(time.RFC3339), Level: 6, Source: "test", Message: "two"},
	} {
		if _, err := v.InsertLog(entry); err != nil {
			t.Fatalf("InsertLog(%s) error = %v", entry.ID, err)
		}
	}

	deleted, err := v.DeleteAllLogs()
	if err != nil {
		t.Fatalf("DeleteAllLogs() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	logs, err := v.ListLogs(domain.LogQuery{Limit: 10})
	if err != nil {
		t.Fatalf("ListLogs() error = %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("remaining logs = %+v, want none", logs)
	}
}

func TestListLogsSupportsObservabilityFilters(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha-logs"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()

	entries := []domain.LogEntry{
		{ID: "match", Timestamp: time.Now().UTC().Format(time.RFC3339), Level: 4, Source: "security", Message: "taint", Event: "external.safety.taint_applied", TraceID: "run-1", Status: "blocked", AttributesJSON: `{"risk":"high"}`},
		{ID: "miss", Timestamp: time.Now().UTC().Format(time.RFC3339), Level: 6, Source: "chat", Message: "done", Event: "chat.run.completed", TraceID: "run-2", Status: "ok", AttributesJSON: `{"risk":"low"}`},
	}
	for _, entry := range entries {
		if _, err := v.InsertLog(entry); err != nil {
			t.Fatalf("InsertLog(%s) error = %v", entry.ID, err)
		}
	}

	logs, err := v.ListLogs(domain.LogQuery{
		EventPrefix: "external.safety",
		TraceID:     "run-1",
		Status:      "blocked",
		Risk:        "high",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListLogs() error = %v", err)
	}
	if len(logs) != 1 || logs[0].ID != "match" {
		t.Fatalf("logs = %+v, want match only", logs)
	}
}
