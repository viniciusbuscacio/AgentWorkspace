package application

import (
	"errors"
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

func normalizeTasksStatus(status string) (string, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return "", nil
	}
	if !domain.IsTasksStatus(status) {
		return "", fmt.Errorf("invalid status %q (use %s)", status, strings.Join(domain.TasksStatuses(), ", "))
	}
	return status, nil
}

// CreateTasksItem validates and appends a new item. Body is optional and
// status defaults to open.
func CreateTasksItem(store ports.TasksStore, title, body, status string) (domain.TasksItem, error) {
	if store == nil {
		return domain.TasksItem{}, errors.New("tasks store is required")
	}
	if !store.IsUnlocked() {
		return domain.TasksItem{}, errVaultLocked
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.TasksItem{}, errors.New("item title is required")
	}
	status, err := normalizeTasksStatus(status)
	if err != nil {
		return domain.TasksItem{}, err
	}
	if status == "" {
		status = domain.TasksStatusOpen
	}
	return store.CreateTasksItem(title, body, status)
}

// ListTasksItems returns the tasks ordered by position.
func ListTasksItems(store ports.TasksStore) ([]domain.TasksItem, error) {
	if store == nil {
		return nil, errors.New("tasks store is required")
	}
	if !store.IsUnlocked() {
		return nil, errVaultLocked
	}
	return store.ListTasksItems()
}

// GetTasksItem returns one item in full (body + attachment metadata).
func GetTasksItem(store ports.TasksStore, id string) (domain.TasksItem, error) {
	if store == nil {
		return domain.TasksItem{}, errors.New("tasks store is required")
	}
	if !store.IsUnlocked() {
		return domain.TasksItem{}, errVaultLocked
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.TasksItem{}, errors.New("item id is required")
	}
	return store.GetTasksItem(id)
}

// UpdateTasksItem changes an item's title, body, status and/or position.
// Empty title/status and nil body/position keep the current values (body uses
// a pointer because an empty body is a valid value to set); at least one
// field must be provided.
func UpdateTasksItem(store ports.TasksStore, id string, title string, body *string, status string, position *int) (domain.TasksItem, error) {
	if store == nil {
		return domain.TasksItem{}, errors.New("tasks store is required")
	}
	if !store.IsUnlocked() {
		return domain.TasksItem{}, errVaultLocked
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.TasksItem{}, errors.New("item id is required")
	}
	title = strings.TrimSpace(title)
	status, err := normalizeTasksStatus(status)
	if err != nil {
		return domain.TasksItem{}, err
	}
	if title == "" && body == nil && status == "" && position == nil {
		return domain.TasksItem{}, errors.New("title, body, status and/or position is required")
	}
	current, err := store.GetTasksItem(id)
	if err != nil {
		return domain.TasksItem{}, err
	}
	if title == "" {
		title = current.Title
	}
	newBody := current.Body
	if body != nil {
		newBody = *body
	}
	if status == "" {
		status = current.Status
	}
	pos := current.Position
	if position != nil {
		pos = *position
	}
	return store.UpdateTasksItem(id, title, newBody, status, pos)
}

// DeleteTasksItem removes an item and returns it (so callers can echo the
// deleted title in results).
func DeleteTasksItem(store ports.TasksStore, id string) (domain.TasksItem, error) {
	if store == nil {
		return domain.TasksItem{}, errors.New("tasks store is required")
	}
	if !store.IsUnlocked() {
		return domain.TasksItem{}, errVaultLocked
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.TasksItem{}, errors.New("item id is required")
	}
	item, err := store.GetTasksItem(id)
	if err != nil {
		return domain.TasksItem{}, err
	}
	if err := store.DeleteTasksItem(id); err != nil {
		return domain.TasksItem{}, err
	}
	return item, nil
}

// AddTasksAttachment validates and stores one image attachment. The caps
// come from AW2 (5 MB per image, 5 images per item) and are a hard fence: the
// vault is an encrypted SQLite file and BLOBs bloat it permanently.
func AddTasksAttachment(store ports.TasksStore, itemID, name, mimeType string, data []byte) (domain.TasksAttachment, error) {
	if store == nil {
		return domain.TasksAttachment{}, errors.New("tasks store is required")
	}
	if !store.IsUnlocked() {
		return domain.TasksAttachment{}, errVaultLocked
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return domain.TasksAttachment{}, errors.New("item id is required")
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if !domain.IsTasksAttachmentMimeType(mimeType) {
		return domain.TasksAttachment{}, fmt.Errorf("unsupported image type %q (use %s)", mimeType, strings.Join(domain.TasksAttachmentMimeTypes(), ", "))
	}
	if len(data) == 0 {
		return domain.TasksAttachment{}, errors.New("attachment data is required")
	}
	if len(data) > domain.TasksAttachmentMaxBytes {
		return domain.TasksAttachment{}, fmt.Errorf("attachment is %d bytes; the limit is 5 MB per image", len(data))
	}
	existing, err := store.ListTasksAttachments(itemID)
	if err != nil {
		return domain.TasksAttachment{}, err
	}
	if len(existing) >= domain.TasksMaxAttachments {
		return domain.TasksAttachment{}, fmt.Errorf("a tasks item can have at most %d image attachments", domain.TasksMaxAttachments)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "image"
	}
	return store.AddTasksAttachment(itemID, name, mimeType, data)
}

// GetTasksAttachment returns one attachment's metadata and content.
func GetTasksAttachment(store ports.TasksStore, attachmentID string) (domain.TasksAttachment, []byte, error) {
	if store == nil {
		return domain.TasksAttachment{}, nil, errors.New("tasks store is required")
	}
	if !store.IsUnlocked() {
		return domain.TasksAttachment{}, nil, errVaultLocked
	}
	attachmentID = strings.TrimSpace(attachmentID)
	if attachmentID == "" {
		return domain.TasksAttachment{}, nil, errors.New("attachment id is required")
	}
	return store.GetTasksAttachment(attachmentID)
}

// DeleteTasksAttachment removes one attachment by id.
func DeleteTasksAttachment(store ports.TasksStore, attachmentID string) error {
	if store == nil {
		return errors.New("tasks store is required")
	}
	if !store.IsUnlocked() {
		return errVaultLocked
	}
	attachmentID = strings.TrimSpace(attachmentID)
	if attachmentID == "" {
		return errors.New("attachment id is required")
	}
	return store.DeleteTasksAttachment(attachmentID)
}
