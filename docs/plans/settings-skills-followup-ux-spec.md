# Spec — Settings as module + Skills search/filter + unsaved-change guard

> **Status:** implemented (2026-06-24) — (1) unsaved-change guard propagates SkillsPage → SettingsModule → AppShell and intercepts Cancel / breadcrumb / page-switch / module-close / sidebar / chat / Apps / ui:navigate before discarding a dirty editor; (2) Settings is now a Fixed (non-Core) catalog module rendered through the normal module/sidebar flow (one shared open path, idempotent, deep-links preserved, `app.navigate settings` always accepted); (3) Skills list gained a `Search skills...` box + `All Skills / User Skills / Built in Skills` origin filter (AND semantics). Go + frontend gates green; wails build packaged.

## 1. Objective

Follow-up UX polish after the first Settings › Skills editor pass.

This spec covers three user-facing issues:

1. Warn before losing unsaved skill editor content.
2. Make **Settings** behave like a normal workspace module in the sidebar when opened.
3. Add search + origin filter to `Settings › Skills`, matching the search pattern from **Open Apps and Modules**.

No REST/MCP changes. No `aw` action changes.

## 2. Problems to solve

### 2.1 Unsaved Add/Edit Skill can be lost silently

Current/target editor flow:

```txt
Settings › Skills › Add Skill
Procedural skills the agent can load on demand.

Create a skill by editing its SKILL.md. The name and description come from frontmatter.

SKILL.md
---
name: my-skill
description: Describe when the agent should use this skill.
---
...

[Save] [Cancel]
```

If the user changes the text and leaves without saving, the app must warn that
the content will be lost.

### 2.2 Settings is not represented as an opened sidebar module

When the user opens Settings, it should appear in the sidebar just like a normal
module/session, and the user should be able to close it normally later.

Expected mental model:

```txt
Sidebar:
- Chat
- Notes
- Settings   ← appears after user opened Settings
```

Closing Settings should behave like closing other non-core modules.

### 2.3 Skills list lacks search/filter

Current list header:

```txt
Settings › Skills
Procedural skills the agent can load on demand.

Skills live in your encrypted vault. Only each skill’s trigger is in the prompt; the agent loads the full procedure on demand.
```

Need a search/filter row copied from the visual pattern of:

```txt
Open Apps and Modules
Choose a module type to open in your workspace.

[search]
```

But for Skills, instead of `Order by: ...`, show an origin filter:

```txt
[Search skills...] [All Skills ▼]
```

Options:

```txt
All Skills
User Skills
Built in Skills
```

## 3. Locked decisions

| # | Decision |
|---|---|
| 1 | Dirty skill editor state must trigger a warning before leaving by Cancel, breadcrumb, Settings navigation, sidebar/module close, or any other in-app navigation path that would discard edits. |
| 2 | The warning copy must be explicit that unsaved content will be lost. |
| 3 | `Save` persists and clears dirty state. `Discard` leaves and loses changes. `Keep editing` stays on the editor. |
| 4 | Settings must be represented as a workspace module/sidebar item once opened. |
| 5 | Settings is not core: it can be closed like other normal/fixed modules. |
| 6 | Skills list gets a search box and origin filter using the same visual language/classes as Home/Open Apps and Modules. |
| 7 | Skills filter options are exactly: `All Skills`, `User Skills`, `Built in Skills`. |
| 8 | Search and filter combine with AND semantics. |
| 9 | This spec does not add or change REST/MCP/aw actions. |
| 10 | `app.navigate { view: "settings" }` / `ui:navigate` to Settings must remain valid even when Settings is closed/hidden/not currently added to the sidebar. It must add/show/focus the singleton Settings module, not fail validation and not open a sidebar-less special surface. |
| 11 | Settings integration uses a registered Settings module view plus optional runtime props. The implementation must not leave the `SettingsModule` integration strategy open-ended. |

## 4. Unsaved-change guard

### 4.0 Architecture requirement: guard must propagate to AppShell

