package application

import (
	"errors"
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// ModuleStatus is one catalog entry plus its workspace state. Hidden means
// closed in the sidebar while still added: the module's actions and prompt
// block stay registered (close is a window operation, remove is the
// capability fence — sidebar-modules spec, Decision 1). SidebarPosition is
// the index in the user's sidebar order for added non-core modules, -1
// otherwise.
type ModuleStatus struct {
	domain.ModuleSpec
	Added           bool `json:"added"`
	Hidden          bool `json:"hidden"`
	SidebarPosition int  `json:"sidebarPosition"`
}

// ListModules returns the full catalog with the added/hidden/order state
// resolved from the store. ComingSoon entries are included (the UI hides
// them); unknown stored ids are ignored.
func ListModules(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec) ([]ModuleStatus, error) {
	added, err := AddedModuleIDs(store, catalog)
	if err != nil {
		return nil, err
	}
	addedSet := make(map[string]bool, len(added))
	for _, id := range added {
		addedSet[id] = true
	}
	hidden, err := hiddenModuleSet(store)
	if err != nil {
		return nil, err
	}
	order, err := SidebarModuleOrder(store, catalog)
	if err != nil {
		return nil, err
	}
	positions := make(map[string]int, len(order))
	for i, id := range order {
		positions[id] = i
	}
	statuses := make([]ModuleStatus, 0, len(catalog))
	for _, spec := range catalog {
		position, inSidebar := positions[spec.ID]
		if !inSidebar {
			position = -1
		}
		statuses = append(statuses, ModuleStatus{
			ModuleSpec:      spec,
			Added:           addedSet[spec.ID],
			Hidden:          addedSet[spec.ID] && hidden[spec.ID],
			SidebarPosition: position,
		})
	}
	return statuses, nil
}

func hiddenModuleSet(store ports.WorkspaceModuleStore) (map[string]bool, error) {
	stored, err := store.LoadHiddenModules()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(stored))
	for _, id := range stored {
		set[legacyModuleID(strings.TrimSpace(id))] = true
	}
	return set, nil
}

// SidebarModuleOrder returns the added non-core module ids in the user's
// order: stored order first (dropping ids that are no longer added), then any
// added module missing from the stored order, in catalog order.
func SidebarModuleOrder(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec) ([]string, error) {
	added, err := AddedModuleIDs(store, catalog)
	if err != nil {
		return nil, err
	}
	sidebar := make(map[string]bool, len(added))
	for _, id := range added {
		if spec, ok := domain.ModuleByID(catalog, id); ok && !spec.Core {
			sidebar[id] = true
		}
	}
	stored, err := store.LoadModuleOrder()
	if err != nil {
		return nil, err
	}
	order := make([]string, 0, len(sidebar))
	seen := make(map[string]bool, len(sidebar))
	for _, id := range stored {
		id = strings.TrimSpace(id)
		if sidebar[id] && !seen[id] {
			order = append(order, id)
			seen[id] = true
		}
	}
	for _, id := range added {
		if sidebar[id] && !seen[id] {
			order = append(order, id)
			seen[id] = true
		}
	}
	return order, nil
}

// HideAllModules closes every non-core added module in a single config write.
// Core modules (chat) are never touched — they have no sidebar item to hide.
// After this call, all non-core modules remain added (their actions are
// untouched) but their sidebar items are gone until reopened.
func HideAllModules(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec) error {
	added, err := AddedModuleIDs(store, catalog)
	if err != nil {
		return err
	}
	hidden, err := store.LoadHiddenModules()
	if err != nil {
		return err
	}
	// Build the updated hidden set: existing hidden + non-core added that are not yet hidden.
	hiddenSet := make(map[string]bool, len(hidden))
	for _, id := range hidden {
		hiddenSet[strings.TrimSpace(id)] = true
	}
	next := append([]string{}, hidden...)
	for _, id := range added {
		spec, ok := domain.ModuleByID(catalog, id)
		if !ok || spec.Core {
			continue
		}
		if !hiddenSet[id] {
			next = append(next, id)
		}
	}
	return store.SaveHiddenModules(next)
}

// ValidateRestoreView returns the saved view if it is in the set of currently
// navigable views; otherwise it falls back to "home". A deleted module, a
// view that was never navigable, or an empty string all fall back to "home".
func ValidateRestoreView(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec, view string) string {
	view = strings.TrimSpace(view)
	if view == "" {
		return "home"
	}
	views, err := NavigableViews(store, catalog)
	if err != nil {
		return "home"
	}
	for _, v := range views {
		if v == view {
			return view
		}
	}
	return "home"
}

