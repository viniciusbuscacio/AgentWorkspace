package application

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
	"aw/skills"
)

// Agent Instructions use cases (Settings → Agent Instructions). Instruction
// documents are trusted configuration markdown (AGENTS.md, USER.md). This file
// owns the SINGLE shared effective-instructions composer used by BOTH runtime
// injection (raw output) and the UI/aw actions (scrubbed output), so the two can
// never drift. See docs/plans/agent-instructions-spec.md §9.4 / decision 16.

// Section labels for the composed instruction block. Exported as constants so
// tests can assert runtime and UI use exactly the same labels.
const (
	InstructionSectionAgents = "### Agent Instructions: AGENTS.md"
	InstructionSectionUser   = "### User Instructions: USER.md"
	InstructionSectionSkills = "### Skills"
	instructionSkillsNote    = "Read-only skills index."
	instructionTrustedPrefix = "Trusted configuration from: "
)

// instructionIDs is the v1 allowlist of editable instruction document ids.
// Compatibility/custom ids are a later phase with explicit validation.
var instructionIDs = map[string]string{
	domain.AgentsDocumentID: "Runtime/project rules for the agent.",
	domain.UserDocumentID:   "User preferences and stable behavior rules.",
}

// ValidateInstructionID reports whether id is an editable instruction document.
func ValidateInstructionID(id string) error {
	if _, ok := instructionIDs[id]; !ok {
		return fmt.Errorf("unknown instruction document %q (allowed: %s)", id, strings.Join(sortedInstructionIDs(), ", "))
	}
	return nil
}

