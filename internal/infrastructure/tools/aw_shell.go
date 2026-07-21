package tools

import "context"

func registerShellActions(reg map[string]AwActionHandler) {
	reg["shell.exec"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		command, err := awRequiredStringArg(args, "command")
		if err != nil {
			return "", err
		}
		dir, _, err := awStringArg(args, "dir")
		if err != nil {
			return "", err
		}
		result, err := w.runShell(ctx, runShellArgs{Command: command, Dir: dir})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}
