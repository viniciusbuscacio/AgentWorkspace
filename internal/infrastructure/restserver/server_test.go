package restserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeBackend struct {
	lastAction string
	lastArgs   string
}

func (b *fakeBackend) CallAw(_ context.Context, action string, argsJSON string) (string, error) {
	b.lastAction, b.lastArgs = action, argsJSON
	if action == "boom" {
		return "", fmt.Errorf("action failed")
	}
	if action == "aw.actions" {
		return `["app.state","app.theme.set"]`, nil
	}
	return `{"ok":true}`, nil
}

func (b *fakeBackend) AwDescription() string { return "test aw dispatcher" }

func startTestServer(t *testing.T) (*Server, *fakeBackend) {
	t.Helper()
	backend := &fakeBackend{}
	server, err := NewServer(backend, Config{Addr: "127.0.0.1:0", Token: "secret-token", Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})
	return server, backend
}

func doJSON(t *testing.T, method string, url string, token string, body string, out any) int {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s %s: %v", method, url, err)
		}
	}
	return resp.StatusCode
}

func TestNewServerValidatesConfig(t *testing.T) {
	if _, err := NewServer(nil, Config{Token: "x"}); err == nil {
		t.Fatal("expected an error without a backend")
	}
	if _, err := NewServer(&fakeBackend{}, Config{Addr: "127.0.0.1:0"}); err == nil {
		t.Fatal("expected an error without a token")
	}
	if _, err := NewServer(&fakeBackend{}, Config{Addr: "0.0.0.0:0", Token: "x"}); err == nil {
		t.Fatal("expected an error for a non-loopback bind address")
	}
}

