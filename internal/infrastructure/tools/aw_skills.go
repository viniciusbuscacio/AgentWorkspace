package tools

import (
	"context"
	"fmt"
)

// SkillFileInput is one file in a skill write action (skill.save/skill.create).
type SkillFileInput struct {
	Path    string
	Content string
}

// SkillManage groups the skill write callbacks. Each returns the full action
// response object (built by the composition root) which the handler emits as is.
type SkillManage struct {
	Detail       func(ctx context.Context, id string) (any, error)
	Save         func(ctx context.Context, id string, files []SkillFileInput) (any, error)
	Create       func(ctx context.Context, id, name, description string, enabled bool, files []SkillFileInput) (any, error)
	SetEnabled   func(ctx context.Context, id string, enabled bool) (any, error)
	ImportFile   func(ctx context.Context, path string) (any, error)
	ImportFolder func(ctx context.Context, path string) (any, error)
	Delete       func(ctx context.Context, id string) (any, error)
	Reset        func(ctx context.Context, id string) (any, error)
}

// registerSkillActions wires progressive-disclosure skill access: the system
// prompt advertises only each skill's trigger (name + description); the agent
// pulls the full SKILL.md (and any auxiliary file) on demand with skill.read.
func registerSkillActions(reg map[string]AwActionHandler) {
	reg["skill.list"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		if w.skillCatalogFn == nil {
			return "", errUnavailable("skills")
		}
		result, err := w.skillCatalogFn(ctx)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["skill.read"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.skillReadFn == nil {
			return "", errUnavailable("skills")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		path, _ := args["path"].(string)
		result, err := w.skillReadFn(ctx, id, path)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}

// registerSkillManageActions wires the skill write actions. Inside an unlocked
// AW vault the agent manages its own skills with no extra confirmation (a bearer
// token + unlocked vault is the authority); see the skill-write-actions spec.
func registerSkillManageActions(reg map[string]AwActionHandler) {
	reg["skill.detail"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.skillManage == nil || w.skillManage.Detail == nil {
			return "", errUnavailable("skills")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.skillManage.Detail(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["skill.save"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.skillManage == nil || w.skillManage.Save == nil {
			return "", errUnavailable("skills")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		files, err := awSkillFileInputs(args)
		if err != nil {
			return "", err
		}
		result, err := w.skillManage.Save(ctx, id, files)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["skill.create"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.skillManage == nil || w.skillManage.Create == nil {
			return "", errUnavailable("skills")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		files, err := awSkillFileInputs(args)
		if err != nil {
			return "", err
		}
		name, _ := args["name"].(string)
		description, _ := args["description"].(string)
		enabled, err := awBoolArg(args, "enabled", true)
		if err != nil {
			return "", err
		}
		result, err := w.skillManage.Create(ctx, id, name, description, enabled, files)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["skill.set_enabled"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.skillManage == nil || w.skillManage.SetEnabled == nil {
			return "", errUnavailable("skills")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		enabled, present, err := awOptionalBoolArg(args, "enabled")
		if err != nil {
			return "", err
		}
		if !present {
			return "", fmt.Errorf("enabled is required")
		}
		result, err := w.skillManage.SetEnabled(ctx, id, enabled)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["skill.import_file"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.skillManage == nil || w.skillManage.ImportFile == nil {
			return "", errUnavailable("skills")
		}
		path, err := awRequiredStringArg(args, "path")
		if err != nil {
			return "", err
		}
		result, err := w.skillManage.ImportFile(ctx, path)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["skill.import_folder"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.skillManage == nil || w.skillManage.ImportFolder == nil {
			return "", errUnavailable("skills")
		}
		path, err := awRequiredStringArg(args, "path")
		if err != nil {
			return "", err
		}
		result, err := w.skillManage.ImportFolder(ctx, path)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["skill.delete"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.skillManage == nil || w.skillManage.Delete == nil {
			return "", errUnavailable("skills")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.skillManage.Delete(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["skill.reset"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.skillManage == nil || w.skillManage.Reset == nil {
			return "", errUnavailable("skills")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.skillManage.Reset(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}

func awSkillFileInputs(args map[string]any) ([]SkillFileInput, error) {
	raw, ok := args["files"]
	if !ok || raw == nil {
		return nil, fmt.Errorf("files is required")
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("files must be an array")
	}
	out := make([]SkillFileInput, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("each file must be an object with path and content")
		}
		path, _ := m["path"].(string)
		content, _ := m["content"].(string)
		out = append(out, SkillFileInput{Path: path, Content: content})
	}
	return out, nil
}
