package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestHarnessStartsRESTWithUnlockedDevVault(t *testing.T) {
	server, err := Start(context.Background(), Config{
		DataDir: t.TempDir(),
		Addr:    "127.0.0.1:0",
		Token:   "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(context.Background())

	actionsURL := strings.TrimSuffix(server.REST.URL(), "/aw") + "/actions"
	actions := getJSON(t, actionsURL, server.Token)
	rawActions, err := json.Marshal(actions)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawActions), "sandbox.status") {
		t.Fatalf("actions missing sandbox.status: %s", rawActions)
	}

	status := postAW(t, server.REST.URL(), server.Token, "sandbox.status", nil)
	rawStatus, _ := json.Marshal(status)
	if !strings.Contains(string(rawStatus), "Workspace folder") {
		t.Fatalf("sandbox.status response = %s", rawStatus)
	}

	logs := postAW(t, server.REST.URL(), server.Token, "logs.list", map[string]any{"limit": 5})
	rawLogs, _ := json.Marshal(logs)
	if !strings.Contains(string(rawLogs), "tool.call") {
		t.Fatalf("logs.list response did not include tool logs: %s", rawLogs)
	}
}

func TestHarnessChatModeRequiresVaultPassword(t *testing.T) {
	// Chat mode unlocks the REAL vault; with no password it must fail clearly
	// instead of silently falling back to the dev vault.
	_, err := Start(context.Background(), Config{
		DataDir:       t.TempDir(),
		Addr:          "127.0.0.1:0",
		Token:         "test-token",
		Chat:          true,
		VaultPassword: "",
	})
	if err == nil {
		t.Fatal("expected chat mode to fail without a vault password")
	}
	if !strings.Contains(err.Error(), "vault password") {
		t.Fatalf("expected a clear vault-password error, got: %v", err)
	}
}

func TestChatBackendCreatesStableAliasForMissingChatID(t *testing.T) {
	server, err := Start(context.Background(), Config{
		DataDir: t.TempDir(),
		Addr:    "127.0.0.1:0",
		Token:   "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(context.Background())

	backend := chatBackend{vault: server.vault, aliases: &chatAliasStore{aliases: map[string]string{}}}
	first, err := backend.resolveChatID("t1")
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first == "t1" {
		t.Fatalf("resolved chat id = %q, want created vault chat id", first)
	}
	second, err := backend.resolveChatID("t1")
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("alias resolved to %q, want %q", second, first)
	}
}

func getJSON(t *testing.T, url string, token string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %s", url, resp.Status)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func postAW(t *testing.T, url string, token string, action string, args map[string]any) map[string]any {
	t.Helper()
	body, err := json.Marshal(map[string]any{"action": action, "args": args})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s action=%s status = %s", url, action, resp.Status)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}
