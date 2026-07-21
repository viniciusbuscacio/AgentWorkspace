# Spec — In-app toast framework (Sonner foundation)

> **Status:** implemented (2026-07-01)
>
> - `frontend/src/components/ui/sonner.tsx` — the `<Toaster/>` wrapper, themed
>   with repo tokens
> - `frontend/src/app/App.tsx` — mounts `<Toaster/>` once at the App root
> - `frontend/src/components/ui/index.ts` — exports `Toaster` from the barrel
> - dependency: `sonner` (`frontend/package.json`)
>
> Foundation only. The **Delete provider on Windows** fix is the planned first
> consumer (`toast.success('… deleted')`) and is a separate spec — out of scope
> here.

## 1. Objective

Give the app a single global, ephemeral, always-visible toast surface, callable
imperatively from anywhere (`import { toast } from 'sonner'`) with no context,
provider, or hook. Auto-dismissing banner at the top-right, themed by the
current `data-theme`.

## 2. Context — why (the gap today)

The app has **no** in-app notification. What exists:

- `notifyService.notify()` — **native OS** notification; a silent no-op if the
  user disabled desktop notifications (`services/notify.service.ts`). Used in
  `AppShell.tsx`, `FontPage`, `PermissionsPage`, `ProvidersPage`, `SecurityPage`,
  `chat-send-state.ts`.
- `SettingsNotice` — a banner **trapped inside each Settings panel**
  (`modules/settings/components/SettingsTemplate.tsx`); it disappears when the
  panel unmounts (e.g. navigating away after a delete). Settings-only.
- ~17 modules keep a local `const [message, setMessage] = useState('')` with a
  fixed inline string: no auto-dismiss, no standardization.

None of these is a global, transient, reusable toast.

## 3. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Sonner, not Radix Toast.** shadcn deprecated its Toast and recommends Sonner. Sonner owns its state via a singleton, so we do **not** introduce the project's first React context — it matches the existing imperative/singleton style (`applyAppTheme`, `chat-send-state`, the event bus). |
| 2 | **No `richColors`** (it paints success green / error red with Sonner's own palette, ignoring the active theme). Every toast uses the themed neutral surface; error/warning get an accent from the repo's own tokens (`destructive` / `primary`, `!`-important over Sonner's base), mirroring `SettingsNotice`. So toasts always match the current theme instead of showing an off-theme green/red. |
| 3 | **Themed via repo tokens, not next-themes.** The repo drives ~11 themes (light and dark) through `document.documentElement.dataset.theme` (`lib/app-theme.ts`), not next-themes. Instead of mapping theme -> light/dark, point Sonner's own CSS vars at the repo tokens so it follows any `data-theme` automatically: `--normal-bg: var(--bg-elevated)`, `--normal-text: var(--text-primary)`, `--normal-border: var(--border)` (all confirmed in `globals.css`). |
| 4 | **Close button styled like the sidebar X**, but the placement/visibility/animation live in `frontend/src/theme/aw-toast.css` (imported in `main.tsx`), which overrides Sonner by specificity: top-right corner inside the box, fixed 22×22 (no outward translate), revealed only on toast hover/focus, `--text-muted` → hover `--bg-elevated`/`--text-primary` (beats Sonner's off-theme white hover with `!important`). The material `close` glyph is set via `icons` in `sonner.tsx`. Toasts also **enter and exit from the right** and use `expand` for a constant size. See `docs/ui-alerts.md`. |
| 5 | **Mounted once** in `app/App.tsx`, as a sibling of `<MainApp/>` (covers loading, lock gate, and shell). Sonner renders through its own portal on `document.body`, so it needn't wrap the tree. Skipped for the PiP window (early-return). |
| 6 | **Position `top-right`, `duration` 4000ms, `closeButton` on.** |

## 4. What already exists — reuse, don't reinvent

- `cn` from `@/lib/utils` for composing classNames.
- The `.nav-chat-close` class (`aw-sidebar.css`, already imported globally).
- Theme tokens `--bg-elevated`, `--text-primary`, `--border` (`globals.css`).
- The thin-wrapper pattern of the other `components/ui/*` primitives.

## 5. Files

- `frontend/package.json` + `package-lock.json` — add `sonner`.
- **new** `frontend/src/components/ui/sonner.tsx` — the `<Toaster/>` wrapper.
- `frontend/src/app/App.tsx` — mount `<Toaster/>`.
- `frontend/src/components/ui/index.ts` — export `Toaster` (barrel).
- **new** `frontend/src/components/ui/sonner.test.tsx` — Vitest.

No backend change. No new theme token. Existing files edited with `edit_file`.

## 6. Tests (Vitest + Testing Library, no jest-dom)

`frontend/src/components/ui/sonner.test.tsx`:

- `toast('Hello world')` inside `act(...)` -> text lands in `document.body`.
- `toast.error(...)` -> a `[data-type="error"]` node carries the message.
- close button carries `nav-chat-close` + `opacity-100`, and its glyph is the
  `material-symbols-outlined` "close" (not Sonner's default SVG).
- the `style` prop points Sonner's CSS vars at the repo tokens on
  `[data-sonner-toaster]`.

Assert on text / `className` / `data-*` directly (repo has no jest-dom). Sonner
mounts its list lazily (only once a toast exists) and via a portal on
`document.body`, so scope queries to `document.body` and `toast.dismiss()`
between tests.

## 7. Gate

```sh
cd frontend && npm run build:frontend
cd frontend && npx vitest run src/components/ui/sonner.test.tsx
```

## 8. Risks / attention

1. **Zoom transform.** AppShell applies a CSS zoom transform; the Toaster is
   mounted **outside** that container, so it keeps a fixed size regardless of
   zoom. Acceptable.
2. Sonner needs `requestAnimationFrame`/timers under happy-dom; if a test is
   flaky, isolate with fake timers and `await act(...)`.
3. Do not over-notify — a toast is for transient confirmation. Errors the user
   must act on can stay inline.

## Next (out of scope)

Wire the **Delete provider on Windows** spec as the first real consumer
(`toast.success('… deleted')`), then gradually migrate `setMessage` /
`notifyService.notify` / `SettingsNotice` where it fits (~17 candidate modules:
Wallpaper, Notes, Passwords, Skills, Providers, ...).