The dirty editor state lives inside:

```txt
frontend/src/modules/settings/pages/SkillsPage.tsx
```

But many navigation/close paths that can discard that state live above it,
especially in `SettingsModule` and `AppShell`:

- sidebar module switching
- sidebar close button
- sidebar context menu → Close
- Chat/Home/Apps navigation
- profile/avatar menu entries
- `ui:navigate` / app navigation commands
- Settings page switches and deep links

Therefore this cannot be implemented only as a local `Cancel` dialog inside
`SkillsPage`. The implementation must expose an explicit dirty-exit guard upward
through the chain:

```txt
SkillsPage → SettingsModule → AppShell
```

or through an equivalent registration contract such as:

```ts
registerBeforeLeaveGuard('settings', guardFn)
```

Acceptance requirement: every path that would unmount or hide Settings while a
Skills editor is dirty must go through the same guard before proceeding.

### 4.1 When to warn

Warn if the skill editor is dirty and the user attempts to leave via:

- `Cancel`
- breadcrumb `Skills`
- breadcrumb `Settings`
- switching Settings page
- closing the Settings module/sidebar item
- opening another module if it unmounts the editor
- app navigation that would discard the editor
- `ui:navigate` or equivalent command-driven navigation

Dirty means:

```ts
JSON.stringify(editor.files) !== editor.original
```

or equivalent existing dirty check.

### 4.2 Dialog behavior

Use the existing app dialog component/pattern, preferably `AlertDialog` if that
is already used in the Skills page.

Suggested copy:

```txt
Discard unsaved skill changes?

You changed this skill but have not saved it. If you leave now, those edits will be lost.

[Keep editing] [Discard changes]
```

Button semantics:

- `Keep editing`: close dialog, stay in editor.
- `Discard changes`: perform the originally requested navigation/close.
- Optional future: `Save and leave` is not required in this spec.

### 4.3 Pending navigation intent

Implementation should store the pending navigation intent so the same dialog can
handle multiple exit sources, including navigation initiated outside the
`SkillsPage` component.

Example state shape:

```ts
type PendingSkillExit =
  | { kind: 'list' }
  | { kind: 'settings-home' }
  | { kind: 'settings-page'; page: SettingsPage }
  | { kind: 'module-close' }
  | { kind: 'sidebar-module'; moduleId: string }
  | { kind: 'home' }
  | { kind: 'external-navigation'; run: () => void };
```

Exact shape is flexible, but behavior must be deterministic.

### 4.4 Acceptance criteria

- Editing `SKILL.md`, clicking `Cancel`, and choosing `Keep editing` preserves all text.
- Editing `SKILL.md`, clicking breadcrumb `Skills`, and choosing `Discard changes` returns to Skills list.
- Dirty create mode and dirty edit mode behave identically.
- Clean editor exits without dialog.
- After successful Save, exiting does not show dialog.

## 5. Settings as a sidebar module

### 5.1 Desired behavior

When the user opens Settings from any entry point, Settings should appear in the
sidebar as a workspace module.

Examples of entry points:

- profile/avatar menu
- gear/settings button
- any existing command that opens Settings directly

After opening:

```txt
Sidebar contains Settings
Current module is Settings
```

The user can close Settings normally from the sidebar close affordance.

### 5.2 Current architecture note

Settings is currently a special AppShell surface, not a normal module. Existing
code treats `chat`, `home` and `settings` as fixed surfaces outside the generic
module catalog/rendering path.

Known areas that likely need review:

- `internal/domain/module.go` — add `settings` to `ModuleCatalog()` as Fixed,
  non-Core.
- `frontend/src/modules/module-views.ts` — register a Settings module view or
  wrapper.
- `frontend/src/app/AppShell.tsx` — migrate Settings open/close/focus through the
  normal module/sidebar flow. Do not keep a second Settings render/open path
  outside the module/sidebar flow.
- `internal/application/modules.go` — update `NavigableViews` so `settings`
  remains accepted exactly once as a built-in navigable id even when Settings is
  not currently added/visible in the sidebar. Other non-core modules should keep
  the existing “added modules only” navigation rule.

