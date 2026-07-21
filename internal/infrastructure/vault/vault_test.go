package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultCreateLockUnlockAndEncryptsFiles(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)

	recoveryKey, err := v.Create("senha1234")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if recoveryKey == "" {
		t.Fatal("Create() recovery key is empty")
	}

	chat, err := v.CreateChat("Test chat")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	const plaintext = "mensagem super secreta do aw"
	if _, err := v.AddMessage(chat.ID, "user", plaintext); err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	if err := v.Unlock("senha errada"); err == nil {
		t.Fatal("Unlock() with wrong password succeeded")
	}
	if err := v.Unlock("senha1234"); err != nil {
		t.Fatalf("Unlock() with correct password error = %v", err)
	}

	messages, err := v.ListMessages(chat.ID)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	found := false
	for _, message := range messages {
		if message.Content == plaintext {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("stored message %q not found", plaintext)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("final Lock() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "vault.db") {
			continue
		}
		bytes, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", entry.Name(), err)
		}
		if strings.Contains(string(bytes), plaintext) {
			t.Fatalf("%s contains plaintext message", entry.Name())
		}
	}
}

func TestVaultCreateStartsWithNoChats(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	chats, err := v.ListChats()
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}
	if len(chats) != 0 {
		t.Fatalf("new vault has %d seeded chats, want 0: %+v", len(chats), chats)
	}
}

func TestVaultRecoverWithKeyResetsPasswordAndKeepsData(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)

	recoveryKey, err := v.Create("senha-antiga")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	chat, err := v.CreateChat("Recover test chat")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	const plaintext = "conteudo preservado depois do recover"
	if _, err := v.AddMessage(chat.ID, "user", plaintext); err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	if _, err := v.RecoverWithKey("recovery-invalida", "senha-nova"); err == nil {
		t.Fatal("RecoverWithKey() with invalid recovery key succeeded")
	}

	newRecoveryKey, err := v.RecoverWithKey(recoveryKey, "senha-nova")
	if err != nil {
		t.Fatalf("RecoverWithKey() error = %v", err)
	}
	if newRecoveryKey == "" || newRecoveryKey == recoveryKey {
		t.Fatalf("RecoverWithKey() returned suspicious recovery key: %q", newRecoveryKey)
	}

	messages, err := v.ListMessages(chat.ID)
	if err != nil {
		t.Fatalf("ListMessages() after recover error = %v", err)
	}
	found := false
	for _, message := range messages {
		if message.Content == plaintext {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("stored message %q not found after recover", plaintext)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() after recover error = %v", err)
	}
	if err := v.Unlock("senha-antiga"); err == nil {
		t.Fatal("Unlock() with old password succeeded after recover")
	}
	if err := v.Unlock("senha-nova"); err != nil {
		t.Fatalf("Unlock() with new password error = %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("final Lock() after recover error = %v", err)
	}
}

func TestVaultChangePasswordSecretsAndRecoveryKey(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)

	initialRecoveryKey, err := v.Create("senha-atual")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !v.VerifyRecoveryKey(initialRecoveryKey) {
		t.Fatal("initial recovery key did not verify")
	}

	const secretName = "openrouter_api_key"
	const secretValue = "sk-aw-super-secreto"
	if err := v.SetSecret(secretName, secretValue); err != nil {
		t.Fatalf("SetSecret() error = %v", err)
	}
	if exists, err := v.HasSecret(secretName); err != nil || !exists {
		t.Fatalf("HasSecret() = %v, %v; want true, nil", exists, err)
	}
	chat, err := v.CreateChat("Change password test chat")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	const plaintext = "mensagem preservada no change password"
	if _, err := v.AddMessage(chat.ID, "user", plaintext); err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}

	if _, err := v.ChangePassword("senha-errada", "senha-nova"); err == nil {
		t.Fatal("ChangePassword() with wrong current password succeeded")
	}

	newRecoveryKey, err := v.ChangePassword("senha-atual", "senha-nova")
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if newRecoveryKey == "" || newRecoveryKey == initialRecoveryKey {
		t.Fatalf("ChangePassword() returned suspicious recovery key: %q", newRecoveryKey)
	}
	if !v.VerifyRecoveryKey(newRecoveryKey) {
		t.Fatal("new recovery key did not verify")
	}
	if v.VerifyRecoveryKey(initialRecoveryKey) {
		t.Fatal("old recovery key still verifies after password change")
	}

	value, exists, err := v.GetSecret(secretName)
	if err != nil || !exists || value != secretValue {
		t.Fatalf("GetSecret() = %q, %v, %v; want %q, true, nil", value, exists, err, secretValue)
	}
	names, err := v.ListSecrets()
	if err != nil {
		t.Fatalf("ListSecrets() error = %v", err)
	}
	foundSecret := false
	for _, name := range names {
		if name == secretName {
			foundSecret = true
			break
		}
	}
	if !foundSecret {
		t.Fatalf("secret %q not listed in %v", secretName, names)
	}

	messages, err := v.ListMessages(chat.ID)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	foundMessage := false
	for _, message := range messages {
		if message.Content == plaintext {
			foundMessage = true
			break
		}
	}
	if !foundMessage {
		t.Fatalf("stored message %q not found after password change", plaintext)
	}

	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if err := v.Unlock("senha-atual"); err == nil {
		t.Fatal("Unlock() with old password succeeded after password change")
	}
	if err := v.Unlock("senha-nova"); err != nil {
		t.Fatalf("Unlock() with new password error = %v", err)
	}
	if err := v.DeleteSecret(secretName); err != nil {
		t.Fatalf("DeleteSecret() error = %v", err)
	}
	if exists, err := v.HasSecret(secretName); err != nil || exists {
		t.Fatalf("HasSecret() after delete = %v, %v; want false, nil", exists, err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("final Lock() error = %v", err)
	}

	assertVaultFilesDoNotContain(t, dir, plaintext)
	assertVaultFilesDoNotContain(t, dir, secretValue)
}

