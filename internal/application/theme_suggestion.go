package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// ThemeSuggestionTimeout bounds the palette → theme LLM call.
const ThemeSuggestionTimeout = 15 * time.Second
const themeSuggestionMaxOutputTokens int32 = 512

// SuggestThemePrompt builds a prompt that asks the LLM to name a theme and
// assign tokens from a color palette. Only hex strings are sent (no image).
func SuggestThemePrompt(palette []string) string {
	return strings.Join([]string{
		"You are a UI designer. Given a palette of dominant hex colors extracted from a screenshot,",
		"suggest a name and map the colors to these UI token slots:",
		"background, surface, surfaceAlt, text, mutedText, border, accent.",
		"",
		"Rules:",
		"- Respond ONLY with valid JSON (no markdown, no explanation).",
		"- All color values must be exactly \"#rrggbb\" lowercase hex strings.",
		"- The name must be a short, evocative title (max 30 chars).",
		"- Do not invent colors — only use the provided palette.",
		"",
		fmt.Sprintf("Palette: %s", strings.Join(palette, ", ")),
		"",
		"Example output: {\"name\":\"Ocean Night\",\"background\":\"#0d1b2a\",\"surface\":\"#1b2b3a\"," +
			"\"surfaceAlt\":\"#1e3045\",\"text\":\"#e8f4f8\",\"mutedText\":\"#7b9db4\"," +
			"\"border\":\"#2a4a5e\",\"accent\":\"#4fc3f7\"}",
	}, "\n")
}

// SuggestThemeFromPalette calls the active LLM with the extracted palette and
// returns a partial token map. Returns (nil, false) on any failure — the
// caller falls back to localThemeFromPalette.
func SuggestThemeFromPalette(
	ctx context.Context,
	runtime ports.ChatTitleRuntime,
	cfg domain.ModelConfig,
	palette []string,
) (map[string]interface{}, bool) {
	if runtime == nil || len(palette) == 0 {
		return nil, false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, ThemeSuggestionTimeout)
	defer cancel()

	reply, err := runtime.GenerateOneShot(ctx, cfg, SuggestThemePrompt(palette), themeSuggestionMaxOutputTokens)
	if err != nil {
		return nil, false
	}
	text := strings.TrimSpace(reply.Text)
	// Strip markdown code fences if the model added them.
	if idx := strings.Index(text, "{"); idx > 0 {
		text = text[idx:]
	}
	if idx := strings.LastIndex(text, "}"); idx >= 0 && idx < len(text)-1 {
		text = text[:idx+1]
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, false
	}
	return result, true
}
