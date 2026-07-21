package vault

import (
	"testing"

	"aw/internal/domain"
)

func TestWebObservationsReplacePerChatBrowserSiteAndClearOnUnlock(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	first := domain.WebObservation{
		ID:          "obs-1",
		ChatID:      "chat-1",
		Browser:     "edge",
		SiteKey:     "https://mail.google.com",
		CapturedAt:  "2026-06-21T10:00:00Z",
		PayloadJSON: `{"items":[{"id":"obs-1"}]}`,
	}
	if _, err := v.SaveWebObservation(first); err != nil {
		t.Fatalf("SaveWebObservation(first) error = %v", err)
	}
	second := first
	second.ID = "obs-2"
	second.CapturedAt = "2026-06-21T10:01:00Z"
	second.PayloadJSON = `{"items":[{"id":"obs-2"}]}`
	if _, err := v.SaveWebObservation(second); err != nil {
		t.Fatalf("SaveWebObservation(second) error = %v", err)
	}
	got, ok, err := v.LatestWebObservation("chat-1", "edge", "https://mail.google.com")
	if err != nil {
		t.Fatalf("LatestWebObservation() error = %v", err)
	}
	if !ok || got.ID != "obs-2" {
		t.Fatalf("latest = %+v, ok=%v; want obs-2", got, ok)
	}

	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if err := v.Unlock("senha1234"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	defer func() { _ = v.Lock() }()
	if got, ok, err := v.LatestWebObservation("chat-1", "edge", "https://mail.google.com"); err != nil || ok {
		t.Fatalf("latest after unlock = %+v, ok=%v, err=%v; want cleared", got, ok, err)
	}
}
