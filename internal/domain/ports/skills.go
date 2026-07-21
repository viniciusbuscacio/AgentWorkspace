package ports

import "aw/internal/domain"

// SkillStore persists skills (as folders of files) and runtime app documents in
// the vault. The seed bootstrap and the effective-catalog use cases in the
// application layer drive it; the agent runtime never touches it directly.
type SkillStore interface {
	VaultUnlockState
	// ListSkills returns all skills. When includeDeleted is false, soft-deleted
	// skills are omitted; the bootstrap passes true so it can tell a builtin
	// that was never seeded apart from one the user removed.
	ListSkills(includeDeleted bool) ([]domain.Skill, error)
	GetSkill(id string) (domain.Skill, bool, error)
	// UpsertSkill writes the skill row and replaces its skill_files set.
	UpsertSkill(skill domain.Skill) error
	SetSkillEnabled(id string, enabled bool) error
	// SoftDeleteSkill marks a skill deleted (builtins are never physically
	// removed so the seed bootstrap can avoid resurrecting them).
	SoftDeleteSkill(id string) error
	// DeleteSkill physically removes a skill and its files (used for user
	// skills, which have no seed to fall back to).
	DeleteSkill(id string) error

	GetAppDocument(id string) (domain.AppDocument, bool, error)
	// ListAppDocuments returns every runtime app document (AGENTS.md, USER.md,
	// …) for the Agent Instructions surface.
	ListAppDocuments() ([]domain.AppDocument, error)
	UpsertAppDocument(doc domain.AppDocument) error
}

// SkillsContextSetter receives the composed skills + runtime AGENTS.md block to
// inject into the agent system instruction, fed after unlock so the agent
// runtime stays decoupled from the vault.
type SkillsContextSetter interface {
	SetSkillsContext(extra string)
}
