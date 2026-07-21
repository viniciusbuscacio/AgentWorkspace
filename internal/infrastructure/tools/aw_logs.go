package tools

import (
	"context"

	"aw/internal/domain"
)

type LogsFuncs struct {
	List func(ctx context.Context, query domain.LogQuery) (any, error)
}

func registerLogsActions(reg map[string]AwActionHandler) {
	reg["logs.list"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		date, _, err := awStringArg(args, "date")
		if err != nil {
			return "", err
		}
		source, _, err := awStringArg(args, "source")
		if err != nil {
			return "", err
		}
		search, _, err := awStringArg(args, "search")
		if err != nil {
			return "", err
		}
		eventPrefix, _, err := awStringArg(args, "eventPrefix")
		if err != nil {
			return "", err
		}
		traceID, _, err := awStringArg(args, "traceId")
		if err != nil {
			return "", err
		}
		status, _, err := awStringArg(args, "status")
		if err != nil {
			return "", err
		}
		moduleID, _, err := awStringArg(args, "moduleId")
		if err != nil {
			return "", err
		}
		sessionID, _, err := awStringArg(args, "sessionId")
		if err != nil {
			return "", err
		}
		risk, _, err := awStringArg(args, "risk")
		if err != nil {
			return "", err
		}
		limit, hasLimit, err := awIntArg(args, "limit")
		if err != nil {
			return "", err
		}
		level, hasLevel, err := awIntArg(args, "level")
		if err != nil {
			return "", err
		}
		var levelPtr *int
		if hasLevel {
			levelPtr = &level
		}
		if !hasLimit {
			limit = 50
		}
		result, err := w.logs.List(ctx, domain.LogQuery{Date: date, Source: source, Search: search, EventPrefix: eventPrefix, TraceID: traceID, Status: status, ModuleID: moduleID, SessionID: sessionID, Risk: risk, Limit: limit, Level: levelPtr})
		if err != nil {
			return "", err
		}
		// Logs can carry text that a third party planted (a web page title, an
		// email subject) captured during an earlier action. The agent still gets
		// every entry verbatim; the envelope only labels the payload as untrusted
		// data so a planted string can't hijack a later step via prompt injection.
		processed := w.processExternalContent(ctx, result, externalProcessOptions{
			SourceType: domain.ExternalSourceToolOutput,
			Origin:     "logs.list",
			Mode:       domain.ExternalContentModePreserveVerbatim,
		})
		return awJSON(map[string]any{
			"logs":            result,
			"external_safety": processed.ExternalSafety,
		})
	}
}
