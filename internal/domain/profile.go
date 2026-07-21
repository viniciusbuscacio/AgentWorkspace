package domain

type ProfileInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Avatar      string `json:"avatar"`
	VaultDir    string `json:"vaultDir"`
	LastUsed    string `json:"lastUsed"`
	CreatedAt   string `json:"createdAt"`
	HasVault    bool   `json:"hasVault"`
	HasRecovery bool   `json:"hasRecovery"`
}
