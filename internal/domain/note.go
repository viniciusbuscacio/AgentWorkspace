package domain

// Note is one entry of the Notes workspace module, stored in the vault.
// Pinned notes sort first in every listing; archived notes are hidden from
// the default list (AW2 parity). InPrompt controls whether the note is
// injected into the agent system context as the notes block (labeled with its
// Notes-module provenance so the agent cites the right source).
type Note struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	UpdatedAt string `json:"updatedAt"`
	Pinned    bool   `json:"pinned"`
	Archived  bool   `json:"archived"`
	InPrompt  bool   `json:"inPrompt"`
}
