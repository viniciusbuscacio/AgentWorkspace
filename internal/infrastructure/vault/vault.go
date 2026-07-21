package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aw/internal/domain"
	"aw/internal/infrastructure/securemem"
	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/vfs/adiantum" // registers encrypted SQLite VFS
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/scrypt"
)

const (
	dbName       = "vault.db"
	saltFile     = "vault.salt"
	recoveryFile = "vault.recovery"
	bcryptRounds = 10
)

var (
	errLocked           = errors.New("vault is locked")
	errInvalidPass      = errors.New("invalid password")
	errInvalidRecovery  = errors.New("invalid recovery key")
	errRecoveryNotFound = errors.New("recovery file missing")
)

type Vault struct {
	mu     sync.Mutex
	dir    string
	db     *sql.DB
	key    *securemem.SecureBytes
	lastPW *securemem.SecureBytes
	// Working-copy state (vault_workcopy.go). workDir == "" is legacy mode:
	// the live DB runs directly in dir.
	workDir          string
	stamp            *masterStamp
	masterChanged    bool
	pendingWriteBack bool
	lastWriteBack    string
}

type Status = domain.VaultStatus

type OperationResult struct {
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
	RecoveryKey string `json:"recoveryKey,omitempty"`
	Status      Status `json:"status"`
	Chats       []Chat `json:"chats,omitempty"`
}

type Chat = domain.Chat
type Attachment = domain.Attachment
type Message = domain.Message
type LLMTurn = domain.LLMTurn
type ChatTitleEntry = domain.ChatTitleEntry
type UserMemoryFact = domain.UserMemoryFact
type UserMemoryDoc = domain.UserMemoryDoc

type recoveryData struct {
	EncryptedMasterKey string `json:"encryptedMasterKey"`
	IV                 string `json:"iv"`
	AuthTag            string `json:"authTag"`
	RecoverySalt       string `json:"recoverySalt"`
}

func DefaultDir() string {
	if override := strings.TrimSpace(os.Getenv("aw_DATA_DIR")); override != "" {
		return filepath.Join(override, "AgentWorkspace")
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "aw", "AgentWorkspace")
}

func New(dir string) *Vault {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultDir()
	}
	return &Vault{dir: dir}
}

func (v *Vault) SetDir(dir string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.db != nil {
		return errors.New("cannot switch vault dir while unlocked")
	}
	if strings.TrimSpace(dir) == "" {
		return errors.New("vault dir is required")
	}
	v.dir = dir
	return nil
}

func (v *Vault) Dir() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.dir
}

func (v *Vault) Status() Status {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.statusLocked()
}

func (v *Vault) Exists() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.existsLocked()
}

func (v *Vault) IsUnlocked() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.db != nil
}

func (v *Vault) Create(password string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	password = strings.TrimSpace(password)
	if len(password) < 4 {
		return "", errors.New("use a password with at least 4 characters")
	}
	if v.existsLocked() {
		return "", errors.New("vault already exists")
	}
	if err := os.MkdirAll(v.dir, 0o700); err != nil {
		return "", err
	}
	if v.workDir != "" {
		if err := os.MkdirAll(v.workDir, 0o700); err != nil {
			return "", err
		}
		// A leftover working copy from a forgotten profile must not be
		// mistaken for the brand-new vault being created.
		v.removeWorkFilesLocked()
	}

	salt := randomHex(16)
	passwordBytes := []byte(password)
	defer securemem.Zero(passwordBytes)
	lastPW, err := securemem.NewSecureBytes(passwordBytes)
	if err != nil {
		return "", err
	}
	defer func() {
		if v.lastPW != lastPW {
			lastPW.Destroy()
		}
	}()

	keyBytes, err := deriveKey(passwordBytes, []byte(salt))
	if err != nil {
		return "", err
	}
	defer securemem.Zero(keyBytes)
	if err := os.WriteFile(v.saltPath(), []byte(salt), 0o600); err != nil {
		return "", err
	}

	db, err := openEncrypted(v.dbPath(), keyBytes)
	if err != nil {
		return "", err
	}
	v.db = db
	if err := v.setKeyLocked(keyBytes); err != nil {
		_ = v.lockLocked()
		return "", err
	}
	v.replaceLastPasswordLocked(lastPW)

	if err := v.initSchemaLocked(); err != nil {
		_ = v.lockLocked()
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword(passwordBytes, bcryptRounds)
	if err != nil {
		_ = v.lockLocked()
		return "", err
	}
	if _, err := v.db.Exec("INSERT INTO meta (key, value) VALUES (?, ?)", "password_hash", string(hash)); err != nil {
		_ = v.lockLocked()
		return "", err
	}

	recoveryKey := generateRecoveryKey()
	if err := v.saveRecoveryDataLocked(recoveryKey, keyBytes); err != nil {
		_ = v.lockLocked()
		return "", err
	}
	// Work mode: the live DB was created in the working copy; the vault only
	// exists once its cold form reaches the master.
	if v.workDir != "" {
		if err := v.snapshotLocked(v.masterDBPath()); err != nil {
			_ = v.lockLocked()
			v.removeWorkFilesLocked()
			return "", err
		}
		v.lastWriteBack = v.liveStateLocked()
		if err := v.writeStampLocked(); err != nil {
			_ = v.lockLocked()
			return "", err
		}
	}
	return recoveryKey, nil
}

func (v *Vault) Unlock(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.db != nil {
		return nil
	}
	if !v.existsLocked() {
		return errors.New("vault does not exist")
	}
	saltBytes, err := os.ReadFile(v.saltPath())
	if err != nil {
		return errors.New("vault salt file missing")
	}
	passwordBytes := []byte(password)
	defer securemem.Zero(passwordBytes)
	lastPW, err := securemem.NewSecureBytes(passwordBytes)
	if err != nil {
		return err
	}
	defer func() {
		if v.lastPW != lastPW {
			lastPW.Destroy()
		}
	}()
	keyBytes, err := deriveKey(passwordBytes, []byte(strings.TrimSpace(string(saltBytes))))
	if err != nil {
		return err
	}
	defer securemem.Zero(keyBytes)

	pendingWriteBack, err := v.hydrateLocked()
	if err != nil {
		return err
	}
	db, err := openEncrypted(v.dbPath(), keyBytes)
	if err != nil {
		return errInvalidPass
	}
	v.db = db

	var hash string
	if err := v.db.QueryRow("SELECT value FROM meta WHERE key = ?", "password_hash").Scan(&hash); err != nil {
		_ = v.lockLocked()
		return errInvalidPass
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), passwordBytes); err != nil {
		_ = v.lockLocked()
		return errInvalidPass
	}
	if err := v.initSchemaLocked(); err != nil {
		_ = v.lockLocked()
		return err
	}
	if err := v.clearWebObservationsLocked(); err != nil {
		_ = v.lockLocked()
		return err
	}
	if err := v.setKeyLocked(keyBytes); err != nil {
		_ = v.lockLocked()
		return err
	}
	v.replaceLastPasswordLocked(lastPW)
	v.masterChanged = false
	v.pendingWriteBack = pendingWriteBack
	v.lastWriteBack = v.liveStateLocked()
	return nil
}

