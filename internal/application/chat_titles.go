package application

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"aw/internal/domain"
)

var numberedChatTitleRe = regexp.MustCompile(`(?i)^Chat\s+(\d+)$`)

// ShouldUseNumberedChatTitle reports whether a requested title is empty or a
// generic placeholder and should be replaced by the next "Chat N" title.
func ShouldUseNumberedChatTitle(requested string) bool {
	trimmed := strings.TrimSpace(requested)
	return trimmed == "" || strings.EqualFold(trimmed, "Chat") || strings.EqualFold(trimmed, "New Chat")
}

// NextNumberedChatTitle returns "Chat N" where N is the smallest positive
// integer not already used by an open, non-archived "Chat <number>".
func NextNumberedChatTitle(chats []domain.Chat) string {
	used := make(map[int]bool)
	for _, chat := range chats {
		if chat.Archived {
			continue
		}
		m := numberedChatTitleRe.FindStringSubmatch(strings.TrimSpace(chat.Title))
		if m == nil {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			used[n] = true
		}
	}
	n := 1
	for used[n] {
		n++
	}
	return fmt.Sprintf("Chat %d", n)
}

func ResolveInitialChatTitle(requested string, existing []domain.Chat) string {
	if !ShouldUseNumberedChatTitle(requested) {
		return strings.TrimSpace(requested)
	}
	return NextNumberedChatTitle(existing)
}
