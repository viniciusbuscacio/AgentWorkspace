package domain

// Skill origins. A skill is either shipped with aw (builtin, tracked against the
// embedded seed) or added by the user / imported (user, never touched by the
// seed update). A builtin the user edited stays origin=builtin; the divergence
// between ContentHash and SeedHash is what marks it "customized".
const (
	SkillOriginBuiltin = "builtin"
	SkillOriginUser    = "user"
)

// SkillFile is one file inside a skill's folder, stored as a row so the whole
// folder (SKILL.md plus any references/assets/scripts) round-trips through the
// encrypted vault. Path is relative to the skill root, slash-separated
// ("SKILL.md", "agents/openai.yaml").
type SkillFile struct {
	Path string `json:"path"`
	// Content is the normalized file content (LF, UTF-8 no BOM, trimmed).
	Content string `json:"content"`
	// ContentHash is the hash of the current normalized Content.
	ContentHash string `json:"contentHash"`
	// SeedHash is the hash of the last builtin seed applied to this file; empty
	// for user files. When ContentHash != SeedHash the user customized the file
	// and the seed update must not overwrite it.
	SeedHash string `json:"seedHash"`
}

// Skill is a procedural skill stored in the vault. Conceptually it is a folder:
// Files carries every file of that folder.
type Skill struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Enabled     bool        `json:"enabled"`
	Origin      string      `json:"origin"`
	SeedVersion string      `json:"seedVersion"`
	Deleted     bool        `json:"deleted"`
	Files       []SkillFile `json:"files"`
}

// SkillMarkdownPath is the canonical entrypoint file every skill folder has.
const SkillMarkdownPath = "SKILL.md"

// SkillBody returns the SKILL.md content of the skill, or "" if absent.
func (s Skill) SkillBody() string {
	for _, f := range s.Files {
		if f.Path == SkillMarkdownPath {
			return f.Content
		}
	}
	return ""
}

// AppDocument is a runtime document injected into the agent system prompt
// (the vault's AGENTS.md and USER.md). It mirrors the skill update-safe
// mechanics with a single content blob instead of a file set.
type AppDocument struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	ContentHash string `json:"contentHash"`
	SeedHash    string `json:"seedHash"`
	Origin      string `json:"origin"`
	SeedVersion string `json:"seedVersion"`
	// UpdatedAt is the vault row's last-write timestamp (RFC3339). Empty for
	// seed/dev-override documents that never round-tripped through the vault.
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// AgentsDocumentID is the id of the runtime AGENTS.md app document.
const AgentsDocumentID = "AGENTS.md"

// UserDocumentID is the id of the user-preferences USER.md app document. It is
// vault-backed and user-origin (never seed-backed), so it is editable but not
// resettable (see the Agent Instructions spec, decisions 4/5/15).
const UserDocumentID = "USER.md"
