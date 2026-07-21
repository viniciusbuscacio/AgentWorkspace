import { useEffect } from 'react';
import { onAwEvent } from '@services/events';
import { uiService } from '@services/ui.service';
import { applyAppTheme, getSavedAppTheme } from '@/lib/app-theme';
import { applyAppFont, getSavedAppFontFamily, getSavedAppFontSize } from '@/lib/app-font';
import { applyCustomThemeTokens, isCustomThemeId } from '@/theme/custom-theme';
import { themeService } from '@services/theme.service';

/**
 * Bridge between the backend aw control actions and the UI-managed appearance
 * state. Applies ui:set-theme / ui:set-font events (emitted by app.theme.set
 * and app.font.set) and reports the active theme/font back to the backend so
 * the aw app.state action answers without a frontend round-trip.
 *
 * Custom themes (id starts with "custom:"): the bridge loads the tokens from
 * config.json and applies the CSS variables before updating state.
 */
export function useUiControlBridge() {
  useEffect(() => {
    // Report the initial theme so app.state answers immediately on first load.
    const initialTheme = getSavedAppTheme();
    void uiService.reportUiState({
      theme: initialTheme,
      fontFamily: getSavedAppFontFamily(),
      fontSize: getSavedAppFontSize(),
    });

    // On startup, if the active theme is custom, load its tokens and apply.
    if (isCustomThemeId(initialTheme)) {
      void themeService.getCustomThemes().then((themes) => {
        const tokens = themes[initialTheme];
        if (tokens) applyCustomThemeTokens(tokens, initialTheme);
      });
    }

    const offTheme = onAwEvent('ui:set-theme', ({ theme }) => {
      if (isCustomThemeId(theme)) {
        void themeService.getCustomThemes().then((themes) => {
          const tokens = themes[theme];
          if (tokens) {
            applyCustomThemeTokens(tokens, theme);
            applyAppTheme(theme); // persist id to localStorage
          }
          void uiService.reportUiState({ theme });
        });
      } else {
        const applied = applyAppTheme(theme);
        void uiService.reportUiState({ theme: applied });
      }
    });

    const offFont = onAwEvent('ui:set-font', ({ family, size }) => {
      const applied = applyAppFont(family || undefined, size || undefined);
      void uiService.reportUiState({ fontFamily: applied.family, fontSize: applied.size });
    });

    return () => {
      offTheme();
      offFont();
    };
  }, []);
}
