package domain

// UserMemoryFact is one long-term fact about the user (v1 — retained for
// migration only; no new writes after user-memory-v2-spec migration).
type UserMemoryFact struct {
	Key       string `json:"key"`
	Category  string `json:"category"`
	Content   string `json:"content"`
	Source    string `json:"source"`
	UpdatedAt string `json:"updatedAt"`
}

// UserMemoryDoc is the v2 living document the agent maintains about the user.
// One freeform text replaces the v1 facts list (user-memory-v2-spec.md).
// The agent appends to it; the user edits it directly in Settings > Memory.
type UserMemoryDoc struct {
	Content         string `json:"content"`
	Backup          string `json:"backup,omitempty"`
	LastCondensedAt string `json:"lastCondensedAt,omitempty"`
	UpdatedAt       string `json:"updatedAt,omitempty"`
}
