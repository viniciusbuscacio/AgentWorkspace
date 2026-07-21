package tools

import (
	"strings"
	"testing"
)

func TestModuleActionsRouteToControl(t *testing.T) {
	ws, control := controlWorkspace(t)

	if _, err := ws.awDispatch(nil, awArgs{Action: "module.list"}); err != nil {
		t.Fatalf("module.list error = %v", err)
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "module.add", Args: `{"id":"notes"}`}); err != nil {
		t.Fatalf("module.add error = %v", err)
	}
	if control.moduleID != "notes" {
		t.Fatalf("module.add routed id = %q", control.moduleID)
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "module.remove", Args: `{"id":"notes"}`}); err != nil {
		t.Fatalf("module.remove error = %v", err)
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "module.add"}); err == nil {
		t.Fatal("module.add without id should fail")
	}
	want := []string{"ListModules", "AddModule", "RemoveModule"}
	for _, call := range want {
		found := false
		for _, got := range control.calls {
			if got == call {
				found = true
			}
		}
		if !found {
			t.Fatalf("control calls = %v, missing %s", control.calls, call)
		}
	}
}

func TestModuleHideShowMoveRouteToControl(t *testing.T) {
	ws, control := controlWorkspace(t)

	// module.hide requires id.
	if _, err := ws.awDispatch(nil, awArgs{Action: "module.hide"}); err == nil {
		t.Fatal("module.hide without id should fail")
	}
	// module.hide routes.
	if _, err := ws.awDispatch(nil, awArgs{Action: "module.hide", Args: `{"id":"notes"}`}); err != nil {
		t.Fatalf("module.hide error = %v", err)
	}
	if control.moduleID != "notes" || !awContains(control.calls, "HideModule") {
		t.Fatalf("HideModule not called correctly; id=%q calls=%v", control.moduleID, control.calls)
	}

	// module.show requires id.
	if _, err := ws.awDispatch(nil, awArgs{Action: "module.show"}); err == nil {
		t.Fatal("module.show without id should fail")
	}
	// module.show routes.
	if _, err := ws.awDispatch(nil, awArgs{Action: "module.show", Args: `{"id":"tasks"}`}); err != nil {
		t.Fatalf("module.show error = %v", err)
	}
	if control.moduleID != "tasks" || !awContains(control.calls, "ShowModule") {
		t.Fatalf("ShowModule not called correctly; id=%q calls=%v", control.moduleID, control.calls)
	}

	// module.move requires both id and up.
	if _, err := ws.awDispatch(nil, awArgs{Action: "module.move", Args: `{"id":"notes"}`}); err == nil ||
		!strings.Contains(err.Error(), "up is required") {
		t.Fatalf("module.move without up error = %v, want required hint", err)
	}
	// module.move routes with up=true.
	if _, err := ws.awDispatch(nil, awArgs{Action: "module.move", Args: `{"id":"notes","up":true}`}); err != nil {
		t.Fatalf("module.move error = %v", err)
	}
	if control.moduleID != "notes" || !control.open {
		t.Fatalf("MoveModule id=%q up=%v, want notes/true", control.moduleID, control.open)
	}
	// module.move with up=false.
	if _, err := ws.awDispatch(nil, awArgs{Action: "module.move", Args: `{"id":"tasks","up":false}`}); err != nil {
		t.Fatalf("module.move down error = %v", err)
	}
	if control.moduleID != "tasks" || control.open {
		t.Fatalf("MoveModule down id=%q up=%v, want tasks/false", control.moduleID, control.open)
	}
}

func TestAwNavigateUsesRegistryDerivedViews(t *testing.T) {
	ws, control := controlWorkspace(t)
	control.views = []string{"chat", "notes"}

	if _, err := ws.awDispatch(nil, awArgs{Action: "app.navigate", Args: `{"view":"notes"}`}); err != nil {
		t.Fatalf("app.navigate to added module error = %v", err)
	}
	if control.view != "notes" {
		t.Fatalf("navigated view = %q", control.view)
	}

	control.views = []string{"chat"}
	_, err := ws.awDispatch(nil, awArgs{Action: "app.navigate", Args: `{"view":"notes"}`})
	if err == nil || !strings.Contains(err.Error(), "unknown view") {
		t.Fatalf("app.navigate to not-added module should fail, got %v", err)
	}
}
