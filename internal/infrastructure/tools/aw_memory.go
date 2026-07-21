package tools

import (
	"context"

	"aw/internal/domain"
)

func registerMemoryActions(reg map[string]AwActionHandler) {
	reg["memory.remember"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.userMemoryFn == nil {
			return "", errUnavailable("user memory")
		}
		key, err := awRequiredStringArg(args, "key")
		if err != nil {
			return "", err
		}
		category, err := awRequiredStringArg(args, "category")
		if err != nil {
			return "", err
		}
		content, err := awRequiredStringArg(args, "content")
		if err != nil {
			return "", err
		}
		if err := w.requireExternalActionGuard(ctx, "memory.remember", domain.ExternalActionPersist, args); err != nil {
			return "", err
		}
		result, err := w.userMemoryFn(ctx, key, category, content)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["memory.forget"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.userMemoryForget == nil {
			return "", errUnavailable("user memory")
		}
		key, err := awRequiredStringArg(args, "key")
		if err != nil {
			return "", err
		}
		result, err := w.userMemoryForget(ctx, key)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["memory.chat.search"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.chatSearchFn == nil {
			return "", errUnavailable("chat memory")
		}
		query, err := awRequiredStringArg(args, "query")
		if err != nil {
			return "", err
		}
		limit, ok, err := awIntArg(args, "limit")
		if err != nil {
			return "", err
		}
		if !ok || limit <= 0 {
			limit = 10
		}
		result, err := w.chatSearchFn(ctx, query, limit)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["memory.chat.open"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.chatHistoryFn == nil {
			return "", errUnavailable("chat memory")
		}
		sessionID, err := awRequiredStringArg(args, "sessionId")
		if err != nil {
			return "", err
		}
		result, err := w.chatHistoryFn(ctx, sessionID)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["memory.chat.recent"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		if w.chatCatalogFn == nil {
			return "", errUnavailable("chat memory")
		}
		result, err := w.chatCatalogFn(ctx)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}

func errUnavailable(name string) error { return &unavailableError{name: name} }

type unavailableError struct{ name string }

func (e *unavailableError) Error() string { return e.name + " is not available" }
