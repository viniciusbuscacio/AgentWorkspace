package appcore

import (
	"encoding/base64"
	"fmt"
	"strings"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
)

// ListTasksItems returns the tasks for the Tasks module view (items
// carry attachment metadata, never the BLOBs).
func (a *App) ListTasksItems() dto.TasksResult {
	a.recordActivity()
	items, err := application.ListTasksItems(a.vault)
	if err != nil {
		return dto.TasksResult{Error: err.Error()}
	}
	return dto.TasksResult{Success: true, Items: items}
}

// AddTasksItem appends a new item; body is optional and status defaults to
// open.
func (a *App) AddTasksItem(title string, body string, status string) dto.TasksItemResult {
	a.recordActivity()
	return tasksItemResult(application.CreateTasksItem(a.vault, title, body, status))
}

// UpdateTasksItem changes an item's title, body, status and/or position.
// Empty title/status keep the current values; body is authoritative (the
// detail editor always sends the full text, and an empty body is a valid
// value); position < 0 keeps the current one.
func (a *App) UpdateTasksItem(id string, title string, body string, status string, position int) dto.TasksItemResult {
	a.recordActivity()
	var positionPtr *int
	if position >= 0 {
		positionPtr = &position
	}
	return tasksItemResult(application.UpdateTasksItem(a.vault, id, title, &body, status, positionPtr))
}

// DeleteTasksItem removes an item and its attachments.
func (a *App) DeleteTasksItem(id string) dto.TasksItemResult {
	a.recordActivity()
	return tasksItemResult(application.DeleteTasksItem(a.vault, id))
}

// AddTasksAttachment stores one image (base64-encoded; a data: URI prefix
// is tolerated) on a tasks item. Limits mirror AW2: 5 MB per image, 5
// images per item, jpeg/png/gif/webp only.
func (a *App) AddTasksAttachment(itemID string, name string, mimeType string, dataBase64 string) dto.TasksAttachmentResult {
	a.recordActivity()
	data, err := decodeAttachmentBase64(dataBase64)
	if err != nil {
		return dto.TasksAttachmentResult{Error: err.Error()}
	}
	attachment, err := application.AddTasksAttachment(a.vault, itemID, name, mimeType, data)
	if err != nil {
		return dto.TasksAttachmentResult{Error: err.Error()}
	}
	return dto.TasksAttachmentResult{Success: true, Attachment: &attachment}
}

// GetTasksAttachmentData returns one attachment's content as a data URI for
// the module view (thumbnails, lightbox, chat hand-off).
func (a *App) GetTasksAttachmentData(attachmentID string) dto.TasksAttachmentDataResult {
	a.recordActivity()
	attachment, data, err := application.GetTasksAttachment(a.vault, attachmentID)
	if err != nil {
		return dto.TasksAttachmentDataResult{Error: err.Error()}
	}
	dataURI := fmt.Sprintf("data:%s;base64,%s", attachment.MimeType, base64.StdEncoding.EncodeToString(data))
	return dto.TasksAttachmentDataResult{Success: true, Attachment: &attachment, DataURI: dataURI}
}

// DeleteTasksAttachment removes one attachment.
func (a *App) DeleteTasksAttachment(attachmentID string) dto.TasksAttachmentResult {
	a.recordActivity()
	if err := application.DeleteTasksAttachment(a.vault, attachmentID); err != nil {
		return dto.TasksAttachmentResult{Error: err.Error()}
	}
	return dto.TasksAttachmentResult{Success: true}
}

func decodeAttachmentBase64(dataBase64 string) ([]byte, error) {
	payload := strings.TrimSpace(dataBase64)
	if idx := strings.Index(payload, ","); idx >= 0 && strings.HasPrefix(payload, "data:") {
		payload = payload[idx+1:]
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("attachment data must be base64: %w", err)
	}
	return data, nil
}

func tasksItemResult(item domain.TasksItem, err error) dto.TasksItemResult {
	if err != nil {
		return dto.TasksItemResult{Error: err.Error()}
	}
	return dto.TasksItemResult{Success: true, Item: &item}
}
