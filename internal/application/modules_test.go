package application

import (
	"strings"
	"testing"

	"aw/internal/domain"
)

type fakeModuleStore struct {
	ids    []string
	hidden []string
	order  []string
}

func (s *fakeModuleStore) LoadAddedModules() ([]string, error)  { return s.ids, nil }
func (s *fakeModuleStore) SaveAddedModules(ids []string) error  { s.ids = ids; return nil }
func (s *fakeModuleStore) LoadHiddenModules() ([]string, error) { return s.hidden, nil }
func (s *fakeModuleStore) SaveHiddenModules(ids []string) error { s.hidden = ids; return nil }
func (s *fakeModuleStore) LoadModuleOrder() ([]string, error)   { return s.order, nil }
func (s *fakeModuleStore) SaveModuleOrder(ids []string) error   { s.order = ids; return nil }

func testCatalog() []domain.ModuleSpec {
	return []domain.ModuleSpec{
		{ID: "chat", Name: "Chat", Core: true},
		{ID: "notes", Name: "Notes"},
		{ID: "tasks", Name: "Tasks"},
		{ID: "future", Name: "Future", ComingSoon: true},
	}
}

func TestAddedModuleIDsDefaultsToCoreOnly(t *testing.T) {
	store := &fakeModuleStore{} // nothing persisted yet (existing installs)
	ids, err := AddedModuleIDs(store, testCatalog())
	if err != nil {
		t.Fatalf("AddedModuleIDs() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != "chat" {
		t.Fatalf("ids = %v, want [chat] (absent state = core only, never empty)", ids)
	}
}

func TestAddedModuleIDsAlwaysIncludesCoreAndDropsUnknown(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes", "ghost", "future"}}
	ids, err := AddedModuleIDs(store, testCatalog())
	if err != nil {
		t.Fatalf("AddedModuleIDs() error = %v", err)
	}
	if strings.Join(ids, ",") != "chat,notes" {
		t.Fatalf("ids = %v, want [chat notes] (core forced in, unknown/coming-soon dropped)", ids)
	}
}

func TestAddModuleGuards(t *testing.T) {
	store := &fakeModuleStore{}
	catalog := testCatalog()

	if err := AddModule(store, catalog, "ghost"); err == nil {
		t.Fatal("AddModule(ghost) should fail for unknown module")
	}
	if err := AddModule(store, catalog, "future"); err == nil {
		t.Fatal("AddModule(future) should fail for coming-soon module")
	}
	if err := AddModule(store, catalog, "notes"); err != nil {
		t.Fatalf("AddModule(notes) error = %v", err)
	}
	if err := AddModule(store, catalog, "notes"); err != nil {
		t.Fatalf("AddModule(notes) twice should be a no-op, got %v", err)
	}
	ids, _ := AddedModuleIDs(store, catalog)
	if strings.Join(ids, ",") != "chat,notes" {
		t.Fatalf("ids after add = %v", ids)
	}
}

func TestRemoveModuleGuards(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes", "tasks"}}
	catalog := testCatalog()

	if err := RemoveModule(store, catalog, "chat"); err == nil {
		t.Fatal("RemoveModule(chat) should fail — core module")
	}
	if err := RemoveModule(store, catalog, "ghost"); err == nil {
		t.Fatal("RemoveModule(ghost) should fail for unknown module")
	}
	if err := RemoveModule(store, catalog, "notes"); err != nil {
		t.Fatalf("RemoveModule(notes) error = %v", err)
	}
	if err := RemoveModule(store, catalog, "notes"); err != nil {
		t.Fatalf("RemoveModule(notes) twice should be a no-op, got %v", err)
	}
	ids, _ := AddedModuleIDs(store, catalog)
	if strings.Join(ids, ",") != "chat,tasks" {
		t.Fatalf("ids after remove = %v", ids)
	}
}

func TestListModulesResolvesAddedFlag(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes"}}
	statuses, err := ListModules(store, testCatalog())
	if err != nil {
		t.Fatalf("ListModules() error = %v", err)
	}
	byID := map[string]ModuleStatus{}
	for _, status := range statuses {
		byID[status.ID] = status
	}
	if !byID["chat"].Added || !byID["notes"].Added {
		t.Fatalf("chat and notes should be added: %+v", statuses)
	}
	if byID["tasks"].Added || byID["future"].Added {
		t.Fatalf("tasks and future should not be added: %+v", statuses)
	}
}

func TestNavigableViewsDerivesFromRegistry(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"tasks"}}
	views, err := NavigableViews(store, testCatalog())
	if err != nil {
		t.Fatalf("NavigableViews() error = %v", err)
	}
	if strings.Join(views, ",") != "home,settings,chat,tasks" {
		t.Fatalf("views = %v", views)
	}
}

