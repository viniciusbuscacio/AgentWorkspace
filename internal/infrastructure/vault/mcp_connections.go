package vault

import (
	"database/sql"

	"aw/internal/domain"
)

// MCP Client persistence (Block B). Connection metadata lives in the
// mcp_connections table; the bearer token lives in the encrypted `secrets`
// table under a deterministic internal key, with the connection row tracking
// only secret_key (so HasSecret can be derived without exposing the token).

const mcpColumns = `id, name, enabled, transport, url, auth_type, secret_key, created_at, updated_at, last_checked_at, last_status, last_error, tool_count`

// CreateMcpConnection inserts a new connection and returns it sanitized.
func (v *Vault) CreateMcpConnection(in domain.McpConnectionInput) (domain.McpConnection, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.McpConnection{}, errLocked
	}
	now := nowString()
	id := "mcpconn-" + randomHex(8)
	if _, err := v.db.Exec(
		`INSERT INTO mcp_connections (id, name, enabled, transport, url, auth_type, secret_key, created_at, updated_at, last_checked_at, last_status, last_error, tool_count)
		   VALUES (?, ?, ?, ?, ?, ?, '', ?, ?, '', ?, '', 0)`,
		id, in.Name, boolToInt(in.Enabled), in.Transport, in.URL, in.AuthType, now, now, domain.McpStatusUnknown,
	); err != nil {
		return domain.McpConnection{}, err
	}
	return v.getMcpConnectionLocked(id)
}

// ListMcpConnections returns every connection, sanitized, newest activity first.
func (v *Vault) ListMcpConnections() ([]domain.McpConnection, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	rows, err := v.db.Query(`SELECT ` + mcpColumns + ` FROM mcp_connections ORDER BY updated_at DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.McpConnection, 0)
	for rows.Next() {
		conn, err := scanMcpConnection(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, conn)
	}
	return out, rows.Err()
}

// GetMcpConnection returns one connection; ok is false when absent.
func (v *Vault) GetMcpConnection(id string) (domain.McpConnection, bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.McpConnection{}, false, errLocked
	}
	conn, err := v.getMcpConnectionLocked(id)
	if err == sql.ErrNoRows {
		return domain.McpConnection{}, false, nil
	}
	if err != nil {
		return domain.McpConnection{}, false, err
	}
	return conn, true, nil
}

// UpdateMcpConnection applies a partial update (nil fields keep current values).
func (v *Vault) UpdateMcpConnection(id string, patch domain.McpConnectionPatch) (domain.McpConnection, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.McpConnection{}, errLocked
	}
	cur, err := v.getMcpConnectionLocked(id)
	if err != nil {
		return domain.McpConnection{}, err
	}
	name, enabled, url, authType := cur.Name, cur.Enabled, cur.URL, cur.AuthType
	if patch.Name != nil {
		name = *patch.Name
	}
	if patch.Enabled != nil {
		enabled = *patch.Enabled
	}
	if patch.URL != nil {
		url = *patch.URL
	}
	if patch.AuthType != nil {
		authType = *patch.AuthType
	}
	if _, err := v.db.Exec(
		`UPDATE mcp_connections SET name = ?, enabled = ?, url = ?, auth_type = ?, updated_at = ? WHERE id = ?`,
		name, boolToInt(enabled), url, authType, nowString(), id,
	); err != nil {
		return domain.McpConnection{}, err
	}
	return v.getMcpConnectionLocked(id)
}

// DeleteMcpConnection removes a connection and its associated secret.
func (v *Vault) DeleteMcpConnection(id string) error {
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
	if _, err := tx.Exec(`DELETE FROM secrets WHERE name = ?`, domain.McpConnectionSecretKey(id)); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM mcp_connections WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// SetMcpConnectionEnabled toggles a connection on/off.
func (v *Vault) SetMcpConnectionEnabled(id string, enabled bool) (domain.McpConnection, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.McpConnection{}, errLocked
	}
	if _, err := v.db.Exec(
		`UPDATE mcp_connections SET enabled = ?, updated_at = ? WHERE id = ?`,
		boolToInt(enabled), nowString(), id,
	); err != nil {
		return domain.McpConnection{}, err
	}
	return v.getMcpConnectionLocked(id)
}

// RecordMcpConnectionTest persists a connection-test outcome.
func (v *Vault) RecordMcpConnectionTest(id, status, lastError string, toolCount int) (domain.McpConnection, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.McpConnection{}, errLocked
	}
	if _, err := v.db.Exec(
		`UPDATE mcp_connections SET last_status = ?, last_error = ?, tool_count = ?, last_checked_at = ?, updated_at = ? WHERE id = ?`,
		status, lastError, toolCount, nowString(), nowString(), id,
	); err != nil {
		return domain.McpConnection{}, err
	}
	return v.getMcpConnectionLocked(id)
}

// SetMcpConnectionSecret stores the bearer token and flags the connection's
// secret_key. The secret lives in the encrypted `secrets` table; the connection
// row only references its key.
func (v *Vault) SetMcpConnectionSecret(connectionID, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	key := domain.McpConnectionSecretKey(connectionID)
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO secrets (id, name, value) VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET value = excluded.value`,
		"secret-"+randomHex(8), key, value,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE mcp_connections SET secret_key = ?, updated_at = ? WHERE id = ?`, key, nowString(), connectionID); err != nil {
		return err
	}
	return tx.Commit()
}

// GetMcpConnectionSecret returns the stored bearer token, if any.
func (v *Vault) GetMcpConnectionSecret(connectionID string) (string, bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return "", false, errLocked
	}
	var value string
	err := v.db.QueryRow(`SELECT value FROM secrets WHERE name = ?`, domain.McpConnectionSecretKey(connectionID)).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// DeleteMcpConnectionSecret removes the stored token and clears secret_key.
func (v *Vault) DeleteMcpConnectionSecret(connectionID string) error {
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
	if _, err := tx.Exec(`DELETE FROM secrets WHERE name = ?`, domain.McpConnectionSecretKey(connectionID)); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE mcp_connections SET secret_key = '', updated_at = ? WHERE id = ?`, nowString(), connectionID); err != nil {
		return err
	}
	return tx.Commit()
}

func (v *Vault) getMcpConnectionLocked(id string) (domain.McpConnection, error) {
	row := v.db.QueryRow(`SELECT `+mcpColumns+` FROM mcp_connections WHERE id = ?`, id)
	return scanMcpConnection(row.Scan)
}

// scanMcpConnection maps a row to the SANITIZED connection: secret_key is read
// only to derive HasSecret and is never placed on the returned struct.
func scanMcpConnection(scan func(dest ...any) error) (domain.McpConnection, error) {
	var conn domain.McpConnection
	var enabled, toolCount int
	var secretKey string
	if err := scan(
		&conn.ID, &conn.Name, &enabled, &conn.Transport, &conn.URL, &conn.AuthType,
		&secretKey, &conn.CreatedAt, &conn.UpdatedAt, &conn.LastCheckedAt, &conn.LastStatus, &conn.LastError, &toolCount,
	); err != nil {
		return domain.McpConnection{}, err
	}
	conn.Enabled = enabled != 0
	conn.ToolCount = toolCount
	conn.HasSecret = secretKey != ""
	return conn, nil
}
