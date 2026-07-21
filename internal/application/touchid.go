package application

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

// TouchIDAccountForVaultDir returns the scoped Keychain account name for a
// vault path. Keeping this in the application layer lets interface adapters use
// the same rule without reaching into infrastructure directly.
func TouchIDAccountForVaultDir(vaultDir string) string {
	abs, err := filepath.Abs(vaultDir)
	if err != nil {
		abs = vaultDir
	}
	sum := sha256.Sum256([]byte(abs))
	return "vault-" + hex.EncodeToString(sum[:])[:24]
}