func (v *Vault) RecoverWithKey(recoveryKey, newPassword string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 4 {
		return "", errors.New("use a password with at least 4 characters")
	}
	if !v.existsLocked() {
		return "", errors.New("vault does not exist")
	}
	if err := v.lockLocked(); err != nil {
		return "", err
	}

	oldKey, err := v.decryptRecoveryMasterKeyLocked(recoveryKey)
	if err != nil {
		return "", err
	}
	defer securemem.Zero(oldKey)
	// Recovery runs from the locked state, so the MASTER is the source of
	// truth — any working copy is stale by definition and gets re-hydrated
	// from the recovered master at the end.
	oldDB, err := openEncrypted(v.masterDBPath(), oldKey)
	if err != nil {
		return "", errInvalidRecovery
	}
	if err := initSchema(oldDB); err != nil {
		_ = oldDB.Close()
		return "", errInvalidRecovery
	}

	newSalt := randomHex(16)
	newPasswordBytes := []byte(newPassword)
	defer securemem.Zero(newPasswordBytes)
	newLastPW, err := securemem.NewSecureBytes(newPasswordBytes)
	if err != nil {
		return "", err
	}
	defer func() {
		if v.lastPW != newLastPW {
			newLastPW.Destroy()
		}
	}()
	newKey, err := deriveKey(newPasswordBytes, []byte(newSalt))
	if err != nil {
		return "", err
	}
	defer securemem.Zero(newKey)

	tempID := randomHex(6)
	tempDBPath := filepath.Join(v.dir, ".vault-recover-"+tempID+".db")
	tempSaltPath := filepath.Join(v.dir, ".vault-recover-"+tempID+".salt")
	tempRecoveryPath := filepath.Join(v.dir, ".vault-recover-"+tempID+".recovery")
	defer cleanupSQLiteFiles(tempDBPath)
	defer os.Remove(tempSaltPath)
	defer os.Remove(tempRecoveryPath)

	newDB, err := openEncrypted(tempDBPath, newKey)
	if err != nil {
		return "", err
	}
	if err := copyVaultData(oldDB, newDB, newPassword); err != nil {
		_ = newDB.Close()
		return "", err
	}

	newRecoveryKey := generateRecoveryKey()
	recoveryBytes, err := encodeRecoveryData(newRecoveryKey, newKey)
	if err != nil {
		_ = newDB.Close()
		return "", err
	}
	if err := os.WriteFile(tempSaltPath, []byte(newSalt), 0o600); err != nil {
		_ = newDB.Close()
		return "", err
	}
	if err := os.WriteFile(tempRecoveryPath, recoveryBytes, 0o600); err != nil {
		_ = newDB.Close()
		return "", err
	}

	_, _ = oldDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	_, _ = newDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err := oldDB.Close(); err != nil {
		_ = newDB.Close()
		return "", err
	}
	if err := newDB.Close(); err != nil {
		return "", err
	}
	removeSQLiteSidecars(v.masterDBPath())
	removeSQLiteSidecars(tempDBPath)

	if err := swapRecoveredVaultFiles(v.masterDBPath(), v.saltPath(), filepath.Join(v.dir, recoveryFile), tempDBPath, tempSaltPath, tempRecoveryPath); err != nil {
		return "", err
	}

	// Adopt the recovered master: in work mode the stale stamp guarantees
	// hydration discards the old working copy and copies the new master in.
	v.stamp = nil
	if v.workDir != "" {
		_ = os.Remove(v.stampPath())
	}
	if _, err := v.hydrateLocked(); err != nil {
		return "", err
	}
	db, err := openEncrypted(v.dbPath(), newKey)
	if err != nil {
		return "", err
	}
	v.db = db
	if err := v.initSchemaLocked(); err != nil {
		_ = v.lockLocked()
		return "", err
	}
	if err := v.setKeyLocked(newKey); err != nil {
		_ = v.lockLocked()
		return "", err
	}
	v.replaceLastPasswordLocked(newLastPW)
	v.masterChanged = false
	v.pendingWriteBack = false
	v.lastWriteBack = v.liveStateLocked()
	return newRecoveryKey, nil
}

func (v *Vault) ChangePassword(currentPassword, newPassword string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	currentPassword = strings.TrimSpace(currentPassword)
	newPassword = strings.TrimSpace(newPassword)
	if v.db == nil {
		return "", errLocked
	}
	if len(newPassword) < 4 {
		return "", errors.New("use a password with at least 4 characters")
	}

	var hash string
	if err := v.db.QueryRow("SELECT value FROM meta WHERE key = ?", "password_hash").Scan(&hash); err != nil {
		return "", errInvalidPass
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)); err != nil {
		return "", errors.New("invalid current password")
	}

	oldKey, err := v.unwrapKeyLocked()
	if err != nil {
		return "", err
	}
	defer securemem.Zero(oldKey)
	newSalt := randomHex(16)
	newPasswordBytes := []byte(newPassword)
	defer securemem.Zero(newPasswordBytes)
	newLastPW, err := securemem.NewSecureBytes(newPasswordBytes)
	if err != nil {
		return "", err
	}
	defer func() {
		if v.lastPW != newLastPW {
			newLastPW.Destroy()
		}
	}()
	newKey, err := deriveKey(newPasswordBytes, []byte(newSalt))
	if err != nil {
		return "", err
	}
	defer securemem.Zero(newKey)

	tempID := randomHex(6)
	// The re-encrypted DB is a live SQLite file while it is being filled — it
	// must be born where the live DB runs (the working copy in work mode),
	// never inside a possibly-synced master folder. Salt/recovery are atomic
	// small files and stay master-side.
	tempDBPath := filepath.Join(filepath.Dir(v.dbPath()), ".vault-change-"+tempID+".db")
	tempSaltPath := filepath.Join(v.dir, ".vault-change-"+tempID+".salt")
	tempRecoveryPath := filepath.Join(v.dir, ".vault-change-"+tempID+".recovery")
	defer cleanupSQLiteFiles(tempDBPath)
	defer os.Remove(tempSaltPath)
	defer os.Remove(tempRecoveryPath)

	newDB, err := openEncrypted(tempDBPath, newKey)
	if err != nil {
		return "", err
	}
	if err := copyVaultData(v.db, newDB, newPassword); err != nil {
		_ = newDB.Close()
		return "", err
	}

	newRecoveryKey := generateRecoveryKey()
	recoveryBytes, err := encodeRecoveryData(newRecoveryKey, newKey)
	if err != nil {
		_ = newDB.Close()
		return "", err
	}
	if err := os.WriteFile(tempSaltPath, []byte(newSalt), 0o600); err != nil {
		_ = newDB.Close()
		return "", err
	}
	if err := os.WriteFile(tempRecoveryPath, recoveryBytes, 0o600); err != nil {
		_ = newDB.Close()
		return "", err
	}

	oldDB := v.db
	v.db = nil
	v.destroyKeyLocked()
	_, _ = oldDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	_, _ = newDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err := oldDB.Close(); err != nil {
		_ = newDB.Close()
		return "", err
	}
	if err := newDB.Close(); err != nil {
		return "", err
	}
	removeSQLiteSidecars(v.dbPath())
	removeSQLiteSidecars(tempDBPath)

	if err := swapRecoveredVaultFiles(v.dbPath(), v.saltPath(), filepath.Join(v.dir, recoveryFile), tempDBPath, tempSaltPath, tempRecoveryPath); err != nil {
		v.reopenLocked(oldKey)
		return "", err
	}

	db, err := openEncrypted(v.dbPath(), newKey)
	if err != nil {
		return "", err
	}
	v.db = db
	if err := v.setKeyLocked(newKey); err != nil {
		_ = v.lockLocked()
		return "", err
	}
	v.replaceLastPasswordLocked(newLastPW)
	if err := v.initSchemaLocked(); err != nil {
		_ = v.lockLocked()
		return "", err
	}
	// Work mode: the master's salt/recovery were just swapped to the NEW key
	// while its vault.db still carries the OLD one — snapshot immediately so
	// the master is never left key-inconsistent longer than necessary. On
	// failure the tick keeps retrying (pendingWriteBack).
	if v.workDir != "" {
		if err := v.snapshotLocked(v.masterDBPath()); err != nil {
			v.pendingWriteBack = true
		} else {
			v.lastWriteBack = v.liveStateLocked()
			v.pendingWriteBack = v.writeStampLocked() != nil
		}
	}
	return newRecoveryKey, nil
}

func (v *Vault) GenerateRecoveryKey() (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.db == nil || v.key == nil {
		return "", errLocked
	}
	recoveryKey := generateRecoveryKey()
	key, err := v.unwrapKeyLocked()
	if err != nil {
		return "", err
	}
	defer securemem.Zero(key)
	if err := v.saveRecoveryDataLocked(recoveryKey, key); err != nil {
		return "", err
	}
	return recoveryKey, nil
}

func (v *Vault) HasRecoveryKey() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return fileExists(filepath.Join(v.dir, recoveryFile))
}

func (v *Vault) VerifyRecoveryKey(recoveryKey string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, err := v.decryptRecoveryMasterKeyLocked(recoveryKey)
	return err == nil
}

func (v *Vault) Lock() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Work mode: final synchronous snapshot before closing (Q8), so the master
	// is fresh the moment the app can be closed or the machine swapped. Master
	// manda: if the master changed elsewhere, local changes are discarded, not
	// written over it. A failed snapshot keeps the working copy + stamp on
	// disk — the pending-snapshot rule lets the next unlock write it back if
	// the master is still ours.
	clean := true
	if v.db != nil && v.workDir != "" {
		switch {
		case v.masterChanged || !v.masterMatchesStampLocked():
			// discard local; nothing to write
		case v.pendingWriteBack || v.liveChangedSinceLocked():
			if err := v.snapshotLocked(v.masterDBPath()); err != nil {
				clean = false
			} else {
				v.lastWriteBack = v.liveStateLocked()
				if err := v.writeStampLocked(); err != nil {
					clean = false
				}
			}
		}
	}
	err := v.lockLocked()
	if v.workDir != "" && clean {
		v.removeWorkFilesLocked()
	}
	v.masterChanged = false
	v.pendingWriteBack = !clean
	return err
}

