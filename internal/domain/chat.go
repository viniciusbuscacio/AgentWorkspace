package domain

type VaultStatus struct {
	Exists      bool   `json:"exists"`
	Unlocked    bool   `json:"unlocked"`
	VaultDir    string `json:"vaultDir"`
	HasRecovery bool   `json:"hasRecovery"`
}

type Chat struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Archived  bool   `json:"archived"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Attachment struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	DataURI string `json:"dataUri"`
}

type Message struct {
	ID          string       `json:"id"`
	SessionID   string       `json:"sessionId"`
	Role        string       `json:"role"`
	Content     string       `json:"content"`
	CreatedAt   string       `json:"createdAt"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// ChatTitleEntry records one title a chat had over time, whether assigned
// automatically (auto-rename cadence) or manually by the user.
type ChatTitleEntry struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Title     string `json:"title"`
	Turn      int    `json:"turn"`
	Source    string `json:"source"` // "auto" | "manual"
	CreatedAt string `json:"createdAt"`
}
