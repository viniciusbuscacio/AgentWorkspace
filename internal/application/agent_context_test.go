package application

import (
	"strings"
	"testing"

	"aw/internal/domain"
)

func TestRefreshAgentContextComposesUserMemoryAndChatCatalog(t *testing.T) {
	chatStore := newMemoryStore()
	userStore := newFakeUserMemoryDocStore()
	userStore.doc = domain.UserMemoryDoc{Content: "- preferred-language: Responde em português."}
	setter := &fakeMemoryContextSetter{}

	RefreshAgentContext(setter, chatStore, userStore, nil, &fakeModuleStore{}, testCatalog(), fakeSubagentModeStore{mode: SubagentModeOff}, SandboxPromptInput{}, BrowserSnapshotInput{}, "", "current")

	if setter.calls != 1 {
		t.Fatalf("setter calls = %d, want 1 (one composed push)", setter.calls)
	}
	if !strings.Contains(setter.lastCtx, "Information loaded from Settings → Memory") ||
		!strings.Contains(setter.lastCtx, "preferred-language") {
		t.Fatalf("memory context missing user-memory block: %q", setter.lastCtx)
	}
	if !strings.Contains(setter.lastCtx, "memory.chat.search") ||
		!strings.Contains(setter.lastCtx, "chat-a") {
		t.Fatalf("memory context missing chat-memory block: %q", setter.lastCtx)
	}
	userIdx := strings.Index(setter.lastCtx, "Information loaded from Settings → Memory")
	chatIdx := strings.Index(setter.lastCtx, "memory.chat.search")
	if userIdx > chatIdx {
		t.Fatalf("user-memory block should come before the chat catalog: %q", setter.lastCtx)
	}
}

func TestRefreshAgentContextWithEmptyUserMemoryKeepsChatBlockOnly(t *testing.T) {
	chatStore := newMemoryStore()
	userStore := newFakeUserMemoryDocStore() // empty doc
	setter := &fakeMemoryContextSetter{}

	RefreshAgentContext(setter, chatStore, userStore, nil, &fakeModuleStore{}, testCatalog(), fakeSubagentModeStore{mode: SubagentModeOff}, SandboxPromptInput{}, BrowserSnapshotInput{}, "", "current")

	if setter.calls != 1 {
		t.Fatalf("setter calls = %d, want 1", setter.calls)
	}
	if strings.Contains(setter.lastCtx, "Information loaded from Settings → Memory") {
		t.Fatalf("empty user memory should not inject a block: %q", setter.lastCtx)
	}
	if !strings.Contains(setter.lastCtx, "memory.chat.search") || !strings.Contains(setter.lastCtx, "chat-a") {
		t.Fatalf("chat-memory block regressed: %q", setter.lastCtx)
	}
}

func TestRefreshAgentContextIncludesSubagentInstruction(t *testing.T) {
	chatStore := newMemoryStore()
	setter := &fakeMemoryContextSetter{}

	RefreshAgentContext(setter, chatStore, newFakeUserMemoryDocStore(), nil, &fakeModuleStore{}, testCatalog(), fakeSubagentModeStore{mode: SubagentModeAggressive}, SandboxPromptInput{}, BrowserSnapshotInput{}, "", "current")

	if setter.calls != 1 {
		t.Fatalf("setter calls = %d, want 1", setter.calls)
	}
	for _, want := range []string{
		"## Generic subagents",
		"Aggressive mode",
		"Use at most 5 parallel subagents",
		"::subagent{task=",
	} {
		if !strings.Contains(setter.lastCtx, want) {
			t.Fatalf("subagent context missing %q: %q", want, setter.lastCtx)
		}
	}
}

func TestRefreshAgentContextClearsWhenLocked(t *testing.T) {
	chatStore := newMemoryStore()
	chatStore.unlocked = false
	setter := &fakeMemoryContextSetter{}

	RefreshAgentContext(setter, chatStore, newFakeUserMemoryDocStore(), nil, &fakeModuleStore{}, testCatalog(), fakeSubagentModeStore{mode: SubagentModeOff}, SandboxPromptInput{}, BrowserSnapshotInput{}, "", "current")

	if setter.calls != 1 || setter.lastCtx != "" {
		t.Fatalf("locked vault should clear context: calls=%d ctx=%q", setter.calls, setter.lastCtx)
	}
}