Implementation must not leave two independent Settings paths: one that opens a
sidebar module and another special path that opens Settings without a sidebar
item.

### 5.3 Module registration

Add/register a module spec for Settings, unless one already exists.

Suggested module spec:

```go
{
  ID:          "settings",
  Name:        "Settings",
  Icon:        "settings",
  Description: "Configure providers, theme, security, memory, skills and app behavior.",
  Fixed:       true,
}
```

Rationale:

- `Fixed: true` matches built-in surfaces that can be opened/closed but not
  removed as user-created content.
- It must not be `Core`; closing Settings must be allowed.

If product direction is that Settings should not appear in the Apps catalog, use
the existing catalog visibility mechanism if one exists. If no such mechanism
exists, it is acceptable for Settings to appear in Open Apps and Modules as a
regular built-in module.

### 5.4 Routing / composition

Settings UI should render through the normal module rendering path, not as a
special overlay that never creates a sidebar item.

Opening Settings should effectively do the same kind of operation as opening any
other module:

```txt
show/open module "settings"
navigate/select module "settings"
```

Do not create duplicate Settings modules if it is already open; focus the
existing one.

### 5.5 Runtime props / SettingsModule integration

`SettingsModule` currently needs props that generic module views may not receive,
for example:

```ts
onLocked
initialPage
```

Use this concrete integration strategy:

1. Expand `ModuleRuntimeProps` with optional Settings-compatible props:

   ```ts
   interface ModuleRuntimeProps {
     createSpecChat?: (title: string, body: string) => Promise<void>;
     onLocked?: () => Promise<void> | void;
     settingsInitialPage?: SettingsPage;
   }
   ```

2. Register a small `SettingsModuleView` wrapper in `MODULE_VIEWS`:

   ```tsx
   const SettingsModuleView = (props: ModuleRuntimeProps) => (
     <SettingsModule
       onLocked={props.onLocked ?? (() => {})}
       initialPage={props.settingsInitialPage}
     />
   );
   ```

3. `AppShell` passes `onLocked` and the current Settings deep-link target through
   the generic module rendering path.

Do not keep an additional standalone `view === 'settings'` render path that can
show Settings without a sidebar item.

### 5.6 Preserve Settings deep links / entry points

Existing Settings entry points must continue to work after Settings becomes a
sidebar module.

Required behavior:

- Profile/avatar menu → `Settings` opens/focuses the Settings module.
- Profile/avatar menu → `Providers` opens/focuses Settings directly at
  `Settings › LLM Providers`.
- Any existing `app.navigate` / `ui:navigate` to Settings opens/focuses the same
  singleton Settings module.
- Repeated opens must not duplicate Settings in the sidebar.

Use one shared helper for every Settings entry point, conceptually:

```ts
openSettingsModule(initialPage?: SettingsPage): Promise<void>
```

That helper must:

1. run the dirty-exit guard if the current view would be discarded;
2. call `modulesService.add('settings')` or an equivalent idempotent add/show
   operation so Settings exists and is visible in the sidebar;
3. update local `modules` state from the returned module list;
4. store the requested `settingsInitialPage` deep-link target;
5. set the current view to `settings`.

`app.navigate { view: "settings" }` / `ui:navigate` semantics are locked:

- navigation validation must accept `settings` even if it is currently closed,
  hidden, or not in the persisted added-module list;
- handling the navigation must add/show/focus the singleton Settings module;
- the result must be indistinguishable from opening Settings from the profile
  menu: one sidebar item, selected, with no duplicate;
- it must never fall back to a legacy sidebar-less Settings surface.

### 5.7 Close behavior

Closing Settings:

- hides/closes the Settings module from the sidebar
- does not delete settings data
- if a dirty Skills editor is active, it must trigger the unsaved-change guard
  from section 4 before close proceeds

### 5.8 Acceptance criteria

