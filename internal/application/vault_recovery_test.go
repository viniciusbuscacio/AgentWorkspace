package application

import "testing"

type fakeRecoveryVault struct {
	recoveredWithRecoveryKey string
	recoveredWithPassword    string
	changedFromPassword      string
	changedToPassword        string
	generated                bool
	valid                    bool
}

func (vault *fakeRecoveryVault) RecoverWithKey(recoveryKey string, newPassword string) (string, error) {
	vault.recoveredWithRecoveryKey = recoveryKey
	vault.recoveredWithPassword = newPassword
	return "new-recovery", nil
}

func (vault *fakeRecoveryVault) ChangePassword(currentPassword string, newPassword string) (string, error) {
	vault.changedFromPassword = currentPassword
	vault.changedToPassword = newPassword
	return "changed-recovery", nil
}

func (vault *fakeRecoveryVault) GenerateRecoveryKey() (string, error) {
	vault.generated = true
	return "generated-recovery", nil
}

func (vault *fakeRecoveryVault) VerifyRecoveryKey(string) bool {
	return vault.valid
}

func TestVaultRecoveryUseCasesCallStore(t *testing.T) {
	vault := &fakeRecoveryVault{valid: true}
	key, err := RecoverVault(vault, "old", "new-pass")
	if err != nil || key != "new-recovery" || vault.recoveredWithRecoveryKey != "old" || vault.recoveredWithPassword != "new-pass" {
		t.Fatalf("RecoverVault() = %q, %v, vault=%+v", key, err, vault)
	}
	key, err = ChangeVaultPassword(vault, "old-pass", "new-pass")
	if err != nil || key != "changed-recovery" || vault.changedFromPassword != "old-pass" || vault.changedToPassword != "new-pass" {
		t.Fatalf("ChangeVaultPassword() = %q, %v, vault=%+v", key, err, vault)
	}
	key, err = GenerateVaultRecoveryKey(vault)
	if err != nil || key != "generated-recovery" || !vault.generated {
		t.Fatalf("GenerateVaultRecoveryKey() = %q, %v, generated=%v", key, err, vault.generated)
	}
	valid, err := VerifyVaultRecoveryKey(vault, "key")
	if err != nil || !valid {
		t.Fatalf("VerifyVaultRecoveryKey() = %v, %v; want true", valid, err)
	}
}