// VerifyPassword checks password against the vault's stored bcrypt hash without
// changing lock state. It requires the vault to be unlocked (the hash lives in
// the encrypted meta table). Web mode uses it to authenticate a session against
// an already-unlocked vault — Unlock returns nil when already open and so would
// accept any password, which VerifyPassword must not.
func (v *Vault) VerifyPassword(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	var hash string
	if err := v.db.QueryRow("SELECT value FROM meta WHERE key = ?", "password_hash").Scan(&hash); err != nil {
		return errInvalidPass
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return errInvalidPass
	}
	return nil
}

func (v *Vault) ListChats() ([]Chat, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}

	rows, err := v.db.Query(`SELECT id, title, archived, created_at, updated_at FROM sessions ORDER BY archived ASC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := make([]Chat, 0)
	for rows.Next() {
		var chat Chat
		var archived int
		if err := rows.Scan(&chat.ID, &chat.Title, &archived, &chat.CreatedAt, &chat.UpdatedAt); err != nil {
			return nil, err
		}
		chat.Archived = archived != 0
		chats = append(chats, chat)
	}
	return chats, rows.Err()
}

func (v *Vault) SetSecret(name, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("secret name is required")
	}
	_, err := v.db.Exec(
		`INSERT INTO secrets (id, name, value) VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET value = excluded.value`,
		"secret-"+randomHex(8),
		name,
		value,
	)
	return err
}

func (v *Vault) GetSecret(name string) (string, bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return "", false, errLocked
	}
	value, exists, err := v.getSecretBytesLocked(name)
	if err != nil || !exists {
		return "", exists, err
	}
	defer securemem.Zero(value)
	return string(value), true, nil
}

func (v *Vault) GetSecretSecure(name string) (*securemem.SecureBytes, bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, false, errLocked
	}
	value, exists, err := v.getSecretBytesLocked(name)
	if err != nil || !exists {
		return nil, exists, err
	}
	defer securemem.Zero(value)
	secureValue, err := securemem.NewSecureBytes(value)
	if err != nil {
		return nil, false, err
	}
	return secureValue, true, nil
}

func (v *Vault) HasSecret(name string) (bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return false, errLocked
	}
	var one int
	if err := v.db.QueryRow(`SELECT 1 FROM secrets WHERE name = ? LIMIT 1`, name).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (v *Vault) ListSecrets() ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	rows, err := v.db.Query(`SELECT name FROM secrets ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (v *Vault) DeleteSecret(name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	_, err := v.db.Exec(`DELETE FROM secrets WHERE name = ?`, name)
	return err
}

func (v *Vault) InsertLog(entry domain.LogEntry) (domain.LogEntry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.LogEntry{}, errLocked
	}
	if entry.ID == "" {
		entry.ID = "log-" + randomHex(12)
	}
	if entry.Timestamp == "" {
		entry.Timestamp = nowString()
	}
	if entry.CreatedAt == "" {
		entry.CreatedAt = entry.Timestamp
	}
	_, err := v.db.Exec(
		`INSERT INTO logs (id, timestamp, level, level_name, source, module_id, module_type, session_id, message, context_json, error_name, error_message, error_stack, created_at,
		                   event, severity, trace_id, span_id, parent_span_id, duration_ms, status, attributes_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.Timestamp, entry.Level, entry.LevelName, entry.Source, entry.ModuleID, entry.ModuleType,
		entry.SessionID, entry.Message, entry.ContextJSON, entry.ErrorName, entry.ErrorMessage, entry.ErrorStack, entry.CreatedAt,
		entry.Event, entry.Severity, entry.TraceID, entry.SpanID, entry.ParentSpanID, entry.DurationMs, entry.Status, entry.AttributesJSON,
	)
	if err != nil {
		return domain.LogEntry{}, err
	}
	return entry, nil
}

func (v *Vault) ListLogs(query domain.LogQuery) ([]domain.LogEntry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	if query.Limit <= 0 {
		query.Limit = 50
	}
	clauses := []string{"1=1"}
	args := []any{}
	if query.Date != "" {
		clauses = append(clauses, "substr(timestamp, 1, 10) = ?")
		args = append(args, query.Date)
	}
	if query.Level != nil {
		clauses = append(clauses, "level = ?")
		args = append(args, *query.Level)
	}
	if query.Source != "" && query.Source != "all" {
		clauses = append(clauses, "source = ?")
		args = append(args, query.Source)
	}
	if query.EventPrefix != "" {
		clauses = append(clauses, "event LIKE ?")
		args = append(args, query.EventPrefix+"%")
	}
	if query.TraceID != "" {
		clauses = append(clauses, "trace_id = ?")
		args = append(args, query.TraceID)
	}
	if query.Status != "" && query.Status != "all" {
		clauses = append(clauses, "status = ?")
		args = append(args, query.Status)
	}
	if query.ModuleID != "" {
		clauses = append(clauses, "module_id = ?")
		args = append(args, query.ModuleID)
	}
	if query.SessionID != "" {
		clauses = append(clauses, "session_id = ?")
		args = append(args, query.SessionID)
	}
	if query.Risk != "" && query.Risk != "all" {
		clauses = append(clauses, "attributes_json LIKE ?")
		args = append(args, `%"risk":"`+query.Risk+`"%`)
	}
	if query.Search != "" {
		clauses = append(clauses, "(message LIKE ? OR source LIKE ? OR level_name LIKE ? OR error_message LIKE ? OR event LIKE ? OR status LIKE ? OR attributes_json LIKE ?)")
		like := "%" + query.Search + "%"
		args = append(args, like, like, like, like, like, like, like)
	}
	args = append(args, query.Limit, query.Offset)
	rows, err := v.db.Query(
		`SELECT id, timestamp, level, level_name, source, COALESCE(module_id,''), COALESCE(module_type,''), COALESCE(session_id,''),
		        message, COALESCE(context_json,''), COALESCE(error_name,''), COALESCE(error_message,''), COALESCE(error_stack,''), created_at,
		        COALESCE(event,''), COALESCE(severity,''), COALESCE(trace_id,''), COALESCE(span_id,''), COALESCE(parent_span_id,''), COALESCE(duration_ms,0), COALESCE(status,''), COALESCE(attributes_json,'')
		   FROM logs WHERE `+strings.Join(clauses, " AND ")+`
		   ORDER BY timestamp DESC LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]domain.LogEntry, 0)
	for rows.Next() {
		var entry domain.LogEntry
		if err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.Level, &entry.LevelName, &entry.Source, &entry.ModuleID, &entry.ModuleType,
			&entry.SessionID, &entry.Message, &entry.ContextJSON, &entry.ErrorName, &entry.ErrorMessage, &entry.ErrorStack, &entry.CreatedAt,
			&entry.Event, &entry.Severity, &entry.TraceID, &entry.SpanID, &entry.ParentSpanID, &entry.DurationMs, &entry.Status, &entry.AttributesJSON); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (v *Vault) ListLogDates() ([]domain.LogDateCount, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	rows, err := v.db.Query(`SELECT COALESCE(date(timestamp), substr(timestamp, 1, 10)) AS day, COUNT(*) FROM logs GROUP BY day ORDER BY day DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.LogDateCount, 0)
	for rows.Next() {
		var item domain.LogDateCount
		if err := rows.Scan(&item.Date, &item.Entries); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (v *Vault) DeleteLogsOlderThan(days int) (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return 0, errLocked
	}
	cutoff := domain.LogRetentionCutoff(days)
	cutoffDay := cutoff
	if len(cutoffDay) > 10 {
		cutoffDay = cutoffDay[:10]
	}
	result, err := v.db.Exec(`DELETE FROM logs WHERE COALESCE(date(timestamp), substr(timestamp, 1, 10)) < ?`, cutoffDay)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (v *Vault) DeleteAllLogs() (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return 0, errLocked
	}
	result, err := v.db.Exec(`DELETE FROM logs`)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (v *Vault) getSecretBytesLocked(name string) ([]byte, bool, error) {
	rows, err := v.db.Query(`SELECT value FROM secrets WHERE name = ?`, name)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	}
	var value []byte
	if err := rows.Scan(&value); err != nil {
		return nil, false, err
	}
	if err := rows.Err(); err != nil {
		securemem.Zero(value)
		return nil, false, err
	}
	return value, true, nil
}

// UpsertUserFact inserts or updates (by key) one long-term fact about the
// user, stamping updated_at. Returns the stored fact.
func (v *Vault) UpsertUserFact(fact UserMemoryFact) (UserMemoryFact, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return UserMemoryFact{}, errLocked
	}
	fact.UpdatedAt = nowString()
	_, err := v.db.Exec(
		`INSERT INTO user_memory (key, category, content, source, updated_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
			category = excluded.category,
			content = excluded.content,
			source = excluded.source,
			updated_at = excluded.updated_at`,
		fact.Key, fact.Category, fact.Content, fact.Source, fact.UpdatedAt,
	)
	if err != nil {
		return UserMemoryFact{}, err
	}
	return fact, nil
}

// ListUserFacts returns every stored user fact, grouped by category and newest
// first within each category.
func (v *Vault) ListUserFacts() ([]UserMemoryFact, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	rows, err := v.db.Query(
		`SELECT key, category, content, source, updated_at FROM user_memory
		 ORDER BY category ASC, updated_at DESC, key ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	facts := make([]UserMemoryFact, 0)
	for rows.Next() {
		var fact UserMemoryFact
		if err := rows.Scan(&fact.Key, &fact.Category, &fact.Content, &fact.Source, &fact.UpdatedAt); err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}

// DeleteUserFact removes one user fact by key. Deleting a missing key is a
// no-op.
func (v *Vault) DeleteUserFact(key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	_, err := v.db.Exec(`DELETE FROM user_memory WHERE key = ?`, key)
	return err
}

// GetUserMemoryDoc returns the v2 user-memory living document. Returns an
// empty doc when no row exists yet (fresh vault or not yet migrated).
func (v *Vault) GetUserMemoryDoc() (domain.UserMemoryDoc, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.UserMemoryDoc{}, errLocked
	}
	var doc domain.UserMemoryDoc
	err := v.db.QueryRow(
		`SELECT content, backup, last_condensed_at, updated_at FROM user_memory_doc WHERE id = 1`,
	).Scan(&doc.Content, &doc.Backup, &doc.LastCondensedAt, &doc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.UserMemoryDoc{}, nil
	}
	return doc, err
}

// SetUserMemoryDoc atomically stores the full user-memory document (content,
// backup, last_condensed_at). Updated_at is stamped by the vault.
func (v *Vault) SetUserMemoryDoc(doc domain.UserMemoryDoc) (domain.UserMemoryDoc, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.UserMemoryDoc{}, errLocked
	}
	doc.UpdatedAt = nowString()
	_, err := v.db.Exec(
		`INSERT OR REPLACE INTO user_memory_doc (id, content, backup, last_condensed_at, updated_at) VALUES (1, ?, ?, ?, ?)`,
		doc.Content, doc.Backup, doc.LastCondensedAt, doc.UpdatedAt,
	)
	if err != nil {
		return domain.UserMemoryDoc{}, err
	}
	return doc, nil
}

// migrateUserMemoryFactsLocked migrates v1 fact rows to the v2 living
// document once. Idempotent: skipped if the doc row already has content or
// if there are no facts to migrate. Must be called with v.mu held.
func (v *Vault) migrateUserMemoryFactsLocked() {
	// Skip if doc already has content.
	var existing string
	_ = v.db.QueryRow(`SELECT content FROM user_memory_doc WHERE id = 1`).Scan(&existing)
	if strings.TrimSpace(existing) != "" {
		return
	}
	// Collect old fact rows.
	rows, err := v.db.Query(`SELECT category, key, content FROM user_memory ORDER BY category ASC, key ASC`)
	if err != nil {
		return
	}
	var lines []string
	for rows.Next() {
		var cat, key, content string
		if rows.Scan(&cat, &key, &content) == nil && strings.TrimSpace(content) != "" {
			lines = append(lines, fmt.Sprintf("- [%s] %s: %s", cat, key, strings.TrimSpace(content)))
		}
	}
	_ = rows.Close()
	if len(lines) == 0 {
		return
	}
	now := nowString()
	doc := strings.Join(lines, "\n")
	_, _ = v.db.Exec(
		`INSERT OR REPLACE INTO user_memory_doc (id, content, backup, last_condensed_at, updated_at) VALUES (1, ?, '', '', ?)`,
		doc, now,
	)
	_, _ = v.db.Exec(`DELETE FROM user_memory`)
}

func (v *Vault) CreateChat(title string) (Chat, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return Chat{}, errLocked
	}

	if strings.TrimSpace(title) == "" {
		title = "New Chat"
	}
	now := nowString()
	chat := Chat{
		ID:        "chat-" + randomHex(6),
		Title:     title,
		Archived:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := v.db.Exec(
		`INSERT INTO sessions (id, title, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		chat.ID, chat.Title, 0, chat.CreatedAt, chat.UpdatedAt,
	)
	return chat, err
}

func (v *Vault) ListMessages(sessionID string) ([]Message, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}

	rows, err := v.db.Query(
		// rowid ASC breaks created_at ties by insertion order for a stable thread.
		`SELECT id, session_id, role, content, created_at, attachments_json FROM messages WHERE session_id = ? ORDER BY created_at ASC, rowid ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]Message, 0)
	for rows.Next() {
		var message Message
		var attachmentsJSON sql.NullString
		if err := rows.Scan(&message.ID, &message.SessionID, &message.Role, &message.Content, &message.CreatedAt, &attachmentsJSON); err != nil {
			return nil, err
		}
		message.Attachments = parseAttachments(attachmentsJSON)
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (v *Vault) AddMessage(sessionID, role, content string) (Message, error) {
	return v.AddMessageWithAttachments(sessionID, role, content, nil)
}

func (v *Vault) AddMessageWithAttachments(sessionID, role, content string, attachments []Attachment) (Message, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return Message{}, errLocked
	}
	sessionID = strings.TrimSpace(sessionID)
	role = strings.TrimSpace(role)
	if sessionID == "" {
		return Message{}, errors.New("chat id is required")
	}
	if role == "" {
		return Message{}, errors.New("message role is required")
	}

	now := nowString()
	message := Message{
		ID:          "msg-" + randomHex(8),
		SessionID:   sessionID,
		Role:        role,
		Content:     content,
		CreatedAt:   now,
		Attachments: normalizeAttachments(attachments),
	}
	attachmentsJSON, err := encodeAttachments(message.Attachments)
	if err != nil {
		return Message{}, err
	}
	tx, err := v.db.Begin()
	if err != nil {
		return Message{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`INSERT INTO messages (id, session_id, role, content, created_at, attachments_json) VALUES (?, ?, ?, ?, ?, ?)`,
		message.ID, message.SessionID, message.Role, message.Content, message.CreatedAt, attachmentsJSON,
	); err != nil {
		return Message{}, err
	}
	if _, err := tx.Exec(
		`INSERT INTO messages_fts (content, message_id, session_id, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		message.Content, message.ID, message.SessionID, message.Role, message.CreatedAt,
	); err != nil {
		return Message{}, err
	}
	if _, err := tx.Exec(`UPDATE sessions SET updated_at = ? WHERE id = ?`, now, sessionID); err != nil {
		return Message{}, err
	}
	if err := tx.Commit(); err != nil {
		return Message{}, err
	}
	return message, nil
}

func (v *Vault) InsertLLMTurn(turn LLMTurn) (LLMTurn, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return LLMTurn{}, errLocked
	}
	turn.SessionID = strings.TrimSpace(turn.SessionID)
	turn.RequestJSON = strings.TrimSpace(turn.RequestJSON)
	if turn.SessionID == "" {
		return LLMTurn{}, errors.New("chat id is required")
	}
	if turn.RequestJSON == "" {
		return LLMTurn{}, errors.New("request json is required")
	}
	if strings.TrimSpace(turn.ID) == "" {
		turn.ID = "llm-turn-" + randomHex(8)
	}
	if strings.TrimSpace(turn.CreatedAt) == "" {
		turn.CreatedAt = nowString()
	}
	if strings.TrimSpace(turn.ToolCallsJSON) == "" {
		turn.ToolCallsJSON = "[]"
	}
	if turn.TurnIndex <= 0 {
		if err := v.db.QueryRow(
			`SELECT COALESCE(MAX(turn_index), 0) + 1 FROM llm_turns WHERE session_id = ?`,
			turn.SessionID,
		).Scan(&turn.TurnIndex); err != nil {
			return LLMTurn{}, err
		}
	}
	_, err := v.db.Exec(
		`INSERT INTO llm_turns (
			id, session_id, turn_index, request_json, response_text, tool_calls_json,
			model, prompt_tokens, completion_tokens, finish_reason, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		turn.ID,
		turn.SessionID,
		turn.TurnIndex,
		turn.RequestJSON,
		turn.ResponseText,
		turn.ToolCallsJSON,
		turn.Model,
		turn.PromptTokens,
		turn.CompletionTokens,
		turn.FinishReason,
		turn.CreatedAt,
	)
	if err != nil {
		return LLMTurn{}, err
	}
	return turn, nil
}

func (v *Vault) ListLLMTurns(sessionID string) ([]LLMTurn, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("chat id is required")
	}
	rows, err := v.db.Query(
		`SELECT id, session_id, turn_index, request_json, response_text, tool_calls_json,
		        model, prompt_tokens, completion_tokens, finish_reason, created_at
		   FROM llm_turns
		  WHERE session_id = ?
		  ORDER BY turn_index ASC, created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	turns := make([]LLMTurn, 0)
	for rows.Next() {
		var turn LLMTurn
		if err := rows.Scan(
			&turn.ID,
			&turn.SessionID,
			&turn.TurnIndex,
			&turn.RequestJSON,
			&turn.ResponseText,
			&turn.ToolCallsJSON,
			&turn.Model,
			&turn.PromptTokens,
			&turn.CompletionTokens,
			&turn.FinishReason,
			&turn.CreatedAt,
		); err != nil {
			return nil, err
		}
		turns = append(turns, turn)
	}
	return turns, rows.Err()
}

func (v *Vault) InsertChatTitle(entry ChatTitleEntry) (ChatTitleEntry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return ChatTitleEntry{}, errLocked
	}
	entry.SessionID = strings.TrimSpace(entry.SessionID)
	entry.Title = strings.TrimSpace(entry.Title)
	entry.Source = strings.TrimSpace(entry.Source)
	if entry.SessionID == "" {
		return ChatTitleEntry{}, errors.New("chat id is required")
	}
	if entry.Title == "" {
		return ChatTitleEntry{}, errors.New("title is required")
	}
	if entry.Source == "" {
		entry.Source = "auto"
	}
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = "chat-title-" + randomHex(8)
	}
	if strings.TrimSpace(entry.CreatedAt) == "" {
		entry.CreatedAt = nowString()
	}
	_, err := v.db.Exec(
		`INSERT INTO chat_titles (id, session_id, title, turn, source, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		entry.ID,
		entry.SessionID,
		entry.Title,
		entry.Turn,
		entry.Source,
		entry.CreatedAt,
	)
	if err != nil {
		return ChatTitleEntry{}, err
	}
	return entry, nil
}

func (v *Vault) ListChatTitles(sessionID string) ([]ChatTitleEntry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("chat id is required")
	}
	rows, err := v.db.Query(
		`SELECT id, session_id, title, turn, source, created_at
		   FROM chat_titles
		  WHERE session_id = ?
		  ORDER BY created_at ASC, rowid ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]ChatTitleEntry, 0)
	for rows.Next() {
		var entry ChatTitleEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.SessionID,
			&entry.Title,
			&entry.Turn,
			&entry.Source,
			&entry.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (v *Vault) SetSessionSummary(sessionID string, summary string, turn int) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("chat id is required")
	}
	_, err := v.db.Exec(
		`UPDATE sessions SET summary = ?, summary_turn = ? WHERE id = ?`,
		strings.TrimSpace(summary),
		turn,
		sessionID,
	)
	return err
}

// SetChatModelOverride persists a per-chat provider/model override on the
// session row (the previously-unused provider/model columns). Empty values
// clear the override so the chat falls back to the global active provider.
func (v *Vault) SetChatModelOverride(sessionID, provider, model string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("chat id is required")
	}
	_, err := v.db.Exec(
		`UPDATE sessions SET provider = ?, model = ? WHERE id = ?`,
		strings.TrimSpace(provider),
		strings.TrimSpace(model),
		sessionID,
	)
	return err
}

// ChatModelOverride returns the per-chat provider/model override, or empty
// strings when the chat uses the global active provider.
func (v *Vault) ChatModelOverride(sessionID string) (string, string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return "", "", errLocked
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", "", errors.New("chat id is required")
	}
	var provider, model sql.NullString
	if err := v.db.QueryRow(
		`SELECT provider, model FROM sessions WHERE id = ?`,
		sessionID,
	).Scan(&provider, &model); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", nil
		}
		return "", "", err
	}
	return strings.TrimSpace(provider.String), strings.TrimSpace(model.String), nil
}

func (v *Vault) SessionSummary(sessionID string) (string, int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return "", 0, errLocked
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", 0, errors.New("chat id is required")
	}
	var (
		summary sql.NullString
		turn    sql.NullInt64
	)
	if err := v.db.QueryRow(
		`SELECT summary, summary_turn FROM sessions WHERE id = ?`,
		sessionID,
	).Scan(&summary, &turn); err != nil {
		return "", 0, err
	}
	return summary.String, int(turn.Int64), nil
}

// SearchMessages runs a lexical FTS5 search across all chat messages, returning
// the best-ranked hits as domain messages (newest-ranked first). The query is
// tokenized and quoted so arbitrary user text never breaks FTS5 syntax.
func (v *Vault) SearchMessages(query string, limit int) ([]Message, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	match := buildFTSMatchQuery(query)
	if match == "" {
		return []Message{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := v.db.Query(
		`SELECT message_id, session_id, role, content, created_at
		   FROM messages_fts
		  WHERE messages_fts MATCH ?
		  ORDER BY rank
		  LIMIT ?`,
		match,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := make([]Message, 0)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		hits = append(hits, m)
	}
	return hits, rows.Err()
}

// buildFTSMatchQuery turns free-form text into a safe FTS5 MATCH expression by
// quoting each token and OR-ing them for better recall. Returns "" when there
// is nothing searchable.
func buildFTSMatchQuery(raw string) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ReplaceAll(f, `"`, `""`)
		if strings.TrimSpace(f) == "" {
			continue
		}
		quoted = append(quoted, `"`+f+`"`)
	}
	return strings.Join(quoted, " OR ")
}

func (v *Vault) ReplaceMessages(sessionID string, messages []Message) ([]Message, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("chat id is required")
	}
	now := time.Now().UTC()
	tx, err := v.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM messages_fts WHERE session_id = ?`, sessionID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID); err != nil {
		return nil, err
	}

	inserted := make([]Message, 0, len(messages))
	for i, input := range messages {
		role := strings.TrimSpace(input.Role)
		if role == "" {
			role = "assistant"
		}
		message := Message{
			ID:          "msg-" + randomHex(8),
			SessionID:   sessionID,
			Role:        role,
			Content:     input.Content,
			CreatedAt:   now.Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano),
			Attachments: normalizeAttachments(input.Attachments),
		}
		attachmentsJSON, err := encodeAttachments(message.Attachments)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(
			`INSERT INTO messages (id, session_id, role, content, created_at, attachments_json) VALUES (?, ?, ?, ?, ?, ?)`,
			message.ID, message.SessionID, message.Role, message.Content, message.CreatedAt, attachmentsJSON,
		); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(
			`INSERT INTO messages_fts (content, message_id, session_id, role, created_at) VALUES (?, ?, ?, ?, ?)`,
			message.Content, message.ID, message.SessionID, message.Role, message.CreatedAt,
		); err != nil {
			return nil, err
		}
		inserted = append(inserted, message)
	}

	if _, err := tx.Exec(`UPDATE sessions SET updated_at = ? WHERE id = ?`, nowString(), sessionID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return inserted, nil
}

func (v *Vault) ClearChat(sessionID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("chat id is required")
	}
	now := nowString()
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM messages_fts WHERE session_id = ?`, sessionID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE sessions SET updated_at = ? WHERE id = ?`, now, sessionID); err != nil {
		return err
	}
	return tx.Commit()
}

func (v *Vault) RenameChat(sessionID, title string) (Chat, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return Chat{}, errLocked
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New Chat"
	}
	now := nowString()
	if _, err := v.db.Exec(`UPDATE sessions SET title = ?, updated_at = ? WHERE id = ?`, title, now, sessionID); err != nil {
		return Chat{}, err
	}
	return v.getChatLocked(sessionID)
}

func (v *Vault) SetChatArchived(sessionID string, archived bool) (Chat, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return Chat{}, errLocked
	}
	now := nowString()
	archivedValue := 0
	if archived {
		archivedValue = 1
	}
	if _, err := v.db.Exec(`UPDATE sessions SET archived = ?, updated_at = ? WHERE id = ?`, archivedValue, now, sessionID); err != nil {
		return Chat{}, err
	}
	return v.getChatLocked(sessionID)
}

func (v *Vault) DeleteChat(sessionID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM messages_fts WHERE session_id = ?`, sessionID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID); err != nil {
		return err
	}
	return tx.Commit()
}

