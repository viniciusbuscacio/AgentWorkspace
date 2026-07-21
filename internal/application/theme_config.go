package application

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"aw/internal/domain/ports"
)

// hexColorRe matches exactly "#rrggbb" (6 hex digits).
var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// customThemeRequiredKeys must be present and valid hex colors.
var customThemeRequiredKeys = []string{
	"background", "surface", "surfaceAlt",
	"text", "mutedText", "border", "accent",
}

// customThemeOptionalColorKeys are passed through if valid hex.
var customThemeOptionalColorKeys = []string{
	"sidebar", "sidebarHover", "sidebarActive", "sidebarActiveText",
	"chatSurface", "chatComposer",
	"userBubble", "userBubbleText", "assistantBubble", "codeSurface",
}

// GetActiveTheme returns the persisted active theme id, defaulting to
// "midnight" when none has been saved.
func GetActiveTheme(store ports.ThemeConfigStore) string {
	theme := store.LoadActiveTheme()
	if theme == "" {
		return "midnight"
	}
	return theme
}

// SaveActiveTheme persists the active theme id (built-in name or "custom:slug").
func SaveActiveTheme(store ports.ThemeConfigStore, id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return store.SaveActiveTheme(id)
}

// GetCustomThemes returns a copy of the saved custom theme map so callers
// cannot mutate the persisted config.
func GetCustomThemes(store ports.ThemeConfigStore) map[string]interface{} {
	themes := store.LoadCustomThemes()
	result := make(map[string]interface{}, len(themes))
	for id, tokens := range themes {
		result[id] = tokens
	}
	return result
}

// SaveCustomTheme validates and persists a custom theme, merging it into the
// existing map. The id must start with "custom:"; tokens must carry a non-empty
// name and the required hex color keys.
func SaveCustomTheme(store ports.ThemeConfigStore, id string, tokens map[string]interface{}) error {
	if err := validateCustomThemeID(id); err != nil {
		return err
	}
	if err := validateCustomThemeTokens(tokens); err != nil {
		return err
	}
	sanitized := sanitizeCustomThemeTokens(tokens)

	existing := store.LoadCustomThemes()
	merged := make(map[string]interface{}, len(existing)+1)
	for k, v := range existing {
		merged[k] = v
	}
	merged[id] = sanitized
	return store.SaveCustomThemes(merged)
}

// DeleteCustomTheme removes a custom theme. Removing an absent id is a no-op.
func DeleteCustomTheme(store ports.ThemeConfigStore, id string) error {
	if err := validateCustomThemeID(id); err != nil {
		return err
	}
	existing := store.LoadCustomThemes()
	if _, ok := existing[id]; !ok {
		return nil
	}
	updated := make(map[string]interface{}, len(existing))
	for k, v := range existing {
		if k != id {
			updated[k] = v
		}
	}
	return store.SaveCustomThemes(updated)
}

func validateCustomThemeID(id string) error {
	if !strings.HasPrefix(id, "custom:") {
		return fmt.Errorf("id must start with \"custom:\"")
	}
	if strings.TrimPrefix(id, "custom:") == "" {
		return fmt.Errorf("id slug must not be empty")
	}
	return nil
}

func validateCustomThemeTokens(tokens map[string]interface{}) error {
	if tokens == nil {
		return fmt.Errorf("tokens are required")
	}
	name, _ := tokens["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if utf8.RuneCountInString(name) > 60 {
		return fmt.Errorf("name must be 60 characters or fewer")
	}
	for _, key := range customThemeRequiredKeys {
		v, _ := tokens[key].(string)
		if !hexColorRe.MatchString(v) {
			return fmt.Errorf("%s must be a #rrggbb hex color", key)
		}
	}
	return nil
}

// sanitizeCustomThemeTokens returns a clean copy with only the name, required
// color keys, and any optional color keys that are valid hex.
func sanitizeCustomThemeTokens(tokens map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	nameStr, _ := tokens["name"].(string)
	out["name"] = strings.TrimSpace(nameStr)
	for _, key := range customThemeRequiredKeys {
		out[key] = tokens[key]
	}
	for _, key := range customThemeOptionalColorKeys {
		v, _ := tokens[key].(string)
		if hexColorRe.MatchString(v) {
			out[key] = v
		}
	}
	return out
}