func TestVaultLockDestroysSealedKeyAndPassword(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)

	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if v.key == nil || v.lastPW == nil {
		t.Fatal("Create() did not seal key and password")
	}
	keySealed := v.key.SealedForTest()
	passwordSealed := v.lastPW.SealedForTest()
	if keySealed == nil || passwordSealed == nil {
		t.Fatal("sealed key/password references are nil before Lock()")
	}

	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if v.key != nil || v.lastPW != nil {
		t.Fatal("Lock() left sealed key/password referenced")
	}
	if !allZero(keySealed.IV) || !allZero(keySealed.Ciphertext) || !allZero(keySealed.Tag) {
		t.Fatal("Lock() did not zero sealed key buffers")
	}
	if !allZero(passwordSealed.IV) || !allZero(passwordSealed.Ciphertext) || !allZero(passwordSealed.Tag) {
		t.Fatal("Lock() did not zero sealed password buffers")
	}

	if err := v.Unlock("senha1234"); err != nil {
		t.Fatalf("Unlock() after zeroizing lock error = %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("final Lock() error = %v", err)
	}
}

func TestVaultGetSecretSecureWrapsAndZeroesPlaintext(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()

	if err := v.SetSecret("api-key", "sk-aw-secure"); err != nil {
		t.Fatalf("SetSecret() error = %v", err)
	}
	secureValue, exists, err := v.GetSecretSecure("api-key")
	if err != nil || !exists {
		t.Fatalf("GetSecretSecure() exists=%v err=%v, want true nil", exists, err)
	}

	var plainRef []byte
	if err := secureValue.Use(func(plain []byte) error {
		plainRef = plain
		if string(plain) != "sk-aw-secure" {
			t.Fatalf("secure plaintext = %q, want sk-aw-secure", plain)
		}
		return nil
	}); err != nil {
		t.Fatalf("secure Use() error = %v", err)
	}
	if !allZero(plainRef) {
		t.Fatal("secure Use() did not zero transient plaintext")
	}

	sealed := secureValue.SealedForTest()
	secureValue.Destroy()
	if !allZero(sealed.IV) || !allZero(sealed.Ciphertext) || !allZero(sealed.Tag) {
		t.Fatal("secure Destroy() did not zero sealed secret buffers")
	}

	missing, exists, err := v.GetSecretSecure("missing")
	if err != nil || exists || missing != nil {
		t.Fatalf("GetSecretSecure(missing) = %+v, %v, %v; want nil false nil", missing, exists, err)
	}
}

func TestVaultGenerateRecoveryKey(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)

	oldRecoveryKey, err := v.Create("senha1234")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	newRecoveryKey, err := v.GenerateRecoveryKey()
	if err != nil {
		t.Fatalf("GenerateRecoveryKey() error = %v", err)
	}
	if newRecoveryKey == "" || newRecoveryKey == oldRecoveryKey {
		t.Fatalf("GenerateRecoveryKey() returned suspicious key: %q", newRecoveryKey)
	}
	if !v.VerifyRecoveryKey(newRecoveryKey) {
		t.Fatal("new recovery key did not verify")
	}
	if v.VerifyRecoveryKey(oldRecoveryKey) {
		t.Fatal("old recovery key still verifies after regeneration")
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if _, err := v.GenerateRecoveryKey(); err == nil {
		t.Fatal("GenerateRecoveryKey() while locked succeeded")
	}
}

func assertVaultFilesDoNotContain(t *testing.T, dir string, plaintext string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "vault.db") {
			continue
		}
		bytes, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", entry.Name(), err)
		}
		if strings.Contains(string(bytes), plaintext) {
			t.Fatalf("%s contains plaintext %q", entry.Name(), plaintext)
		}
	}
}

func allZero(buf []byte) bool {
	for _, b := range buf {
		if b != 0 {
			return false
		}
	}
	return true
}

func TestDeleteChatRemovesMessagesFromDatabase(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	chat, err := v.CreateChat("Doomed chat")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := v.AddMessage(chat.ID, "user", "mensagem para deletar"); err != nil {
			t.Fatalf("AddMessage() error = %v", err)
		}
	}
	count, err := v.CountMessages(chat.ID)
	if err != nil {
		t.Fatalf("CountMessages() error = %v", err)
	}
	if count != 3 {
		t.Fatalf("CountMessages() = %d, want 3 before delete", count)
	}

	if err := v.DeleteChat(chat.ID); err != nil {
		t.Fatalf("DeleteChat() error = %v", err)
	}

	count, err = v.CountMessages(chat.ID)
	if err != nil {
		t.Fatalf("CountMessages() after delete error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountMessages() = %d, want 0 after delete (orphan messages left behind)", count)
	}

	chats, err := v.ListChats()
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}
	for _, c := range chats {
		if c.ID == chat.ID {
			t.Fatalf("deleted chat %s still present in ListChats()", chat.ID)
		}
	}
}
