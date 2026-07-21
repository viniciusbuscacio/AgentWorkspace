package ports

import "aw/internal/domain"

// TasksStore persists the Tasks module data in the vault. Attachment
// binary content travels separately from item/attachment metadata so list
// results never carry BLOBs.
type TasksStore interface {
	VaultUnlockState
	CreateTasksItem(title, body, status string) (domain.TasksItem, error)
	ListTasksItems() ([]domain.TasksItem, error)
	GetTasksItem(id string) (domain.TasksItem, error)
	UpdateTasksItem(id, title, body, status string, position int) (domain.TasksItem, error)
	DeleteTasksItem(id string) error
	ListTasksAttachments(itemID string) ([]domain.TasksAttachment, error)
	AddTasksAttachment(itemID, name, mimeType string, data []byte) (domain.TasksAttachment, error)
	GetTasksAttachment(attachmentID string) (domain.TasksAttachment, []byte, error)
	DeleteTasksAttachment(attachmentID string) error
}