func TestRefreshAgentContextIncludesModulesBlockAndTogglesInProcess(t *testing.T) {
	chatStore := newMemoryStore()
	moduleStore := &fakeModuleStore{ids: []string{"notes"}}
	catalog := []domain.ModuleSpec{
		{ID: "chat", Name: "Chat", Core: true},
		{
			ID: "notes", Name: "Notes", Description: "Personal notes.",
			Prompt:  "Use notes for durable text the user wants to keep.",
			Actions: []domain.ModuleAction{{Name: "notes.list", Args: "{}", Summary: "List notes."}},
		},
	}
	setter := &fakeMemoryContextSetter{}

	RefreshAgentContext(setter, chatStore, newFakeUserMemoryDocStore(), nil, moduleStore, catalog, fakeSubagentModeStore{mode: SubagentModeOff}, SandboxPromptInput{}, BrowserSnapshotInput{}, "", "current")
	if !strings.Contains(setter.lastCtx, "## Workspace modules") ||
		!strings.Contains(setter.lastCtx, "### Notes (notes, added — removable)") ||
		!strings.Contains(setter.lastCtx, "notes.list {} — List notes.") {
		t.Fatalf("modules block missing or incomplete: %q", setter.lastCtx)
	}
	modulesIdx := strings.Index(setter.lastCtx, "## Workspace modules")
	chatIdx := strings.Index(setter.lastCtx, "memory.chat.search")
	if modulesIdx > chatIdx {
		t.Fatalf("modules block should come before the chat catalog: %q", setter.lastCtx)
	}

	// Removing the module changes the next composed context in-process.
	moduleStore.ids = nil
	RefreshAgentContext(setter, chatStore, newFakeUserMemoryDocStore(), nil, moduleStore, catalog, fakeSubagentModeStore{mode: SubagentModeOff}, SandboxPromptInput{}, BrowserSnapshotInput{}, "", "current")
	if strings.Contains(setter.lastCtx, "notes.list") {
		t.Fatalf("removed module still documented: %q", setter.lastCtx)
	}
}

func TestRefreshAgentContextIncludesFreshBrowserSnapshot(t *testing.T) {
	chatStore := newMemoryStore()
	setter := &fakeMemoryContextSetter{}

	RefreshAgentContext(setter, chatStore, newFakeUserMemoryDocStore(), nil, &fakeModuleStore{}, testCatalog(), fakeSubagentModeStore{mode: SubagentModeOff}, SandboxPromptInput{}, BrowserSnapshotInput{
		RefreshedAt: "2026-06-13T10:30:00Z",
		Browsers: []BrowserSnapshotBrowser{{
			ID:   domain.BrowserModuleEdge,
			Name: "Microsoft Edge",
			Status: domain.BrowserStatus{
				ID:         domain.BrowserModuleEdge,
				Running:    true,
				ProfileDir: "/data/browser-profiles/browser-edge",
				Port:       9323,
			},
			Tabs: []domain.BrowserTab{
				{ID: "t1", Title: "ChatGPT", URL: "https://chatgpt.com/c/secret-token-should-not-show"},
				{ID: "t2", Title: "GitHub", URL: "https://github.com/openai/codex/pulls/1"},
			},
		}},
	}, "", "current")

	for _, want := range []string{
		"## Browser snapshot",
		"Fresh turn-start snapshot",
		"Microsoft Edge",
		"Profile: aw",
		"Open tabs: 2",
		"ChatGPT — https://chatgpt.com",
		"GitHub — https://github.com",
	} {
		if !strings.Contains(setter.lastCtx, want) {
			t.Fatalf("browser snapshot missing %q:\n%s", want, setter.lastCtx)
		}
	}
	if strings.Contains(setter.lastCtx, "secret-token-should-not-show") {
		t.Fatalf("browser snapshot should not include full sensitive URL: %q", setter.lastCtx)
	}
}

