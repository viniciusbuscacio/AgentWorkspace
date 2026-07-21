package tools

import (
	"context"
	"fmt"
)

// registerModuleActions adds the workspace-module management actions to the aw
// registry. The agent manages the workspace through the same use cases the UI
// uses — same catalog, same core-module guard.
func registerModuleActions(reg map[string]AwActionHandler) {
	reg["module.list"] = func(_ context.Context, _ map[string]any, w *workspace) (string, error) {
		result, err := w.control.ListModules()
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["module.add"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.control.AddModule(id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["module.remove"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.control.RemoveModule(id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["module.hide"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.control.HideModule(id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["module.show"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.control.ShowModule(id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["module.move"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		up, hasUp, err := awOptionalBoolArg(args, "up")
		if err != nil {
			return "", err
		}
		if !hasUp {
			return "", fmt.Errorf("up is required (true = move up, false = move down)")
		}
		result, err := w.control.MoveModule(id, up)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}
