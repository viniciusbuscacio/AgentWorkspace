# Fonts (how the app font picker works, and how to add fonts)

The Settings → Fonts picker lets the user pick the application font family and
base size. This doc is the canonical description of how fonts are bundled and
how to add a new one.

## The core idea: bundle, don't rely on the OS

Font names alone (`"Inter"`, `"Segoe UI"`, ...) only render if that font is
**installed on the user's machine**. Otherwise the browser silently falls back
to a generic family, so several picker entries can collapse to the *same* system
font (this is why the old `Inter / SF Pro / Segoe` options looked identical on
macOS — none were installed, all fell back to San Francisco).

The fix, and the standard this repo follows: every non-`system` option is a web
font **shipped inside the app** as a `.woff2` file and declared with
`@font-face`. It then renders the same on macOS, Windows and Linux regardless of
what is installed. The only OS-dependent option is `System Default`, which is
intentionally the native UI font.

## The four sources of truth (keep in sync)

Adding/removing a font touches these four places:

| # | File | Role |
|---|------|------|
| 1 | `frontend/src/assets/fonts/<id>-<weight>.woff2` | the actual font files (weights 400 & 700, latin subset) |
| 2 | `frontend/src/theme/fonts.css` | one `@font-face` block per file; imported in `frontend/src/main.tsx` |
| 3 | `frontend/src/lib/app-font.ts` → `APP_FONT_OPTIONS` | picker entries (`id`, `label`, `group`, `css` stack) |
| 4 | `internal/infrastructure/tools/aw_app.go` → `awFontFamilies` | backend allowlist for the agent's `app.font.set` action |

If (3) and (4) drift, the UI will offer a font the agent tool rejects (or vice
versa). The `app.font.set` usage string in `tools.go` is generated from
`awFontFamilies`, so it stays in sync automatically.

## How rendering flows

1. `main.tsx` imports `theme/fonts.css` (registers all `@font-face`s) and calls
   `applyAppFont()`.
2. `applyAppFont(family, size)` (`lib/app-font.ts`) looks up the option by id,
   then sets CSS variables on `<html>`: `--font-family` (the `css` stack) and
   `--font-size`. It persists both to `localStorage` (`aw.font.family`,
   `aw.font.size`).
3. `theme/globals.css` applies `font-family: var(--font-family)` /
   `font-size: var(--font-size)` to the body; components inherit.
4. The agent can drive it via the `app.font.set` action, which validates against
   `awFontFamilies` and emits `ui:set-font`; `hooks/ui-control.hooks.ts` applies
   it and reports state back.

## Adding a font (step by step)

1. **Pick a free font.** It must be OFL-1.1 or Apache-2.0 (or similar
   redistributable license) so we can legally ship the file. Google Fonts is all
   free. Proprietary system fonts (Segoe UI, SF Pro, Calibri, Helvetica, ...)
   **cannot** be bundled — use a metric-compatible free clone instead
   (Arial→Arimo, Times New Roman→Tinos, Courier New→Cousine, Georgia→Gelasio,
   Calibri→Carlito, Cambria→Caladea, Comic Sans→Comic Neue, Futura→Jost,
   Baskerville→Libre Baskerville).

2. **Download the files.** Add an `id|Family Name` line to
   `scripts/fetch-fonts.sh` and run it:

   ```sh
   ./scripts/fetch-fonts.sh
   ```

   This writes `frontend/src/assets/fonts/<id>-400.woff2` and `-700.woff2`
   (latin subset). Use a hyphenated slug for `<id>` (e.g. `space-grotesk`).

3. **Declare `@font-face`** in `frontend/src/theme/fonts.css` — two blocks
   (400 and 700), matching the existing pattern:

   ```css
   @font-face {
     font-family: 'Space Grotesk';
     font-style: normal;
     font-weight: 400;
     font-display: swap;
     src: url('../assets/fonts/space-grotesk-400.woff2') format('woff2');
   }
   /* ...and the 700 block */
   ```

4. **Add a picker option** to `APP_FONT_OPTIONS` in
   `frontend/src/lib/app-font.ts`. Put the family name first in the `css` stack,
   followed by the right generic fallback, and set `group` to
   `Sans-serif` | `Serif` | `Monospace`:

   ```ts
   {
     id: 'space-grotesk',
     label: 'Space Grotesk',
     group: 'Sans-serif',
     css: '"Space Grotesk", ui-sans-serif, system-ui, sans-serif',
   },
   ```

   For a clone, label it with the familiar name, e.g. `'Arimo (Arial-like)'`.

5. **Allow it for the agent.** Add the same `id` to `awFontFamilies` in
   `internal/infrastructure/tools/aw_app.go`.

6. **Run the gate.**

   ```sh
   cd frontend && npm run build:frontend   # lint + typecheck + vitest + vite build
   go run ./tools/buildgate --skip-build    # go lint + tests (includes aw_app allowlist test)
   ```

## Notes

- **Weights.** We ship 400 + 700 per family; intermediate weights (500/600) are
  synthesized by the browser. Add more weights only if a font looks wrong.
- **Size.** Each latin-subset `.woff2` is ~10–50 KB; the whole set is ~1–2 MB.
- **Licenses.** Attribution and the per-family license list live in
  `frontend/src/assets/fonts/README.md`; the OFL text is `OFL.txt` there.
- **Do not** re-add proprietary fonts by name — they render only where installed
  and make the picker dishonest. Bundle a clone instead.
