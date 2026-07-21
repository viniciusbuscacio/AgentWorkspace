import * as React from 'react';
import { Toaster as SonnerToaster } from 'sonner';

/**
 * App-wide toast surface (Sonner). Mounted once at the App root; call the
 * imperative `toast()` singleton from anywhere — no context/provider needed.
 *
 * Theming: the repo drives ~11 themes via `document.documentElement.dataset.theme`
 * (not next-themes), so instead of mapping theme -> light/dark we point Sonner's
 * own CSS vars at the repo tokens (`globals.css`). The toast then follows any
 * `data-theme` automatically. We deliberately do NOT use `richColors` (it paints
 * success green / error red with Sonner's own palette, ignoring the theme).
 * Instead every toast uses the themed neutral surface, and error/warning get an
 * accent from the repo's own tokens (destructive / primary), mirroring
 * `SettingsNotice` — so the toast always matches the active theme.
 *
 * Close-button placement (top-right, inside the box), hover-only reveal, and the
 * right-to-left enter animation are done in `theme/aw-toast.css` (imported in
 * main.tsx) because they override Sonner's own stylesheet by specificity — see
 * that file. Here we only set the material "close" glyph.
 */
function Toaster({ ...props }: React.ComponentProps<typeof SonnerToaster>) {
  return (
    <SonnerToaster
      position="top-right"
      // Always expanded so the toast keeps a constant size: otherwise Sonner
      // toggles data-expanded on hover (collapsed -> expanded), which resized
      // the box when the pointer entered.
      expand
      closeButton
      duration={4000}
      icons={{
        close: <span className="material-symbols-outlined text-[16px] leading-none">close</span>,
      }}
      style={
        {
          '--normal-bg': 'var(--bg-elevated)',
          '--normal-text': 'var(--text-primary)',
          '--normal-border': 'var(--border)',
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          // Base: themed neutral surface (success/info/default look like this).
          toast: 'rounded-lg shadow-xl',
          // Error/warning accents come from the repo's own theme tokens (not
          // Sonner's palette), matching SettingsNotice. `!` beats Sonner's
          // var-driven base background/border/text.
          // Solid surface (destructive tinted into the elevated bg) — a 10%
          // translucent red over the wallpaper was unreadable.
          error: '!border-destructive/50 !bg-[color-mix(in_srgb,var(--destructive)_18%,var(--bg-elevated))] !text-destructive',
          warning: '!border-primary/35 !bg-primary/10 !text-foreground',
        },
      }}
      {...props}
    />
  );
}

export { Toaster };
