package domain

// SelfDevConfig is the persisted self-development posture. It mirrors the
// infrastructure config shape so the application and interface layers can read
// it without importing appconfig. JSON tags match the persisted form so the
// self-dev state view serializes identically.
type SelfDevConfig struct {
	Enabled     bool   `json:"enabled,omitempty"`
	RepoRoot    string `json:"repoRoot,omitempty"`
	AllowShell  *bool  `json:"allowShell,omitempty"`
	AutoApprove *bool  `json:"autoApprove,omitempty"`
}