func TestBrowserSnapshotLabelsUnknownProfileDir(t *testing.T) {
	got := BrowserSnapshotPromptBlock(BrowserSnapshotInput{
		Browsers: []BrowserSnapshotBrowser{{
			ID:   domain.BrowserModuleEdge,
			Name: "Microsoft Edge",
			Status: domain.BrowserStatus{
				ID:         domain.BrowserModuleEdge,
				Running:    true,
				ProfileDir: "/somewhere/else",
				Notice:     "Connected to an already-running browser endpoint.",
			},
			Tabs: []domain.BrowserTab{{ID: "t1", Title: "Inbox", URL: "https://mail.google.com/mail/u/0/#inbox"}},
		}},
	})

	for _, want := range []string{
		"Profile: unknown",
		"Notice: Connected to an already-running browser endpoint.",
		"Inbox — https://mail.google.com",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("browser snapshot missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Profile: aw") {
		t.Fatalf("a non-aw profile dir must not be labeled aw:\n%s", got)
	}
}

func TestPromptBlocksStripControlCharacters(t *testing.T) {
	got := ModulesInstruction([]domain.ModuleSpec{{
		ID:          "browser-edge",
		Name:        "Agent Browser\x1b[118;1:3",
		Description: "Drive\x00 pages.",
		Prompt:      "Use title\x1b[118;1:3 safely.",
		Actions: []domain.ModuleAction{{
			Name:    "browser.close_tab",
			Args:    "{titl\x1b[118;1:3ue?}",
			Summary: "Close\x7f a tab.",
		}},
	}})
	if strings.Contains(got, "\x1b") || strings.Contains(got, "\x00") || strings.Contains(got, "\x7f") || strings.Contains(got, "[118;1:3") {
		t.Fatalf("module prompt should strip control characters: %q", got)
	}
	if !strings.Contains(got, "{title?}") {
		t.Fatalf("sanitized prompt should preserve useful text: %q", got)
	}
}

func TestRefreshAgentContextStripsControlCharactersFromFinalContext(t *testing.T) {
	chatStore := newMemoryStore()
	moduleStore := &fakeModuleStore{ids: []string{"browser-edge"}}
	setter := &fakeMemoryContextSetter{}
	catalog := []domain.ModuleSpec{
		{ID: "chat", Name: "Chat", Core: true},
		{
			ID:          "browser-edge",
			Name:        "Agent Browser",
			Description: "Drives Edge.",
			Actions: []domain.ModuleAction{{
				Name:    "browser.close_tab",
				Args:    "{tab? | url? | titl\x1b[118;1:3ue?, browser?}",
				Summary: "Close a tab.",
			}},
		},
	}

	RefreshAgentContext(setter, chatStore, newFakeUserMemoryDocStore(), nil, moduleStore, catalog, fakeSubagentModeStore{mode: SubagentModeOff}, SandboxPromptInput{}, BrowserSnapshotInput{}, "", "current")

	if strings.Contains(setter.lastCtx, "\x1b") || strings.Contains(setter.lastCtx, "[118;1:3") {
		t.Fatalf("final context should strip control characters: %q", setter.lastCtx)
	}
	if !strings.Contains(setter.lastCtx, "{tab? | url? | title?, browser?}") {
		t.Fatalf("final context should preserve sanitized action args: %q", setter.lastCtx)
	}
}

func TestModulesInstructionGeneratedFromSpecs(t *testing.T) {
	got := ModulesInstruction([]domain.ModuleSpec{
		{ID: "chat", Name: "Chat", Description: "Chat with the agent.", Core: true},
		{
			ID: "tasks", Name: "Tasks", Description: "Task tasks.",
			Prompt:  "Track actionable items here.",
			Actions: []domain.ModuleAction{{Name: "tasks.add", Args: "{title}", Summary: "Add an item."}},
		},
	})
	for _, want := range []string{
		"## Workspace modules",
		"### Chat (chat, core — cannot be removed)",
		"### Tasks (tasks, added — removable)",
		"Track actionable items here.",
		"- tasks.add {title} — Add an item.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("instruction missing %q:\n%s", want, got)
		}
	}
	if ModulesInstruction(nil) != "" {
		t.Fatal("no added modules should yield an empty block")
	}
}

