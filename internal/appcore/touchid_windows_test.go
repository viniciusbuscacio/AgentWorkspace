//go:build windows

package appcore

import (
	"strings"
	"testing"
	"time"
)

// Quick-unlock credentials live in the real Windows Credential Manager, so the
// tests use a dedicated service name and clean up after themselves.
const testCredService = "AgentWorkspace-Vault-Test"

func TestQuickUnlockCredentialRoundtrip(t *testing.T) {
	account := "vault-roundtrip"
	t.Cleanup(func() { _ = touchIDKeychainDelete(testCredService, account) })

	if touchIDKeychainHas(testCredService, account) {
		t.Fatal("credential must not exist before enrollment")
	}
	if err := touchIDKeychainSet(testCredService, account, "senha-super"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !touchIDKeychainHas(testCredService, account) {
		t.Fatal("credential missing after enrollment")
	}
	got, err := touchIDKeychainGet(testCredService, account)
	if err != nil || got != "senha-super" {
		t.Fatalf("get = %q err=%v, want senha-super", got, err)
	}
	if err := touchIDKeychainDelete(testCredService, account); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if touchIDKeychainHas(testCredService, account) {
		t.Fatal("credential must be gone after delete")
	}
	// Deleting a missing credential is a no-op, not an error.
	if err := touchIDKeychainDelete(testCredService, account); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

func TestQuickUnlockCredentialExpiresAfterTTL(t *testing.T) {
	account := "vault-expiry"
	t.Cleanup(func() { _ = touchIDKeychainDelete(testCredService, account) })

	stale := time.Now().Add(-quickUnlockTTL - time.Hour)
	if err := touchIDKeychainSetAt(testCredService, account, "senha-velha", stale); err != nil {
		t.Fatalf("set stale: %v", err)
	}
	// Presence survives expiry (so auto-repair re-arms on the next manual
	// unlock); only Get enforces the TTL.
	if !touchIDKeychainHas(testCredService, account) {
		t.Fatal("expired credential must still report present")
	}
	if _, err := touchIDKeychainGet(testCredService, account); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("get expired = %v, want expiry error", err)
	}
	if touchIDKeychainHas(testCredService, account) {
		t.Fatal("Get must sweep the expired credential")
	}
	// Get sweeps the stale entry; re-enrolling re-arms cleanly.
	if err := touchIDKeychainSet(testCredService, account, "senha-nova"); err != nil {
		t.Fatalf("re-enroll: %v", err)
	}
	if got, err := touchIDKeychainGet(testCredService, account); err != nil || got != "senha-nova" {
		t.Fatalf("get after re-enroll = %q err=%v", got, err)
	}
}
