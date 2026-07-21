package application

import (
	"errors"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

const (
	// userMemoryPromptCap is the maximum rune count injected into the agent prompt.
	userMemoryPromptCap = 8000
	// userMemoryCondenseThreshold is the document size (runes) that triggers
	// a one-shot LLM condensation on unlock (checked at most once per day).
	UserMemoryCondenseThreshold = 6000
)

// GetUserMemoryDoc returns the v2 living document for the Settings page.
func GetUserMemoryDoc(store ports.UserMemoryDocStore) (domain.UserMemoryDoc, error) {
	if store == nil {
		return domain.UserMemoryDoc{}, errors.New("user memory store is required")
	}
	if !store.IsUnlocked() {
		return domain.UserMemoryDoc{}, errVaultLocked
	}
	return store.GetUserMemoryDoc()
}

// SetUserMemoryDoc saves an edited document from the Settings page. The
// content is scrubbed for secrets before persist.
func SetUserMemoryDoc(store ports.UserMemoryDocStore, content string) (domain.UserMemoryDoc, error) {
	if store == nil {
		return domain.UserMemoryDoc{}, errors.New("user memory store is required")
	}
	if !store.IsUnlocked() {
		return domain.UserMemoryDoc{}, errVaultLocked
	}
	content = ScrubChatSecrets(strings.TrimSpace(content))
	doc, err := store.GetUserMemoryDoc()
	if err != nil {
		return domain.UserMemoryDoc{}, err
	}
	doc.Content = content
	return store.SetUserMemoryDoc(doc)
}

// AppendUserMemoryLine appends one line to the living document. Called by
// the aw action memory.remember. Scrubs secrets before persist.
func AppendUserMemoryLine(store ports.UserMemoryDocStore, key, category, content string) (domain.UserMemoryDoc, error) {
	if store == nil {
		return domain.UserMemoryDoc{}, errors.New("user memory store is required")
	}
	if !store.IsUnlocked() {
		return domain.UserMemoryDoc{}, errVaultLocked
	}
	key = NormalizeUserFactKey(key)
	content = strings.TrimSpace(content)
	if key == "" || content == "" {
		return domain.UserMemoryDoc{}, errors.New("key and content are required")
	}
	// Build the line (include category when provided for human readability).
	category = strings.ToLower(strings.TrimSpace(category))
	var line string
	if category != "" {
		line = "- [" + category + "] " + key + ": " + content
	} else {
		line = "- " + key + ": " + content
	}
	line = ScrubChatSecrets(line)

	doc, err := store.GetUserMemoryDoc()
	if err != nil {
		return domain.UserMemoryDoc{}, err
	}
	// True upsert: replace the first line carrying this key (any category)
	// and drop other duplicates — repeated remembers must never stack lines.
	lines := strings.Split(strings.TrimSpace(doc.Content), "\n")
	out := make([]string, 0, len(lines)+1)
	replaced := false
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if memoryLineHasKey(l, key) {
			if !replaced {
				out = append(out, line)
				replaced = true
			}
			continue
		}
		out = append(out, l)
	}
	if !replaced {
		out = append(out, line)
	}
	doc.Content = strings.Join(out, "\n")
	return store.SetUserMemoryDoc(doc)
}

// RemoveUserMemoryLine deletes every line whose key matches (any category).
// Backs the aw action memory.forget. Returns how many lines were removed.
func RemoveUserMemoryLine(store ports.UserMemoryDocStore, key string) (int, error) {
	if store == nil {
		return 0, errors.New("user memory store is required")
	}
	if !store.IsUnlocked() {
		return 0, errVaultLocked
	}
	key = NormalizeUserFactKey(key)
	if key == "" {
		return 0, errors.New("key is required")
	}
	doc, err := store.GetUserMemoryDoc()
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(doc.Content), "\n")
	out := make([]string, 0, len(lines))
	removed := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if memoryLineHasKey(l, key) {
			removed++
			continue
		}
		out = append(out, l)
	}
	if removed == 0 {
		return 0, nil
	}
	doc.Content = strings.Join(out, "\n")
	if _, err := store.SetUserMemoryDoc(doc); err != nil {
		return 0, err
	}
	return removed, nil
}

// memoryLineHasKey reports whether a doc line ("- [cat] key: ...", "- key: ...")
// carries the given normalized key, ignoring the category.
func memoryLineHasKey(line, key string) bool {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
	if strings.HasPrefix(trimmed, "[") {
		if end := strings.Index(trimmed, "]"); end >= 0 {
			trimmed = strings.TrimSpace(trimmed[end+1:])
		}
	}
	if i := strings.Index(trimmed, ":"); i >= 0 {
		trimmed = trimmed[:i]
	}
	return NormalizeUserFactKey(trimmed) == key
}

// UserMemoryInstruction renders the living document as a labeled context block
// for the agent system prompt. Returns "" when the document is empty.
// Caps at userMemoryPromptCap runes; truncation is logged.
func UserMemoryInstruction(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	header := "## Information loaded from Settings → Memory\n" +
		"Background information about the user, maintained across chats:\n"
	full := header + content
	runes := []rune(full)
	if len(runes) > userMemoryPromptCap {
		full = string(runes[:userMemoryPromptCap])
	}
	return full
}

// NormalizeUserFactKey turns free text into the stable slug used as the
// line key: lowercase, words joined by hyphens.
func NormalizeUserFactKey(key string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(key))), "-")
}
