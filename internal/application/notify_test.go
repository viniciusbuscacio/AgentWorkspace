package application

import (
	"strings"
	"testing"
)

// fakeNotifier records what it was asked to display.
type fakeNotifier struct {
	calls []struct{ title, body string }
}

func (f *fakeNotifier) Notify(title, body string) error {
	f.calls = append(f.calls, struct{ title, body string }{title, body})
	return nil
}

func TestNotifyUser_DisabledIsNoop(t *testing.T) {
	n := &fakeNotifier{}
	if err := NotifyUser(n, false, "aw", "Chat reply ready."); err != nil {
		t.Fatalf("unexpected error = %v", err)
	}
	if len(n.calls) != 0 {
		t.Fatalf("notifier called %d times with pref off, want 0", len(n.calls))
	}
}

func TestNotifyUser_NilNotifierIsNoop(t *testing.T) {
	if err := NotifyUser(nil, true, "aw", "hello"); err != nil {
		t.Fatalf("unexpected error = %v", err)
	}
}

func TestNotifyUser_CapApplied(t *testing.T) {
	n := &fakeNotifier{}
	// Use a repeated non-secret word so the scrubber does not redact it.
	long := strings.Repeat("hello ", 60) // 360 runes, no secret patterns
	if err := NotifyUser(n, true, "aw", long); err != nil {
		t.Fatalf("NotifyUser error = %v", err)
	}
	if len(n.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(n.calls))
	}
	got := []rune(n.calls[0].body)
	if len(got) != notifyBodyMaxRunes {
		t.Fatalf("body runes = %d, want %d", len(got), notifyBodyMaxRunes)
	}
}

func TestNotifyUser_ScrubApplied(t *testing.T) {
	n := &fakeNotifier{}
	// A Bearer token in the body must be redacted.
	body := "Done — token: Bearer sk-abcdefghijklmnopqrstuvwx"
	if err := NotifyUser(n, true, "aw", body); err != nil {
		t.Fatalf("NotifyUser error = %v", err)
	}
	if len(n.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(n.calls))
	}
	if strings.Contains(n.calls[0].body, "sk-") {
		t.Fatalf("body contains raw secret: %q", n.calls[0].body)
	}
}

func TestNotifyUser_TitleAndBodyPassThrough(t *testing.T) {
	n := &fakeNotifier{}
	if err := NotifyUser(n, true, "Agent Workspace", "Settings saved."); err != nil {
		t.Fatalf("NotifyUser error = %v", err)
	}
	if n.calls[0].title != "Agent Workspace" || n.calls[0].body != "Settings saved." {
		t.Fatalf("got title=%q body=%q", n.calls[0].title, n.calls[0].body)
	}
}
