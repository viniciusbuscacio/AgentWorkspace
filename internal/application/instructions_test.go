package application

import (
	"errors"
	"strings"
	"testing"

	"aw/internal/domain"
)

func instructionStore(t *testing.T) *fakeSkillStore {
	t.Helper()
	store := newFakeSkillStore()
	store.docs[domain.AgentsDocumentID] = docSeed(domain.AgentsDocumentID, "SEED AGENTS RULES")
	return store
}

func TestListInstructionDocumentsReturnsAgentsAndUser(t *testing.T) {
	store := instructionStore(t)
	if err := EnsureUserDocument(store); err != nil {
		t.Fatal(err)
	}
	docs, err := ListInstructionDocuments(store, SkillsPromptInput{})
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
	if docs[0].ID != domain.AgentsDocumentID || docs[1].ID != domain.UserDocumentID {
		t.Fatalf("unexpected docs: %s, %s", docs[0].ID, docs[1].ID)
	}
	// AGENTS.md is editable + resettable (seed-backed); USER.md editable, not resettable.
	if !docs[0].Editable || !docs[0].Resettable {
		t.Error("AGENTS.md should be editable and resettable")
	}
	if !docs[1].Editable || docs[1].Resettable {
		t.Error("USER.md should be editable but not resettable")
	}
	if !contains(docs[1].Status, domain.InstructionStatusEmpty) {
		t.Errorf("freshly seeded USER.md should be empty, status=%v", docs[1].Status)
	}
}

func TestSaveInstructionValidatesID(t *testing.T) {
	store := instructionStore(t)
	if err := SaveInstructionDocument(store, "CLAUDE.md", "x"); err == nil {
		t.Error("saving a non-allowlisted id should error")
	}
	if err := ValidateInstructionID("USER.md"); err != nil {
		t.Errorf("USER.md should be valid: %v", err)
	}
}

func TestSaveAgentsMarksCustomizedAndReadsBack(t *testing.T) {
	store := instructionStore(t)
	if err := SaveInstructionDocument(store, domain.AgentsDocumentID, "EDITED AGENTS"); err != nil {
		t.Fatal(err)
	}
	got, err := GetInstructionDocument(store, SkillsPromptInput{}, domain.AgentsDocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "EDITED AGENTS") {
		t.Errorf("read content = %q", got.Content)
	}
	if got.Origin != domain.InstructionOriginVault || !contains(got.Status, domain.InstructionStatusCustomized) {
		t.Errorf("edited builtin should read as customized vault override, origin=%s status=%v", got.Origin, got.Status)
	}
}

func TestResetOnlySeedBacked(t *testing.T) {
	store := instructionStore(t)
	if err := EnsureUserDocument(store); err != nil {
		t.Fatal(err)
	}
	// USER.md is not seed-backed → reset must fail.
	if err := ResetInstructionDocument(store, domain.AppDocument{}, false, domain.UserDocumentID); err == nil {
		t.Error("resetting USER.md should fail (not seed-backed)")
	}
	// Customize AGENTS.md, then reset restores the seed (no longer customized).
	if err := SaveInstructionDocument(store, domain.AgentsDocumentID, "EDITED"); err != nil {
		t.Fatal(err)
	}
	seed := docSeed(domain.AgentsDocumentID, "SEED AGENTS RULES")
	if err := ResetInstructionDocument(store, seed, true, domain.AgentsDocumentID); err != nil {
		t.Fatalf("reset error = %v", err)
	}
	got, _ := GetInstructionDocument(store, SkillsPromptInput{}, domain.AgentsDocumentID)
	if strings.Contains(got.Content, "EDITED") {
		t.Error("reset should discard the edit")
	}
	if contains(got.Status, domain.InstructionStatusCustomized) {
		t.Error("reset document should not be customized")
	}
}

