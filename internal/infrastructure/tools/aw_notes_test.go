package tools

import (
	"context"
	"strings"
	"testing"

	"aw/internal/domain"
)

// notesCall records the arguments of the last notes.update / notes.list
// dispatch so tests can assert the partial-update plumbing.
type notesCall struct {
	includeArchived            bool
	title, content             *string
	pinned, archived, inPrompt *bool
	createInPrompt             *bool
}

func notesWorkspace(t *testing.T, added *[]string) (*workspace, *notesCall) {
	t.Helper()
	last := &notesCall{}
	ws := &workspace{
		root:           t.TempDir(),
		control:        &fakeControl{},
		addedModulesFn: func() []string { return *added },
		notes: &NotesFuncs{
			List: func(_ context.Context, includeArchived bool) (any, error) {
				last.includeArchived = includeArchived
				return []string{}, nil
			},
			Get: func(_ context.Context, id string) (any, error) { return map[string]any{"id": id}, nil },
			Create: func(_ context.Context, title, _ string, inPrompt bool) (any, error) {
				last.createInPrompt = &inPrompt
				return map[string]any{"title": title}, nil
			},
			Update: func(_ context.Context, id string, title, content *string, pinned, archived, inPrompt *bool) (any, error) {
				last.title, last.content, last.pinned, last.archived, last.inPrompt = title, content, pinned, archived, inPrompt
				return map[string]any{"id": id}, nil
			},
			Delete: func(_ context.Context, id string) (any, error) { return map[string]any{"id": id}, nil },
		},
	}
	return ws, last
}

func TestNotesActionsGatedByModuleAddedAndHotToggle(t *testing.T) {
	added := []string{"chat"}
	ws, last := notesWorkspace(t, &added)

	// Not added: the action group does not exist at all.
	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.list"}); err == nil ||
		!strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("notes.list without the module should be unknown, got %v", err)
	}

	// Added: same process, no rebuild — the registry is per-dispatch.
	added = []string{"chat", "notes"}
	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.list"}); err != nil {
		t.Fatalf("notes.list with the module added error = %v", err)
	}
	result, err := ws.awDispatch(nil, awArgs{Action: "notes.create", Args: `{"title":"Plano"}`})
	if err != nil {
		t.Fatalf("notes.create error = %v", err)
	}
	if !strings.Contains(result.Result, "Plano") {
		t.Fatalf("notes.create result = %s", result.Result)
	}
	// Omitted inPrompt defaults to true; an explicit false is forwarded.
	if last.createInPrompt == nil || !*last.createInPrompt {
		t.Fatalf("notes.create without inPrompt must default to true, got %v", last.createInPrompt)
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.create", Args: `{"title":"Privada","inPrompt":false}`}); err != nil {
		t.Fatalf("notes.create inPrompt=false error = %v", err)
	}
	if last.createInPrompt == nil || *last.createInPrompt {
		t.Fatalf("notes.create inPrompt=false must forward false, got %v", last.createInPrompt)
	}

	// Removed again: fenced off in the same process.
	added = []string{"chat"}
	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.create", Args: `{"title":"x"}`}); err == nil {
		t.Fatal("notes.create after removal should be unknown")
	}
}

func TestNotesUpdateRequiresSomeField(t *testing.T) {
	added := []string{"notes"}
	ws, _ := notesWorkspace(t, &added)
	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.update", Args: `{"id":"n1"}`}); err == nil {
		t.Fatal("notes.update without title/content/pinned/archived/inPrompt should fail")
	}
}

func TestNotesUpdatePassesOnlyProvidedFields(t *testing.T) {
	added := []string{"notes"}
	ws, last := notesWorkspace(t, &added)

	// Flags-only update: text pointers stay nil so current values are kept.
	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.update", Args: `{"id":"n1","pinned":true}`}); err != nil {
		t.Fatalf("notes.update pinned-only error = %v", err)
	}
	if last.title != nil || last.content != nil || last.archived != nil || last.inPrompt != nil {
		t.Fatalf("pinned-only update leaked fields: %+v", last)
	}
	if last.pinned == nil || !*last.pinned {
		t.Fatalf("pinned not passed: %+v", last.pinned)
	}

	// Provided-but-empty content is a real value (clears the note), and
	// archived=false is a real value (unarchive).
	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.update", Args: `{"id":"n1","content":"","archived":false}`}); err != nil {
		t.Fatalf("notes.update clear-content error = %v", err)
	}
	if last.content == nil || *last.content != "" {
		t.Fatalf("empty content not passed: %+v", last.content)
	}
	if last.archived == nil || *last.archived {
		t.Fatalf("archived=false not passed: %+v", last.archived)
	}

	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.update", Args: `{"id":"n1","pinned":"yes"}`}); err == nil {
		t.Fatal("notes.update with non-boolean pinned should fail")
	}
}

func TestNotesListPassesIncludeArchived(t *testing.T) {
	added := []string{"notes"}
	ws, last := notesWorkspace(t, &added)

	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.list"}); err != nil {
		t.Fatalf("notes.list error = %v", err)
	}
	if last.includeArchived {
		t.Fatal("notes.list should default to includeArchived=false")
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.list", Args: `{"includeArchived":true}`}); err != nil {
		t.Fatalf("notes.list includeArchived error = %v", err)
	}
	if !last.includeArchived {
		t.Fatal("includeArchived=true not passed through")
	}
}

// TestNotesSpecActionsMatchRegistry kills registry/prompt drift: every action
// the catalog's ModuleSpec documents must exist in the dispatcher, and every
// registered notes.* action must be documented in the spec.
func TestNotesSpecActionsMatchRegistry(t *testing.T) {
	added := []string{"notes"}
	ws, _ := notesWorkspace(t, &added)
	reg := ws.awRegistry()

	spec, ok := domain.ModuleByID(domain.ModuleCatalog(), "notes")
	if !ok {
		t.Fatal("notes module missing from the catalog")
	}
	documented := map[string]bool{}
	for _, action := range spec.Actions {
		documented[action.Name] = true
		if _, registered := reg[action.Name]; !registered {
			t.Errorf("spec documents %s but the registry does not register it", action.Name)
		}
	}
	for name := range reg {
		if strings.HasPrefix(name, "notes.") && !documented[name] {
			t.Errorf("registry registers %s but the ModuleSpec does not document it", name)
		}
	}
}

func TestNotesUpdatePassesInPromptFlag(t *testing.T) {
	added := []string{"notes"}
	ws, last := notesWorkspace(t, &added)

	// inPrompt=false should be passed through as a real bool pointer.
	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.update", Args: `{"id":"n1","inPrompt":false}`}); err != nil {
		t.Fatalf("notes.update inPrompt=false error = %v", err)
	}
	if last.inPrompt == nil || *last.inPrompt {
		t.Fatalf("inPrompt=false not passed: %+v", last.inPrompt)
	}
	if last.title != nil || last.content != nil || last.pinned != nil || last.archived != nil {
		t.Fatalf("inPrompt-only update leaked other fields: %+v", last)
	}

	// inPrompt=true.
	if _, err := ws.awDispatch(nil, awArgs{Action: "notes.update", Args: `{"id":"n1","inPrompt":true}`}); err != nil {
		t.Fatalf("notes.update inPrompt=true error = %v", err)
	}
	if last.inPrompt == nil || !*last.inPrompt {
		t.Fatalf("inPrompt=true not passed: %+v", last.inPrompt)
	}
}
