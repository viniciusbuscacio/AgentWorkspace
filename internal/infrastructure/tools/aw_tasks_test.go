package tools

import (
	"context"
	"strings"
	"testing"

	"aw/internal/domain"
)

func tasksWorkspace(t *testing.T, added *[]string) *workspace {
	t.Helper()
	return &workspace{
		root:           t.TempDir(),
		control:        &fakeControl{},
		addedModulesFn: func() []string { return *added },
		tasks: &TasksFuncs{
			List: func(_ context.Context) (any, error) { return []string{}, nil },
			Get: func(_ context.Context, id string) (any, error) {
				return map[string]any{"id": id, "attachments": []string{"shot.png"}}, nil
			},
			Add: func(_ context.Context, title, body, status string) (any, error) {
				return map[string]any{"title": title, "body": body, "status": status}, nil
			},
			Update: func(_ context.Context, id, _ string, body *string, status string, position *int) (any, error) {
				out := map[string]any{"id": id, "status": status}
				if body != nil {
					out["body"] = *body
				}
				if position != nil {
					out["position"] = *position
				}
				return out, nil
			},
			Delete: func(_ context.Context, id string) (any, error) { return map[string]any{"id": id}, nil },
		},
	}
}

func TestTasksActionsGatedByModuleAdded(t *testing.T) {
	added := []string{"chat"}
	ws := tasksWorkspace(t, &added)

	if _, err := ws.awDispatch(nil, awArgs{Action: "tasks.list"}); err == nil ||
		!strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("tasks.list without the module should be unknown, got %v", err)
	}

	added = []string{"chat", "tasks"}
	result, err := ws.awDispatch(nil, awArgs{Action: "tasks.add", Args: `{"title":"Revisar","body":"Detalhes","status":"in-progress"}`})
	if err != nil {
		t.Fatalf("tasks.add error = %v", err)
	}
	if !strings.Contains(result.Result, `"body": "Detalhes"`) || !strings.Contains(result.Result, `"status": "in-progress"`) {
		t.Fatalf("tasks.add result = %s", result.Result)
	}
	result, err = ws.awDispatch(nil, awArgs{Action: "tasks.update", Args: `{"id":"task-1","status":"completed","position":3}`})
	if err != nil {
		t.Fatalf("tasks.update error = %v", err)
	}
	if !strings.Contains(result.Result, `"position": 3`) || !strings.Contains(result.Result, `"status": "completed"`) {
		t.Fatalf("tasks.update result = %s", result.Result)
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "tasks.update", Args: `{"id":"task-1"}`}); err == nil {
		t.Fatal("tasks.update without fields should fail")
	}
}

// tasks.update with only a body is valid (presence semantics: an empty
// body clears the description); tasks.get returns attachment metadata.
func TestTasksBodyAndGetActions(t *testing.T) {
	added := []string{"tasks"}
	ws := tasksWorkspace(t, &added)

	result, err := ws.awDispatch(nil, awArgs{Action: "tasks.update", Args: `{"id":"task-1","body":""}`})
	if err != nil {
		t.Fatalf("tasks.update body-only error = %v", err)
	}
	if !strings.Contains(result.Result, `"body": ""`) {
		t.Fatalf("tasks.update result = %s", result.Result)
	}

	result, err = ws.awDispatch(nil, awArgs{Action: "tasks.get", Args: `{"id":"task-1"}`})
	if err != nil {
		t.Fatalf("tasks.get error = %v", err)
	}
	if !strings.Contains(result.Result, "shot.png") {
		t.Fatalf("tasks.get result = %s", result.Result)
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "tasks.get", Args: `{}`}); err == nil {
		t.Fatal("tasks.get without id should fail")
	}
}

// Spec-vs-registry drift guard, same as Notes.
func TestTasksSpecActionsMatchRegistry(t *testing.T) {
	added := []string{"tasks"}
	ws := tasksWorkspace(t, &added)
	reg := ws.awRegistry()

	spec, ok := domain.ModuleByID(domain.ModuleCatalog(), "tasks")
	if !ok {
		t.Fatal("tasks module missing from the catalog")
	}
	documented := map[string]bool{}
	for _, action := range spec.Actions {
		documented[action.Name] = true
		if _, registered := reg[action.Name]; !registered {
			t.Errorf("spec documents %s but the registry does not register it", action.Name)
		}
	}
	for name := range reg {
		if strings.HasPrefix(name, "tasks.") && !documented[name] {
			t.Errorf("registry registers %s but the ModuleSpec does not document it", name)
		}
	}
}
