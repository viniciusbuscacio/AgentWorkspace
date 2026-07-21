package application

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// BootstrapSkills seeds and updates the builtin skills and the runtime
// AGENTS.md in the vault using the update-safe rule (seed_hash + content_hash).
// It is idempotent and safe to run on every unlock/create:
//   - a builtin never seeded is inserted enabled;
//   - a builtin the user did not modify is updated to the new seed;
//   - a builtin the user modified is preserved (update available, not applied);
//   - a user skill is never touched;
//   - a soft-deleted builtin is not resurrected.
func BootstrapSkills(store ports.SkillStore, seeds []domain.Skill, agentsDoc domain.AppDocument, hasAgents bool) error {
	if store == nil {
		return errors.New("skill store is required")
	}
	if !store.IsUnlocked() {
		return errVaultLocked
	}
	existing, err := store.ListSkills(true) // include deleted to detect removals
	if err != nil {
		return err
	}
	byID := make(map[string]domain.Skill, len(existing))
	for _, s := range existing {
		byID[s.ID] = s
	}
	for _, seed := range seeds {
		cur, ok := byID[seed.ID]
		if !ok {
			seed.Origin = domain.SkillOriginBuiltin
			seed.Enabled = true
			seed.Deleted = false
			if err := store.UpsertSkill(seed); err != nil {
				return err
			}
			continue
		}
		if cur.Deleted || cur.Origin == domain.SkillOriginUser {
			continue
		}
		merged, changed := mergeBuiltinSkill(cur, seed)
		if changed {
			if err := store.UpsertSkill(merged); err != nil {
				return err
			}
		}
	}
	if hasAgents {
		if err := bootstrapAppDocument(store, agentsDoc); err != nil {
			return err
		}
	}
	return nil
}

// mergeBuiltinSkill applies the per-file update-safe rule to a builtin skill the
// user has not removed. A file is replaced by the new seed only when the user
// did not change it (content_hash == seed_hash); user-changed files and any
// extra files are preserved. Returns the merged skill and whether anything
// changed (so an unchanged seed is a no-op).
func mergeBuiltinSkill(cur, seed domain.Skill) (domain.Skill, bool) {
	out := cur
	out.Origin = domain.SkillOriginBuiltin
	out.Deleted = false

	curByPath := make(map[string]domain.SkillFile, len(cur.Files))
	for _, f := range cur.Files {
		curByPath[f.Path] = f
	}
	seedPaths := make(map[string]bool, len(seed.Files))

	changed := false
	skillMdFromSeed := false
	merged := make([]domain.SkillFile, 0, len(seed.Files)+len(cur.Files))

	for _, sf := range seed.Files {
		seedPaths[sf.Path] = true
		cf, ok := curByPath[sf.Path]
		if !ok {
			merged = append(merged, domain.SkillFile{Path: sf.Path, Content: sf.Content, ContentHash: sf.ContentHash, SeedHash: sf.SeedHash})
			changed = true
			if sf.Path == domain.SkillMarkdownPath {
				skillMdFromSeed = true
			}
			continue
		}
		if cf.ContentHash == cf.SeedHash {
			// User did not modify this builtin file: take the new seed.
			if cf.ContentHash != sf.ContentHash {
				changed = true
			}
			merged = append(merged, domain.SkillFile{Path: sf.Path, Content: sf.Content, ContentHash: sf.ContentHash, SeedHash: sf.SeedHash})
			if sf.Path == domain.SkillMarkdownPath {
				skillMdFromSeed = true
			}
			continue
		}
		// User customized this file: keep it untouched (update available).
		merged = append(merged, cf)
	}
	for _, cf := range cur.Files {
		if !seedPaths[cf.Path] {
			merged = append(merged, cf)
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Path < merged[j].Path })
	out.Files = merged

	if skillMdFromSeed {
		if out.Name != seed.Name || out.Description != seed.Description {
			changed = true
		}
		out.Name = seed.Name
		out.Description = seed.Description
	}
	if changed {
		out.SeedVersion = seed.SeedVersion
	}
	return out, changed
}

func bootstrapAppDocument(store ports.SkillStore, seed domain.AppDocument) error {
	cur, ok, err := store.GetAppDocument(seed.ID)
	if err != nil {
		return err
	}
	if !ok {
		seed.Origin = domain.SkillOriginBuiltin
		return store.UpsertAppDocument(seed)
	}
	if cur.Origin == domain.SkillOriginUser {
		return nil
	}
	if cur.ContentHash != cur.SeedHash {
		return nil // user customized AGENTS.md: keep it
	}
	if cur.ContentHash == seed.ContentHash {
		return nil // already current
	}
	seed.Origin = domain.SkillOriginBuiltin
	return store.UpsertAppDocument(seed)
}