// HideModule closes a module in the sidebar. The module stays added: actions
// and prompt block are untouched — hiding is never a capability change.
// Hiding core modules is rejected (they have no sidebar item to close).
func HideModule(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec, id string) error {
	id = strings.TrimSpace(id)
	spec, ok := domain.ModuleByID(catalog, id)
	if !ok {
		return fmt.Errorf("unknown module %q", id)
	}
	if spec.Core {
		return fmt.Errorf("module %q is part of the core workspace and cannot be closed", id)
	}
	hidden, err := store.LoadHiddenModules()
	if err != nil {
		return err
	}
	for _, existing := range hidden {
		if strings.TrimSpace(existing) == id {
			return nil
		}
	}
	return store.SaveHiddenModules(append(hidden, id))
}

// ShowModule reopens a hidden module in the sidebar. Showing a module that is
// not hidden is a no-op.
func ShowModule(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec, id string) error {
	id = strings.TrimSpace(id)
	if _, ok := domain.ModuleByID(catalog, id); !ok {
		return fmt.Errorf("unknown module %q", id)
	}
	hidden, err := store.LoadHiddenModules()
	if err != nil {
		return err
	}
	kept := make([]string, 0, len(hidden))
	for _, existing := range hidden {
		if strings.TrimSpace(existing) != id {
			kept = append(kept, existing)
		}
	}
	if len(kept) == len(hidden) {
		return nil
	}
	return store.SaveHiddenModules(kept)
}