func (v *Vault) CountMessages(sessionID string) (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return 0, errLocked
	}
	var count int
	if err := v.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id = ?`, sessionID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (v *Vault) RecentMessages(sessionID string, limit int) ([]Message, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := v.db.Query(
		// rowid DESC breaks created_at ties by insertion order, so the latest
		// message is deterministic even when two messages share a timestamp.
		`SELECT id, session_id, role, content, created_at, attachments_json
		 FROM messages
		 WHERE session_id = ?
		 ORDER BY created_at DESC, rowid DESC
		 LIMIT ?`,
		sessionID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reversed := make([]Message, 0)
	for rows.Next() {
		var message Message
		var attachmentsJSON sql.NullString
		if err := rows.Scan(&message.ID, &message.SessionID, &message.Role, &message.Content, &message.CreatedAt, &attachmentsJSON); err != nil {
			return nil, err
		}
		message.Attachments = parseAttachments(attachmentsJSON)
		reversed = append(reversed, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed, nil
}

func (v *Vault) getChatLocked(sessionID string) (Chat, error) {
	var chat Chat
	var archived int
	if err := v.db.QueryRow(
		`SELECT id, title, archived, created_at, updated_at FROM sessions WHERE id = ?`,
		sessionID,
	).Scan(&chat.ID, &chat.Title, &archived, &chat.CreatedAt, &chat.UpdatedAt); err != nil {
		return Chat{}, err
	}
	chat.Archived = archived != 0
	return chat, nil
}

func (v *Vault) initSchemaLocked() error {
	if err := initSchema(v.db); err != nil {
		return err
	}
	v.migrateUserMemoryFactsLocked() // idempotent, best-effort
	return nil
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS secrets (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			value TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT 'New Chat',
			source TEXT NOT NULL DEFAULT 'desktop',
			model TEXT,
			archived INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			attachments_json TEXT,
			created_at TEXT NOT NULL,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);
		CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
			content,
			message_id UNINDEXED,
			session_id UNINDEXED,
			role UNINDEXED,
			created_at UNINDEXED
		);
		CREATE TABLE IF NOT EXISTS llm_turns (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			turn_index INTEGER NOT NULL,
			request_json TEXT NOT NULL,
			response_text TEXT,
			tool_calls_json TEXT,
			model TEXT,
			prompt_tokens INTEGER,
			completion_tokens INTEGER,
			finish_reason TEXT,
			created_at TEXT NOT NULL,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_llm_turns_session ON llm_turns(session_id);
		CREATE TABLE IF NOT EXISTS chat_titles (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			title TEXT NOT NULL,
			turn INTEGER,
			source TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_chat_titles_session ON chat_titles(session_id);
		CREATE TABLE IF NOT EXISTS notes (
			id         TEXT PRIMARY KEY,
			title      TEXT NOT NULL,
			content    TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			pinned     INTEGER NOT NULL DEFAULT 0,
			archived   INTEGER NOT NULL DEFAULT 0,
			in_prompt  INTEGER NOT NULL DEFAULT 1
		);
		CREATE TABLE IF NOT EXISTS backlog_items (
			id       TEXT PRIMARY KEY,
			title    TEXT NOT NULL,
			status   TEXT NOT NULL,
			position INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS backlog_attachments (
			id         TEXT PRIMARY KEY,
			item_id    TEXT NOT NULL,
			name       TEXT NOT NULL,
			mime_type  TEXT NOT NULL,
			size       INTEGER NOT NULL,
			data       BLOB NOT NULL,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS user_memory (
			key        TEXT PRIMARY KEY,
			category   TEXT NOT NULL,
			content    TEXT NOT NULL,
			source     TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS user_memory_doc (
			id               INTEGER PRIMARY KEY CHECK (id = 1),
			content          TEXT NOT NULL DEFAULT '',
			backup           TEXT NOT NULL DEFAULT '',
			last_condensed_at TEXT NOT NULL DEFAULT '',
			updated_at       TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS logs (
			id TEXT PRIMARY KEY,
			timestamp TEXT NOT NULL,
			level INTEGER NOT NULL,
			level_name TEXT NOT NULL,
			source TEXT NOT NULL,
			module_id TEXT,
			module_type TEXT,
			session_id TEXT,
			message TEXT NOT NULL,
			context_json TEXT,
			error_name TEXT,
			error_message TEXT,
			error_stack TEXT,
			created_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
		CREATE INDEX IF NOT EXISTS idx_logs_level ON logs(level);
		CREATE INDEX IF NOT EXISTS idx_logs_source ON logs(source);
		CREATE TABLE IF NOT EXISTS web_observations (
			id TEXT PRIMARY KEY,
			chat_id TEXT NOT NULL,
			run_id TEXT,
			browser TEXT NOT NULL,
			tab_id TEXT,
			site_key TEXT NOT NULL,
			url TEXT,
			title TEXT,
			captured_at TEXT NOT NULL,
			payload_json TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_web_observations_lookup
			ON web_observations(chat_id, browser, site_key, captured_at DESC);
		CREATE TABLE IF NOT EXISTS skills (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL,
			description  TEXT NOT NULL DEFAULT '',
			enabled      INTEGER NOT NULL DEFAULT 1,
			origin       TEXT NOT NULL DEFAULT 'user',
			seed_version TEXT NOT NULL DEFAULT '',
			deleted_at   TEXT,
			created_at   TEXT NOT NULL DEFAULT '',
			updated_at   TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS skill_files (
			skill_id     TEXT NOT NULL,
			path         TEXT NOT NULL,
			content      TEXT NOT NULL DEFAULT '',
			content_hash TEXT NOT NULL DEFAULT '',
			seed_hash    TEXT,
			updated_at   TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (skill_id, path)
		);
		CREATE TABLE IF NOT EXISTS app_documents (
			id           TEXT PRIMARY KEY,
			content      TEXT NOT NULL DEFAULT '',
			content_hash TEXT NOT NULL DEFAULT '',
			seed_hash    TEXT,
			origin       TEXT NOT NULL DEFAULT 'user',
			seed_version TEXT NOT NULL DEFAULT '',
			updated_at   TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS mcp_connections (
			id              TEXT PRIMARY KEY,
			name            TEXT NOT NULL,
			enabled         INTEGER NOT NULL DEFAULT 0,
			transport       TEXT NOT NULL,
			url             TEXT NOT NULL DEFAULT '',
			auth_type       TEXT NOT NULL DEFAULT 'none',
			secret_key      TEXT NOT NULL DEFAULT '',
			created_at      TEXT NOT NULL,
			updated_at      TEXT NOT NULL,
			last_checked_at TEXT NOT NULL DEFAULT '',
			last_status     TEXT NOT NULL DEFAULT 'unknown',
			last_error      TEXT NOT NULL DEFAULT '',
			tool_count      INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_mcp_connections_enabled ON mcp_connections(enabled, updated_at);
	`)
	if err != nil {
		return err
	}
	if err := ensureColumn(db, "sessions", "source", "TEXT NOT NULL DEFAULT 'desktop'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "sessions", "model", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "sessions", "provider", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "sessions", "archived", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, "sessions", "summary", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "sessions", "summary_turn", "INTEGER"); err != nil {
		return err
	}
	if err := ensureColumn(db, "messages", "attachments_json", "TEXT"); err != nil {
		return err
	}
	// Observability v2 (OTel-lite) log columns. Additive: ALTER'd in on existing
	// vaults; new rows populate them, old rows read as empty via COALESCE.
	for _, col := range [][2]string{
		{"event", "TEXT"},
		{"severity", "TEXT"},
		{"trace_id", "TEXT"},
		{"span_id", "TEXT"},
		{"parent_span_id", "TEXT"},
		{"duration_ms", "INTEGER"},
		{"status", "TEXT"},
		{"attributes_json", "TEXT"},
	} {
		if err := ensureColumn(db, "logs", col[0], col[1]); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_logs_event ON logs(event)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_logs_trace ON logs(trace_id)`); err != nil {
		return err
	}
	// Notes pin/archive flags (AW2 parity). Additive only: on a vault created
	// before the columns existed, the guards ALTER TABLE them in with their
	// defaults; the index comes after so the columns are guaranteed to exist.
	if err := ensureColumn(db, "notes", "pinned", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, "notes", "archived", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	// in_prompt flag: notes with this on are injected into the agent context
	// as a notes block. Default 1 so existing notes opt in automatically.
	if err := ensureColumn(db, "notes", "in_prompt", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_notes_visible ON notes(archived, pinned, updated_at)`); err != nil {
		return err
	}
	// Tasks parity (AW2): body column, created_at, and the one-time status
	// remap todo->open / done->completed. Additive only — the columns are
	// ALTER'd in on pre-parity vaults, then the legacy values are migrated
	// idempotently (re-running matches nothing once remapped).
	if err := ensureColumn(db, "backlog_items", "body", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "backlog_items", "created_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE backlog_items SET status = ? WHERE status = ?`,
		domain.TasksStatusOpen, domain.TasksStatusLegacyTodo); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE backlog_items SET status = ? WHERE status = ?`,
		domain.TasksStatusCompleted, domain.TasksStatusLegacyDone); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_backlog_attachments_item ON backlog_attachments(item_id)`); err != nil {
		return err
	}
	return removeLegacyDemoChats(db)
}

