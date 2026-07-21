package application

import "regexp"

var chatSecretPatterns = []struct {
	re          *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{16,}\b`), "Bearer [redacted]"},
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`), "[redacted]"},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "[redacted]"},
	{regexp.MustCompile(`(?i)\b(password|passwd|pwd|api[_-]?key|secret|token)\s*[:=]\s*("[^"\s]{4,}"|'[^'\s]{4,}'|[^\s,;&]{4,})`), "$1=[redacted]"},
	{regexp.MustCompile(`\b[0-9a-fA-F]{32,}\b`), "[redacted]"},
	{regexp.MustCompile(`\b[A-Za-z0-9+/=_-]{40,}\b`), "[redacted]"},
}

// ScrubChatSecrets masks obvious secret-looking values before chat-derived
// summaries are sent to a model or persisted. It is intentionally heuristic.
func ScrubChatSecrets(value string) string {
	for _, pattern := range chatSecretPatterns {
		value = pattern.re.ReplaceAllString(value, pattern.replacement)
	}
	return value
}
