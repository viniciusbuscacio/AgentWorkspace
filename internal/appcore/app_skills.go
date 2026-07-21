package appcore

import (
	"context"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
	"aw/internal/infrastructure/tools"
	"aw/skills"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Skills management panel bindings (Settings → Skills). Reads/writes go through
// application use cases over the vault SkillStore; every mutation refreshes the
// agent's injected skills index so changes take effect live.

func (a *App) ListSkills() dto.SkillsResult {
	seeds, _ := skills.BundledSkills()
	views, err := application.ListSkillViews(a.vault, seeds)
	if err != nil {
		return dto.SkillsResult{Success: false, Error: err.Error()}
	}
	out := make([]dto.SkillView, len(views))
	for i, v := range views {
		out[i] = dto.SkillView(v)
	}
	return dto.SkillsResult{Success: true, Skills: out}
}

// CreateSkill is the UI bridge to application.CreateSkill: it creates a user
// skill from supplied files (Add Skill in Settings) and refreshes the runtime
// skills context, like SaveSkill. It is a Wails binding only — not a new
// REST/MCP action and no new persistence semantics.
func (a *App) CreateSkill(id string, name string, description string, enabled bool, files []domain.SkillFile) dto.SkillResult {
	if err := application.CreateSkill(a.vault, id, name, description, enabled, files); err != nil {
		return dto.SkillResult{Success: false, Error: err.Error()}
	}
	a.refreshSkillsContext()
	return dto.SkillResult{Success: true}
}

func (a *App) GetSkillDetail(id string) dto.SkillDetailResult {
	skill, err := application.GetSkillDetail(a.vault, id)
	if err != nil {
		return dto.SkillDetailResult{Success: false, Error: err.Error()}
	}
	return dto.SkillDetailResult{Success: true, Skill: &skill}
}

func (a *App) SetSkillEnabled(id string, enabled bool) dto.SkillResult {
	if err := application.SetSkillEnabled(a.vault, id, enabled); err != nil {
		return dto.SkillResult{Success: false, Error: err.Error()}
	}
	a.refreshSkillsContext()
	return dto.SkillResult{Success: true}
}

func (a *App) SaveSkill(id string, files []domain.SkillFile) dto.SkillResult {
	if err := application.SaveSkillFiles(a.vault, id, files); err != nil {
		return dto.SkillResult{Success: false, Error: err.Error()}
	}
	a.refreshSkillsContext()
	return dto.SkillResult{Success: true}
}

// SelectSkillFile opens a native file picker for a skill's SKILL.md. The name
// filter is a hint only; the backend validates the basename.
func (a *App) SelectSkillFile() (dto.FolderDialogResult, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Select a skill's SKILL.md",
		Filters: []wailsruntime.FileFilter{{DisplayName: "SKILL.md", Pattern: "SKILL.md"}},
	})
	if err != nil {
		return dto.FolderDialogResult{Canceled: true}, err
	}
	if path == "" {
		return dto.FolderDialogResult{Canceled: true}, nil
	}
	return dto.FolderDialogResult{Canceled: false, Path: path}, nil
}

// ImportSkillFromFile imports the single skill whose SKILL.md is at path.
func (a *App) ImportSkillFromFile(path string) dto.SkillResult {
	if err := application.ImportSkillFromFile(a.vault, path); err != nil {
		return dto.SkillResult{Success: false, Error: err.Error()}
	}
	a.refreshSkillsContext()
	return dto.SkillResult{Success: true}
}

// SelectImportFolder opens a native directory picker for a parent folder that
// contains one or more skill subfolders.
func (a *App) SelectImportFolder() (dto.FolderDialogResult, error) {
	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select folder containing skill folders",
	})
	if err != nil {
		return dto.FolderDialogResult{Canceled: true}, err
	}
	if dir == "" {
		return dto.FolderDialogResult{Canceled: true}, nil
	}
	return dto.FolderDialogResult{Canceled: false, Path: dir}, nil
}

// ImportSkillsFromParent imports every direct subfolder of path that has a
// SKILL.md, returning a per-skill summary (partial import). refreshSkillsContext
// runs once at the end when anything was imported.
func (a *App) ImportSkillsFromParent(path string) dto.ImportSkillsResult {
	summary, err := application.ImportSkillsFromParent(a.vault, path)
	if err != nil {
		return dto.ImportSkillsResult{Success: false, Error: err.Error()}
	}
	if len(summary.Imported) > 0 {
		a.refreshSkillsContext()
	}
	skipped := make([]dto.SkillSkip, len(summary.Skipped))
	for i, s := range summary.Skipped {
		skipped[i] = dto.SkillSkip{ID: s.ID, Reason: s.Reason}
	}
	return dto.ImportSkillsResult{
		Success: true,
		Summary: dto.ImportSummary{Imported: summary.Imported, Skipped: skipped},
	}
}

