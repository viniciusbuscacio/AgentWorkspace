package tools

import (
	"context"
	"fmt"
)

func registerFsActions(reg map[string]AwActionHandler) {
	reg["fs.read"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		path, err := awRequiredStringArg(args, "path")
		if err != nil {
			return "", err
		}
		result, err := w.readFile(ctx, readFileArgs{Path: path})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["fs.write"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		path, err := awRequiredStringArg(args, "path")
		if err != nil {
			return "", err
		}
		content, _, err := awStringArg(args, "content")
		if err != nil {
			return "", err
		}
		result, err := w.writeFile(ctx, writeFileArgs{Path: path, Content: content})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["fs.list"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		path, ok, err := awStringArg(args, "path")
		if err != nil {
			return "", err
		}
		if !ok {
			path = "."
		}
		result, err := w.listDirectory(ctx, listDirArgs{Path: path})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["fs.edit"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		path, err := awRequiredStringArg(args, "path")
		if err != nil {
			return "", err
		}
		raw, ok := args["edits"].([]any)
		if !ok || len(raw) == 0 {
			return "", errEditsRequired()
		}
		edits := make([]fileEdit, 0, len(raw))
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				return "", errEditsRequired()
			}
			oldText, _ := m["oldText"].(string)
			newText, _ := m["newText"].(string)
			edits = append(edits, fileEdit{OldText: oldText, NewText: newText})
		}
		result, err := w.editFile(ctx, editFileArgs{Path: path, Edits: edits})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}

func errEditsRequired() error { return fmt.Errorf("edits must be a non-empty array") }
