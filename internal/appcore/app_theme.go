package appcore

import (
	"aw/internal/application"
	"aw/internal/dto"
)

// GetActiveTheme returns the persisted active theme id. Returns "midnight"
// when no theme has been saved yet.
func (a *App) GetActiveTheme() string {
	return application.GetActiveTheme(a.workspace)
}

// SaveActiveTheme persists the active theme id to config.json. The id may be
// a built-in name ("midnight", "light", …) or a custom id ("custom:slug").
func (a *App) SaveActiveTheme(id string) dto.OperationResult {
	a.recordActivity()
	if err := application.SaveActiveTheme(a.workspace, id); err != nil {
		return dto.OperationResult{Error: err.Error()}
	}
	return dto.OperationResult{Success: true}
}

// GetCustomThemes returns the full map of saved custom themes from config.json.
// Each key is a "custom:slug" id; each value is the token map.
func (a *App) GetCustomThemes() map[string]interface{} {
	return application.GetCustomThemes(a.workspace)
}

// SaveCustomTheme creates or updates a custom theme in config.json. Validation
// (id shape, required hex tokens, name length) and sanitization live in the
// application use case.
func (a *App) SaveCustomTheme(id string, tokens map[string]interface{}) dto.OperationResult {
	a.recordActivity()
	if err := application.SaveCustomTheme(a.workspace, id, tokens); err != nil {
		return dto.OperationResult{Error: err.Error()}
	}
	return dto.OperationResult{Success: true}
}

// DeleteCustomTheme removes a custom theme from config.json. If the id is not
// present the call is a no-op (returns Success: true).
func (a *App) DeleteCustomTheme(id string) dto.OperationResult {
	a.recordActivity()
	if err := application.DeleteCustomTheme(a.workspace, id); err != nil {
		return dto.OperationResult{Error: err.Error()}
	}
	return dto.OperationResult{Success: true}
}

// SuggestThemeFromPalette asks the active LLM to map a color palette to theme
// tokens. Only hex strings are sent to the model — no image data leaves the
// machine (Decision 3). When the model is unavailable or returns bad JSON the
// frontend falls back to localThemeFromPalette automatically.
func (a *App) SuggestThemeFromPalette(palette []string) dto.ThemeSuggestionResult {
	a.recordActivity()
	if len(palette) == 0 {
		return dto.ThemeSuggestionResult{Error: "palette is empty"}
	}
	if a.agent == nil || a.vault == nil {
		return dto.ThemeSuggestionResult{Error: "agent not ready"}
	}
	runtimeConfig, err := application.ResolveProviderRuntimeConfig(a.vault)
	if err != nil {
		return dto.ThemeSuggestionResult{Error: err.Error()}
	}
	suggestion, ok := application.SuggestThemeFromPalette(
		a.contextOrBackground(),
		a.agent,
		application.ModelConfigFromProviderRuntimeConfig(runtimeConfig),
		palette,
	)
	if !ok {
		return dto.ThemeSuggestionResult{Error: "LLM suggestion unavailable"}
	}
	return dto.ThemeSuggestionResult{Success: true, Suggestion: suggestion}
}
