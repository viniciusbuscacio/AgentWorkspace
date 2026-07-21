package vault

import "testing"

// Regression fence for chat-identity-spec Decision 5: the schema guarantees
// every message belongs to exactly one session (chat) and dies with it.
// Verified sound on 2026-06-12 — this test keeps it true.
func TestMessagesScopedToSessionAndCascadeOnDelete(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	chatA, err := v.CreateChat("Chat A")
	if err != nil {
		t.Fatalf("CreateChat(A) error = %v", err)
	}
	chatB, err := v.CreateChat("Chat B")
	if err != nil {
		t.Fatalf("CreateChat(B) error = %v", err)
	}
	if _, err := v.AddMessage(chatA.ID, "user", "message in A"); err != nil {
		t.Fatalf("AddMessage(A) error = %v", err)
	}
	if _, err := v.AddMessage(chatB.ID, "user", "message in B"); err != nil {
		t.Fatalf("AddMessage(B) error = %v", err)
	}

	// Loads are strictly session-scoped: A's messages never include B's.
	inA, err := v.ListMessages(chatA.ID)
	if err != nil {
		t.Fatalf("ListMessages(A) error = %v", err)
	}
	if len(inA) != 1 || inA[0].Content != "message in A" {
		t.Fatalf("ListMessages(A) = %+v, want only A's message", inA)
	}

	// Deleting a chat removes its messages and ONLY its messages.
	if err := v.DeleteChat(chatA.ID); err != nil {
		t.Fatalf("DeleteChat(A) error = %v", err)
	}
	goneA, err := v.ListMessages(chatA.ID)
	if err != nil {
		t.Fatalf("ListMessages(deleted A) error = %v", err)
	}
	if len(goneA) != 0 {
		t.Fatalf("deleted chat still has %d message(s)", len(goneA))
	}
	stillB, err := v.ListMessages(chatB.ID)
	if err != nil {
		t.Fatalf("ListMessages(B) error = %v", err)
	}
	if len(stillB) != 1 || stillB[0].Content != "message in B" {
		t.Fatalf("ListMessages(B) after deleting A = %+v", stillB)
	}
}