1. Open Settings from the existing menu/button.
2. Sidebar shows `Settings` with the settings icon.
3. Current content is the Settings screen.
4. Closing Settings removes it from the sidebar.
5. Reopening Settings focuses/recreates it normally.
6. If Settings contains a dirty Add/Edit Skill editor, closing Settings asks
   before discarding.
7. Opening Providers from the profile/avatar menu opens Settings in the sidebar
   directly at `Settings › LLM Providers`.
8. `ui:navigate` / app navigation to Settings succeeds even when Settings is
   closed/hidden/not currently added, then adds/shows/focuses the same singleton
   Settings module and does not duplicate it.

## 6. Skills search + origin filter

### 6.1 Layout

In `Settings › Skills`, below the intro copy and above the skill list, add a
home-toolbar-style row.

Preferred visual pattern:

```txt
[ search icon | Search skills...                      ] [ All Skills ▼ ]
```

Use the same classes/patterns as **Open Apps and Modules** where practical:

- `.home-toolbar`
- `.home-search`
- `.home-search-input`
- `.chat-search-clear`
- `.home-order-select` or equivalent select styling

Do **not** include icon-size controls for the Skills list in this spec.

### 6.2 Search behavior

Search should match, case-insensitive, against:

- `skill.id`
- `skill.name`
- `skill.description`
- optionally file names if already available in the list view

Search is local/client-side over the loaded skills list.

Empty search shows all skills subject to current origin filter.

### 6.3 Origin filter behavior

Filter select options and semantics:

| Label | Meaning |
|---|---|
| `All Skills` | Show all non-filtered skills. |
| `User Skills` | Show skills where `origin === 'user'`. Imported skills count as user skills if that is how the domain models them. |
| `Built in Skills` | Show skills where `origin === 'builtin'`. |

Use exactly the visible label `Built in Skills` unless product copy later decides
on `Built-in Skills` globally. The user requested `Built in Skills`.

Soft-deleted skills, if returned by the current list view for reset/restore
purposes, remain visible in `All Skills` and participate in the origin filter by
their existing `origin` value. This spec does not change deleted-skill visibility.

### 6.4 Combined filtering

Search and origin filter combine with AND semantics:

```txt
visible = skills
  filtered by origin
  filtered by search query
```

Examples:

- `User Skills` + `gmail` shows user-origin skills matching `gmail`.
- `Built in Skills` + empty search shows all builtin skills.
- `All Skills` + `azure` shows all origins matching `azure`.

### 6.5 Empty states

If no skills match, show a concise empty state:

```txt
No skills match your filters.
```

If search is non-empty, optionally include the query:

```txt
No skills match "azure".
```

Do not show the generic `No skills yet.` unless the unfiltered list is truly
empty.

### 6.6 Acceptance criteria

1. Skills page shows a search input with placeholder `Search skills...`.
2. Skills page shows a select with `All Skills`, `User Skills`, `Built in Skills`.
3. Searching by ID filters the list.
4. Searching by description filters the list.
5. Selecting `User Skills` hides builtin skills.
6. Selecting `Built in Skills` hides user skills.
7. Search clear button resets search.
8. Empty filtered state is distinct from truly empty list state.

## 7. Tests

### 7.1 Skills editor dirty guard tests

Add/update component tests:

1. Dirty edit + Cancel opens discard dialog.
2. Dirty create + Cancel opens discard dialog.
3. Keep editing preserves textarea content.
4. Discard changes returns to Skills list.
5. Clean edit + Cancel returns immediately without dialog.
6. Successful Save clears dirty state; subsequent exit has no dialog.
7. Dirty edit + breadcrumb `Skills` opens discard dialog.
8. Dirty edit/create + module close path opens discard dialog if that path is
   testable at component/integration level.

### 7.2 Settings module tests

Add/update module/sidebar tests:

1. Opening Settings creates/shows a Settings sidebar item.
2. Opening Settings twice does not duplicate the sidebar item.
3. Closing Settings removes/hides it from sidebar.
4. Settings module is not core and is closeable.
5. Dirty Settings child editor blocks close until discard is confirmed.
6. Profile menu → Providers opens Settings in the sidebar and lands directly on
   LLM Providers.
