package agent

import (
	"testing"

	"google.golang.org/genai"
)

func TestHasSpawnCall(t *testing.T) {
	spawn := &genai.Content{Parts: []*genai.Part{
		{Text: "vou disparar os agentes"},
		{FunctionCall: &genai.FunctionCall{Name: "aw", Args: map[string]any{"action": "system.spawn", "args": "{}"}}},
	}}
	if !hasSpawnCall(spawn) {
		t.Fatal("hasSpawnCall() = false for a system.spawn call")
	}

	other := &genai.Content{Parts: []*genai.Part{
		{FunctionCall: &genai.FunctionCall{Name: "aw", Args: map[string]any{"action": "memory.remember"}}},
	}}
	if hasSpawnCall(other) {
		t.Fatal("hasSpawnCall() = true for a non-spawn action")
	}

	foreign := &genai.Content{Parts: []*genai.Part{
		{FunctionCall: &genai.FunctionCall{Name: "browser", Args: map[string]any{"action": "system.spawn"}}},
	}}
	if hasSpawnCall(foreign) {
		t.Fatal("hasSpawnCall() = true for a non-aw tool")
	}

	if hasSpawnCall(nil) || hasSpawnCall(&genai.Content{Parts: []*genai.Part{{Text: "texto"}, nil}}) {
		t.Fatal("hasSpawnCall() = true for content without a spawn call")
	}
}
