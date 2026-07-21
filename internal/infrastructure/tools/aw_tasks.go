package tools

import (
	"context"
	"fmt"
)

// TasksFuncs are the injected Tasks-module operations (wired by the
// composition root to application use cases against the vault). Body uses a
// pointer on Update because an empty body is a valid value to set; attachment
// binary content never crosses this boundary — tasks.get returns metadata
// (names, sizes, count) only.
type TasksFuncs struct {
	List   func(ctx context.Context) (any, error)
	Get    func(ctx context.Context, id string) (any, error)
	Add    func(ctx context.Context, title, body, status string) (any, error)
	Update func(ctx context.Context, id, title string, body *string, status string, position *int) (any, error)
	Delete func(ctx context.Context, id string) (any, error)
}

// registerTasksActions adds the Tasks module's action group. Callers
// register it only when the module is added.
func registerTasksActions(reg map[string]AwActionHandler) {
	reg["tasks.list"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		result, err := w.tasks.List(ctx)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["tasks.get"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.tasks.Get(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["tasks.add"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		title, err := awRequiredStringArg(args, "title")
		if err != nil {
			return "", err
		}
		body, _, err := awStringArg(args, "body")
		if err != nil {
			return "", err
		}
		status, _, err := awStringArg(args, "status")
		if err != nil {
			return "", err
		}
		result, err := w.tasks.Add(ctx, title, body, status)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["tasks.update"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		title, _, err := awStringArg(args, "title")
		if err != nil {
			return "", err
		}
		body, hasBody, err := awStringArg(args, "body")
		if err != nil {
			return "", err
		}
		var bodyPtr *string
		if hasBody {
			bodyPtr = &body
		}
		status, _, err := awStringArg(args, "status")
		if err != nil {
			return "", err
		}
		position, hasPosition, err := awIntArg(args, "position")
		if err != nil {
			return "", err
		}
		var positionPtr *int
		if hasPosition {
			positionPtr = &position
		}
		if title == "" && bodyPtr == nil && status == "" && positionPtr == nil {
			return "", fmt.Errorf("title, body, status and/or position is required")
		}
		result, err := w.tasks.Update(ctx, id, title, bodyPtr, status, positionPtr)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["tasks.delete"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.tasks.Delete(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}
