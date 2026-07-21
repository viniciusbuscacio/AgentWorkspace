package vault

import "testing"

func TestChatModelOverrideRoundTrip(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	chat, err := v.CreateChat("Override chat")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}

	// A fresh chat has no override.
	provider, model, err := v.ChatModelOverride(chat.ID)
	if err != nil {
		t.Fatalf("ChatModelOverride() error = %v", err)
	}
	if provider != "" || model != "" {
		t.Fatalf("new chat should have no override, got %q/%q", provider, model)
	}

	if err := v.SetChatModelOverride(chat.ID, "openai", "gpt-5.5"); err != nil {
		t.Fatalf("SetChatModelOverride() error = %v", err)
	}
	provider, model, err = v.ChatModelOverride(chat.ID)
	if err != nil {
		t.Fatalf("ChatModelOverride() error = %v", err)
	}
	if provider != "openai" || model != "gpt-5.5" {
		t.Fatalf("override = %q/%q, want openai/gpt-5.5", provider, model)
	}

	// The override survives lock/unlock (persisted on the session row).
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if err := v.Unlock("senha1234"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	provider, model, err = v.ChatModelOverride(chat.ID)
	if err != nil {
		t.Fatalf("ChatModelOverride() after unlock error = %v", err)
	}
	if provider != "openai" || model != "gpt-5.5" {
		t.Fatalf("override after unlock = %q/%q, want openai/gpt-5.5", provider, model)
	}

	// Empty values clear it.
	if err := v.SetChatModelOverride(chat.ID, "", ""); err != nil {
		t.Fatalf("SetChatModelOverride(clear) error = %v", err)
	}
	provider, model, _ = v.ChatModelOverride(chat.ID)
	if provider != "" || model != "" {
		t.Fatalf("cleared override should be empty, got %q/%q", provider, model)
	}
}
