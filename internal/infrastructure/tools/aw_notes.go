package tools

import (
	"context"
	"fmt"
)

// NotesFuncs are the injected Notes-module operations (the composition root
// wires them to application use cases against the vault — this package never
// touches the vault directly).
type NotesFuncs struct {
	List   func(ctx context.Context, includeArchived bool) (any, error)
	Get    func(ctx context.Context, id string) (any, error)
	Create func(ctx context.Context, title, content string, inPrompt bool) (any, error)
	Update func(ctx context.Context, id string, title, content *string, pinned, archived, inPrompt *bool) (any, error)
	Delete func(ctx context.Context, id string) (any, error)
}

// registerNotesActions adds the Notes module's action group. Callers register
// it only when the module is added — module fencing is structural.
func registerNotesActions(reg map[string]AwActionHandler) {
	reg["notes.list"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		includeArchived, err := awBoolArg(args, "includeArchived", false)
		if err != nil {
			return "", err
		}
		result, err := w.notes.List(ctx, includeArchived)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["notes.get"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.notes.Get(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["notes.create"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		title, err := awRequiredStringArg(args, "title")
		if err != nil {
			return "", err
		}
		content, _, err := awStringArg(args, "content")
		if err != nil {
			return "", err
		}
		// inPrompt is optional and defaults to true (a new note feeds the agent
		// context unless the caller opts out).
		inPrompt, hasInPrompt, err := awOptionalBoolArg(args, "inPrompt")
		if err != nil {
			return "", err
		}
		if !hasInPrompt {
			inPrompt = true
		}
		result, err := w.notes.Create(ctx, title, content, inPrompt)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["notes.update"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		title, hasTitle, err := awStringArg(args, "title")
		if err != nil {
			return "", err
		}
		content, hasContent, err := awStringArg(args, "content")
		if err != nil {
			return "", err
		}
		pinned, hasPinned, err := awOptionalBoolArg(args, "pinned")
		if err != nil {
			return "", err
		}
		archived, hasArchived, err := awOptionalBoolArg(args, "archived")
		if err != nil {
			return "", err
		}
		inPrompt, hasInPrompt, err := awOptionalBoolArg(args, "inPrompt")
		if err != nil {
			return "", err
		}
		if !hasTitle && !hasContent && !hasPinned && !hasArchived && !hasInPrompt {
			return "", fmt.Errorf("title, content, pinned, archived and/or inPrompt is required")
		}
		result, err := w.notes.Update(ctx,
			id,
			optionalArg(title, hasTitle),
			optionalArg(content, hasContent),
			optionalArg(pinned, hasPinned),
			optionalArg(archived, hasArchived),
			optionalArg(inPrompt, hasInPrompt),
		)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["notes.delete"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.notes.Delete(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}

// optionalArg turns a (value, present) pair into the pointer the partial
// update funcs expect: nil when the argument was not passed at all.
func optionalArg[T any](value T, present bool) *T {
	if !present {
		return nil
	}
	return &value
}
