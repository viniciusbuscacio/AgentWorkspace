package appcore

import (
	"aw/internal/application"
	"aw/internal/dto"
	"aw/internal/infrastructure/appconfig"
)

func (a *App) GetSubagentMode() dto.SubagentModeResponse {
	return dto.SubagentModeResponse{Mode: application.LoadSubagentMode(appconfig.Store{})}
}

func (a *App) SetSubagentMode(mode string) OperationResult {
	err := application.SaveSubagentMode(appconfig.Store{}, mode)
	return a.basicOperationResult(err)
}
