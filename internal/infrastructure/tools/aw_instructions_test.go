package tools

import (
	"context"
	"strings"
	"testing"
)

func TestInstructionActionsRegisteredWhenWired(t *testing.T) {
	ws := &workspace{instructions: &InstructionFuncs{
		List: func(context.Context) (any, error) { return map[string]any{"documents": []any{}}, nil },
	}}
	reg := ws.awRegistry()
	for _, action := range []string{
		"instructions.list", "instructions.read", "instructions.save",
		"instructions.reset", "instructions.effective", "instructions.sources",
	} {
		if _, ok := reg[action]; !ok {
			t.Errorf("action %q not registered", action)
		}
	}
}

func TestInstructionActionsAbsentWhenNil(t *testing.T) {
	ws := &workspace{}
	if _, ok := ws.awRegistry()["instructions.list"]; ok {
		t.Error("instructions.list must not register without funcs")
	}
}

func TestInstructionUnavailableWhenCallbackNil(t *testing.T) {
	// Funcs struct wired but the specific callback is nil → clear unavailable error.
	ws := &workspace{instructions: &InstructionFuncs{
		List: func(context.Context) (any, error) { return map[string]any{}, nil },
	}}
	_, err := ws.awRegistry()["instructions.read"](context.Background(), map[string]any{"id": "AGENTS.md"}, ws)
	if err == nil || !strings.Contains(err.Error(), "instructions") {
		t.Errorf("expected unavailable error, got %v", err)
	}
}

func TestInstructionListForwards(t *testing.T) {
	ws := &workspace{instructions: &InstructionFuncs{
		List: func(context.Context) (any, error) {
			return map[string]any{"documents": []string{"AGENTS.md", "USER.md"}}, nil
		},
	}}
	out, err := ws.awRegistry()["instructions.list"](context.Background(), nil, ws)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if !strings.Contains(out, "AGENTS.md") || !strings.Contains(out, "USER.md") {
		t.Errorf("list output missing documents: %s", out)
	}
}