func removeLegacyDemoChats(db *sql.DB) error {
	// Early aw builds accidentally seeded every newly-created vault with four
	// demo sessions. New vaults must start blank. For vaults already affected,
	// remove only the exact seeded sessions if they are still empty, so real user
	// chats are never touched.
	demos := []struct {
		id    string
		title string
	}{
		{"chat-123", "Chat 123"},
		{"chat-4455", "Chat 4455"},
		{"chat-antigo", "Chat antigo"},
		{"chat-teste", "Chat teste"},
	}
	for _, demo := range demos {
		if _, err := db.Exec(
			`DELETE FROM sessions
			  WHERE id = ?
			    AND title = ?
			    AND NOT EXISTS (SELECT 1 FROM messages WHERE messages.session_id = sessions.id)`,
			demo.id, demo.title,
		); err != nil {
			return err
		}
	}
	return nil
}

func (v *Vault) saveRecoveryDataLocked(recoveryKey string, masterKey []byte) error {
	encoded, err := encodeRecoveryData(recoveryKey, masterKey)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(v.dir, recoveryFile), encoded, 0o600)
}

func (v *Vault) decryptRecoveryMasterKeyLocked(recoveryKey string) ([]byte, error) {
	path := filepath.Join(v.dir, recoveryFile)
	if !fileExists(path) {
		return nil, errRecoveryNotFound
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data recoveryData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, errInvalidRecovery
	}
	recoveryDerivedKey, err := deriveRecoveryKey(recoveryKey, data.RecoverySalt)
	if err != nil {
		return nil, errInvalidRecovery
	}
	defer securemem.Zero(recoveryDerivedKey)
	block, err := aes.NewCipher(recoveryDerivedKey)
	if err != nil {
		return nil, errInvalidRecovery
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errInvalidRecovery
	}
	iv, err := hex.DecodeString(data.IV)
	if err != nil {
		return nil, errInvalidRecovery
	}
	ciphertext, err := hex.DecodeString(data.EncryptedMasterKey)
	if err != nil {
		return nil, errInvalidRecovery
	}
	tag, err := hex.DecodeString(data.AuthTag)
	if err != nil {
		return nil, errInvalidRecovery
	}
	ciphertextWithTag := make([]byte, 0, len(ciphertext)+len(tag))
	ciphertextWithTag = append(ciphertextWithTag, ciphertext...)
	ciphertextWithTag = append(ciphertextWithTag, tag...)
	defer securemem.Zero(ciphertextWithTag)
	plain, err := gcm.Open(nil, iv, ciphertextWithTag, nil)
	if err != nil {
		return nil, errInvalidRecovery
	}
	defer securemem.Zero(plain)
	if len(plain) != 64 {
		return nil, errInvalidRecovery
	}
	masterKey := make([]byte, 32)
	if _, err := hex.Decode(masterKey, plain); err != nil {
		securemem.Zero(masterKey)
		return nil, errInvalidRecovery
	}
	return masterKey, nil
}

