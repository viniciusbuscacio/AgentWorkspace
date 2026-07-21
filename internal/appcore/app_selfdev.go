package appcore

import (
	"context"
	"time"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/infrastructure/appconfig"
)

// selfDevStateView is the typed, JSON-serialisable snapshot returned to the
// self-dev `system.state` tool action. It replaces the previous ad-hoc
// map[string]any: the composition root owns the runtime fields (workspace,
// zoom, agent readiness, active runs) while the vault-derived business slice
// comes from application.LoadSelfDevVaultState.
type selfDevStateView struct {
	Timestamp            string                              `json:"timestamp"`
	WorkspaceRoot        string                              `json:"workspaceRoot"`
	AppZoomPercent       int                                 `json:"appZoomPercent"`
	SelfDev              domain.SelfDevConfig                `json:"selfDev"`
	AgentReady           bool                                `json:"agentReady"`
	AgentError           string                              `json:"agentError,omitempty"`
	CurrentProfile       string                              `json:"currentProfile"`
	ChatRunCount         int                                 `json:"chatRunCount"`
	VaultAvailable       bool                                `json:"vaultAvailable"`
	Vault                *VaultStatusResponse                `json:"vault,omitempty"`
	ProviderStatus       *domain.ProviderStatus              `json:"providerStatus,omitempty"`
	RuntimeProvider      *application.SelfDevRuntimeProvider `json:"runtimeProvider,omitempty"`
	RuntimeProviderError string                              `json:"runtimeProviderError,omitempty"`
	Chats                []domain.Chat                       `json:"chats"`
}

// selfDevState assembles the self-dev state view consumed by the aw tool's
// system.state action.
func (a *App) selfDevState(ctx context.Context) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	view := selfDevStateView{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		WorkspaceRoot:  a.workspaceRoot,
		AppZoomPercent: a.GetAppZoomPercent(),
		SelfDev:        application.LoadSelfDevConfig(appconfig.Store{}),
		AgentReady:     a.agentErr == nil,
		CurrentProfile: a.currentProfileID,
		ChatRunCount:   a.chatRunner.ActiveCount(),
		Chats:          []domain.Chat{},
	}
	if a.agentErr != nil {
		view.AgentError = a.agentErr.Error()
	}
	if a.vault == nil {
		return view, nil
	}
	view.VaultAvailable = true
	vaultStatus := a.vaultStatus()
	view.Vault = &vaultStatus

	vaultState, err := application.LoadSelfDevVaultState(a.vault)
	if err != nil {
		return nil, err
	}
	providerStatus := vaultState.ProviderStatus
	view.ProviderStatus = &providerStatus
	view.RuntimeProvider = vaultState.RuntimeProvider
	view.RuntimeProviderError = vaultState.RuntimeProviderError
	view.Chats = vaultState.Chats
	return view, nil
}
