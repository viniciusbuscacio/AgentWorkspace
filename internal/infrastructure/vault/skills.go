package vault

import (
	"database/sql"

	"aw/internal/domain"
)

// ListSkills returns every skill with its files. Soft-deleted skills are
// included only when includeDeleted is set (the seed bootstrap needs them to
// avoid resurrecting a builtin the user removed).
func (v *Vault) ListSkills(includeDeleted bool) ([]domain.Skill, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}

	query := `SELECT id, name, description, enabled, origin, seed_version, deleted_at FROM skills`
	if !includeDeleted {
		query += ` WHERE deleted_at IS NULL`
	}
	query += ` ORDER BY id ASC`

	rows, err := v.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	skills := make([]domain.Skill, 0)
	for rows.Next() {
		skill, err := scanSkill(rows.Scan)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range skills {
		files, err := v.skillFilesLocked(skills[i].ID)
		if err != nil {
			return nil, err
		}
		skills[i].Files = files
	}
	return skills, nil
}

// GetSkill returns one skill by id with its files; ok is false when absent.
func (v *Vault) GetSkill(id string) (domain.Skill, bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.Skill{}, false, errLocked
	}
	row := v.db.QueryRow(
		`SELECT id, name, description, enabled, origin, seed_version, deleted_at FROM skills WHERE id = ?`, id,
	)
	skill, err := scanSkill(row.Scan)
	if err == sql.ErrNoRows {
		return domain.Skill{}, false, nil
	}
	if err != nil {
		return domain.Skill{}, false, err
	}
	files, err := v.skillFilesLocked(id)
	if err != nil {
		return domain.Skill{}, false, err
	}
	skill.Files = files
	return skill, true, nil
}

// UpsertSkill writes the skill row and replaces its file set in one transaction.
func (v *Vault) UpsertSkill(skill domain.Skill) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	now := nowString()
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var deletedAt any
	if skill.Deleted {
		deletedAt = now
	}
	if _, err := tx.Exec(
		`INSERT INTO skills (id, name, description, enabled, origin, seed_version, deleted_at, created_at, updated_at)
		   VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name,
		   description = excluded.description,
		   enabled = excluded.enabled,
		   origin = excluded.origin,
		   seed_version = excluded.seed_version,
		   deleted_at = excluded.deleted_at,
		   updated_at = excluded.updated_at`,
		skill.ID, skill.Name, skill.Description, boolToInt(skill.Enabled),
		originOrDefault(skill.Origin), skill.SeedVersion, deletedAt, now, now,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM skill_files WHERE skill_id = ?`, skill.ID); err != nil {
		return err
	}
	for _, f := range skill.Files {
		var seedHash any
		if f.SeedHash != "" {
			seedHash = f.SeedHash
		}
		if _, err := tx.Exec(
			`INSERT INTO skill_files (skill_id, path, content, content_hash, seed_hash, updated_at)
			   VALUES (?, ?, ?, ?, ?, ?)`,
			skill.ID, f.Path, f.Content, f.ContentHash, seedHash, now,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetSkillEnabled toggles a skill on/off without touching its files.
func (v *Vault) SetSkillEnabled(id string, enabled bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	_, err := v.db.Exec(
		`UPDATE skills SET enabled = ?, updated_at = ? WHERE id = ?`,
		boolToInt(enabled), nowString(), id,
	)
	return err
}

// SoftDeleteSkill marks a skill deleted so the seed bootstrap will not recreate
// it. Files are kept so a later un-delete can restore the folder.
func (v *Vault) SoftDeleteSkill(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	_, err := v.db.Exec(
		`UPDATE skills SET deleted_at = ?, enabled = 0, updated_at = ? WHERE id = ? AND deleted_at IS NULL`,
		nowString(), nowString(), id,
	)
	return err
}

// DeleteSkill physically removes a skill and its files. Used for user skills;
// builtins use SoftDeleteSkill so the seed bootstrap won't resurrect them.
func (v *Vault) DeleteSkill(id string) error {
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
	if _, err := tx.Exec(`DELETE FROM skill_files WHERE skill_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM skills WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// GetAppDocument returns a runtime app document (e.g. AGENTS.md); ok is false
// when absent.
func (v *Vault) GetAppDocument(id string) (domain.AppDocument, bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.AppDocument{}, false, errLocked
	}
	row := v.db.QueryRow(
		`SELECT id, content, content_hash, seed_hash, origin, seed_version, updated_at FROM app_documents WHERE id = ?`, id,
	)
	doc, err := scanAppDocument(row.Scan)
	if err == sql.ErrNoRows {
		return domain.AppDocument{}, false, nil
	}
	if err != nil {
		return domain.AppDocument{}, false, err
	}
	return doc, true, nil
}

