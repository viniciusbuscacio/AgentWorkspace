package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewDispatcherRequiresRoot(t *testing.T) {
	dispatcher, err := NewDispatcher(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if dispatcher != nil {
		t.Fatal("expected nil dispatcher without a workspace root")
	}
}

func TestDispatcherCallRoutesActions(t *testing.T) {
	control := &fakeControl{}
	dispatcher, err := NewDispatcher(Options{Root: t.TempDir(), Control: control})
	if err != nil {
		t.Fatal(err)
	}
	out, err := dispatcher.Call(context.Background(), "aw.actions", "")
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	if err := json.Unmarshal([]byte(out), &actions); err != nil {
		t.Fatalf("aw.actions must return a JSON list: %v", err)
	}
	found := false
	for _, action := range actions {
		if action == "app.theme.set" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected app.theme.set in actions, got %v", actions)
	}

	if _, err := dispatcher.Call(context.Background(), "app.theme.set", `{"theme":"ocean"}`); err != nil {
		t.Fatal(err)
	}
	if control.theme != "ocean" {
		t.Fatalf("expected theme ocean, got %q", control.theme)
	}
}

func TestDispatcherCallRejectsUnknownAction(t *testing.T) {
	dispatcher, err := NewDispatcher(Options{Root: t.TempDir(), Control: &fakeControl{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dispatcher.Call(context.Background(), "nope.nothing", ""); err == nil {
		t.Fatal("expected an error for an unknown action")
	}
}

func TestDispatcherDescriptionMatchesOptions(t *testing.T) {
	dispatcher, err := NewDispatcher(Options{Root: t.TempDir(), Control: &fakeControl{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dispatcher.Description(), "app.theme.set") {
		t.Fatal("expected the description to document app control actions")
	}
}
