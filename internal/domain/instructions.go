package domain

// Agent Instructions domain types (Settings → Agent Instructions). Instruction
// documents are trusted configuration markdown (AGENTS.md, USER.md) that shape
// how the agent behaves. They are distinct from Skills (on-demand procedural
// knowledge) and Memory (curated facts). See docs/plans/agent-instructions-spec.md.

// Instruction document origins (a subset of the spec's source kinds that v1
// actually emits for editable documents).
const (
	InstructionOriginBuiltin     = "builtin"      // embedded app seed (AGENTS.md)
	InstructionOriginVault       = "vault"        // user-customized vault override of a builtin
	InstructionOriginUser        = "user"         // user-created vault document (USER.md)
	InstructionOriginDevOverride = "dev_override" // AW_SKILLS_DIR AGENTS.md
	InstructionOriginComputed    = "computed"     // the effective composed block
)

// Instruction document status tokens (multiple may apply at once).
const (
	InstructionStatusActive      = "active"
	InstructionStatusEmpty       = "empty"
	InstructionStatusCustomized  = "customized"
	InstructionStatusBuiltin     = "built-in"
	InstructionStatusResettable  = "resettable"
	InstructionStatusReadOnly    = "read-only"
	InstructionStatusDevOverride = "dev-override"
)

// InstructionDocument is one instruction file plus the derived state the
// Agent Instructions UI / actions need (origin, statuses, editability).
type InstructionDocument struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Origin      string   `json:"origin"`
	Status      []string `json:"status"`
	Editable    bool     `json:"editable"`
	Resettable  bool     `json:"resettable"`
	Enabled     bool     `json:"enabled"`
	ContentHash string   `json:"contentHash,omitempty"`
	SeedHash    string   `json:"seedHash,omitempty"`
	SeedVersion string   `json:"seedVersion,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
}

// InstructionDocumentContent is a single document with its content, returned by
// instructions.read for the editor.
type InstructionDocumentContent struct {
	ID          string   `json:"id"`
	Content     string   `json:"content"`
	Origin      string   `json:"origin"`
	Editable    bool     `json:"editable"`
	Resettable  bool     `json:"resettable"`
	Status      []string `json:"status"`
	ContentHash string   `json:"contentHash,omitempty"`
	SeedHash    string   `json:"seedHash,omitempty"`
	SeedVersion string   `json:"seedVersion,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
}

// InstructionSource is one entry in the effective-instructions source breakdown
// and the instructions.sources inventory.
type InstructionSource struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Origin  string `json:"origin,omitempty"`
	Enabled bool   `json:"enabled"`
	Active  bool   `json:"active"`
	Chars   int    `json:"chars,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// EffectiveInstructionsOutputMode selects how the shared composer renders its
// content. Runtime injection gets the raw trusted text; UI/actions get the same
// text after secret scrubbing. There is exactly one composer; only this mode
// differs, so runtime and UI can never drift in source order or section labels.
type EffectiveInstructionsOutputMode string

const (
	EffectiveInstructionsRaw      EffectiveInstructionsOutputMode = "raw"
	EffectiveInstructionsScrubbed EffectiveInstructionsOutputMode = "scrubbed"
)

// EffectiveInstructionsMetadata carries non-content fields for the UI.
type EffectiveInstructionsMetadata struct {
	GeneratedAt       string `json:"generatedAt,omitempty"`
	CharCount         int    `json:"charCount"`
	DevOverrideActive bool   `json:"devOverrideActive"`
}

// EffectiveInstructions is the composed instruction block plus its source
// breakdown and metadata, produced by the single shared composer.
type EffectiveInstructions struct {
	Content  string                        `json:"content"`
	Sources  []InstructionSource           `json:"sources"`
	Metadata EffectiveInstructionsMetadata `json:"metadata"`
	Redacted bool                          `json:"redacted"`
}
