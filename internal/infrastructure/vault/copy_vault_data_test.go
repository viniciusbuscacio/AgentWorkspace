package vault

import (
	"testing"

	"aw/internal/domain"
)

// criticalTables are the tables that copyVaultData previously dropped silently
// on ChangePassword / RecoverWithKey (plus the ones it already copied). The
// regression guarantee is that NONE of them come out empty after a rekey.
var criticalTables = []string{
	"meta",
	"secrets",
	"sessions",
	"messages",
	"messages_fts",
	"llm_turns",
	"chat_titles",
	"notes",
	"backlog_items",
	"backlog_attachments",
	"user_memory",
	"user_memory_doc",
	"logs",
	"skills",
	"skill_files",
	"app_documents",
}

// seedEveryTable inserts one sentinel row into every table copyVaultData must
// preserve, returning a session id used for the FK-bound rows.
func seedEveryTable(t *testing.T, v *Vault) {
	t.Helper()

	chat, err := v.CreateChat("Rekey survival chat")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	if _, err := v.AddMessage(chat.ID, "user", "mensagem que precisa sobreviver"); err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}
	if err := v.SetSecret("api_key", "sk-aw-keep-me"); err != nil {
		t.Fatalf("SetSecret() error = %v", err)
	}

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := v.db.Exec(query, args...); err != nil {
			t.Fatalf("seed exec %q error = %v", query, err)
		}
	}

	exec(`INSERT INTO llm_turns (id, session_id, turn_index, request_json, created_at) VALUES (?, ?, ?, ?, ?)`,
		"turn-1", chat.ID, 0, `{"k":"v"}`, "2026-06-24T00:00:00Z")
	exec(`INSERT INTO chat_titles (id, session_id, title, turn, source, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"title-1", chat.ID, "Auto title", 1, "auto", "2026-06-24T00:00:00Z")
	exec(`INSERT INTO notes (id, title, content, updated_at) VALUES (?, ?, ?, ?)`,
		"note-1", "Nota", "conteudo da nota", "2026-06-24T00:00:00Z")
	exec(`INSERT INTO backlog_items (id, title, status, position) VALUES (?, ?, ?, ?)`,
		"item-1", "Tasks item", "open", 0)
	exec(`INSERT INTO backlog_attachments (id, item_id, name, mime_type, size, data, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"att-1", "item-1", "file.txt", "text/plain", 3, []byte("abc"), "2026-06-24T00:00:00Z")
	exec(`INSERT INTO user_memory (key, category, content, source, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"fact-1", "general", "o usuario gosta de pt-BR", "agent", "2026-06-24T00:00:00Z")
	exec(`INSERT INTO user_memory_doc (id, content) VALUES (1, ?)`,
		"documento de memoria do usuario")
	exec(`INSERT INTO logs (id, timestamp, level, level_name, source, message, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"log-1", "2026-06-24T00:00:00Z", 1, "info", "vault", "linha de log", "2026-06-24T00:00:00Z")

	// Skills, skill files and app documents (the new vault tables) must ride the
	// same generic copy — this is the Fase 0.5 <-> Fase 1 link: adding tables
	// must never reintroduce the silent-drop bug.
	if err := v.UpsertSkill(domain.Skill{
		ID: "gmail-web", Name: "gmail-web", Enabled: true, Origin: domain.SkillOriginBuiltin, SeedVersion: "1",
		Files: []domain.SkillFile{{Path: domain.SkillMarkdownPath, Content: "BODY", ContentHash: "h", SeedHash: "h"}},
	}); err != nil {
		t.Fatalf("UpsertSkill() error = %v", err)
	}
	if err := v.UpsertAppDocument(domain.AppDocument{
		ID: domain.AgentsDocumentID, Content: "AGENTS", ContentHash: "h", SeedHash: "h", Origin: domain.SkillOriginBuiltin,
	}); err != nil {
		t.Fatalf("UpsertAppDocument() error = %v", err)
	}
}

func assertNoTableEmpty(t *testing.T, v *Vault) {
	t.Helper()
	for _, table := range criticalTables {
		var count int
		if err := v.db.QueryRow(`SELECT count(*) FROM ` + quoteIdent(table)).Scan(&count); err != nil {
			t.Fatalf("count %s error = %v", table, err)
		}
		if count == 0 {
			t.Fatalf("table %q is empty after rekey — copyVaultData dropped it", table)
		}
	}
}

func TestChangePasswordPreservesEveryTable(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha-atual"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	seedEveryTable(t, v)

	if _, err := v.ChangePassword("senha-atual", "senha-nova"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	assertNoTableEmpty(t, v)
}

func TestRecoverWithKeyPreservesEveryTable(t *testing.T) {
	v := New(t.TempDir())
	recoveryKey, err := v.Create("senha-atual")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	seedEveryTable(t, v)

	if _, err := v.RecoverWithKey(recoveryKey, "senha-nova"); err != nil {
		t.Fatalf("RecoverWithKey() error = %v", err)
	}
	if err := v.Unlock("senha-nova"); err != nil {
		t.Fatalf("Unlock() after recover error = %v", err)
	}

	assertNoTableEmpty(t, v)
}

// TestCopyableTablesSkipsFTSShadowTables guards the FTS handling: the fts5
// virtual table and its shadow tables must never appear in the generic copy
// list (they are rebuilt from messages), or copyTable would corrupt the index.
func TestCopyableTablesSkipsFTSShadowTables(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	tables, err := copyableTables(v.db)
	if err != nil {
		t.Fatalf("copyableTables() error = %v", err)
	}
	for _, name := range tables {
		if name == "messages_fts" || hasPrefix(name, "messages_fts_") || hasPrefix(name, "sqlite_") {
			t.Fatalf("copyableTables() must not include virtual/shadow/internal table %q", name)
		}
	}
	// messages must be copyable.
	if !containsString(tables, "messages") {
		t.Fatalf("copyableTables() missing messages: %v", tables)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
