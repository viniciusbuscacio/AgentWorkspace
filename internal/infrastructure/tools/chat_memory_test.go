package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestChatMemoryActionsRegisteredOnlyWhenWired(t *testing.T) {
	empty := &workspace{root: t.TempDir()}
	for _, action := range []string{"memory.chat.search", "memory.chat.open", "memory.chat.recent"} {
		if _, ok := empty.awRegistry()[action]; ok {
			t.Fatalf("%s should not be registered when unwired", action)
		}
	}

	ws := &workspace{
		root: t.TempDir(),
		chatSearchFn: func(_ context.Context, query string, limit int) (any, error) {
			return map[string]any{"query": query, "limit": limit}, nil
		},
		chatHistoryFn: func(_ context.Context, sessionID string) (any, error) {
			return map[string]any{"sessionId": sessionID}, nil
		},
		chatCatalogFn: func(_ context.Context) (any, error) {
			return []string{"chat-a"}, nil
		},
	}
	for _, action := range []string{"memory.chat.search", "memory.chat.open", "memory.chat.recent"} {
		if _, ok := ws.awRegistry()[action]; !ok {
			t.Fatalf("%s should be registered when wired", action)
		}
	}
}

func TestMemoryChatSearchActionCallsInjectedFn(t *testing.T) {
	var gotQuery string
	var gotLimit int
	ws := &workspace{
		root: t.TempDir(),
		chatSearchFn: func(_ context.Context, query string, limit int) (any, error) {
			gotQuery = query
			gotLimit = limit
			return map[string]any{"ok": true}, nil
		},
	}
	out, err := ws.dispatchAction(context.Background(), awArgs{Action: "memory.chat.search", Args: `{"query":"deploy","limit":5}`})
	if err != nil {
		t.Fatalf("memory.chat.search error = %v", err)
	}
	if gotQuery != "deploy" || gotLimit != 5 {
		t.Fatalf("injected fn got query=%q limit=%d", gotQuery, gotLimit)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out.Result), &parsed); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.Result)
	}
	if parsed["ok"] != true {
		t.Fatalf("unexpected output: %s", out.Result)
	}
}

func TestChatMemoryActionsErrorWhenUnwired(t *testing.T) {
	ws := &workspace{root: t.TempDir()}
	for _, input := range []awArgs{
		{Action: "memory.chat.search", Args: `{"query":"x"}`},
		{Action: "memory.chat.open", Args: `{"sessionId":"s"}`},
		{Action: "memory.chat.recent"},
	} {
		if _, err := ws.dispatchAction(context.Background(), input); err == nil {
			t.Fatalf("expected error when %s is unwired", input.Action)
		}
	}
}
