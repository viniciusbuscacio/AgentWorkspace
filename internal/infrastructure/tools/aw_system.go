package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func registerSystemActions(reg map[string]AwActionHandler) {
	reg["system.selfcode"] = func(_ context.Context, _ map[string]any, w *workspace) (string, error) {
		path := filepath.Join(w.root, "docs", "SELFCODE.md")
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read SELFCODE.md at %s: %w", path, err)
		}
		return string(data), nil
	}

	reg["system.state"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		if w.stateFn == nil {
			return "", fmt.Errorf("system.state not wired")
		}
		state, err := w.stateFn(contextOrBackground(ctx))
		if err != nil {
			return "", err
		}
		return awJSON(state)
	}

}

func registerSubagentActions(reg map[string]AwActionHandler) {
	// system.spawn routes a {tasks:[...]} list to the generic parallel spawn
	// engine; a bare {task} runs as a single generic worker. The browser-read
	// subagent is gone — page reads happen directly in the main chat, always
	// sanitized/enveloped/tainted.
	reg["system.spawn"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		tasks, isBatch, err := spawnTasksFromArgs(args)
		if err != nil {
			return "", err
		}
		if !isBatch {
			task, terr := awRequiredStringArg(args, "task")
			if terr != nil {
				return "", terr
			}
			tasks = []SpawnTaskInput{{ID: "task-1", Task: task, Context: spawnStringField(args, "context")}}
		}
		if w.spawnFn == nil {
			return "", fmt.Errorf("spawn runtime is not available")
		}
		timeoutMs := 0
		if ms, ok, err := awIntArg(args, "timeoutMs"); err != nil {
			return "", err
		} else if ok {
			timeoutMs = ms
		}
		results, err := w.spawnFn(ctx, tasks, timeoutMs)
		if err != nil {
			return "", err
		}
		return awJSON(map[string]any{"results": results})
	}
}

// spawnTasksFromArgs parses the generic multi-task form. The bool reports whether
// a tasks[] list was present at all (so a bare {task} can fall back to the single
// subagent path).
func spawnTasksFromArgs(args map[string]any) ([]SpawnTaskInput, bool, error) {
	raw, ok := args["tasks"]
	if !ok {
		return nil, false, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, true, fmt.Errorf("spawn: tasks must be an array of {id, task, context?}")
	}
	tasks := make([]SpawnTaskInput, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, true, fmt.Errorf("spawn: each task must be an object with a task field")
		}
		tasks = append(tasks, SpawnTaskInput{
			ID:      spawnStringField(m, "id"),
			Task:    spawnStringField(m, "task"),
			Context: spawnStringField(m, "context"),
		})
	}
	return tasks, true, nil
}

func spawnStringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
