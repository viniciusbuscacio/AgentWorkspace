package application

import (
	"strings"

	"aw/internal/domain/ports"
)

// notifyBodyMaxRunes is the maximum body length before truncation.
// Decision 4: 200-rune cap, scrubbed with the existing secret scrubber.
const notifyBodyMaxRunes = 200

// NotifyUser delivers a transient desktop notification. The preference flag
// is checked here so callers never need to — a disabled pref is a transparent
// no-op. The body is capped to notifyBodyMaxRunes and scrubbed via
// ScrubChatSecrets before dispatch.
//
// A nil notifier or enabled=false is a silent no-op (not an error), which
// lets callers pass the live notifier unconditionally.
func NotifyUser(notifier ports.Notifier, enabled bool, title, body string) error {
	if notifier == nil || !enabled {
		return nil
	}
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	body = ScrubChatSecrets(body)
	runes := []rune(body)
	if len(runes) > notifyBodyMaxRunes {
		body = string(runes[:notifyBodyMaxRunes])
	}
	return notifier.Notify(title, body)
}