func encodeRecoveryData(recoveryKey string, masterKey []byte) ([]byte, error) {
	recoverySalt := randomHex(16)
	recoveryDerivedKey, err := deriveRecoveryKey(recoveryKey, recoverySalt)
	if err != nil {
		return nil, err
	}
	defer securemem.Zero(recoveryDerivedKey)
	block, err := aes.NewCipher(recoveryDerivedKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	iv := randomBytes(gcm.NonceSize())
	masterKeyHex := make([]byte, hex.EncodedLen(len(masterKey)))
	hex.Encode(masterKeyHex, masterKey)
	defer securemem.Zero(masterKeyHex)
	ciphertextWithTag := gcm.Seal(nil, iv, masterKeyHex, nil)
	tagStart := len(ciphertextWithTag) - gcm.Overhead()
	data := recoveryData{
		EncryptedMasterKey: hex.EncodeToString(ciphertextWithTag[:tagStart]),
		IV:                 hex.EncodeToString(iv),
		AuthTag:            hex.EncodeToString(ciphertextWithTag[tagStart:]),
		RecoverySalt:       recoverySalt,
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

// copyVaultData migrates every row of the old vault into a freshly keyed new
// vault (used by ChangePassword and RecoverWithKey). It enumerates the schema
// dynamically instead of listing tables by hand, so a table added to initSchema
// later can never again be silently dropped on a password change or recovery —
// the bug that previously discarded notes, tasks, memory, logs and more.
func copyVaultData(oldDB, newDB *sql.DB, newPassword string) error {
	if err := initSchema(newDB); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptRounds)
	if err != nil {
		return err
	}

	tables, err := copyableTables(oldDB)
	if err != nil {
		return err
	}

	// Disable foreign keys for the bulk copy so cross-table insert order is
	// irrelevant; the PRAGMA must be set outside a transaction, and the single
	// pooled connection (MaxOpenConns=1) makes it stick for the transaction
	// that follows. Restored on the way out.
	if _, err := newDB.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer func() { _, _ = newDB.Exec(`PRAGMA foreign_keys = ON`) }()

	tx, err := newDB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range tables {
		if err := copyTable(oldDB, tx, table); err != nil {
			return fmt.Errorf("copy table %s: %w", table, err)
		}
	}
	// messages_fts is an fts5 virtual table (skipped by copyableTables); rebuild
	// its index from the messages rows just copied.
	if err := rebuildMessagesFTS(tx); err != nil {
		return err
	}
	// Overwrite the migrated password hash with one derived from the new
	// password (the generic copy carried the old hash over from meta).
	if _, err := tx.Exec(
		`INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		"password_hash",
		string(hash),
	); err != nil {
		return err
	}
	return tx.Commit()
}

// copyableTables lists the real, copyable tables in creation order, excluding
// SQLite internal tables (sqlite_*) and fts5 virtual tables together with their
// shadow tables (those are rebuilt from their content, not copied row-by-row).
func copyableTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT name, sql FROM sqlite_master WHERE type = 'table' ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type tableRow struct{ name, sql string }
	var all []tableRow
	var virtual []string
	for rows.Next() {
		var name string
		var createSQL sql.NullString
		if err := rows.Scan(&name, &createSQL); err != nil {
			return nil, err
		}
		all = append(all, tableRow{name: name, sql: createSQL.String})
		if strings.Contains(strings.ToUpper(createSQL.String), "CREATE VIRTUAL TABLE") {
			virtual = append(virtual, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	isVirtualOrShadow := func(name string) bool {
		for _, v := range virtual {
			if name == v || strings.HasPrefix(name, v+"_") {
				return true
			}
		}
		return false
	}

	var names []string
	for _, r := range all {
		if strings.HasPrefix(r.name, "sqlite_") || isVirtualOrShadow(r.name) {
			continue
		}
		names = append(names, r.name)
	}
	return names, nil
}

// copyTable copies every row of one table from src into the new vault's tx,
// reading the live column list so any schema shape is preserved.
func copyTable(src *sql.DB, tx *sql.Tx, table string) error {
	cols, err := tableColumns(src, table)
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		return nil
	}
	quoted := quoteIdents(cols)
	colList := strings.Join(quoted, ", ")
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	insertSQL := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, quoteIdent(table), colList, placeholders)

	rows, err := src.Query(fmt.Sprintf(`SELECT %s FROM %s`, colList, quoteIdent(table)))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		if _, err := tx.Exec(insertSQL, values...); err != nil {
			return err
		}
	}
	return rows.Err()
}

func tableColumns(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, quoteIdent(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

func quoteIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

func quoteIdents(idents []string) []string {
	out := make([]string, len(idents))
	for i, id := range idents {
		out[i] = quoteIdent(id)
	}
	return out
}

// rebuildMessagesFTS repopulates the messages_fts index from the messages table
// inside the migration transaction. messages_fts is a standalone fts5 table (no
// external content), so it is not copied by copyTable and must be reindexed.
func rebuildMessagesFTS(tx *sql.Tx) error {
	if _, err := tx.Exec(`DELETE FROM messages_fts`); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO messages_fts (content, message_id, session_id, role, created_at)
		SELECT content, id, session_id, role, created_at FROM messages`)
	return err
}

func ensureColumn(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, definition))
	return err
}