func TestNavigableViewsListsSettingsOnceWhenAdded(t *testing.T) {
	// settings is both a hardcoded fixed surface and (now) a catalog module that
	// can be added — NavigableViews must not list it twice.
	store := &fakeModuleStore{ids: []string{"settings"}}
	views, err := NavigableViews(store, domain.ModuleCatalog())
	if err != nil {
		t.Fatalf("NavigableViews() error = %v", err)
	}
	count := 0
	for _, v := range views {
		if v == "settings" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("settings should appear exactly once, got %d in %v", count, views)
	}
}

func TestSettingsIsAFixedNonCoreCatalogModule(t *testing.T) {
	spec, ok := domain.ModuleByID(domain.ModuleCatalog(), "settings")
	if !ok {
		t.Fatal("settings must be in the module catalog")
	}
	if !spec.Fixed {
		t.Fatal("settings must be Fixed (openable/closeable, not removable)")
	}
	if spec.Core {
		t.Fatal("settings must not be Core (it must be closeable)")
	}
}

// Close ≠ remove (sidebar-modules spec, Decision 1): hiding keeps the module
// added — the capability surface must not change.
func TestHideModuleKeepsModuleAdded(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes", "tasks"}}
	if err := HideModule(store, testCatalog(), "notes"); err != nil {
		t.Fatalf("HideModule() error = %v", err)
	}
	if err := HideModule(store, testCatalog(), "notes"); err != nil {
		t.Fatalf("HideModule() twice error = %v", err)
	}
	if len(store.hidden) != 1 || store.hidden[0] != "notes" {
		t.Fatalf("hidden = %v, want [notes]", store.hidden)
	}
	added, err := AddedModuleIDs(store, testCatalog())
	if err != nil {
		t.Fatalf("AddedModuleIDs() error = %v", err)
	}
	if strings.Join(added, ",") != "chat,notes,tasks" {
		t.Fatalf("added = %v — hiding must not remove", added)
	}
	specs, err := AddedModuleSpecs(store, testCatalog())
	if err != nil {
		t.Fatalf("AddedModuleSpecs() error = %v", err)
	}
	if len(specs) != 3 {
		t.Fatalf("specs = %d — the prompt/actions surface must ignore hidden", len(specs))
	}
}

func TestHideModuleRejectsCoreAndUnknown(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes"}}
	if err := HideModule(store, testCatalog(), "chat"); err == nil {
		t.Fatal("hiding the core chat module should fail")
	}
	if err := HideModule(store, testCatalog(), "ghost"); err == nil {
		t.Fatal("hiding an unknown module should fail")
	}
}

func TestShowModuleUnhides(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes"}, hidden: []string{"notes"}}
	if err := ShowModule(store, testCatalog(), "notes"); err != nil {
		t.Fatalf("ShowModule() error = %v", err)
	}
	if len(store.hidden) != 0 {
		t.Fatalf("hidden = %v, want empty", store.hidden)
	}
	if err := ShowModule(store, testCatalog(), "notes"); err != nil {
		t.Fatalf("ShowModule() on visible module error = %v", err)
	}
}