func (a *App) DeleteSkill(id string) dto.SkillResult {
	if err := application.DeleteSkill(a.vault, id); err != nil {
		return dto.SkillResult{Success: false, Error: err.Error()}
	}
	a.refreshSkillsContext()
	return dto.SkillResult{Success: true}
}

func (a *App) ResetSkillToSeed(id string) dto.SkillResult {
	seeds, _ := skills.BundledSkills()
	if err := application.ResetSkillToSeed(a.vault, seeds, id); err != nil {
		return dto.SkillResult{Success: false, Error: err.Error()}
	}
	a.refreshSkillsContext()
	return dto.SkillResult{Success: true}
}

// refreshSkillsContext re-injects the effective skills index into the agent
// after a panel mutation, so the change is reflected without an unlock cycle.
func (a *App) refreshSkillsContext() {
	application.RefreshSkillsContext(a.agent, a.vault, a.skillsPromptInput())
}

// skillManageFuncs builds the skill write callbacks exposed through the aw
// action gateway (REST/MCP and the in-app agent). Each callback persists to the
// vault, refreshes the runtime skills context, and returns the spec response
// shape (success/contextRefreshed/restartRequired, plus dev-override status).
func (a *App) skillManageFuncs() *tools.SkillManage {
	return &tools.SkillManage{
		Detail: func(_ context.Context, id string) (any, error) {
			skill, err := application.GetSkillDetail(a.vault, id)
			if err != nil {
				return nil, err
			}
			return map[string]any{"skill": skill}, nil
		},
		Save: func(_ context.Context, id string, files []tools.SkillFileInput) (any, error) {
			if err := application.SaveSkillFiles(a.vault, id, toDomainSkillFiles(files)); err != nil {
				return nil, err
			}
			return a.skillMutationResponse(id), nil
		},
		Create: func(_ context.Context, id, name, description string, enabled bool, files []tools.SkillFileInput) (any, error) {
			if err := application.CreateSkill(a.vault, id, name, description, enabled, toDomainSkillFiles(files)); err != nil {
				return nil, err
			}
			return a.skillMutationResponse(id), nil
		},
		SetEnabled: func(_ context.Context, id string, enabled bool) (any, error) {
			if err := application.SetSkillEnabled(a.vault, id, enabled); err != nil {
				return nil, err
			}
			return a.skillMutationResponse(id), nil
		},
		ImportFile: func(_ context.Context, path string) (any, error) {
			if err := application.ImportSkillFromFile(a.vault, path); err != nil {
				return nil, err
			}
			return a.skillMutationResponse(""), nil
		},
		ImportFolder: func(_ context.Context, path string) (any, error) {
			summary, err := application.ImportSkillsFromParent(a.vault, path)
			if err != nil {
				return nil, err
			}
			resp := a.skillRefreshStatus(len(summary.Imported) > 0)
			resp["imported"] = summary.Imported
			resp["skipped"] = summary.Skipped
			return resp, nil
		},
		Delete: func(_ context.Context, id string) (any, error) {
			if err := application.DeleteSkill(a.vault, id); err != nil {
				return nil, err
			}
			return a.skillMutationResponse(""), nil
		},
		Reset: func(_ context.Context, id string) (any, error) {
			seeds, _ := skills.BundledSkills()
			if err := application.ResetSkillToSeed(a.vault, seeds, id); err != nil {
				return nil, err
			}
			return a.skillMutationResponse(id), nil
		},
	}
}

// skillMutationResponse refreshes the runtime context and returns the standard
// response, including a compact {id,name,enabled} when id is known.
func (a *App) skillMutationResponse(id string) map[string]any {
	resp := a.skillRefreshStatus(true)
	if id != "" {
		if skill, err := application.GetSkillDetail(a.vault, id); err == nil {
			resp["skill"] = map[string]any{"id": skill.ID, "name": skill.Name, "enabled": skill.Enabled}
		}
	}
	return resp
}

// skillRefreshStatus refreshes the agent skills context (unless a dev override
// serves the effective catalog) and reports it honestly. doRefresh is false when
// nothing changed (e.g. a folder import that imported nothing).
func (a *App) skillRefreshStatus(doRefresh bool) map[string]any {
	input := a.skillsPromptInput()
	if input.Active {
		return map[string]any{
			"success":           true,
			"contextRefreshed":  false,
			"restartRequired":   false,
			"devOverrideActive": true,
			"warning":           "AW_SKILLS_DIR is active; mutation was persisted to the vault but the effective runtime catalog is currently served from the dev override.",
		}
	}
	if doRefresh {
		application.RefreshSkillsContext(a.agent, a.vault, input)
		return map[string]any{"success": true, "contextRefreshed": true, "restartRequired": false}
	}
	return map[string]any{"success": true, "contextRefreshed": false, "restartRequired": false}
}

func toDomainSkillFiles(files []tools.SkillFileInput) []domain.SkillFile {
	out := make([]domain.SkillFile, len(files))
	for i, f := range files {
		out[i] = domain.SkillFile{Path: f.Path, Content: f.Content}
	}
	return out
}
