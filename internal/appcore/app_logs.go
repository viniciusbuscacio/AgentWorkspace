package appcore

import (
	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
)

func (a *App) ListLogs(date string, level int, source string, search string, eventPrefix string, traceID string, status string, moduleID string, sessionID string, risk string, limit int, offset int) dto.LogsResult {
	a.recordActivity()
	var levelPtr *int
	if level >= 0 {
		levelPtr = &level
	}
	logs, err := application.ListLogs(a.vault, domain.LogQuery{
		Date: date, Level: levelPtr, Source: source, Search: search, EventPrefix: eventPrefix, TraceID: traceID, Status: status, ModuleID: moduleID, SessionID: sessionID, Risk: risk, Limit: limit, Offset: offset,
	})
	if err != nil {
		return dto.LogsResult{Error: err.Error()}
	}
	return dto.LogsResult{Success: true, Logs: logs}
}

func (a *App) ListLogDates() dto.LogDatesResult {
	a.recordActivity()
	dates, err := application.ListLogDates(a.vault)
	if err != nil {
		return dto.LogDatesResult{Error: err.Error()}
	}
	return dto.LogDatesResult{Success: true, Dates: dates}
}

func (a *App) CleanOldLogs(days int) dto.LogRetentionResult {
	a.recordActivity()
	deleted, err := application.DeleteLogsOlderThan(a.vault, days)
	if err != nil {
		return dto.LogRetentionResult{Error: err.Error()}
	}
	return dto.LogRetentionResult{Success: true, Deleted: deleted}
}

func (a *App) ClearAllLogs() dto.LogRetentionResult {
	a.recordActivity()
	deleted, err := application.DeleteAllLogs(a.vault)
	if err != nil {
		return dto.LogRetentionResult{Error: err.Error()}
	}
	return dto.LogRetentionResult{Success: true, Deleted: deleted}
}
