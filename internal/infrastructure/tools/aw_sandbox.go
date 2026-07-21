package tools

import (
	"context"
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/infrastructure/sandbox"
)

// sandboxConfigDeniedMessage is AW2's exact refusal: the agent can see and
// test the fence, never move it. Permissions only change in the Settings UI,
// behind a native confirmation dialog.
const sandboxConfigDeniedMessage = "Access denied. Change Permissions in the app."

// registerSandboxActions wires the always-on permissions actions:
// sandbox.status (what the fence looks like), sandbox.test (dry-run a
// command) and sandbox.set_mode (registered so the agent gets a
// deterministic refusal instead of "unknown action").
func registerSandboxActions(reg map[string]AwActionHandler) {
	reg["sandbox.status"] = func(_ context.Context, _ map[string]any, w *workspace) (string, error) {
		policy := w.sandboxPolicy()
		lines := []string{
			fmt.Sprintf("Mode: %s", policy.Config.Mode),
			fmt.Sprintf("Workspace folder: %s", policy.WorkspaceRoot),
			fmt.Sprintf("Allowed folders: %s", listOrNone(policy.Config.AllowedFolders)),
		}
		return strings.Join(lines, "\n"), nil
	}

	reg["sandbox.test"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		command, err := awRequiredStringArg(args, "command")
		if err != nil {
			return "", err
		}
		policy := w.sandboxPolicy()
		check := sandbox.CheckCommand(command, "", policy)
		result := "BLOCKED"
		if check.Allowed {
			result = "ALLOWED"
		}
		lines := []string{
			fmt.Sprintf("Result: %s", result),
			fmt.Sprintf("Mode: %s", policy.Config.Mode),
			fmt.Sprintf("Paths detected: %s", listOrNone(check.Paths)),
		}
		switch {
		case len(check.BlockedPaths) > 0:
			lines = append(lines,
				fmt.Sprintf("Blocked paths: %s", strings.Join(check.BlockedPaths, ", ")),
				fmt.Sprintf("Reason: Access denied to %d path(s).", len(check.BlockedPaths)))
		case len(check.Paths) == 0 && check.Allowed:
			lines = append(lines, "Reason: Command is allowed without explicit filesystem paths.")
		case len(check.Paths) == 0:
			lines = append(lines, "Reason: Command is not allowed by the current Permissions mode.")
		default:
			lines = append(lines, "Reason: All paths are allowed under current config.")
		}
		return strings.Join(lines, "\n"), nil
	}

	reg["sandbox.set_mode"] = func(_ context.Context, args map[string]any, _ *workspace) (string, error) {
		mode, err := awRequiredStringArg(args, "mode")
		if err != nil {
			return "", err
		}
		if !domain.SandboxMode(mode).Valid() {
			return fmt.Sprintf("Error: invalid mode %q. Valid: %s", mode, validSandboxModes()), nil
		}
		return sandboxConfigDeniedMessage, nil
	}
}

func listOrNone(list []string) string {
	if len(list) == 0 {
		return "(none)"
	}
	return strings.Join(list, ", ")
}

func validSandboxModes() string {
	modes := domain.SandboxModes()
	names := make([]string, len(modes))
	for i, mode := range modes {
		names[i] = string(mode)
	}
	return strings.Join(names, ", ")
}
