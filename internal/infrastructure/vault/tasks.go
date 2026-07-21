package vault

import (
	"database/sql"
	"errors"

	"aw/internal/domain"
)

type TasksItem = domain.TasksItem

var (
	errTasksItemNotFound       = errors.New("tasks item not found")
	errTasksAttachmentNotFound = errors.New("tasks attachment not found")
)

// CreateTasksItem appends a new item at the end of the tasks. Body may be
// empty and status is taken as given (the application layer validates it and
// defaults it to open).
func (v *Vault) CreateTasksItem(title, body, status string) (TasksItem, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return TasksItem{}, errLocked
	}
	var maxPosition sql.NullInt64
	if err := v.db.QueryRow(`SELECT MAX(position) FROM backlog_items`).Scan(&maxPosition); err != nil {
		return TasksItem{}, err
	}
	item := TasksItem{
		ID:          "task-" + randomHex(6),
		Title:       title,
		Body:        body,
		Status:      status,
		Position:    int(maxPosition.Int64) + 1,
		CreatedAt:   nowString(),
		Attachments: []domain.TasksAttachment{},
	}
	_, err := v.db.Exec(
		`INSERT INTO backlog_items (id, title, body, status, position, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		item.ID, item.Title, item.Body, item.Status, item.Position, item.CreatedAt,
	)
	if err != nil {
		return TasksItem{}, err
	}
	return item, nil
}

// ListTasksItems returns the tasks ordered by position, each item carrying
// its attachment metadata (never the BLOB content).
func (v *Vault) ListTasksItems() ([]TasksItem, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	rows, err := v.db.Query(
		`SELECT id, title, body, status, position, created_at FROM backlog_items ORDER BY position ASC, id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]TasksItem, 0)
	for rows.Next() {
		item, err := scanTasksItem(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		attachments, err := v.listTasksAttachmentsLocked(items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].Attachments = attachments
	}
	return items, nil
}

// GetTasksItem returns one item by id, including its attachment metadata.
func (v *Vault) GetTasksItem(id string) (TasksItem, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return TasksItem{}, errLocked
	}
	item, err := v.getTasksItemLocked(id)
	if err != nil {
		return TasksItem{}, err
	}
	attachments, err := v.listTasksAttachmentsLocked(id)
	if err != nil {
		return TasksItem{}, err
	}
	item.Attachments = attachments
	return item, nil
}

// UpdateTasksItem replaces a tasks item's title, body, status and
// position. Partial-update merging is the application layer's job; the store
// writes the full row.
func (v *Vault) UpdateTasksItem(id, title, body, status string, position int) (TasksItem, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return TasksItem{}, errLocked
	}
	result, err := v.db.Exec(
		`UPDATE backlog_items SET title = ?, body = ?, status = ?, position = ? WHERE id = ?`,
		title, body, status, position, id,
	)
	if err != nil {
		return TasksItem{}, err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return TasksItem{}, errTasksItemNotFound
	}
	item, err := v.getTasksItemLocked(id)
	if err != nil {
		return TasksItem{}, err
	}
	attachments, err := v.listTasksAttachmentsLocked(id)
	if err != nil {
		return TasksItem{}, err
	}
	item.Attachments = attachments
	return item, nil
}

// DeleteTasksItem removes one item and its attachments by id. Deleting a
// missing item is a no-op.
func (v *Vault) DeleteTasksItem(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	if _, err := v.db.Exec(`DELETE FROM backlog_attachments WHERE item_id = ?`, id); err != nil {
		return err
	}
	_, err := v.db.Exec(`DELETE FROM backlog_items WHERE id = ?`, id)
	return err
}

// ListTasksAttachments returns the metadata of every attachment of an item
// (never the BLOB content), oldest first.
func (v *Vault) ListTasksAttachments(itemID string) ([]domain.TasksAttachment, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, errLocked
	}
	return v.listTasksAttachmentsLocked(itemID)
}

// AddTasksAttachment stores one image BLOB against an item and returns its
// metadata. The item must exist (foreign-key fence enforced here since SQLite
// FKs are off by default).
func (v *Vault) AddTasksAttachment(itemID, name, mimeType string, data []byte) (domain.TasksAttachment, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.TasksAttachment{}, errLocked
	}
	if _, err := v.getTasksItemLocked(itemID); err != nil {
		return domain.TasksAttachment{}, err
	}
	attachment := domain.TasksAttachment{
		ID:        "att-" + randomHex(6),
		ItemID:    itemID,
		Name:      name,
		MimeType:  mimeType,
		Size:      len(data),
		CreatedAt: nowString(),
	}
	_, err := v.db.Exec(
		`INSERT INTO backlog_attachments (id, item_id, name, mime_type, size, data, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		attachment.ID, attachment.ItemID, attachment.Name, attachment.MimeType, attachment.Size, data, attachment.CreatedAt,
	)
	if err != nil {
		return domain.TasksAttachment{}, err
	}
	return attachment, nil
}

// GetTasksAttachment returns one attachment's metadata and its BLOB content.
func (v *Vault) GetTasksAttachment(attachmentID string) (domain.TasksAttachment, []byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return domain.TasksAttachment{}, nil, errLocked
	}
	var (
		attachment domain.TasksAttachment
		data       []byte
	)
	err := v.db.QueryRow(
		`SELECT id, item_id, name, mime_type, size, data, created_at FROM backlog_attachments WHERE id = ?`,
		attachmentID,
	).Scan(&attachment.ID, &attachment.ItemID, &attachment.Name, &attachment.MimeType, &attachment.Size, &data, &attachment.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TasksAttachment{}, nil, errTasksAttachmentNotFound
	}
	if err != nil {
		return domain.TasksAttachment{}, nil, err
	}
	return attachment, data, nil
}

// DeleteTasksAttachment removes one attachment by id. Deleting a missing
// attachment is a no-op.
func (v *Vault) DeleteTasksAttachment(attachmentID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return errLocked
	}
	_, err := v.db.Exec(`DELETE FROM backlog_attachments WHERE id = ?`, attachmentID)
	return err
}

func (v *Vault) listTasksAttachmentsLocked(itemID string) ([]domain.TasksAttachment, error) {
	rows, err := v.db.Query(
		`SELECT id, item_id, name, mime_type, size, created_at FROM backlog_attachments WHERE item_id = ? ORDER BY created_at ASC, id ASC`,
		itemID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attachments := make([]domain.TasksAttachment, 0)
	for rows.Next() {
		var attachment domain.TasksAttachment
		if err := rows.Scan(&attachment.ID, &attachment.ItemID, &attachment.Name, &attachment.MimeType, &attachment.Size, &attachment.CreatedAt); err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	return attachments, rows.Err()
}

func (v *Vault) getTasksItemLocked(id string) (TasksItem, error) {
	row := v.db.QueryRow(
		`SELECT id, title, body, status, position, created_at FROM backlog_items WHERE id = ?`, id,
	)
	item, err := scanTasksItem(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return TasksItem{}, errTasksItemNotFound
	}
	if err != nil {
		return TasksItem{}, err
	}
	return item, nil
}

func scanTasksItem(scan func(dest ...any) error) (TasksItem, error) {
	var item TasksItem
	if err := scan(&item.ID, &item.Title, &item.Body, &item.Status, &item.Position, &item.CreatedAt); err != nil {
		return TasksItem{}, err
	}
	item.Attachments = []domain.TasksAttachment{}
	return item, nil
}
