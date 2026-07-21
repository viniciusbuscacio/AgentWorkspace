//go:build windows

package appcore

// Windows twin of the macOS Touch ID flow (touchid_darwin.go): the vault
// password is stored in the Windows Credential Manager, scoped per vault, and
// released on a plain click — no biometric gate, by design (owner decision,
// 2026-07-21: anyone inside his Windows session already has his files, so the
// vault is not the weakest link there). The protection level is therefore the
// Windows login itself. The stored credential self-expires after 7 idle days;
// every successful manual password unlock re-saves it (touchIDAutoRepair), so
// in practice the window is rolling and only lapses after a week of no use.

import (
	"encoding/json"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// quickUnlockTTL is how long a stored credential stays valid with NO manual
// password unlock renewing it.
const quickUnlockTTL = 7 * 24 * time.Hour

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
)

var (
	advapi32        = windows.NewLazySystemDLL("advapi32.dll")
	procCredReadW   = advapi32.NewProc("CredReadW")
	procCredWriteW  = advapi32.NewProc("CredWriteW")
	procCredDeleteW = advapi32.NewProc("CredDeleteW")
	procCredFree    = advapi32.NewProc("CredFree")
)

// winCredential mirrors advapi32's CREDENTIALW.
type winCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

type quickUnlockBlob struct {
	Password    string `json:"password"`
	SavedAtUnix int64  `json:"savedAtUnix"`
}

func credTargetName(service, account string) string { return service + "/" + account }

func touchIDAvailableNative() bool { return true }

// touchIDPromptNative is a deliberate no-op: quick unlock on Windows is a
// plain click, no Windows Hello gate.
func touchIDPromptNative(_ string) error { return nil }

func touchIDKeychainSet(service, account, password string) error {
	return touchIDKeychainSetAt(service, account, password, time.Now())
}

// touchIDKeychainSetAt exists so the expiry path is testable.
func touchIDKeychainSetAt(service, account, password string, savedAt time.Time) error {
	blob, err := json.Marshal(quickUnlockBlob{Password: password, SavedAtUnix: savedAt.Unix()})
	if err != nil {
		return err
	}
	defer zeroBytes(blob)
	target, err := windows.UTF16PtrFromString(credTargetName(service, account))
	if err != nil {
		return err
	}
	user, err := windows.UTF16PtrFromString("aw")
	if err != nil {
		return err
	}
	cred := winCredential{
		Type:               credTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     &blob[0],
		Persist:            credPersistLocalMachine,
		UserName:           user,
	}
	ret, _, callErr := procCredWriteW.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if ret == 0 {
		return fmt.Errorf("credential manager write failed: %w", callErr)
	}
	return nil
}

func touchIDKeychainGet(service, account string) (string, error) {
	blob, ok, err := readCredentialBlob(credTargetName(service, account))
	if err != nil || !ok {
		if err == nil {
			err = fmt.Errorf("no stored quick-unlock credential")
		}
		return "", err
	}
	defer zeroBytes(blob)
	var parsed quickUnlockBlob
	if json.Unmarshal(blob, &parsed) != nil {
		return "", fmt.Errorf("stored quick-unlock credential is unreadable")
	}
	if time.Since(time.Unix(parsed.SavedAtUnix, 0)) > quickUnlockTTL {
		_ = touchIDKeychainDelete(service, account)
		return "", fmt.Errorf("quick unlock expired after a week without a password unlock — enter your password once to re-arm it")
	}
	return parsed.Password, nil
}

// touchIDKeychainHas reports presence WITHOUT enforcing the TTL — Get does
// that. An expired-but-present credential must still count here so (1) the
// lock screen's quick-unlock button surfaces the clear "expired, enter your
// password once" message instead of silently vanishing, and (2)
// touchIDAutoRepair re-arms it on the very next manual password unlock.
func touchIDKeychainHas(service, account string) bool {
	blob, ok, err := readCredentialBlob(credTargetName(service, account))
	if err != nil || !ok {
		return false
	}
	defer zeroBytes(blob)
	var parsed quickUnlockBlob
	return json.Unmarshal(blob, &parsed) == nil
}

func touchIDKeychainDelete(service, account string) error {
	target, err := windows.UTF16PtrFromString(credTargetName(service, account))
	if err != nil {
		return err
	}
	ret, _, callErr := procCredDeleteW.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0)
	if ret == 0 {
		if errno, isErrno := callErr.(windows.Errno); isErrno && errno == windows.ERROR_NOT_FOUND {
			return nil
		}
		return fmt.Errorf("credential manager delete failed: %w", callErr)
	}
	return nil
}

// readCredentialBlob returns (blob, found, err). A copy of the blob is made
// before CredFree so the caller owns (and can zero) the memory.
func readCredentialBlob(targetName string) ([]byte, bool, error) {
	target, err := windows.UTF16PtrFromString(targetName)
	if err != nil {
		return nil, false, err
	}
	var credPtr *winCredential
	ret, _, callErr := procCredReadW.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&credPtr)))
	if ret == 0 {
		if errno, isErrno := callErr.(windows.Errno); isErrno && errno == windows.ERROR_NOT_FOUND {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("credential manager read failed: %w", callErr)
	}
	defer func() { _, _, _ = procCredFree.Call(uintptr(unsafe.Pointer(credPtr))) }()
	if credPtr == nil || credPtr.CredentialBlobSize == 0 || credPtr.CredentialBlob == nil {
		return nil, false, nil
	}
	raw := unsafe.Slice(credPtr.CredentialBlob, credPtr.CredentialBlobSize)
	blob := make([]byte, len(raw))
	copy(blob, raw)
	return blob, true, nil
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
