# Spec — Theme studio (port of the AW2 custom-theme system)

> **Status:** implemented (2026-06-12, commits: e77ba61, 26b3adc, 969a1f7).
> second validation pass (items 1-partial and 2 of 2026-06-12). Port of
> Superseded in part by polish-wave-3-spec.md (2026-06-12): Decision 1 (Theme page search/order/size controls) now delivered.
> AW2's theme system. Reference implementation in
> `AgentWorkspace2`:
>
> - `src/renderer/theme/custom-theme.ts` — `CustomThemeTokens` (7 required
>   colors + 10 optional), id slugs (`custom:name`), `applyCustomThemeTokens`
> - `src/renderer/theme/palette.ts` — client-side palette extraction from an
>   image (96×96 resample, luminance buckets, top-8 colors) +
>   `localThemeFromPalette` (darkest→background, contrast→accent)
> - `src/renderer/modules/settings/components/ThemeSettingsPanel.tsx` and
>   `AppPanels.tsx:20-131` — the gallery + the custom-theme editor
>
> When in doubt, open the AW2 file and copy.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Settings › Theme becomes a gallery in the Apps-screen style** (item 1): card grid with live preview swatches, a search bar, an "Order by" select and the card-size slider — same controls, same look as the Apps screen. Built-in themes + the user's custom themes in one grid. |
| 2 | **Custom themes ported from AW2**: create from scratch, duplicate an existing theme, edit token-by-token with `input[type=color]` — `CustomThemeTokens` shape ported (7 required: background, surface, surfaceAlt, text, mutedText, border, accent; 10 optional with AW2's smart defaults). Ids are `custom:slug` like AW2. |
| 3 | **Theme from a reference image**: upload PNG/JPEG/WebP → AW2's client-side palette extraction (port `palette.ts` verbatim) → local suggestion; PLUS an optional "Refine with AI" that sends the extracted palette (the hex list, NOT the image) through the existing one-shot LLM helper pattern (like the transcript-cleanup call) to name the theme and assign tokens. LLM unavailable/slow → the local suggestion stands (AW2's fallback order). |
| 4 | **Persistence in `config.json`** (`customThemes` map + the active theme id already lives client-side — move the active id to config too so it survives localStorage wipes). Deviation from AW2 (vault `_config_custom_themes`): cosmetic state is appconfig in aw, the vault is for secrets. |
| 5 | **Agent actions**: `app.theme.set` learns custom ids; new `app.theme.custom.set {name, background, surface, text, accent, ...}` (create/update, returns the id) and `app.theme.custom.delete {id}`. "Me faz um tema neon" should just work. Validation mirrors `app.theme.set`'s shape; the catalog drift convention (Go↔frontend theme lists) extends to custom ids being pass-through. |
| 6 | Applying stays CSS-variable-based on the existing token system — map AW2's token names onto aw's `tokens.css` variables ONCE in a single translation table; never sprinkle per-component overrides. |

## 3. What already exists — reuse, don't reinvent

- aw's theme tokens (`frontend/src/theme/tokens.css`) and the
  `app.theme.set` action/validation shape.
- The Apps-screen grid/search/order/size controls (port-share the
  components, don't duplicate — if Apps doesn't have the size slider yet,
  build it shared from the start).
- The one-shot LLM helper pattern (transcript cleanup) for Decision 3.
- `config.json` merge-on-save.

## 4. Phases

### Phase 1 — Custom themes core

Token type + translation table + apply path + config persistence + the
editor (create/duplicate/edit/delete). Vitest: apply sets the vars;
required-token validation; duplicate copies tokens.

### Phase 2 — Gallery page + image suggestion

The Apps-style gallery (search/order/size), image upload → palette →
local suggestion → optional AI refine. Vitest: palette extraction on a
fixture image yields stable colors; fallback used when the LLM helper
rejects.

### Phase 3 — Agent actions

The three actions + SELFCODE + drift tests.

## 5. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. A custom theme with illegible colors is the user's right, but the editor
   shows a live contrast hint (AW2's `readableTextFor` logic) — port it.
2. `app.theme.custom.set` lets the agent restyle the app — values are
   validated as colors (`#rrggbb`), names length-capped; nothing else gets
   through. The user can always switch back in Settings.
3. The image never leaves the machine (Decision 3 sends only hex strings to
   the LLM) — keep it that way.
4. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart.