// MoveModule moves a module one slot up or down in the sidebar order. Moving
// past an edge is a no-op.
func MoveModule(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec, id string, up bool) error {
	id = strings.TrimSpace(id)
	order, err := SidebarModuleOrder(store, catalog)
	if err != nil {
		return err
	}
	index := -1
	for i, existing := range order {
		if existing == id {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("module %q is not in the sidebar", id)
	}
	target := index + 1
	if up {
		target = index - 1
	}
	if target < 0 || target >= len(order) {
		return nil
	}
	order[index], order[target] = order[target], order[index]
	return store.SaveModuleOrder(order)
}

// AddedModuleIDs returns the ids of the modules in the workspace, in catalog
// order. Core modules are always present, whatever the store says — an absent
// or empty stored state means "core only", never an empty workspace. Stored
// ids that are unknown or coming-soon are dropped.
// legacyModuleID maps ids persisted by older builds to their current names,
// so existing workspaces keep their modules across renames.
func legacyModuleID(id string) string {
	if id == "backlog" {
		return "tasks"
	}
	return id
}

func AddedModuleIDs(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec) ([]string, error) {
	if store == nil {
		return nil, errors.New("workspace module store is required")
	}
	stored, err := store.LoadAddedModules()
	if err != nil {
		return nil, err
	}
	storedSet := make(map[string]bool, len(stored))
	for _, id := range stored {
		storedSet[legacyModuleID(strings.TrimSpace(id))] = true
	}
	ids := make([]string, 0, len(catalog))
	for _, spec := range catalog {
		if spec.Core || (storedSet[spec.ID] && !spec.ComingSoon) {
			ids = append(ids, spec.ID)
		}
	}
	return ids, nil
}

// AddedModuleSpecs returns the full specs of the added modules, in catalog
// order.
func AddedModuleSpecs(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec) ([]domain.ModuleSpec, error) {
	ids, err := AddedModuleIDs(store, catalog)
	if err != nil {
		return nil, err
	}
	specs := make([]domain.ModuleSpec, 0, len(ids))
	for _, id := range ids {
		if spec, ok := domain.ModuleByID(catalog, id); ok {
			specs = append(specs, spec)
		}
	}
	return specs, nil
}

// AddModule puts a module in the workspace. Adding an already-added module
// unhides it (the reopen path for module.add — sidebar-modules spec,
// Decision 4). Unknown and coming-soon modules are rejected.
func AddModule(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec, id string) error {
	id = strings.TrimSpace(id)
	spec, ok := domain.ModuleByID(catalog, id)
	if !ok {
		return fmt.Errorf("unknown module %q", id)
	}
	if spec.ComingSoon {
		return fmt.Errorf("module %q is not available yet", id)
	}
	added, err := AddedModuleIDs(store, catalog)
	if err != nil {
		return err
	}
	alreadyAdded := false
	for _, existing := range added {
		if existing == id {
			alreadyAdded = true
			break
		}
	}
	if !alreadyAdded {
		if err := store.SaveAddedModules(append(added, id)); err != nil {
			return err
		}
	}
	return ShowModule(store, catalog, id)
}

// RemoveModule takes a module out of the workspace. Core modules (chat) and
// Fixed modules (built-in surfaces like Wallpaper) cannot be removed; removing
// a module that is not added is a no-op.
func RemoveModule(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec, id string) error {
	id = strings.TrimSpace(id)
	spec, ok := domain.ModuleByID(catalog, id)
	if !ok {
		return fmt.Errorf("unknown module %q", id)
	}
	if spec.Core {
		return fmt.Errorf("module %q is part of the core workspace and cannot be removed", id)
	}
	if spec.Fixed {
		return fmt.Errorf("module %q is a built-in workspace surface and cannot be removed", id)
	}
	added, err := AddedModuleIDs(store, catalog)
	if err != nil {
		return err
	}
	kept := make([]string, 0, len(added))
	for _, existing := range added {
		if existing != id {
			kept = append(kept, existing)
		}
	}
	if len(kept) == len(added) {
		return nil
	}
	if err := store.SaveAddedModules(kept); err != nil {
		return err
	}
	// A future re-add starts visible — stale hidden state must not survive.
	return ShowModule(store, catalog, id)
}

// ModulesInstruction renders the workspace-modules prompt block for the added
// modules: name, description, the module's own prompt section and its action
// catalog with arg hints — all generated from ModuleSpec. Never hand-write
// module capabilities in prompt text; that is the drift this kills.
func ModulesInstruction(added []domain.ModuleSpec) string {
	return ModulesInstructionWithCatalog(added, nil)
}

// ModulesInstructionWithCatalog also names the modules AVAILABLE to add, so
// the agent answers "do you have Notes?" from facts instead of guessing
// (2026-06-12: asked about Notes, it invented "no notes module exists" —
// the catalog had it one module.add away).
func ModulesInstructionWithCatalog(added []domain.ModuleSpec, catalog []domain.ModuleSpec) string {
	if len(added) == 0 && len(catalog) == 0 {
		return ""
	}
	addedSet := make(map[string]bool, len(added))
	for _, spec := range added {
		addedSet[spec.ID] = true
	}
	available := make([]string, 0, len(catalog))
	for _, spec := range catalog {
		if !addedSet[spec.ID] && !spec.ComingSoon {
			available = append(available, spec.ID)
		}
	}
	var b strings.Builder
	b.WriteString("## Workspace modules\n")
	b.WriteString("The workspace is composed of modules (mini-apps) the user added. You operate ")
	b.WriteString("them through their aw actions below and open their views with app.navigate ")
	b.WriteString("{view: <module id>}. Manage the set with module.list / module.add / module.remove. ")
	b.WriteString("Modules not listed here are not in the workspace: their actions do not exist ")
	b.WriteString("until the module is added.\n")
	if len(available) > 0 {
		b.WriteString("Available to add (module.add {id}): ")
		b.WriteString(strings.Join(available, ", "))
		b.WriteString(". When the user asks for one of these, offer to add and open it.\n")
	}
	for _, spec := range added {
		b.WriteString("\n### ")
		b.WriteString(promptText(spec.Name))
		b.WriteString(" (")
		b.WriteString(promptText(spec.ID))
		if spec.Core {
			b.WriteString(", core — cannot be removed")
		} else {
			b.WriteString(", added — removable")
		}
		b.WriteString(")\n")
		if desc := strings.TrimSpace(promptText(spec.Description)); desc != "" {
			b.WriteString(desc)
			b.WriteString("\n")
		}
		if prompt := strings.TrimSpace(promptText(spec.Prompt)); prompt != "" {
			b.WriteString(prompt)
			b.WriteString("\n")
		}
		if len(spec.Actions) > 0 {
			b.WriteString("Actions:\n")
			for _, action := range spec.Actions {
				b.WriteString("- ")
				b.WriteString(promptText(action.Name))
				if args := strings.TrimSpace(promptText(action.Args)); args != "" {
					b.WriteString(" ")
					b.WriteString(args)
				}
				if summary := strings.TrimSpace(promptText(action.Summary)); summary != "" {
					b.WriteString(" — ")
					b.WriteString(summary)
				}
				b.WriteString("\n")
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// NavigableViews lists every view id app.navigate accepts: the fixed app
// surfaces plus the added modules. Derived from the registry — the frontend
// View switch and this list share the module catalog as the single source.
func NavigableViews(store ports.WorkspaceModuleStore, catalog []domain.ModuleSpec) ([]string, error) {
	added, err := AddedModuleIDs(store, catalog)
	if err != nil {
		return nil, err
	}
	// "settings" is always navigable even when it is not currently added/visible
	// in the sidebar — it is a fixed built-in surface. Dedupe so it appears
	// exactly once when it also shows up in the added-module list.
	views := []string{"home", "settings"}
	seen := map[string]bool{"home": true, "settings": true}
	for _, id := range added {
		if seen[id] {
			continue
		}
		seen[id] = true
		views = append(views, id)
	}
	return views, nil
}