// fakeNotesContextStore is a minimal NotesStore for agent-context tests.
type fakeNotesContextStore struct {
	unlocked bool
	notes    []domain.Note
}

func (s *fakeNotesContextStore) IsUnlocked() bool { return s.unlocked }
func (s *fakeNotesContextStore) CreateNote(_, _ string, _ bool) (domain.Note, error) {
	return domain.Note{}, nil
}
func (s *fakeNotesContextStore) ListNotes(_ bool) ([]domain.Note, error) {
	return s.notes, nil
}
func (s *fakeNotesContextStore) GetNote(_ string) (domain.Note, error) { return domain.Note{}, nil }
func (s *fakeNotesContextStore) UpdateNote(_, _, _ string, _, _, _ bool) (domain.Note, error) {
	return domain.Note{}, nil
}
func (s *fakeNotesContextStore) DeleteNote(_ string) error { return nil }

type fakeSubagentModeStore struct{ mode string }

func (s fakeSubagentModeStore) LoadSubagentMode() string { return s.mode }
func (s fakeSubagentModeStore) SaveSubagentMode(string) error {
	return nil
}

// TestRefreshAgentContextDecision8NotesDoNotClobberOtherBlocks is the
// mandatory regression for Decision 8: a second SetMemoryContext call must
// never clobber the composed context. This test verifies that after adding
// the notes block the user-memory AND chat-catalog blocks are still present
// in the single SetMemoryContext push, and that SetMemoryContext is still
// called exactly once.
func TestRefreshAgentContextDecision8NotesDoNotClobberOtherBlocks(t *testing.T) {
	chatStore := newMemoryStore()
	userStore := newFakeUserMemoryDocStore()
	userStore.doc = domain.UserMemoryDoc{Content: "- lang: Português."}
	notesStore := &fakeNotesContextStore{
		unlocked: true,
		notes: []domain.Note{
			{ID: "n1", Title: "Plan", Content: "Do X.", InPrompt: true},
			{ID: "n2", Title: "Archived skip", Content: "y", Archived: true, InPrompt: true},
		},
	}
	setter := &fakeMemoryContextSetter{}

	RefreshAgentContext(setter, chatStore, userStore, notesStore, &fakeModuleStore{}, testCatalog(), fakeSubagentModeStore{mode: SubagentModeOff}, SandboxPromptInput{}, BrowserSnapshotInput{}, "", "current")

	// Decision 8: exactly ONE SetMemoryContext call — never two separate pushes.
	if setter.calls != 1 {
		t.Fatalf("setter.calls = %d, want 1 (Decision 8: single composed push)", setter.calls)
	}
	// User-memory block must be present.
	if !strings.Contains(setter.lastCtx, "Information loaded from Settings → Memory") {
		t.Fatalf("user-memory block missing: %q", setter.lastCtx)
	}
	// Chat-catalog block must be present.
	if !strings.Contains(setter.lastCtx, "memory.chat.search") {
		t.Fatalf("chat-catalog block missing: %q", setter.lastCtx)
	}
	// Notes block must be present and must not include the archived note.
	if !strings.Contains(setter.lastCtx, "## Information loaded from the Notes module") {
		t.Fatalf("notes block missing: %q", setter.lastCtx)
	}
	if !strings.Contains(setter.lastCtx, "### Note: Plan") {
		t.Fatalf("notes block missing 'Note: Plan': %q", setter.lastCtx)
	}
	if strings.Contains(setter.lastCtx, "Archived skip") {
		t.Fatalf("archived note must not appear in context: %q", setter.lastCtx)
	}
	// Notes block must precede the chat-catalog block.
	notesIdx := strings.Index(setter.lastCtx, "## Information loaded from the Notes module")
	chatIdx := strings.Index(setter.lastCtx, "memory.chat.search")
	if notesIdx > chatIdx {
		t.Fatalf("notes block should come before chat catalog: %q", setter.lastCtx)
	}
}