func TestEndpointsRequireBearerToken(t *testing.T) {
	server, _ := startTestServer(t)

	req, err := http.NewRequest(http.MethodPost, server.URL(), strings.NewReader(`{"action":"app.state"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("WWW-Authenticate"), "resource_metadata=") {
		t.Fatalf("expected RFC 9728 hint, got %q", resp.Header.Get("WWW-Authenticate"))
	}

	if status := doJSON(t, http.MethodPost, server.URL(), "wrong-token", `{"action":"app.state"}`, nil); status != http.StatusUnauthorized {
		t.Fatalf("expected 401 with a wrong token, got %d", status)
	}
}

func TestCallAwWithObjectAndStringArgs(t *testing.T) {
	server, backend := startTestServer(t)

	var result struct {
		Action string          `json:"action"`
		Result json.RawMessage `json:"result"`
	}
	status := doJSON(t, http.MethodPost, server.URL(), "secret-token",
		`{"action":"app.theme.set","args":{"theme":"ocean"}}`, &result)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if backend.lastAction != "app.theme.set" {
		t.Fatalf("expected action app.theme.set, got %q", backend.lastAction)
	}
	var args map[string]string
	if err := json.Unmarshal([]byte(backend.lastArgs), &args); err != nil || args["theme"] != "ocean" {
		t.Fatalf("expected object args to reach the backend as JSON, got %q", backend.lastArgs)
	}
	if string(result.Result) != `{"ok":true}` {
		t.Fatalf("expected raw JSON result, got %s", result.Result)
	}

	status = doJSON(t, http.MethodPost, server.URL(), "secret-token",
		`{"action":"app.theme.set","args":"{\"theme\":\"rose\"}"}`, nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 for string args, got %d", status)
	}
	if !strings.Contains(backend.lastArgs, "rose") {
		t.Fatalf("expected string args to pass through, got %q", backend.lastArgs)
	}
}

func TestCallAwErrorsAreJSON(t *testing.T) {
	server, _ := startTestServer(t)

	var failure struct {
		Error string `json:"error"`
	}
	if status := doJSON(t, http.MethodPost, server.URL(), "secret-token", `{"action":"boom"}`, &failure); status != http.StatusBadRequest {
		t.Fatalf("expected 400 for a failing action, got %d", status)
	}
	if failure.Error != "action failed" {
		t.Fatalf("expected the action error, got %q", failure.Error)
	}

	if status := doJSON(t, http.MethodPost, server.URL(), "secret-token", `{"action":"x","args":[1,2]}`, nil); status != http.StatusBadRequest {
		t.Fatalf("expected 400 for array args, got %d", status)
	}
	if status := doJSON(t, http.MethodPost, server.URL(), "secret-token", `not json`, nil); status != http.StatusBadRequest {
		t.Fatalf("expected 400 for an invalid body, got %d", status)
	}
}

func TestActionsAndIndexEndpoints(t *testing.T) {
	server, _ := startTestServer(t)
	base := "http://" + server.Addr()

	var listing struct {
		Actions []string `json:"actions"`
	}
	if status := doJSON(t, http.MethodGet, base+"/api/actions", "secret-token", "", &listing); status != http.StatusOK {
		t.Fatalf("expected 200 for /api/actions, got %d", status)
	}
	if len(listing.Actions) != 2 || listing.Actions[0] != "app.state" {
		t.Fatalf("unexpected actions: %v", listing.Actions)
	}

	var index struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if status := doJSON(t, http.MethodGet, base+"/api", "secret-token", "", &index); status != http.StatusOK {
		t.Fatalf("expected 200 for /api, got %d", status)
	}
	if index.Description != "test aw dispatcher" {
		t.Fatalf("expected the aw description on the index, got %q", index.Description)
	}
}

func TestResourceMetadataIsServedWithoutToken(t *testing.T) {
	server, _ := startTestServer(t)

	var payload struct {
		Resource string `json:"resource"`
	}
	if status := doJSON(t, http.MethodGet, server.metadataURL(), "", "", &payload); status != http.StatusOK {
		t.Fatal("expected 200 for protected-resource metadata")
	}
	if payload.Resource != "http://"+server.Addr()+"/api" {
		t.Fatalf("unexpected resource %q", payload.Resource)
	}
}

type fakeChatBackend struct {
	lastChatID string
	lastText   string
}

func (b *fakeChatBackend) RunChat(_ context.Context, chatID string, text string, _ bool) (ChatReply, error) {
	b.lastChatID, b.lastText = chatID, text
	if text == "boom" {
		return ChatReply{}, fmt.Errorf("chat failed")
	}
	return ChatReply{ChatID: chatID, Reply: "reply for " + text, AssistantMessageID: "m-1"}, nil
}

func TestChatRouteOnlyRegisteredWhenBackendSet(t *testing.T) {
	server, _ := startTestServer(t)
	// No ChatBackend configured: /api/chat must not exist (404).
	if status := doJSON(t, http.MethodPost, "http://"+server.Addr()+"/api/chat", "secret-token", `{"chatId":"t1","text":"hi"}`, nil); status != http.StatusNotFound {
		t.Fatalf("expected 404 for /api/chat without backend, got %d", status)
	}
}

func TestChatBackendDrivesFullTurn(t *testing.T) {
	chat := &fakeChatBackend{}
	server, err := NewServer(&fakeBackend{}, Config{Addr: "127.0.0.1:0", Token: "secret-token", Version: "test", ChatBackend: chat})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})
	url := "http://" + server.Addr() + "/api/chat"

	var ok struct {
		ChatID             string `json:"chatId"`
		Reply              string `json:"reply"`
		AssistantMessageID string `json:"assistantMessageId"`
	}
	if status := doJSON(t, http.MethodPost, url, "secret-token", `{"chatId":"t1","text":"hello"}`, &ok); status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if chat.lastChatID != "t1" || chat.lastText != "hello" {
		t.Fatalf("backend received chatID=%q text=%q", chat.lastChatID, chat.lastText)
	}
	if ok.Reply != "reply for hello" || ok.AssistantMessageID != "m-1" {
		t.Fatalf("unexpected chat response: %+v", ok)
	}

	// Validation: missing fields -> 400.
	if status := doJSON(t, http.MethodPost, url, "secret-token", `{"text":"hi"}`, nil); status != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing chatId, got %d", status)
	}
	if status := doJSON(t, http.MethodPost, url, "secret-token", `{"chatId":"t1"}`, nil); status != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing text, got %d", status)
	}
	// Backend error -> 400 JSON error.
	var failure struct {
		Error string `json:"error"`
	}
	if status := doJSON(t, http.MethodPost, url, "secret-token", `{"chatId":"t1","text":"boom"}`, &failure); status != http.StatusBadRequest {
		t.Fatalf("expected 400 for failing chat, got %d", status)
	}
	if failure.Error != "chat failed" {
		t.Fatalf("expected chat error, got %q", failure.Error)
	}
	// Auth still enforced.
	if status := doJSON(t, http.MethodPost, url, "wrong", `{"chatId":"t1","text":"hi"}`, nil); status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without valid token, got %d", status)
	}
}

func TestIsLoopbackRemote(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:51234":   true,
		"[::1]:51234":       true,
		"192.168.1.5:51234": false,
		"not-an-address":    false,
	}
	for addr, want := range cases {
		if got := isLoopbackRemote(addr); got != want {
			t.Errorf("isLoopbackRemote(%q) = %v, want %v", addr, got, want)
		}
	}
}
