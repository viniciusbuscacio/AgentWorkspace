package tools

import (
	"context"
)

// InstructionFuncs wires the Agent Instructions use cases into the aw action
// gateway (Settings → Agent Instructions). Each callback delegates to the
// application layer, persists to the vault, and — for mutations — refreshes the
// runtime instruction context. Mirrors the SkillManage pattern: a nil callback
// makes the action return errUnavailable("instructions").
type InstructionFuncs struct {
	List      func(ctx context.Context) (any, error)
	Read      func(ctx context.Context, id string) (any, error)
	Save      func(ctx context.Context, id, content string, enabled bool) (any, error)
	Reset     func(ctx context.Context, id string) (any, error)
	Effective func(ctx context.Context, includeContent bool) (any, error)
	Sources   func(ctx context.Context) (any, error)
}

// registerInstructionActions wires the instructions.* actions. Registered only
// when the instruction funcs are wired (the vault-backed surface exists).
func registerInstructionActions(reg map[string]AwActionHandler) {
	reg["instructions.list"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		if w.instructions == nil || w.instructions.List == nil {
			return "", errUnavailable("instructions")
		}
		result, err := w.instructions.List(ctx)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["instructions.read"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.instructions == nil || w.instructions.Read == nil {
			return "", errUnavailable("instructions")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.instructions.Read(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["instructions.save"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.instructions == nil || w.instructions.Save == nil {
			return "", errUnavailable("instructions")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		content, _, err := awStringArg(args, "content")
		if err != nil {
			return "", err
		}
		enabled, err := awBoolArg(args, "enabled", true)
		if err != nil {
			return "", err
		}
		result, err := w.instructions.Save(ctx, id, content, enabled)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["instructions.reset"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.instructions == nil || w.instructions.Reset == nil {
			return "", errUnavailable("instructions")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.instructions.Reset(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["instructions.effective"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.instructions == nil || w.instructions.Effective == nil {
			return "", errUnavailable("instructions")
		}
		includeContent, err := awBoolArg(args, "includeContent", true)
		if err != nil {
			return "", err
		}
		result, err := w.instructions.Effective(ctx, includeContent)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["instructions.sources"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		if w.instructions == nil || w.instructions.Sources == nil {
			return "", errUnavailable("instructions")
		}
		result, err := w.instructions.Sources(ctx)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}
