package appcore

import (
	"context"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
)

const promptDebugModeSecret = "_config_prompt_debug_mode"

func (a *App) resetPromptDebugModeForNewAppSession() {
	if a.vault == nil {
		return
	}
	_ = application.SetSecret(a.vault, promptDebugModeSecret, "false")
}

func (a *App) emitPromptDebugSnapshot(_ context.Context, snapshot domain.PromptDebugSnapshot) {
	if a.vault == nil {
		return
	}
	value, err := application.GetSecret(a.vault, promptDebugModeSecret)
	if err != nil || value.Value != "true" {
		return
	}
	if snapshot.ModuleID != "" {
		snapshot.PlanMode = a.isPlanModeEnabled(snapshot.ModuleID)
	}
	a.emitChatEvent("prompt:debug", map[string]any{"snapshot": snapshot})
}

func (a *App) GetPromptDebugMode() dto.PromptDebugModeResponse {
	if a.vault == nil {
		return dto.PromptDebugModeResponse{Enabled: false}
	}
	value, err := application.GetSecret(a.vault, promptDebugModeSecret)
	return dto.PromptDebugModeResponse{Enabled: err == nil && value.Value == "true"}
}

func (a *App) SetPromptDebugMode(enabled bool) OperationResult {
	value := "false"
	if enabled {
		value = "true"
	}
	err := application.SetSecret(a.vault, promptDebugModeSecret, value)
	return a.basicOperationResult(err)
}
