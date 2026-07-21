package ports

import "aw/internal/domain"

type VaultLifecycle interface {
	Status() domain.VaultStatus
	Create(password string) (string, error)
	Unlock(password string) error
	Lock() error
}

type VaultUnlockState interface {
	IsUnlocked() bool
}

type VaultDirectoryStore interface {
	Dir() string
	SetDir(dir string) error
}

type VaultStatusReader interface {
	Status() domain.VaultStatus
	Dir() string
}

type VaultRecoveryStore interface {
	RecoverWithKey(recoveryKey string, newPassword string) (string, error)
	ChangePassword(currentPassword string, newPassword string) (string, error)
	GenerateRecoveryKey() (string, error)
	VerifyRecoveryKey(recoveryKey string) bool
}

type VaultProfileStore interface {
	VaultDirectoryStore
	VaultUnlockState
	Lock() error
}

// VaultWriteBacker drives the working-copy write-back loop (spec:
// docs/specs/vault-working-copy.md).
type VaultWriteBacker interface {
	WriteBackTick() domain.VaultWriteBackResult
	Resync() error
	MasterChanged() bool
}

// VaultWorkDirConfigurer points the vault at a profile's ephemeral working
// copy directory (locked-only; empty disables).
type VaultWorkDirConfigurer interface {
	SetWorkDir(dir string) error
}

type ChatReader interface {
	ListChats() ([]domain.Chat, error)
}

type ChatRepository interface {
	ChatReader
	CreateChat(title string) (domain.Chat, error)
	RenameChat(chatID string, title string) (domain.Chat, error)
	SetChatArchived(chatID string, archived bool) (domain.Chat, error)
	ClearChat(chatID string) error
	DeleteChat(chatID string) error
	ListMessages(chatID string) ([]domain.Message, error)
	RecentMessages(chatID string, limit int) ([]domain.Message, error)
	AddMessage(chatID string, role string, content string) (domain.Message, error)
	AddMessageWithAttachments(chatID string, role string, content string, attachments []domain.Attachment) (domain.Message, error)
}

type ChatConversationStore interface {
	ChatRepository
	VaultUnlockState
}

type VaultOperationSnapshotStore interface {
	VaultStatusReader
	VaultUnlockState
	ChatReader
}

type ChatCompactionStore interface {
	VaultUnlockState
	ListMessages(chatID string) ([]domain.Message, error)
	ReplaceMessages(chatID string, messages []domain.Message) ([]domain.Message, error)
}

type ChatAutoRenameStore interface {
	ChatRepository
	ChatTitleWriter
	SessionSummaryWriter
	SetSecret(name string, value string) error
	GetSecret(name string) (string, bool, error)
}

// SessionSummaryWriter stores the short index summary of a chat session.
type SessionSummaryWriter interface {
	SetSessionSummary(sessionID string, summary string, turn int) error
}

// ChatModelOverrideStore persists a per-chat provider/model override (empty =
// use the global active provider).
type ChatModelOverrideStore interface {
	ChatModelOverride(chatID string) (provider string, model string, err error)
	SetChatModelOverride(chatID string, provider string, model string) error
}

// MemoryContextSetter receives the dynamic chat-memory block (catalog + recall
// guidance) to compose into the agent system instruction.
type MemoryContextSetter interface {
	SetMemoryContext(extra string)
}

// ChatTitleWriter records a title change for a chat (auto or manual).
type ChatTitleWriter interface {
	InsertChatTitle(entry domain.ChatTitleEntry) (domain.ChatTitleEntry, error)
}

// ChatTitleStore reads and writes the title history of a chat.
type ChatTitleStore interface {
	ChatTitleWriter
	ListChatTitles(sessionID string) ([]domain.ChatTitleEntry, error)
}

type ChatSessionInfoStore interface {
	VaultUnlockState
	CountMessages(chatID string) (int, error)
}

type LLMTurnStore interface {
	InsertLLMTurn(turn domain.LLMTurn) (domain.LLMTurn, error)
	ListLLMTurns(sessionID string) ([]domain.LLMTurn, error)
}

// ChatMemoryStore exposes the read paths used to recall past conversations:
// the session catalog, lexical FTS5 search, and full session history.
type ChatMemoryStore interface {
	VaultUnlockState
	ListChats() ([]domain.Chat, error)
	ListMessages(chatID string) ([]domain.Message, error)
	RecentMessages(chatID string, limit int) ([]domain.Message, error)
	SearchMessages(query string, limit int) ([]domain.Message, error)
	SessionSummary(sessionID string) (string, int, error)
}

// UserMemoryStore persists long-term facts about the user (v1 — retained for
// migration and existing tests; no new writes after v2 migration).
type UserMemoryStore interface {
	VaultUnlockState
	UpsertUserFact(fact domain.UserMemoryFact) (domain.UserMemoryFact, error)
	ListUserFacts() ([]domain.UserMemoryFact, error)
	DeleteUserFact(key string) error
}

// UserMemoryDocStore persists the v2 user-memory living document — one
// freeform text the agent and user jointly maintain (user-memory-v2-spec.md).
type UserMemoryDocStore interface {
	VaultUnlockState
	GetUserMemoryDoc() (domain.UserMemoryDoc, error)
	SetUserMemoryDoc(doc domain.UserMemoryDoc) (domain.UserMemoryDoc, error)
}

type SecretStore interface {
	SetSecret(name string, value string) error
	GetSecret(name string) (string, bool, error)
	HasSecret(name string) (bool, error)
	DeleteSecret(name string) error
	ListSecrets() ([]string, error)
}

type LogStore interface {
	InsertLog(entry domain.LogEntry) (domain.LogEntry, error)
	ListLogs(query domain.LogQuery) ([]domain.LogEntry, error)
	ListLogDates() ([]domain.LogDateCount, error)
	DeleteLogsOlderThan(days int) (int, error)
	DeleteAllLogs() (int, error)
}

type WebObservationStore interface {
	VaultUnlockState
	SaveWebObservation(obs domain.WebObservation) (domain.WebObservation, error)
	LatestWebObservation(chatID, browser, siteKey string) (domain.WebObservation, bool, error)
	ClearWebObservations() error
}