7. Dirty editor → clicking Chat opens the discard dialog; Keep editing keeps
   Settings active; Discard navigates to Chat.
8. Dirty editor → clicking Home/Apps opens the discard dialog with the same
   semantics.
9. Dirty editor → sidebar context menu Close on Settings opens the discard dialog.
10. `ui:navigate` to Settings succeeds even when Settings is not currently
    added/visible, then adds/shows/focuses exactly one Settings sidebar item.
11. `ui:navigate` to Settings does not use or render any legacy sidebar-less
    Settings surface.
12. Session restore: if Settings was open at shutdown, it can restore as a
    sidebar item; if Settings was closed before shutdown, it must not reappear
    solely because it is a fixed/builtin surface.

### 7.3 Skills search/filter tests

Add/update Skills page tests:

1. Search input renders.
2. Filter select renders with the exact labels.
3. Search by name/id filters.
4. Search by description filters.
5. `User Skills` filters to `origin=user`.
6. `Built in Skills` filters to `origin=builtin`.
7. Search + origin filter combine.
8. Clear search restores origin-filtered results.
9. Empty filtered state copy appears when appropriate.

## 8. Manual smoke

Prerequisite: have at least one visible user skill and one visible builtin skill
for origin-filter validation. If needed, create a temporary user skill and delete
it after the smoke.

1. Open Settings.
2. Verify Settings appears in sidebar.
3. Open `Settings › Skills`.
4. Verify search input and filter select are visible.
5. Type `gmail`; verify list filters.
6. Select `User Skills`; verify builtin skills disappear.
7. Select `Built in Skills`; verify user skills disappear.
8. Click `Add Skill`.
9. Modify the template text.
10. Click breadcrumb `Skills`; verify discard warning.
11. Choose `Keep editing`; verify text remains.
12. Click `Cancel`; choose `Discard changes`; verify list returns.
13. Open Add Skill again, modify text, then close Settings from sidebar; verify discard warning.
14. Reopen Settings; verify it appears in sidebar again without duplicate entries.

## 9. Gates

Run normal frontend/backend checks:

```sh
go test ./...
cd frontend && npm run build:frontend
```

If module registration or Wails bindings change, run the normal AW build gate.

## 10. Non-goals

- No REST/MCP/aw action changes.
- No new skill persistence behavior beyond what already exists.
- No multi-file skill editor.
- No sorting controls for Skills list.
- No icon-size slider for Skills list.
- No global unsaved-change framework for every Settings page unless the existing
  architecture already has one. This spec only requires Skills editor coverage.

## 11. Risks / attention

1. **Navigation interception complexity:** sidebar close, breadcrumb clicks and
   page transitions live in different components. A local-only dialog in
   `SkillsPage` is insufficient; the guard must be exposed upward to AppShell or
   registered in a central before-leave mechanism.
2. **Settings module duplication:** opening Settings from multiple entry points
   must focus existing Settings, not create duplicates.
3. **Close semantics:** Settings is closeable but not removable user content.
4. **Filter naming:** use `Built in Skills` exactly as requested unless a broader
   product copy pass changes it.
5. **Dirty create loss:** Add Skill text can be long. The discard dialog must fire
   reliably for create mode, not only edit mode.
6. **Settings special-surface migration:** Settings currently behaves like a
   fixed AppShell surface. Moving it into the module/sidebar flow may touch
   backend module catalog, frontend module view registration, AppShell routing
   and navigable view persistence.
7. **Deep-link regression:** existing menu entries such as Providers must still
   open Settings directly to the expected page after the module migration.
8. **Navigation validation drift:** `settings` is a built-in navigable module id
   and must remain accepted by `app.navigate` even when it is closed/hidden/not
   currently added. Avoid making validation depend solely on the persisted
   added-module list for this one fixed built-in surface.