func (v *Vault) setKeyLocked(key []byte) error {
	secureKey, err := securemem.NewSecureBytes(key)
	if err != nil {
		return err
	}
	v.destroyKeyLocked()
	v.key = secureKey
	return nil
}

func (v *Vault) unwrapKeyLocked() ([]byte, error) {
	if v.key == nil {
		return nil, errLocked
	}
	key, err := v.key.Unwrap()
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (v *Vault) destroyKeyLocked() {
	if v.key != nil {
		v.key.Destroy()
		v.key = nil
	}
}

func (v *Vault) replaceLastPasswordLocked(password *securemem.SecureBytes) {
	v.destroyLastPasswordLocked()
	v.lastPW = password
}

func (v *Vault) destroyLastPasswordLocked() {
	if v.lastPW != nil {
		v.lastPW.Destroy()
		v.lastPW = nil
	}
}

func normalizeAttachments(attachments []Attachment) []Attachment {
	normalized := make([]Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		name := strings.TrimSpace(attachment.Name)
		kind := strings.TrimSpace(attachment.Type)
		dataURI := strings.TrimSpace(attachment.DataURI)
		if name == "" && kind == "" && dataURI == "" {
			continue
		}
		normalized = append(normalized, Attachment{
			Name:    name,
			Type:    kind,
			DataURI: dataURI,
		})
	}
	return normalized
}

func encodeAttachments(attachments []Attachment) (any, error) {
	attachments = normalizeAttachments(attachments)
	if len(attachments) == 0 {
		return nil, nil
	}
	bytes, err := json.Marshal(attachments)
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}

func parseAttachments(value sql.NullString) []Attachment {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	var attachments []Attachment
	if err := json.Unmarshal([]byte(value.String), &attachments); err != nil {
		return nil
	}
	return normalizeAttachments(attachments)
}

func swapRecoveredVaultFiles(dbPath, saltPath, recoveryPath, tempDBPath, tempSaltPath, tempRecoveryPath string) error {
	backupID := randomHex(6)
	type filePair struct {
		original string
		temp     string
		backup   string
	}
	files := []filePair{
		{original: dbPath, temp: tempDBPath, backup: dbPath + ".backup-" + backupID},
		{original: saltPath, temp: tempSaltPath, backup: saltPath + ".backup-" + backupID},
		{original: recoveryPath, temp: tempRecoveryPath, backup: recoveryPath + ".backup-" + backupID},
	}

	rollback := func() {
		for _, file := range files {
			_ = os.Remove(file.original)
			if fileExists(file.backup) {
				_ = os.Rename(file.backup, file.original)
			}
		}
	}

	for _, file := range files {
		_ = os.Remove(file.backup)
		if fileExists(file.original) {
			if err := os.Rename(file.original, file.backup); err != nil {
				rollback()
				return err
			}
		}
	}
	for _, file := range files {
		if err := os.Rename(file.temp, file.original); err != nil {
			rollback()
			return err
		}
	}
	for _, file := range files {
		_ = os.Remove(file.backup)
	}
	return nil
}

func (v *Vault) lockLocked() error {
	v.destroyKeyLocked()
	v.destroyLastPasswordLocked()
	if v.db == nil {
		return nil
	}
	db := v.db
	v.db = nil
	_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return db.Close()
}

func (v *Vault) reopenLocked(key []byte) {
	if len(key) == 0 {
		return
	}
	db, err := openEncrypted(v.dbPath(), key)
	if err != nil {
		return
	}
	v.db = db
	_ = v.setKeyLocked(key)
	_ = v.initSchemaLocked()
}

func (v *Vault) statusLocked() Status {
	return Status{
		Exists:      v.existsLocked(),
		Unlocked:    v.db != nil,
		VaultDir:    v.dir,
		HasRecovery: fileExists(filepath.Join(v.dir, recoveryFile)),
	}
}

// existsLocked asks about the MASTER: the durable artifact defines whether
// the vault exists, never the ephemeral working copy.
func (v *Vault) existsLocked() bool {
	return fileExists(v.masterDBPath())
}

// dbPath is where the LIVE database runs: the working copy when configured,
// the master folder otherwise (legacy mode).
func (v *Vault) dbPath() string {
	if v.workDir != "" {
		return filepath.Join(v.workDir, dbName)
	}
	return filepath.Join(v.dir, dbName)
}

func (v *Vault) saltPath() string {
	return filepath.Join(v.dir, saltFile)
}

func openEncrypted(dbPath string, key []byte) (*sql.DB, error) {
	uriPath := strings.ReplaceAll(filepath.ToSlash(dbPath), " ", "%20")
	dsn := "file:" + uriPath + "?vfs=adiantum&_txlock=immediate"
	keyHex := make([]byte, hex.EncodedLen(len(key)))
	hex.Encode(keyHex, key)
	defer securemem.Zero(keyHex)

	db, err := driver.Open(dsn, func(conn *sqlite3.Conn) error {
		// SQLite's PRAGMA API requires a string; keep it scoped to connection
		// setup and zero the source bytes immediately after Open returns.
		return conn.Exec(fmt.Sprintf(`
			PRAGMA hexkey='%s';
			PRAGMA busy_timeout = 5000;
			PRAGMA temp_store = MEMORY;
			PRAGMA foreign_keys = ON;
			PRAGMA journal_mode = WAL;
		`, string(keyHex)))
	})
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func deriveKey(password, salt []byte) ([]byte, error) {
	key, err := scrypt.Key(password, salt, 1<<14, 8, 1, 32)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func deriveRecoveryKey(recoveryKey, salt string) ([]byte, error) {
	material := normalizeRecoveryKey(recoveryKey)
	if material == "" {
		return nil, errInvalidRecovery
	}
	materialBytes := []byte(material)
	defer securemem.Zero(materialBytes)
	return scrypt.Key(materialBytes, []byte(salt), 1<<14, 8, 1, 32)
}

func normalizeRecoveryKey(recoveryKey string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '-', ' ', '\t', '\n', '\r':
			return -1
		default:
			return r
		}
	}, strings.TrimSpace(recoveryKey))
}

func randomBytes(n int) []byte {
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		panic(err)
	}
	return buf
}

func randomHex(n int) string {
	return hex.EncodeToString(randomBytes(n))
}

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func generateRecoveryKey() string {
	bytes := randomBytes(32)
	groups := make([]string, 0, 6)
	for group := 0; group < 6; group++ {
		var b strings.Builder
		for c := 0; c < 4; c++ {
			idx := group*4 + c
			b.WriteByte(base62[int(bytes[idx])%len(base62)])
		}
		groups = append(groups, b.String())
	}
	return strings.Join(groups, "-")
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func removeSQLiteSidecars(path string) {
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
}

func cleanupSQLiteFiles(path string) {
	_ = os.Remove(path)
	removeSQLiteSidecars(path)
}