func sortedInstructionIDs() []string {
	out := make([]string, 0, len(instructionIDs))
	for id := range instructionIDs {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// --- shared composer -------------------------------------------------------

// ComposeEffectiveInstructions builds the effective instruction block from
// AGENTS.md, USER.md, and the read-only Skills index, in that fixed order. It is
// the ONLY composer: RefreshSkillsContext feeds the raw output to the runtime,
// and InstructionsEffective scrubs the same output for the UI. Locked vault (and
// no dev override) yields an empty block.
func ComposeEffectiveInstructions(store ports.SkillStore, dev SkillsPromptInput, mode domain.EffectiveInstructionsOutputMode) domain.EffectiveInstructions {
	var blocks []string
	var sources []domain.InstructionSource

	agentsContent, agentsOrigin, agentsLabel := effectiveAgents(store, dev)
	if agentsContent != "" {
		blocks = append(blocks, InstructionSectionAgents+"\n"+instructionTrustedPrefix+agentsLabel+"\n\n"+agentsContent)
		sources = append(sources, domain.InstructionSource{
			ID: domain.AgentsDocumentID, Kind: "instruction", Origin: agentsOrigin,
			Enabled: true, Active: true, Chars: len(agentsContent),
		})
	}

	userContent := vaultUserDoc(store)
	if userContent != "" {
		blocks = append(blocks, InstructionSectionUser+"\n"+instructionTrustedPrefix+"vault\n\n"+userContent)
		sources = append(sources, domain.InstructionSource{
			ID: domain.UserDocumentID, Kind: "instruction", Origin: domain.InstructionOriginUser,
			Enabled: true, Active: true, Chars: len(userContent),
		})
	}

	list, _ := EffectiveSkills(store, dev)
	if index := skills.InstructionForSkills(list); index != "" {
		blocks = append(blocks, InstructionSectionSkills+"\n"+instructionSkillsNote+"\n\n"+index)
		sources = append(sources, domain.InstructionSource{
			ID: "skills", Kind: "skills", Origin: domain.InstructionOriginComputed,
			Enabled: true, Active: true, Chars: len(index),
		})
	}

	content := strings.Join(blocks, "\n\n")
	redacted := false
	if mode == domain.EffectiveInstructionsScrubbed {
		content = ScrubChatSecrets(content)
		redacted = true
	}
	return domain.EffectiveInstructions{
		Content: content,
		Sources: sources,
		Metadata: domain.EffectiveInstructionsMetadata{
			CharCount:         len(content),
			DevOverrideActive: dev.Active,
		},
		Redacted: redacted,
	}
}

// effectiveAgents resolves the AGENTS.md content actually in effect, plus its
// origin token and a human "trusted from" label. Dev override (AW_SKILLS_DIR)
// wins; otherwise the vault document.
func effectiveAgents(store ports.SkillStore, dev SkillsPromptInput) (content, origin, label string) {
	if dev.Active {
		if dev.HasAgentsDoc {
			return strings.TrimSpace(dev.AgentsDoc.Content), domain.InstructionOriginDevOverride, "dev override (AW_SKILLS_DIR)"
		}
		return "", domain.InstructionOriginDevOverride, "dev override (AW_SKILLS_DIR)"
	}
	if store == nil || !store.IsUnlocked() {
		return "", domain.InstructionOriginBuiltin, "bundled seed"
	}
	doc, ok, err := store.GetAppDocument(domain.AgentsDocumentID)
	if err != nil || !ok {
		return "", domain.InstructionOriginBuiltin, "bundled seed"
	}
	if isCustomized(doc) {
		return strings.TrimSpace(doc.Content), domain.InstructionOriginVault, "vault override"
	}
	return strings.TrimSpace(doc.Content), domain.InstructionOriginBuiltin, "bundled seed"
}

// vaultUserDoc returns the trimmed vault USER.md content (empty when locked,
// absent, or empty). USER.md is always vault-backed, even under a dev override.
func vaultUserDoc(store ports.SkillStore) string {
	if store == nil || !store.IsUnlocked() {
		return ""
	}
	doc, ok, err := store.GetAppDocument(domain.UserDocumentID)
	if err != nil || !ok {
		return ""
	}
	return strings.TrimSpace(doc.Content)
}

func isCustomized(doc domain.AppDocument) bool {
	return doc.SeedHash != "" && doc.ContentHash != doc.SeedHash
}

// --- document listing / reading --------------------------------------------

// ListInstructionDocuments returns the v1 editable instruction documents
// (AGENTS.md, USER.md) with derived origin/status/editability.
func ListInstructionDocuments(store ports.SkillStore, dev SkillsPromptInput) ([]domain.InstructionDocument, error) {
	if store == nil {
		return nil, errors.New("instruction store is required")
	}
	if !store.IsUnlocked() {
		return nil, errVaultLocked
	}
	agents := agentsDocumentView(store, dev)
	user := userDocumentView(store)
	return []domain.InstructionDocument{agents, user}, nil
}

func agentsDocumentView(store ports.SkillStore, dev SkillsPromptInput) domain.InstructionDocument {
	doc := domain.InstructionDocument{
		ID:          domain.AgentsDocumentID,
		Title:       domain.AgentsDocumentID,
		Description: instructionIDs[domain.AgentsDocumentID],
		Enabled:     true,
	}
	if dev.Active {
		doc.Origin = domain.InstructionOriginDevOverride
		doc.Status = []string{domain.InstructionStatusDevOverride, domain.InstructionStatusActive, domain.InstructionStatusReadOnly}
		doc.Editable = false
		doc.Resettable = false
		return doc
	}
	vaultDoc, ok, _ := store.GetAppDocument(domain.AgentsDocumentID)
	doc.Editable = true
	doc.Resettable = true
	doc.ContentHash = vaultDoc.ContentHash
	doc.SeedHash = vaultDoc.SeedHash
	doc.SeedVersion = vaultDoc.SeedVersion
	doc.UpdatedAt = vaultDoc.UpdatedAt
	status := []string{domain.InstructionStatusActive}
	if !ok || strings.TrimSpace(vaultDoc.Content) == "" {
		status = []string{domain.InstructionStatusEmpty}
	} else if isCustomized(vaultDoc) {
		doc.Origin = domain.InstructionOriginVault
		status = append(status, domain.InstructionStatusCustomized, domain.InstructionStatusResettable)
	} else {
		doc.Origin = domain.InstructionOriginBuiltin
		status = append(status, domain.InstructionStatusBuiltin)
	}
	if doc.Origin == "" {
		doc.Origin = domain.InstructionOriginBuiltin
	}
	doc.Status = status
	return doc
}

func userDocumentView(store ports.SkillStore) domain.InstructionDocument {
	doc := domain.InstructionDocument{
		ID:          domain.UserDocumentID,
		Title:       domain.UserDocumentID,
		Description: instructionIDs[domain.UserDocumentID],
		Origin:      domain.InstructionOriginUser,
		Editable:    true,
		Resettable:  false,
		Enabled:     true,
	}
	vaultDoc, ok, _ := store.GetAppDocument(domain.UserDocumentID)
	doc.ContentHash = vaultDoc.ContentHash
	doc.UpdatedAt = vaultDoc.UpdatedAt
	if !ok || strings.TrimSpace(vaultDoc.Content) == "" {
		doc.Status = []string{domain.InstructionStatusEmpty}
	} else {
		doc.Status = []string{domain.InstructionStatusActive}
	}
	return doc
}

// GetInstructionDocument reads one instruction document with its content. For
// AGENTS.md under a dev override it returns the effective (dev) content as
// read-only so the editor shows what is actually loaded.
func GetInstructionDocument(store ports.SkillStore, dev SkillsPromptInput, id string) (domain.InstructionDocumentContent, error) {
	if err := ValidateInstructionID(id); err != nil {
		return domain.InstructionDocumentContent{}, err
	}
	if store == nil {
		return domain.InstructionDocumentContent{}, errors.New("instruction store is required")
	}
	if !store.IsUnlocked() {
		return domain.InstructionDocumentContent{}, errVaultLocked
	}
	var view domain.InstructionDocument
	if id == domain.AgentsDocumentID {
		view = agentsDocumentView(store, dev)
	} else {
		view = userDocumentView(store)
	}
	out := domain.InstructionDocumentContent{
		ID: id, Origin: view.Origin, Editable: view.Editable, Resettable: view.Resettable,
		Status: view.Status, ContentHash: view.ContentHash, SeedHash: view.SeedHash,
		SeedVersion: view.SeedVersion, UpdatedAt: view.UpdatedAt,
	}
	if id == domain.AgentsDocumentID && dev.Active {
		content, _, _ := effectiveAgents(store, dev)
		out.Content = content
		return out, nil
	}
	doc, ok, err := store.GetAppDocument(id)
	if err != nil {
		return domain.InstructionDocumentContent{}, err
	}
	if ok {
		out.Content = doc.Content
	}
	return out, nil
}

// --- mutations -------------------------------------------------------------

// SaveInstructionDocument persists an edited instruction document. AGENTS.md
// keeps its seed hash (so customized/reset mechanics still work); USER.md is
// user-origin. It does not refresh runtime context — the caller (app layer)
// owns that, mirroring the skills mutation flow.
func SaveInstructionDocument(store ports.SkillStore, id, content string) error {
	if err := ValidateInstructionID(id); err != nil {
		return err
	}
	if store == nil {
		return errors.New("instruction store is required")
	}
	if !store.IsUnlocked() {
		return errVaultLocked
	}
	normalized := skills.Normalize(content)
	doc := domain.AppDocument{
		ID:          id,
		Content:     normalized,
		ContentHash: skills.Hash(normalized),
	}
	switch id {
	case domain.AgentsDocumentID:
		// Preserve the seed hash/version so an edited builtin reads as customized
		// and the seed-update path leaves it alone.
		cur, ok, err := store.GetAppDocument(id)
		if err != nil {
			return err
		}
		doc.Origin = domain.SkillOriginBuiltin
		if ok {
			doc.SeedHash = cur.SeedHash
			doc.SeedVersion = cur.SeedVersion
		}
	case domain.UserDocumentID:
		doc.Origin = domain.SkillOriginUser
	}
	return store.UpsertAppDocument(doc)
}

// ResetInstructionDocument restores a seed-backed instruction document to its
// bundled seed. Only AGENTS.md is seed-backed; USER.md reset is an error.
func ResetInstructionDocument(store ports.SkillStore, seed domain.AppDocument, hasSeed bool, id string) error {
	if err := ValidateInstructionID(id); err != nil {
		return err
	}
	if store == nil {
		return errors.New("instruction store is required")
	}
	if !store.IsUnlocked() {
		return errVaultLocked
	}
	if id != domain.AgentsDocumentID || !hasSeed {
		return fmt.Errorf("%q is not seed-backed and cannot be reset", id)
	}
	seed.ID = domain.AgentsDocumentID
	seed.Origin = domain.SkillOriginBuiltin
	return store.UpsertAppDocument(seed)
}

// EnsureUserDocument seeds an empty vault USER.md when absent, so the Agent
// Instructions UI always shows it (spec open question 1: seed empty). Empty
// content means it never adds noise to the runtime prompt until the user fills it.
func EnsureUserDocument(store ports.SkillStore) error {
	if store == nil || !store.IsUnlocked() {
		return errVaultLocked
	}
	if _, ok, err := store.GetAppDocument(domain.UserDocumentID); err != nil {
		return err
	} else if ok {
		return nil
	}
	return store.UpsertAppDocument(domain.AppDocument{
		ID:          domain.UserDocumentID,
		Content:     "",
		ContentHash: skills.Hash(""),
		Origin:      domain.SkillOriginUser,
	})
}

// --- sources inventory -----------------------------------------------------

// InstructionSourcesInventory returns the source inventory for instructions.sources:
// which instruction sources exist and whether they are active.
func InstructionSourcesInventory(store ports.SkillStore, dev SkillsPromptInput) []domain.InstructionSource {
	sources := []domain.InstructionSource{}

	// AGENTS.md: bundled seed always exists; the vault override is active unless a
	// dev override masks it.
	agentsActive := !dev.Active
	sources = append(sources, domain.InstructionSource{
		Kind: domain.InstructionOriginBuiltin, ID: domain.AgentsDocumentID, Active: agentsActive,
	})
	devReason := "AW_SKILLS_DIR_not_set"
	if dev.Active && !dev.HasAgentsDoc {
		devReason = "AW_SKILLS_DIR_has_no_AGENTS.md"
	}
	sources = append(sources, domain.InstructionSource{
		Kind: domain.InstructionOriginDevOverride, ID: domain.AgentsDocumentID,
		Active: dev.Active && dev.HasAgentsDoc, Reason: devReasonIf(!(dev.Active && dev.HasAgentsDoc), devReason),
	})

	// USER.md: vault-backed, active when it has content.
	sources = append(sources, domain.InstructionSource{
		Kind: domain.InstructionOriginUser, ID: domain.UserDocumentID,
		Active: vaultUserDoc(store) != "", Reason: devReasonIf(vaultUserDoc(store) == "", "empty"),
	})
	return sources
}

func devReasonIf(cond bool, reason string) string {
	if cond {
		return reason
	}
	return ""
}
