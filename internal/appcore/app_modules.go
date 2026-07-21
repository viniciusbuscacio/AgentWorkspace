package appcore

import (
	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
)

// ListModules returns the workspace-module catalog with the added state.
func (a *App) ListModules() dto.ModulesResult {
	return dto.ModulesResult{Success: true, Modules: a.moduleInfos()}
}

// AddModule puts a module in the workspace (Home card click or agent action).
func (a *App) AddModule(id string) dto.ModulesResult {
	a.recordActivity()
	return a.modulesResult(application.AddModule(a.workspace, domain.ModuleCatalog(), id))
}

// RemoveModule takes a module out of the workspace. The core chat module is
// rejected by the use case.
func (a *App) RemoveModule(id string) dto.ModulesResult {
	a.recordActivity()
	return a.modulesResult(application.RemoveModule(a.workspace, domain.ModuleCatalog(), id))
}

// HideModule closes a module in the sidebar (the hover X). The module stays
// added: actions and prompt block are untouched.
func (a *App) HideModule(id string) dto.ModulesResult {
	a.recordActivity()
	return a.modulesResult(application.HideModule(a.workspace, domain.ModuleCatalog(), id))
}

// ShowModule reopens a hidden module in the sidebar (Apps card, app.navigate).
func (a *App) ShowModule(id string) dto.ModulesResult {
	a.recordActivity()
	return a.modulesResult(application.ShowModule(a.workspace, domain.ModuleCatalog(), id))
}

// MoveModule moves a module one slot up or down in the sidebar order.
func (a *App) MoveModule(id string, up bool) dto.ModulesResult {
	a.recordActivity()
	return a.modulesResult(application.MoveModule(a.workspace, domain.ModuleCatalog(), id, up))
}

// HideAllModules closes every non-core module in a single config write — the
// "Show desktop" action. Modules remain added; their actions are untouched.
func (a *App) HideAllModules() dto.ModulesResult {
	a.recordActivity()
	return a.modulesResult(application.HideAllModules(a.workspace, domain.ModuleCatalog()))
}

// GetLastView returns the view id that was active when the app was last closed,
// or "home" if nothing was saved. The caller must validate the id against the
// current navigable views before restoring it.
func (a *App) GetLastView() string {
	if v := application.LoadLastSession(a.workspace).View; v != "" {
		return v
	}
	return "home"
}

// GetLastChatID returns the chat id that was selected when the app was last
// closed, or "" if nothing was saved.
func (a *App) GetLastChatID() string {
	return application.LoadLastSession(a.workspace).ChatID
}

// SaveLastSession persists the current view and selected chat id to config.json
// so they can be restored after the next unlock.
func (a *App) SaveLastSession(view, chatID string) dto.OperationResult {
	if err := application.SaveLastSession(a.workspace, view, chatID); err != nil {
		return dto.OperationResult{Error: err.Error()}
	}
	return dto.OperationResult{Success: true}
}

// modulesResult turns a mutation outcome plus the current catalog into the
// Wails response, broadcasting modules:changed after successful mutations.
func (a *App) modulesResult(mutationErr error) dto.ModulesResult {
	if mutationErr != nil {
		return dto.ModulesResult{Error: mutationErr.Error(), Modules: a.moduleInfos()}
	}
	infos := a.moduleInfos()
	a.emitChatEvent("modules:changed", map[string]any{"modules": infos})
	return dto.ModulesResult{Success: true, Modules: infos}
}

func (a *App) moduleInfos() []dto.ModuleInfo {
	statuses, err := application.ListModules(a.workspace, domain.ModuleCatalog())
	if err != nil {
		return nil
	}
	infos := make([]dto.ModuleInfo, 0, len(statuses))
	for _, status := range statuses {
		infos = append(infos, dto.ModuleInfo{
			ID:          status.ID,
			Name:        status.Name,
			Icon:        status.Icon,
			Description: status.Description,
			Core:        status.Core,
			Fixed:       status.Fixed,
			ComingSoon:  status.ComingSoon,
			Added:       status.Added,
			Hidden:      status.Hidden,
			SidebarPos:  status.SidebarPosition,
		})
	}
	return infos
}
