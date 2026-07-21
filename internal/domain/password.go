package domain

type PasswordEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	URL       string `json:"url"`
	Notes     string `json:"notes"`
	UpdatedAt int64  `json:"updatedAt"`
}

// PasswordSummary is the agent-facing catalog view of a credential: enough to
// know what exists and pick one (passwords.list), never the secret material.
// Notes stay out too — they routinely hold recovery codes.
type PasswordSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	URL       string `json:"url"`
	UpdatedAt int64  `json:"updatedAt"`
}

// Summary strips a credential down to its agent-facing catalog view.
func (e PasswordEntry) Summary() PasswordSummary {
	return PasswordSummary{
		ID:        e.ID,
		Name:      e.Name,
		Username:  e.Username,
		URL:       e.URL,
		UpdatedAt: e.UpdatedAt,
	}
}