// ListAppDocuments returns every runtime app document (AGENTS.md, USER.md, …).
func (v *Vault) ListAppDocuments() ([]domain.AppDocument, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	rows, err := v.db.Query(
		`SELECT id, content, content_hash, seed_hash, origin, seed_version, updated_at FROM app_documents ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	docs := make([]domain.AppDocument, 0)
	for rows.Next() {
		doc, err := scanAppDocument(rows.Scan)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// UpsertAppDocument writes a runtime app document.
func (v *Vault) UpsertAppDocument(doc domain.AppDocument) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	var seedHash any
	if doc.SeedHash != "" {
		seedHash = doc.SeedHash
	}
	_, err := v.db.Exec(
		`INSERT INTO app_documents (id, content, content_hash, seed_hash, origin, seed_version, updated_at)
		   VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   content = excluded.content,
		   content_hash = excluded.content_hash,
		   seed_hash = excluded.seed_hash,
		   origin = excluded.origin,
		   seed_version = excluded.seed_version,
		   updated_at = excluded.updated_at`,
		doc.ID, doc.Content, doc.ContentHash, seedHash,
		originOrDefault(doc.Origin), doc.SeedVersion, nowString(),
	)
	return err
}

func (v *Vault) skillFilesLocked(skillID string) ([]domain.SkillFile, error) {
	rows, err := v.db.Query(
		`SELECT path, content, content_hash, seed_hash FROM skill_files WHERE skill_id = ? ORDER BY path ASC`, skillID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]domain.SkillFile, 0)
	for rows.Next() {
		var f domain.SkillFile
		var seedHash sql.NullString
		if err := rows.Scan(&f.Path, &f.Content, &f.ContentHash, &seedHash); err != nil {
			return nil, err
		}
		f.SeedHash = seedHash.String
		files = append(files, f)
	}
	return files, rows.Err()
}

func scanSkill(scan func(dest ...any) error) (domain.Skill, error) {
	var skill domain.Skill
	var enabled int
	var deletedAt sql.NullString
	if err := scan(&skill.ID, &skill.Name, &skill.Description, &enabled, &skill.Origin, &skill.SeedVersion, &deletedAt); err != nil {
		return domain.Skill{}, err
	}
	skill.Enabled = enabled != 0
	skill.Deleted = deletedAt.Valid && deletedAt.String != ""
	return skill, nil
}

func scanAppDocument(scan func(dest ...any) error) (domain.AppDocument, error) {
	var doc domain.AppDocument
	var seedHash, updatedAt sql.NullString
	if err := scan(&doc.ID, &doc.Content, &doc.ContentHash, &seedHash, &doc.Origin, &doc.SeedVersion, &updatedAt); err != nil {
		return domain.AppDocument{}, err
	}
	doc.SeedHash = seedHash.String
	doc.UpdatedAt = updatedAt.String
	return doc, nil
}

func originOrDefault(origin string) string {
	if origin == domain.SkillOriginBuiltin {
		return domain.SkillOriginBuiltin
	}
	return domain.SkillOriginUser
}