func TestComposerSourceOrderAndNoDrift(t *testing.T) {
	store := instructionStore(t)
	store.docs[domain.UserDocumentID] = domain.AppDocument{ID: domain.UserDocumentID, Content: "Reply in pt-BR. api_key=SUPERSECRET123", Origin: domain.SkillOriginUser}
	store.skills["gmail-web"] = seedSkill("gmail-web", "---\nname: gmail-web\ndescription: x\n---\n\nBODY")

	raw := ComposeEffectiveInstructions(store, SkillsPromptInput{}, domain.EffectiveInstructionsRaw)
	scrubbed := ComposeEffectiveInstructions(store, SkillsPromptInput{}, domain.EffectiveInstructionsScrubbed)

	// Source order is fixed: AGENTS.md, USER.md, skills — identical in both modes.
	if len(raw.Sources) != 3 || len(scrubbed.Sources) != 3 {
		t.Fatalf("expected 3 sources, raw=%d scrubbed=%d", len(raw.Sources), len(scrubbed.Sources))
	}
	for i, want := range []string{domain.AgentsDocumentID, domain.UserDocumentID, "skills"} {
		if raw.Sources[i].ID != want || scrubbed.Sources[i].ID != want {
			t.Fatalf("source[%d] drift: raw=%s scrubbed=%s want=%s", i, raw.Sources[i].ID, scrubbed.Sources[i].ID, want)
		}
	}
	// Same section labels and order in the composed text.
	for _, label := range []string{InstructionSectionAgents, InstructionSectionUser, InstructionSectionSkills} {
		if !strings.Contains(raw.Content, label) || !strings.Contains(scrubbed.Content, label) {
			t.Errorf("missing label %q (raw or scrubbed)", label)
		}
	}
	if idxAgents, idxUser := strings.Index(raw.Content, InstructionSectionAgents), strings.Index(raw.Content, InstructionSectionUser); idxAgents > idxUser {
		t.Error("AGENTS.md must come before USER.md")
	}
	// Scrubbing redacts secrets; raw keeps the trusted text verbatim.
	if !strings.Contains(raw.Content, "SUPERSECRET123") {
		t.Error("raw runtime content must keep trusted text verbatim")
	}
	if strings.Contains(scrubbed.Content, "SUPERSECRET123") {
		t.Error("scrubbed UI content must redact secret-looking values")
	}
}

func TestEffectiveLockedVaultIsEmpty(t *testing.T) {
	store := instructionStore(t)
	store.unlocked = false
	if _, err := ListInstructionDocuments(store, SkillsPromptInput{}); !errors.Is(err, errVaultLocked) {
		t.Errorf("locked list err = %v, want errVaultLocked", err)
	}
	eff := ComposeEffectiveInstructions(store, SkillsPromptInput{}, domain.EffectiveInstructionsRaw)
	if eff.Content != "" {
		t.Errorf("locked vault should compose empty content, got %q", eff.Content)
	}
}

func TestDevOverrideAgentsAndSources(t *testing.T) {
	store := instructionStore(t)
	dev := SkillsPromptInput{Active: true, HasAgentsDoc: true, AgentsDoc: domain.AppDocument{Content: "DEV OVERRIDE AGENTS"}}

	docs, err := ListInstructionDocuments(store, dev)
	if err != nil {
		t.Fatal(err)
	}
	agents := docs[0]
	if agents.Origin != domain.InstructionOriginDevOverride || agents.Editable {
		t.Errorf("AGENTS.md under dev override should be read-only dev_override, got origin=%s editable=%v", agents.Origin, agents.Editable)
	}
	if !contains(agents.Status, domain.InstructionStatusReadOnly) {
		t.Errorf("dev override AGENTS.md should be read-only, status=%v", agents.Status)
	}
	eff := ComposeEffectiveInstructions(store, dev, domain.EffectiveInstructionsRaw)
	if !strings.Contains(eff.Content, "DEV OVERRIDE AGENTS") {
		t.Error("effective should use the dev override AGENTS.md")
	}
	if !eff.Metadata.DevOverrideActive {
		t.Error("metadata should flag dev override active")
	}
	sources := InstructionSourcesInventory(store, dev)
	devActive := false
	for _, s := range sources {
		if s.Kind == domain.InstructionOriginDevOverride && s.Active {
			devActive = true
		}
	}
	if !devActive {
		t.Error("sources inventory should report dev_override active")
	}
}

func TestEnsureUserDocumentSeedsEmptyOnce(t *testing.T) {
	store := instructionStore(t)
	if err := EnsureUserDocument(store); err != nil {
		t.Fatal(err)
	}
	doc, ok, _ := store.GetAppDocument(domain.UserDocumentID)
	if !ok || doc.Content != "" || doc.Origin != domain.SkillOriginUser {
		t.Fatalf("USER.md seed = %+v ok=%v", doc, ok)
	}
	// Idempotent: a second call must not overwrite user content.
	store.docs[domain.UserDocumentID] = domain.AppDocument{ID: domain.UserDocumentID, Content: "MY PREFS", Origin: domain.SkillOriginUser}
	if err := EnsureUserDocument(store); err != nil {
		t.Fatal(err)
	}
	doc, _, _ = store.GetAppDocument(domain.UserDocumentID)
	if doc.Content != "MY PREFS" {
		t.Error("EnsureUserDocument must not overwrite existing USER.md")
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