// module.add on an already-added module is the reopen path (Decision 4).
func TestAddModuleUnhides(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes"}, hidden: []string{"notes"}}
	if err := AddModule(store, testCatalog(), "notes"); err != nil {
		t.Fatalf("AddModule() error = %v", err)
	}
	if len(store.hidden) != 0 {
		t.Fatalf("hidden = %v — add must unhide", store.hidden)
	}
	if strings.Join(store.ids, ",") != "notes" && strings.Join(store.ids, ",") != "chat,notes" {
		t.Fatalf("ids = %v — add of added module must not duplicate", store.ids)
	}
}

func TestRemoveModuleClearsHiddenState(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes"}, hidden: []string{"notes"}}
	if err := RemoveModule(store, testCatalog(), "notes"); err != nil {
		t.Fatalf("RemoveModule() error = %v", err)
	}
	if len(store.hidden) != 0 {
		t.Fatalf("hidden = %v — a re-add must start visible", store.hidden)
	}
}

func TestSidebarModuleOrderAndMove(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes", "tasks"}}
	order, err := SidebarModuleOrder(store, testCatalog())
	if err != nil {
		t.Fatalf("SidebarModuleOrder() error = %v", err)
	}
	if strings.Join(order, ",") != "notes,tasks" {
		t.Fatalf("order = %v, want catalog order with nothing stored", order)
	}

	if err := MoveModule(store, testCatalog(), "tasks", true); err != nil {
		t.Fatalf("MoveModule(up) error = %v", err)
	}
	if strings.Join(store.order, ",") != "tasks,notes" {
		t.Fatalf("order = %v after move up", store.order)
	}
	// Edge moves are no-ops.
	if err := MoveModule(store, testCatalog(), "tasks", true); err != nil {
		t.Fatalf("MoveModule(up at top) error = %v", err)
	}
	if strings.Join(store.order, ",") != "tasks,notes" {
		t.Fatalf("order = %v — moving past the edge must be a no-op", store.order)
	}
	// Stored ids that are no longer added are dropped from the order.
	store.ids = []string{"tasks"}
	order, err = SidebarModuleOrder(store, testCatalog())
	if err != nil {
		t.Fatalf("SidebarModuleOrder() error = %v", err)
	}
	if strings.Join(order, ",") != "tasks" {
		t.Fatalf("order = %v after notes removed", order)
	}
}

func TestListModulesExposesHiddenAndPositions(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes", "tasks"}, hidden: []string{"tasks", "ghost"}, order: []string{"tasks", "notes"}}
	statuses, err := ListModules(store, testCatalog())
	if err != nil {
		t.Fatalf("ListModules() error = %v", err)
	}
	byID := map[string]ModuleStatus{}
	for _, status := range statuses {
		byID[status.ID] = status
	}
	if byID["notes"].Hidden || !byID["tasks"].Hidden {
		t.Fatalf("hidden flags wrong: %+v", byID)
	}
	if byID["tasks"].SidebarPosition != 0 || byID["notes"].SidebarPosition != 1 {
		t.Fatalf("positions wrong: tasks=%d notes=%d", byID["tasks"].SidebarPosition, byID["notes"].SidebarPosition)
	}
	if byID["chat"].SidebarPosition != -1 || byID["future"].SidebarPosition != -1 {
		t.Fatalf("non-sidebar entries must have position -1: %+v", byID)
	}
}

func TestHideAllModulesIsOneWrite(t *testing.T) {
	// Both notes and tasks are added; calling HideAllModules must produce
	// exactly one SaveHiddenModules call that includes both non-core ids.
	store := &countingModuleStore{
		fakeModuleStore: fakeModuleStore{ids: []string{"notes", "tasks"}},
	}
	if err := HideAllModules(store, testCatalog()); err != nil {
		t.Fatalf("HideAllModules() error = %v", err)
	}
	if store.saveCount != 1 {
		t.Fatalf("SaveHiddenModules called %d times, want exactly 1", store.saveCount)
	}
}

func TestHideAllModulesSkipsCore(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes"}}
	if err := HideAllModules(store, testCatalog()); err != nil {
		t.Fatalf("HideAllModules() error = %v", err)
	}
	for _, id := range store.hidden {
		if id == "chat" {
			t.Fatalf("HideAllModules must not hide core module 'chat'")
		}
	}
}

