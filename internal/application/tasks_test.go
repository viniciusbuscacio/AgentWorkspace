package application

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"aw/internal/domain"
)

type fakeTasksStore struct {
	unlocked    bool
	items       map[string]domain.TasksItem
	attachments map[string]domain.TasksAttachment
	blobs       map[string][]byte
	seq         int
}

func newFakeTasksStore() *fakeTasksStore {
	return &fakeTasksStore{
		unlocked:    true,
		items:       map[string]domain.TasksItem{},
		attachments: map[string]domain.TasksAttachment{},
		blobs:       map[string][]byte{},
	}
}

func (s *fakeTasksStore) IsUnlocked() bool { return s.unlocked }

func (s *fakeTasksStore) CreateTasksItem(title, body, status string) (domain.TasksItem, error) {
	s.seq++
	item := domain.TasksItem{ID: fmt.Sprintf("task-%d", s.seq), Title: title, Body: body, Status: status, Position: s.seq}
	s.items[item.ID] = item
	return item, nil
}

func (s *fakeTasksStore) ListTasksItems() ([]domain.TasksItem, error) {
	items := make([]domain.TasksItem, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	return items, nil
}

func (s *fakeTasksStore) GetTasksItem(id string) (domain.TasksItem, error) {
	item, ok := s.items[id]
	if !ok {
		return domain.TasksItem{}, errors.New("tasks item not found")
	}
	return item, nil
}

func (s *fakeTasksStore) UpdateTasksItem(id, title, body, status string, position int) (domain.TasksItem, error) {
	item, ok := s.items[id]
	if !ok {
		return domain.TasksItem{}, errors.New("tasks item not found")
	}
	item.Title, item.Body, item.Status, item.Position = title, body, status, position
	s.items[id] = item
	return item, nil
}

func (s *fakeTasksStore) DeleteTasksItem(id string) error {
	delete(s.items, id)
	return nil
}

func (s *fakeTasksStore) ListTasksAttachments(itemID string) ([]domain.TasksAttachment, error) {
	attachments := make([]domain.TasksAttachment, 0)
	for _, attachment := range s.attachments {
		if attachment.ItemID == itemID {
			attachments = append(attachments, attachment)
		}
	}
	return attachments, nil
}

func (s *fakeTasksStore) AddTasksAttachment(itemID, name, mimeType string, data []byte) (domain.TasksAttachment, error) {
	if _, ok := s.items[itemID]; !ok {
		return domain.TasksAttachment{}, errors.New("tasks item not found")
	}
	s.seq++
	attachment := domain.TasksAttachment{ID: fmt.Sprintf("att-%d", s.seq), ItemID: itemID, Name: name, MimeType: mimeType, Size: len(data)}
	s.attachments[attachment.ID] = attachment
	s.blobs[attachment.ID] = data
	return attachment, nil
}

func (s *fakeTasksStore) GetTasksAttachment(attachmentID string) (domain.TasksAttachment, []byte, error) {
	attachment, ok := s.attachments[attachmentID]
	if !ok {
		return domain.TasksAttachment{}, nil, errors.New("tasks attachment not found")
	}
	return attachment, s.blobs[attachmentID], nil
}

func (s *fakeTasksStore) DeleteTasksAttachment(attachmentID string) error {
	delete(s.attachments, attachmentID)
	delete(s.blobs, attachmentID)
	return nil
}

func TestCreateTasksItemValidatesTitleAndStatus(t *testing.T) {
	store := newFakeTasksStore()
	if _, err := CreateTasksItem(store, "  ", "", ""); err == nil {
		t.Fatal("empty title should fail")
	}
	item, err := CreateTasksItem(store, " Revisar spec ", "Corpo", "")
	if err != nil {
		t.Fatalf("CreateTasksItem() error = %v", err)
	}
	if item.Title != "Revisar spec" || item.Status != domain.TasksStatusOpen || item.Body != "Corpo" {
		t.Fatalf("item = %+v", item)
	}
	staged, err := CreateTasksItem(store, "Outra", "", domain.TasksStatusInProgress)
	if err != nil {
		t.Fatalf("CreateTasksItem(in-progress) error = %v", err)
	}
	if staged.Status != domain.TasksStatusInProgress {
		t.Fatalf("status = %q", staged.Status)
	}
	// The legacy statuses are rejected after the migration.
	if _, err := CreateTasksItem(store, "Velha", "", "todo"); err == nil {
		t.Fatal("legacy status todo should fail")
	}
}

func TestUpdateTasksItemMergesAndValidatesStatus(t *testing.T) {
	store := newFakeTasksStore()
	item, _ := CreateTasksItem(store, "Tarefa", "Corpo original", "")

	updated, err := UpdateTasksItem(store, item.ID, "", nil, domain.TasksStatusNeedsValidation, nil)
	if err != nil {
		t.Fatalf("UpdateTasksItem(status only) error = %v", err)
	}
	if updated.Title != "Tarefa" || updated.Status != domain.TasksStatusNeedsValidation ||
		updated.Body != "Corpo original" || updated.Position != item.Position {
		t.Fatalf("partial update = %+v", updated)
	}

	// Body uses presence semantics: setting an empty body clears it.
	empty := ""
	cleared, err := UpdateTasksItem(store, item.ID, "", &empty, "", nil)
	if err != nil {
		t.Fatalf("UpdateTasksItem(clear body) error = %v", err)
	}
	if cleared.Body != "" || cleared.Status != domain.TasksStatusNeedsValidation {
		t.Fatalf("cleared = %+v", cleared)
	}

	pos := 7
	moved, err := UpdateTasksItem(store, item.ID, "", nil, "", &pos)
	if err != nil {
		t.Fatalf("UpdateTasksItem(position only) error = %v", err)
	}
	if moved.Position != 7 {
		t.Fatalf("position = %d, want 7", moved.Position)
	}

	if _, err := UpdateTasksItem(store, item.ID, "", nil, "", nil); err == nil {
		t.Fatal("update with no fields should fail")
	}
	if _, err := UpdateTasksItem(store, item.ID, "", nil, "blocked", nil); err == nil {
		t.Fatal("invalid status should fail")
	}
	if _, err := UpdateTasksItem(store, item.ID, "", nil, "done", nil); err == nil {
		t.Fatal("legacy status done should fail")
	}
}

func TestGetTasksItemRequiresID(t *testing.T) {
	store := newFakeTasksStore()
	item, _ := CreateTasksItem(store, "Para ler", "", "")
	if _, err := GetTasksItem(store, " "); err == nil {
		t.Fatal("empty id should fail")
	}
	got, err := GetTasksItem(store, item.ID)
	if err != nil || got.ID != item.ID {
		t.Fatalf("GetTasksItem() = %+v, %v", got, err)
	}
}

func TestDeleteTasksItemEchoesItem(t *testing.T) {
	store := newFakeTasksStore()
	item, _ := CreateTasksItem(store, "Para apagar", "", "")
	deleted, err := DeleteTasksItem(store, item.ID)
	if err != nil {
		t.Fatalf("DeleteTasksItem() error = %v", err)
	}
	if deleted.Title != "Para apagar" {
		t.Fatalf("deleted = %+v", deleted)
	}
	if _, err := DeleteTasksItem(store, item.ID); err == nil {
		t.Fatal("deleting a missing item should fail")
	}
}

func TestAddTasksAttachmentEnforcesLimits(t *testing.T) {
	store := newFakeTasksStore()
	item, _ := CreateTasksItem(store, "Com imagens", "", "")

	attachment, err := AddTasksAttachment(store, item.ID, "shot.png", "image/png", []byte{1, 2, 3})
	if err != nil {
		t.Fatalf("AddTasksAttachment() error = %v", err)
	}
	if attachment.Size != 3 {
		t.Fatalf("attachment = %+v", attachment)
	}

	// Unsupported types fail with a clear message.
	if _, err := AddTasksAttachment(store, item.ID, "doc.pdf", "application/pdf", []byte{1}); err == nil ||
		!strings.Contains(err.Error(), "unsupported image type") {
		t.Fatalf("pdf attachment error = %v", err)
	}

	// Oversized payloads fail with a clear message (Risk 3: the vault is an
	// encrypted SQLite file — cap explicitly, never truncate).
	oversized := bytes.Repeat([]byte{0xab}, domain.TasksAttachmentMaxBytes+1)
	if _, err := AddTasksAttachment(store, item.ID, "big.png", "image/png", oversized); err == nil ||
		!strings.Contains(err.Error(), "5 MB") {
		t.Fatalf("oversized attachment error = %v", err)
	}

	// At most 5 attachments per item.
	for i := 1; i < domain.TasksMaxAttachments; i++ {
		if _, err := AddTasksAttachment(store, item.ID, "x.png", "image/png", []byte{byte(i)}); err != nil {
			t.Fatalf("attachment %d error = %v", i, err)
		}
	}
	if _, err := AddTasksAttachment(store, item.ID, "extra.png", "image/png", []byte{9}); err == nil ||
		!strings.Contains(err.Error(), "at most 5") {
		t.Fatalf("sixth attachment error = %v", err)
	}

	// Empty payloads are rejected.
	if _, err := AddTasksAttachment(store, item.ID, "empty.png", "image/png", nil); err == nil {
		t.Fatal("empty attachment should fail")
	}
}

func TestTasksAttachmentRoundTripAndDelete(t *testing.T) {
	store := newFakeTasksStore()
	item, _ := CreateTasksItem(store, "Item", "", "")
	attachment, _ := AddTasksAttachment(store, item.ID, "a.png", "image/png", []byte{1, 2})

	meta, data, err := GetTasksAttachment(store, attachment.ID)
	if err != nil || meta.ID != attachment.ID || !bytes.Equal(data, []byte{1, 2}) {
		t.Fatalf("GetTasksAttachment() = %+v, %v, %v", meta, data, err)
	}
	if err := DeleteTasksAttachment(store, attachment.ID); err != nil {
		t.Fatalf("DeleteTasksAttachment() error = %v", err)
	}
	if _, _, err := GetTasksAttachment(store, attachment.ID); err == nil {
		t.Fatal("attachment should be gone")
	}
	if err := DeleteTasksAttachment(store, " "); err == nil {
		t.Fatal("empty attachment id should fail")
	}
}

func TestTasksRequiresUnlockedVault(t *testing.T) {
	store := newFakeTasksStore()
	store.unlocked = false
	if _, err := CreateTasksItem(store, "x", "", ""); !errors.Is(err, errVaultLocked) {
		t.Fatalf("CreateTasksItem error = %v, want errVaultLocked", err)
	}
	if _, err := ListTasksItems(store); !errors.Is(err, errVaultLocked) {
		t.Fatalf("ListTasksItems error = %v, want errVaultLocked", err)
	}
	if _, err := AddTasksAttachment(store, "task-1", "a.png", "image/png", []byte{1}); !errors.Is(err, errVaultLocked) {
		t.Fatalf("AddTasksAttachment error = %v, want errVaultLocked", err)
	}
}
