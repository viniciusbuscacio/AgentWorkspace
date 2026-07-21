# UI alerts & feedback

How the app tells the user something happened. Read this before adding a new
"Saved."/"Deleted."/"Failed" message to a module.

## The three mechanisms — when to use which

| Mechanism | Use for | Notes |
|---|---|---|
| **`toast()` (Sonner)** | Transient feedback for an action the user should see **even after navigating away** — "X deleted", "Saved", "Failed to save". | The app-wide standard for action feedback. Always visible; auto-dismisses. |
| **`SettingsNotice`** (`modules/settings/components/SettingsTemplate.tsx`) | A message that belongs **next to a form/field** and should persist while the user is there — inline validation, contextual warnings. | Lives inside a panel; unmounts when you leave the screen (so it is the wrong tool for post-navigation confirmation). |
| **`notifyService.notify`** (`services/notify.service.ts`) | **Background / cross-app** events — e.g. "chat reply ready" when the window is not focused. | Native OS notification. Silent no-op when the user disabled notifications, so never rely on it as the only feedback. |

Rule of thumb: if the action **navigates away** or the confirmation must be seen
regardless of where the user is, use a **toast**. If the message is tied to the
form the user is looking at, use **`SettingsNotice`**.

## Toast usage

The `<Toaster/>` is mounted once at the app root (`app/App.tsx`); Sonner keeps a
module-level queue, so just call the imperative singleton from anywhere — no
context/provider, no props to thread:

```ts
import { toast } from 'sonner';

toast.success('Custom OpenAI-compatible deleted'); // themed neutral surface
toast.error('Failed to save');                     // destructive accent
toast('Configuration saved.');                     // plain (same as success visually)
toast.warning('Heads up');                         // primary accent
```

Do **not** import or mount another `<Toaster/>`, and do not add a toast library —
`sonner` is already wired.

## Styling (centralized — don't re-theme per module)

All toasts follow the active theme automatically; individual modules never style
them. The look is defined in two places:

- **`frontend/src/components/ui/sonner.tsx`** — the `<Toaster/>` wrapper:
  - `position="top-right"`, `duration={4000}`, `closeButton`, `expand`.
  - **No `richColors`** (it paints success green / error red with Sonner's own
    palette, ignoring the theme). Instead Sonner's CSS vars point at the repo
    tokens: `--normal-bg → var(--bg-elevated)`, `--normal-text → var(--text-primary)`,
    `--normal-border → var(--border)`, so toasts match any `data-theme`.
  - Error/warning accents come from the repo's own tokens (`destructive` /
    `primary`), mirroring `SettingsNotice`, via `toastOptions.classNames`.
  - `expand` keeps a constant size (Sonner otherwise resized the box on hover).
  - The close button uses the material `close` glyph (via `icons`).

- **`frontend/src/theme/aw-toast.css`** (imported in `main.tsx`) — overrides Sonner
  by specificity (extra `[data-sonner-toaster]` ancestor / `!important` where it
  ties Sonner's own rules):
  - Close button inside the **top-right** corner, fixed 22×22, no outward
    translate (so revealing it never resizes the toast).
  - Close button **revealed only on toast hover / focus**, styled exactly like
    the sidebar X (`--text-muted` → hover `--bg-elevated` / `--text-primary`;
    beats Sonner's off-theme white hover).
  - **Enter and exit from the right** (slide right↔left) at the top-right corner,
    instead of dropping in from the top.

To change toast appearance globally, edit those two files — not the call sites.

## History / specs

- `docs/plans/in-app-toast-framework-spec.md` — why Sonner (not Radix Toast), and
  the foundation.
- `docs/plans/provider-delete-windows-dialog-spec.md` — the first real consumer
  (provider delete → `toast.success`).
