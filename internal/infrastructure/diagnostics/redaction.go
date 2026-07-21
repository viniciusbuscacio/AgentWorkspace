package diagnostics

import (
	"regexp"
	"strings"
)

// eventMessageCap bounds a single redacted log message (spec §5.8: ~2 KB).
const eventMessageCap = 2048

var (
	reMAC      = regexp.MustCompile(`(?i)\b[0-9a-f]{2}(?:[:-][0-9a-f]{2}){5}\b`)
	reIPv4     = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	reGUID     = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	reLongHex  = regexp.MustCompile(`(?i)\b(?:0x)?[0-9a-f]{16,}\b`)
	reEmail    = regexp.MustCompile(`(?i)\b[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}\b`)
	reWinUser  = regexp.MustCompile(`(?i)([A-Z]:\\Users\\)[^\\/:*?"<>|]+`)
	reNixUser  = regexp.MustCompile(`(?i)(/(?:home|Users)/)[^/\s]+`)
	reURLQuery = regexp.MustCompile(`(?i)(https?://[^\s?]+)\?[^\s]*`)
	reWS       = regexp.MustCompile(`[ \t]+`)
)

// redactText strips MAC/IP/GUID/long-hex IDs, emails, URL query strings, and
// usernames embedded in home paths from arbitrary text (e.g. a log message). It
// never executes or trusts the text — it only sanitizes it for display.
func redactText(s string) string {
	if s == "" {
		return s
	}
	s = reURLQuery.ReplaceAllString(s, "$1?[redacted]")
	s = reWinUser.ReplaceAllString(s, "${1}[user]")
	s = reNixUser.ReplaceAllString(s, "${1}[user]")
	s = reMAC.ReplaceAllString(s, "[mac]")
	s = reEmail.ReplaceAllString(s, "[email]")
	s = reGUID.ReplaceAllString(s, "[guid]")
	s = reLongHex.ReplaceAllString(s, "[id]")
	s = reIPv4.ReplaceAllString(s, "[ip]")
	return s
}

// redactMessage redacts a log message and truncates it to the per-event cap.
func redactMessage(s string) string {
	s = strings.TrimSpace(reWS.ReplaceAllString(redactText(s), " "))
	if len(s) > eventMessageCap {
		s = s[:eventMessageCap] + "…[truncated]"
	}
	return s
}

// redactDeviceName generalizes device names that look personal (user-named
// Bluetooth/USB peripherals such as "John's iPhone") and strips embedded
// identifiers, so a redacted device inventory never leaks a person's name.
func redactDeviceName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	// "<Name>'s Device" / "<Name>’s Device" → "[user]'s Device".
	if idx := strings.IndexAny(name, "'’"); idx > 0 {
		rest := name[idx:]
		if strings.HasPrefix(rest, "'s ") || strings.HasPrefix(rest, "’s ") {
			name = "[user]" + rest
		}
	}
	name = reMAC.ReplaceAllString(name, "[mac]")
	name = reGUID.ReplaceAllString(name, "[guid]")
	return name
}

// floatPtr wraps an optional float reading; a nil pointer means "not available",
// which is distinct from a zero reading.
//
//nolint:unused // used by Linux/Windows diagnostics backends; Darwin keeps the shared package compiling.
func floatPtr(f float64) *float64 { return &f }
