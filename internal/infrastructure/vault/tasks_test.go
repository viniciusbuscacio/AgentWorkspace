package vault

import (
	"bytes"
	"testing"

	"aw/internal/domain"
)

func TestTasksCrudRoundTrip(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	first, err := v.CreateTasksItem("Revisar spec", "corpo da spec", domain.TasksStatusOpen)
	if err != nil {
		t.Fatalf("CreateTasksItem() error = %v", err)
	}
	second, err := v.CreateTasksItem("Implementar fase 4", "", domain.TasksStatusOpen)
	if err != nil {
		t.Fatalf("CreateTasksItem() error = %v", err)
	}
	if first.Status != domain.TasksStatusOpen || second.Position != first.Position+1 {
		t.Fatalf("positions/status wrong: first=%+v second=%+v", first, second)
	}
	if first.Body != "corpo da spec" {
		t.Fatalf("body not persisted: %+v", first)
	}

	done, err := v.UpdateTasksItem(first.ID, first.Title, first.Body, domain.TasksStatusCompleted, first.Position)
	if err != nil {
		t.Fatalf("UpdateTasksItem() error = %v", err)
	}
	if done.Status != domain.TasksStatusCompleted {
		t.Fatalf("status = %q, want completed", done.Status)
	}
	if _, err := v.UpdateTasksItem("task-missing", "x", "", domain.TasksStatusOpen, 1); err == nil {
		t.Fatal("UpdateTasksItem() on missing item should fail")
	}

	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if _, err := v.ListTasksItems(); err == nil {
		t.Fatal("ListTasksItems() on locked vault succeeded")
	}
	if err := v.Unlock("senha1234"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	items, err := v.ListTasksItems()
	if err != nil {
		t.Fatalf("ListTasksItems() error = %v", err)
	}
	if len(items) != 2 || items[0].ID != first.ID || items[1].ID != second.ID {
		t.Fatalf("items = %+v, want position order", items)
	}
	if items[0].Status != domain.TasksStatusCompleted {
		t.Fatalf("status lost after lock/unlock: %+v", items[0])
	}

	if err := v.DeleteTasksItem(first.ID); err != nil {
		t.Fatalf("DeleteTasksItem() error = %v", err)
	}
	items, _ = v.ListTasksItems()
	if len(items) != 1 {
		t.Fatalf("items after delete = %d, want 1", len(items))
	}
}

// TestTasksLegacyStatusMigration verifies the one-time todo->open and
// done->completed remap runs on unlock (a vault written before parity).
func TestTasksLegacyStatusMigration(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	// Write legacy rows directly, bypassing the validated create path.
	v.mu.Lock()
	if _, err := v.db.Exec(
		`INSERT INTO backlog_items (id, title, body, status, position, created_at) VALUES
		  ('task-old1', 'antiga todo', '', 'todo', 1, ''),
		  ('task-old2', 'antiga done', '', 'done', 2, '')`,
	); err != nil {
		v.mu.Unlock()
		t.Fatalf("seed legacy rows error = %v", err)
	}
	v.mu.Unlock()

	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if err := v.Unlock("senha1234"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	items, err := v.ListTasksItems()
	if err != nil {
		t.Fatalf("ListTasksItems() error = %v", err)
	}
	got := map[string]string{}
	for _, item := range items {
		got[item.ID] = item.Status
	}
	if got["task-old1"] != domain.TasksStatusOpen {
		t.Fatalf("todo not migrated: %q", got["task-old1"])
	}
	if got["task-old2"] != domain.TasksStatusCompleted {
		t.Fatalf("done not migrated: %q", got["task-old2"])
	}
}

func TestTasksAttachmentsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	item, err := v.CreateTasksItem("com anexo", "", domain.TasksStatusOpen)
	if err != nil {
		t.Fatalf("CreateTasksItem() error = %v", err)
	}

	payload := []byte{0x89, 0x50, 0x4e, 0x47}
	att, err := v.AddTasksAttachment(item.ID, "shot.png", "image/png", payload)
	if err != nil {
		t.Fatalf("AddTasksAttachment() error = %v", err)
	}
	if att.Size != len(payload) {
		t.Fatalf("size = %d, want %d", att.Size, len(payload))
	}

	// Attachment metadata rides on the item, never the BLOB.
	got, err := v.GetTasksItem(item.ID)
	if err != nil {
		t.Fatalf("GetTasksItem() error = %v", err)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].ID != att.ID {
		t.Fatalf("attachments = %+v, want one", got.Attachments)
	}

	meta, data, err := v.GetTasksAttachment(att.ID)
	if err != nil {
		t.Fatalf("GetTasksAttachment() error = %v", err)
	}
	if meta.Name != "shot.png" || !bytes.Equal(data, payload) {
		t.Fatalf("attachment roundtrip wrong: meta=%+v data=%v", meta, data)
	}

	// Adding to a missing item fails.
	if _, err := v.AddTasksAttachment("task-missing", "x.png", "image/png", payload); err == nil {
		t.Fatal("AddTasksAttachment() on missing item should fail")
	}

	// Deleting the item cascades to its attachments.
	if err := v.DeleteTasksItem(item.ID); err != nil {
		t.Fatalf("DeleteTasksItem() error = %v", err)
	}
	if _, _, err := v.GetTasksAttachment(att.ID); err == nil {
		t.Fatal("attachment should be gone after item delete")
	}
}
