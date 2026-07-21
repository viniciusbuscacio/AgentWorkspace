package application

import (
	"encoding/json"
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

var logLevelNames = map[int]string{
	0: "Emergency", 1: "Alert", 2: "Critical", 3: "Error",
	4: "Warning", 5: "Notice", 6: "Info", 7: "Debug",
}

func WriteLog(store ports.LogStore, entry domain.LogEntry) {
	if store == nil {
		return
	}
	entry.Message = ScrubChatSecrets(entry.Message)
	entry.ErrorName = ScrubChatSecrets(entry.ErrorName)
	entry.ErrorMessage = ScrubChatSecrets(entry.ErrorMessage)
	entry.ErrorStack = ScrubChatSecrets(entry.ErrorStack)
	entry.ContextJSON = ScrubChatSecrets(entry.ContextJSON)
	entry.AttributesJSON = ScrubChatSecrets(entry.AttributesJSON)
	if entry.LevelName == "" {
		entry.LevelName = logLevelNames[entry.Level]
	}
	if entry.LevelName == "" {
		entry.LevelName = "Info"
	}
	if entry.Source == "" {
		entry.Source = "app"
	}
	_, _ = store.InsertLog(entry)
}

func WriteLogContext(store ports.LogStore, level int, source, message string, context map[string]any) {
	data, _ := json.Marshal(context)
	WriteLog(store, domain.LogEntry{
		Level:       level,
		Source:      source,
		Message:     message,
		ContextJSON: string(data),
	})
}

func ListLogs(store ports.LogStore, query domain.LogQuery) ([]domain.LogEntry, error) {
	if store == nil {
		return nil, fmt.Errorf("log store is required")
	}
	if query.Limit <= 0 {
		query.Limit = 50
	}
	if query.Limit > 200 {
		query.Limit = 200
	}
	return store.ListLogs(query)
}

func ListLogDates(store ports.LogStore) ([]domain.LogDateCount, error) {
	if store == nil {
		return nil, fmt.Errorf("log store is required")
	}
	return store.ListLogDates()
}

func DeleteLogsOlderThan(store ports.LogStore, days int) (int, error) {
	if store == nil {
		return 0, fmt.Errorf("log store is required")
	}
	if days < 1 || days > 90 {
		return 0, fmt.Errorf("days must be between 1 and 90")
	}
	return store.DeleteLogsOlderThan(days)
}

func DeleteAllLogs(store ports.LogStore) (int, error) {
	if store == nil {
		return 0, fmt.Errorf("log store is required")
	}
	return store.DeleteAllLogs()
}

func LogActionOutcome(store ports.LogStore, action string, ok bool, err error) {
	if strings.HasPrefix(action, "logs.") {
		return
	}
	level := 6
	outcome := "ok"
	if err != nil {
		level = 3
		outcome = "error"
	} else if !ok {
		level = 4
		outcome = "blocked"
	}
	WriteLogContext(store, level, "aw", "aw action "+outcome, map[string]any{
		"action":  action,
		"outcome": outcome,
	})
}

func RetentionCutoff(days int) string {
	return domain.LogRetentionCutoff(days)
}
