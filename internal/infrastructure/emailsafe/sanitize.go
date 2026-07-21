// Package emailsafe preserves the Gmail safe-reading API while delegating the
// actual external-content quarantine work to externalsafe.
package emailsafe

import "aw/internal/infrastructure/externalsafe"

const DefaultMaxChars = externalsafe.DefaultMaxChars

type Result = externalsafe.Result

func Sanitize(raw string) Result {
	return externalsafe.SanitizeEmail(raw)
}

func SanitizeWithLimit(raw string, maxChars int) Result {
	return externalsafe.SanitizeContent(externalsafe.Input{
		SourceType: externalsafe.SourceEmail,
		Content:    raw,
		MaxChars:   maxChars,
	})
}
