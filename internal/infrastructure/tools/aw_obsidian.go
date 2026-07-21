package tools

import (
	"context"
	"encoding/json"
)

// ObsidianFuncs wires the Obsidian module's vault-jailed file operations.
// The obsidian.* actions register only when this is set AND the obsidian
// module is added; write/append additionally require the module's Write
// toggle (enforced inside the funcs, which read live config).
type ObsidianFuncs struct {
	List   func(ctx context.Context, folder string) (any, error)
	Search func(ctx context.Context, query string, limit int) (any, error)
	Read   func(ctx context.Context, path string) (string, error)
	Write  func(ctx context.Context, path, content string) error
	Append func(ctx context.Context, path, content string) error
	Delete func(ctx context.Context, path string) error
}

// registerObsidianActions adds the Obsidian module's action group. Callers
// register it only while the module is added — module fencing is structural.
func registerObsidianActions(reg map[string]AwActionHandler) {
	reg["obsidian.list"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		folder, _ := args["folder"].(string)
		result, err := w.obsidian.List(ctx, folder)
		if err != nil {
			return "", err
		}
		return awJSON(map[string]any{"entries": result})
	}
	reg["obsidian.search"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		query, err := awRequiredStringArg(args, "query")
		if err != nil {
			return "", err
		}
		limit := 0
		if n, ok := args["max"].(float64); ok {
			limit = int(n)
		}
		result, err := w.obsidian.Search(ctx, query, limit)
		if err != nil {
			return "", err
		}
		return awJSON(map[string]any{"matches": result})
	}
	reg["obsidian.read"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		path, err := awRequiredStringArg(args, "path")
		if err != nil {
			return "", err
		}
		text, err := w.obsidian.Read(ctx, path)
		if err != nil {
			return "", err
		}
		return awJSON(map[string]any{"path": path, "content": text})
	}
	reg["obsidian.write"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		return obsidianMutate(ctx, args, w, false)
	}
	reg["obsidian.append"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		return obsidianMutate(ctx, args, w, true)
	}
	reg["obsidian.delete"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		path, err := awRequiredStringArg(args, "path")
		if err != nil {
			return "", err
		}
		if err := w.obsidian.Delete(ctx, path); err != nil {
			return "", err
		}
		payload, err := json.Marshal(map[string]any{"path": path, "deleted": true})
		if err != nil {
			return "", err
		}
		return string(payload), nil
	}
}

func obsidianMutate(ctx context.Context, args map[string]any, w *workspace, appendMode bool) (string, error) {
	path, err := awRequiredStringArg(args, "path")
	if err != nil {
		return "", err
	}
	content, err := awRequiredStringArg(args, "content")
	if err != nil {
		return "", err
	}
	if appendMode {
		err = w.obsidian.Append(ctx, path, content)
	} else {
		err = w.obsidian.Write(ctx, path, content)
	}
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{"path": path, "written": true})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}
