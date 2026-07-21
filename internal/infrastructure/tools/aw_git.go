package tools

import (
	"context"
	"strings"
)

func registerGitActions(reg map[string]AwActionHandler) {
	status := func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		result, err := w.runShell(ctx, runShellArgs{Command: "git status --porcelain=v1 -b"})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
	execGit := func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		command, err := awRequiredStringArg(args, "command")
		if err != nil {
			return "", err
		}
		result, err := w.runShell(ctx, runShellArgs{Command: normalizeGitCommand(command)})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
	execGithub := func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		command, err := awRequiredStringArg(args, "command")
		if err != nil {
			return "", err
		}
		result, err := w.runShell(ctx, runShellArgs{Command: normalizeGithubCommand(command)})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["git.status"] = status
	reg["git.exec"] = execGit
	reg["github.status"] = status
	reg["github.exec"] = execGithub
}

func normalizeGitCommand(command string) string {
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, "git ") {
		return command
	}
	return "git " + command
}

func normalizeGithubCommand(command string) string {
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, "git ") || strings.HasPrefix(command, "gh ") {
		return command
	}
	return "git " + command
}