func TestHideAllModulesIdempotent(t *testing.T) {
	// If notes is already hidden, HideAllModules should not duplicate it.
	store := &fakeModuleStore{ids: []string{"notes"}, hidden: []string{"notes"}}
	if err := HideAllModules(store, testCatalog()); err != nil {
		t.Fatalf("HideAllModules() error = %v", err)
	}
	count := 0
	for _, id := range store.hidden {
		if id == "notes" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("'notes' appears %d times in hidden list, want 1", count)
	}
}

func TestValidateRestoreViewFallsBackToHome(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"notes"}}
	cat := testCatalog()
	// "notes" is added → valid
	if got := ValidateRestoreView(store, cat, "notes"); got != "notes" {
		t.Fatalf("ValidateRestoreView(notes) = %q, want notes", got)
	}
	// "tasks" is not added → falls back
	if got := ValidateRestoreView(store, cat, "tasks"); got != "home" {
		t.Fatalf("ValidateRestoreView(tasks) = %q, want home", got)
	}
	// empty string → home
	if got := ValidateRestoreView(store, cat, ""); got != "home" {
		t.Fatalf("ValidateRestoreView('') = %q, want home", got)
	}
	// fixed surfaces are always valid
	if got := ValidateRestoreView(store, cat, "settings"); got != "settings" {
		t.Fatalf("ValidateRestoreView(settings) = %q, want settings", got)
	}
	if got := ValidateRestoreView(store, cat, "home"); got != "home" {
		t.Fatalf("ValidateRestoreView(home) = %q, want home", got)
	}
}

// countingModuleStore wraps fakeModuleStore and counts SaveHiddenModules calls.
type countingModuleStore struct {
	fakeModuleStore
	saveCount int
}

func (s *countingModuleStore) SaveHiddenModules(ids []string) error {
	s.saveCount++
	return s.fakeModuleStore.SaveHiddenModules(ids)
}

// The agent must know what CAN be added, not only what is added — asked
// about Notes with the module not added, it guessed "no notes module
// exists" instead of offering module.add (2026-06-12).
func TestModulesInstructionNamesAvailableModules(t *testing.T) {
	catalog := testCatalog()
	added := []domain.ModuleSpec{catalog[0]} // chat only
	block := ModulesInstructionWithCatalog(added, catalog)
	if !strings.Contains(block, "Available to add") {
		t.Fatalf("missing available section: %q", block)
	}
	if !strings.Contains(block, "notes") || !strings.Contains(block, "tasks") {
		t.Fatalf("available list incomplete: %q", block)
	}
	if strings.Contains(block, "future") {
		t.Fatalf("coming-soon modules must not be offered: %q", block)
	}
	// Core vs added is labeled so the agent never calls a removable module
	// "integrated" (it did, 2026-06-12).
	labeled := ModulesInstructionWithCatalog([]domain.ModuleSpec{catalog[0], catalog[1]}, catalog)
	if !strings.Contains(labeled, "core — cannot be removed") || !strings.Contains(labeled, "added — removable") {
		t.Fatalf("core/added labels missing: %q", labeled)
	}

	// An added module never shows as available.
	blockWithNotes := ModulesInstructionWithCatalog([]domain.ModuleSpec{catalog[0], catalog[1]}, catalog)
	if strings.Contains(strings.Split(blockWithNotes, "###")[0], "Available to add (module.add {id}): notes") {
		t.Fatalf("added module offered as available: %q", blockWithNotes)
	}
}

// Vaults written before the Backlog -> Tasks rename still store the old id;
// the loaders must keep the module in the workspace across the rename.
func TestLegacyBacklogIDStillLoadsAsTasks(t *testing.T) {
	store := &fakeModuleStore{ids: []string{"backlog"}}
	ids, err := AddedModuleIDs(store, domain.ModuleCatalog())
	if err != nil {
		t.Fatalf("AddedModuleIDs error = %v", err)
	}
	found := false
	for _, id := range ids {
		if id == "tasks" {
			found = true
		}
	}
	if !found {
		t.Fatalf("legacy 'backlog' id should load as 'tasks', got %v", ids)
	}
}
