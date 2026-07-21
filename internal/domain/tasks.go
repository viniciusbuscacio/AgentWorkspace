package domain

// Tasks item statuses (AW2 parity). The legacy todo/done values are kept
// only for the one-time vault migration (todo→open, done→completed) and are
// rejected everywhere else.
const (
	TasksStatusOpen            = "open"
	TasksStatusInProgress      = "in-progress"
	TasksStatusNeedsValidation = "needs-validation"
	TasksStatusCompleted       = "completed"

	// Legacy pre-parity statuses, mapped once by the vault migration.
	TasksStatusLegacyTodo = "todo"
	TasksStatusLegacyDone = "done"
)

// Image-attachment limits, copied from AW2 (save-tasks-attachment.ts).
// Attachments are BLOBs inside the encrypted vault, so the caps are a hard
// fence against bloating the database — exceeding them is a clear error,
// never a silent truncation.
const (
	TasksAttachmentMaxBytes = 5 * 1024 * 1024
	TasksMaxAttachments     = 5
)

// TasksStatuses lists every valid status in cycle order (open →
// in-progress → needs-validation → completed).
func TasksStatuses() []string {
	return []string{
		TasksStatusOpen,
		TasksStatusInProgress,
		TasksStatusNeedsValidation,
		TasksStatusCompleted,
	}
}

// IsTasksStatus reports whether status is one of the four valid values.
func IsTasksStatus(status string) bool {
	for _, valid := range TasksStatuses() {
		if status == valid {
			return true
		}
	}
	return false
}

// TasksAttachmentMimeTypes lists the accepted image MIME types (AW2 accepts
// jpg/jpeg, png, gif and webp files).
func TasksAttachmentMimeTypes() []string {
	return []string{"image/jpeg", "image/png", "image/gif", "image/webp"}
}

// IsTasksAttachmentMimeType reports whether mimeType is an accepted image
// type.
func IsTasksAttachmentMimeType(mimeType string) bool {
	for _, valid := range TasksAttachmentMimeTypes() {
		if mimeType == valid {
			return true
		}
	}
	return false
}

// TasksItem is one entry of the Tasks workspace module, stored in the
// vault. Position orders the list (ascending). Attachments carry metadata
// only — binary content stays in the vault and is fetched per attachment.
type TasksItem struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Status      string            `json:"status"` // open | in-progress | needs-validation | completed
	Position    int               `json:"position"`
	CreatedAt   string            `json:"createdAt,omitempty"`
	Attachments []TasksAttachment `json:"attachments"`
}

// TasksAttachment is the metadata of one image attached to a tasks item.
// The BLOB itself never rides on this struct.
type TasksAttachment struct {
	ID        string `json:"id"`
	ItemID    string `json:"itemId"`
	Name      string `json:"name"`
	MimeType  string `json:"mimeType"`
	Size      int    `json:"size"`
	CreatedAt string `json:"createdAt,omitempty"`
}
