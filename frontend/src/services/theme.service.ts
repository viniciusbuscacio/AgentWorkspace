import {
  DeleteCustomTheme,
  GetActiveTheme,
  GetCustomThemes,
  SaveActiveTheme,
  SaveCustomTheme,
  SuggestThemeFromPalette,
} from '@wails/go/main/App';
import { normalizeCustomThemeTokens, type CustomThemeTokens } from '@/theme/custom-theme';

/**
 * themeService — wraps the Wails app_theme.go bindings for the theme studio.
 * Persistence is config.json (via the Go backend); never localStorage-only.
 * All methods are best-effort: failures are swallowed so callers stay clean.
 */
export const themeService = {
  /** Returns the persisted active theme id (built-in or "custom:slug"). */
  async getActiveTheme(): Promise<string> {
    try {
      return await GetActiveTheme();
    } catch {
      return 'midnight';
    }
  },

  /** Persists the active theme id to config.json. */
  async saveActiveTheme(id: string): Promise<void> {
    try {
      await SaveActiveTheme(id);
    } catch {
      // Non-fatal — localStorage fallback is already written by applyAppTheme.
    }
  },

  /** Returns all saved custom themes, normalised. */
  async getCustomThemes(): Promise<Record<string, CustomThemeTokens>> {
    try {
      const raw = await GetCustomThemes();
      const result: Record<string, CustomThemeTokens> = {};
      for (const [id, tokens] of Object.entries(raw ?? {})) {
        result[id] = normalizeCustomThemeTokens(tokens as Partial<CustomThemeTokens>);
      }
      return result;
    } catch {
      return {};
    }
  },

  /**
   * Creates or updates a custom theme. The tokens are sent to Go for
   * validation + sanitization before being written to config.json.
   * Returns the error message on failure, or null on success.
   */
  async saveCustomTheme(
    id: string,
    tokens: CustomThemeTokens,
  ): Promise<string | null> {
    try {
      const result = await SaveCustomTheme(id, tokens as unknown as Record<string, unknown>);
      return result.success ? null : (result.error ?? 'Save failed');
    } catch (err) {
      return err instanceof Error ? err.message : 'Save failed';
    }
  },

  /** Removes a custom theme from config.json. Returns error string or null. */
  async deleteCustomTheme(id: string): Promise<string | null> {
    try {
      const result = await DeleteCustomTheme(id);
      return result.success ? null : (result.error ?? 'Delete failed');
    } catch (err) {
      return err instanceof Error ? err.message : 'Delete failed';
    }
  },

  /**
   * Refines a color palette into a full custom theme via the backend LLM.
   * Only hex strings are sent (Decision 3 of theme-studio-spec). Best-effort:
   * on any failure (AI unavailable, error result or throw) returns
   * `{ tokens: null, error }` so the caller can fall back to a local suggestion.
   */
  async suggestFromPalette(
    palette: string[],
  ): Promise<{ tokens: CustomThemeTokens | null; error: string | null }> {
    try {
      const result = await SuggestThemeFromPalette(palette);
      if (result.success && result.suggestion) {
        return {
          tokens: normalizeCustomThemeTokens(result.suggestion as Partial<CustomThemeTokens>),
          error: null,
        };
      }
      return { tokens: null, error: result.error ?? 'AI unavailable' };
    } catch {
      return { tokens: null, error: 'AI unavailable' };
    }
  },
};
