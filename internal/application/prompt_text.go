package application

import (
	"regexp"
	"strings"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;:?]*[A-Za-z]?`)

func promptText(value string) string {
	value = ScrubChatSecrets(value)
	value = ansiEscapePattern.ReplaceAllString(value, "")
	value = strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t' || r == '\r':
			return r
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			return -1
		default:
			return r
		}
	}, value)
	return value
}