// SkillsPromptInput carries the resolved AW_SKILLS_DIR dev override (read by the
// composition root via the skills package). When Active it replaces the vault
// catalog entirely — overlay would be a precedence nightmare.
type SkillsPromptInput struct {
	Active       bool
	Skills       []domain.Skill
	AgentsDoc    domain.AppDocument
	HasAgentsDoc bool
	Source       string
}

// EffectiveSkills returns the skills to inject plus a source label. Precedence:
// dev override (AW_SKILLS_DIR) > enabled non-deleted vault skills > nothing.
func EffectiveSkills(store ports.SkillStore, dev SkillsPromptInput) ([]domain.Skill, string) {
	if dev.Active {
		return dev.Skills, "dev-override"
	}
	if store == nil || !store.IsUnlocked() {
		return nil, "locked"
	}
	list, err := store.ListSkills(false)
	if err != nil {
		return nil, "error"
	}
	out := make([]domain.Skill, 0, len(list))
	for _, s := range list {
		if s.Enabled && !s.Deleted {
			out = append(out, s)
		}
	}
	return out, "vault"
}

// SkillSummary is the compact catalog entry returned by the aw skill.list
// action (progressive disclosure level 1 — the trigger, not the body).
// Customized/UpdateAvailable mirror the management view so the agent can see
// that a builtin it is about to follow diverged from the current bundle and
// self-heal (skill.reset with the user's OK) instead of following stale steps.
type SkillSummary struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Files           []string `json:"files,omitempty"`
	Customized      bool     `json:"customized,omitempty"`
	UpdateAvailable bool     `json:"updateAvailable,omitempty"`
	Guidance        string   `json:"guidance,omitempty"`
}

// SkillCatalog returns the effective skills as compact summaries for skill.list.
// seeds is the current bundled seed set (nil under the dev override), used to
// flag customized builtins whose bundled version moved on.
func SkillCatalog(store ports.SkillStore, dev SkillsPromptInput, seeds []domain.Skill) []SkillSummary {
	list, _ := EffectiveSkills(store, dev)
	seedMdHash := make(map[string]string, len(seeds))
	for _, s := range seeds {
		if md, ok := skillMdFile(s); ok {
			seedMdHash[s.ID] = md.ContentHash
		}
	}
	out := make([]SkillSummary, 0, len(list))
	for _, s := range list {
		paths := make([]string, 0, len(s.Files))
		for _, f := range s.Files {
			paths = append(paths, f.Path)
		}
		summary := SkillSummary{ID: s.ID, Name: s.Name, Description: s.Description, Files: paths}
		if !dev.Active && s.Origin == domain.SkillOriginBuiltin {
			if md, ok := skillMdFile(s); ok && md.SeedHash != "" && md.ContentHash != md.SeedHash {
				summary.Customized = true
				if seedHash, ok := seedMdHash[s.ID]; ok && seedHash != md.SeedHash {
					summary.UpdateAvailable = true
					summary.Guidance = "This builtin skill was customized and a NEWER bundled version exists; its instructions may be stale. With the user's OK, run skill.reset {id: \"" + s.ID + "\"} to adopt the current bundled version (discards the customization), then re-read it before following its steps."
				}
			}
		}
		out = append(out, summary)
	}
	return out
}

// ReadSkillFile returns the content of one file of an effective (enabled) skill
// for skill.read. Empty path defaults to SKILL.md (progressive disclosure level
// 2); other paths reach auxiliary files (level 3).
func ReadSkillFile(store ports.SkillStore, dev SkillsPromptInput, id, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = domain.SkillMarkdownPath
	}
	skill, ok := findEffectiveSkill(store, dev, id)
	if !ok {
		return "", fmt.Errorf("skill %q is not available", id)
	}
	for _, f := range skill.Files {
		if f.Path == path {
			return f.Content, nil
		}
	}
	return "", fmt.Errorf("skill %q has no file %q", id, path)
}

func findEffectiveSkill(store ports.SkillStore, dev SkillsPromptInput, id string) (domain.Skill, bool) {
	list, _ := EffectiveSkills(store, dev)
	for _, s := range list {
		if s.ID == id {
			return s, true
		}
	}
	return domain.Skill{}, false
}

// RefreshSkillsContext composes the runtime instruction block (AGENTS.md +
// USER.md + the read-only Skills index) into the agent's dedicated slot. It
// delegates to the single shared composer in RAW mode, so the runtime and the
// UI's instructions.effective can never drift in source order or labels. The
// block is cleared when the vault is locked, so sensitive text never loads
// before unlock.
func RefreshSkillsContext(setter ports.SkillsContextSetter, store ports.SkillStore, dev SkillsPromptInput) {
	if setter == nil {
		return
	}
	effective := ComposeEffectiveInstructions(store, dev, domain.EffectiveInstructionsRaw)
	setter.SetSkillsContext(effective.Content)
}
