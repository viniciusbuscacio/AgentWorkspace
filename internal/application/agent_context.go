package application

import (
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// NotesPromptBlock builds the "Information loaded from the Notes module"
// block for the agent system
// context. Only non-archived notes with InPrompt set are included.
// Order: pinned first, then most recently updated (matches ListNotes order).
// Per-note cap: 4000 runes; whole-block cap: 12000 runes; truncation appends
// [truncated]. Archived notes are never injected (Decision 9).
func NotesPromptBlock(notes []domain.Note) string {
	const (
		perNoteCap = 4000
		blockCap   = 12000
	)

	var candidates []domain.Note
	for _, n := range notes {
		if !n.Archived && n.InPrompt {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) == 0 {
		return ""
	}

	parts := make([]string, 0, len(candidates))
	for _, n := range candidates {
		entry := fmt.Sprintf("### Note: %s\n%s", n.Title, n.Content)
		if r := []rune(entry); len(r) > perNoteCap {
			entry = string(r[:perNoteCap]) + "[truncated]"
		}
		parts = append(parts, entry)
	}

	block := "## Information loaded from the Notes module\n" +
		"Notes the user flagged \"Insert into agent prompt\":\n\n" + strings.Join(parts, "\n\n")
	if r := []rune(block); len(r) > blockCap {
		block = string(r[:blockCap]) + "[truncated]"
	}
	return block
}

// RefreshAgentContext recomputes the dynamic context — the workspace-modules
// block, the user-memory document block, the user notes block, the permissions
// (Filesystem Access) block and the past-session chat catalog — and pushes all
// of them, composed, through a single SetMemoryContext call. SetMemoryContext
// takes one block, so the sources must be joined here; refreshing one of them
// separately would clobber the others.
//
// It is best-effort: a locked vault clears the block, a chat-catalog error
// leaves the previous context untouched, and a modules/user-memory/sandbox
// error just omits that block.
func RefreshAgentContext(
	setter ports.MemoryContextSetter,
	chatStore ports.ChatMemoryStore,
	userStore ports.UserMemoryDocStore,
	notesStore ports.NotesStore,
	moduleStore ports.WorkspaceModuleStore,
	catalog []domain.ModuleSpec,
	subagentStore ports.SubagentModeStore,
	sandboxPrompt SandboxPromptInput,
	browserSnapshot BrowserSnapshotInput,
	obsidianBlock string,
	currentSessionID string,
) {
	if setter == nil || chatStore == nil {
		return
	}
	if !chatStore.IsUnlocked() {
		setter.SetMemoryContext("")
		return
	}
	entries, err := GetSessionCatalog(chatStore, currentSessionID, 0)
	if err != nil {
		return
	}
	blocks := make([]string, 0, 5)
	if moduleStore != nil {
		if added, err := AddedModuleSpecs(moduleStore, catalog); err == nil {
			if block := ModulesInstructionWithCatalog(added, catalog); block != "" {
				blocks = append(blocks, block)
			}
		}
	}
	if block := SubagentInstruction(LoadSubagentMode(subagentStore)); block != "" {
		blocks = append(blocks, block)
	}
	if userStore != nil {
		if doc, err := userStore.GetUserMemoryDoc(); err == nil {
			if block := UserMemoryInstruction(doc.Content); block != "" {
				blocks = append(blocks, block)
			}
		}
	}
	if notesStore != nil && notesStore.IsUnlocked() {
		if notesAll, err := notesStore.ListNotes(false); err == nil {
			if block := NotesPromptBlock(notesAll); block != "" {
				blocks = append(blocks, block)
			}
		}
	}
	if block := SandboxPromptBlock(sandboxPrompt); block != "" {
		blocks = append(blocks, block)
	}
	if block := BrowserSnapshotPromptBlock(browserSnapshot); block != "" {
		blocks = append(blocks, block)
	}
	// Obsidian always-read notes: rendered by the composition root (file I/O
	// lives in infrastructure); empty while the module is off or unset.
	if obsidianBlock != "" {
		blocks = append(blocks, obsidianBlock)
	}
	blocks = append(blocks, ChatMemoryInstruction(entries))
	setter.SetMemoryContext(promptText(strings.Join(blocks, "\n\n")))
}
