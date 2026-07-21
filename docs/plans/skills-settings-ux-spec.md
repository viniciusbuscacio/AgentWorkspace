# Spec — Settings Skills UX polish: breadcrumb edit state + Add Skill editor

> **Status:** implemented (2026-06-24) — edit/create render through the Settings breadcrumb (`Settings › Skills › <id>` / `Add Skill`), no local Back button or duplicate title; Add Skill opens the SKILL.md template editor; create validates the id as `^[a-z0-9][a-z0-9-]*$` (UI + backend); App.CreateSkill Wails bridge added (no REST/MCP change). Go + frontend gates green; wails build packaged.

## 1. Objective

Polish **Settings › Skills** so skill editing follows the same detail-navigation
pattern used by the rest of Settings, and add a direct **Add Skill** flow that
lets the user author a skill from plain text without importing a folder/file.

This spec is primarily UI/UX and must use the existing skill persistence path.
It does not implement new `aw` tool actions and does not change REST/MCP
behavior.

Repository preflight result, verified before implementation:

- Existing persistence use case: `internal/application/skills_manage.go` exposes
  `application.CreateSkill(...)` for direct user-skill creation.
- Existing agent/REST/MCP action: `skill.create` is already wired through
  `internal/infrastructure/tools/aw_skills.go` and `App.skillManageFuncs()`.
- Missing UI bridge: there is currently no Wails binding exported to the
  frontend, and `frontend/src/services/skills.service.ts` has no `create(...)`
  wrapper.

Therefore this spec is ready for implementation with one explicit in-scope UI
bridge: add a Wails `App.CreateSkill(...)` method that delegates to the existing
`application.CreateSkill(...)`, refreshes skills context like `SaveSkill`, and
is exposed through `skillsService.create(...)`. This is not a new REST/MCP action
and must not introduce new persistence semantics.

## 2. Observed issue

Current edit mode inside `Settings › Skills` renders an internal header like:

```txt
Editing avell-health
Edits to a builtin skill mark it customized; use “Reset” on the list to restore the bundled version.
[Back]
```

This is inconsistent with the Settings navigation pattern. The expected title
should be represented in the main Settings breadcrumb/header:

```txt
Settings › Skills › avell-health
```

The local **Back** button is also out of pattern. Navigation should use the
existing Settings breadcrumb behavior and the standard Save/Cancel footer.

The page is also missing an **Add Skill** button for creating a new skill from a
simple text editor.

## 3. Locked decisions

| # | Decision |
|---|---|
| 1 | Skill edit/create state must appear in the main Settings breadcrumb as `Settings › Skills › <skill name/id>` or `Settings › Skills › Add Skill`. |
| 2 | Remove the local `Back` button from the Skills edit screen. It is outside the current Settings detail pattern. |
| 3 | Keep navigation consistent: clicking `Settings` returns to Settings home; clicking `Skills` returns to the Skills list when in a child edit/create state. |
| 4 | Add an **Add Skill** button in the Skills list toolbar, alongside `Import Skill` and `Import Folder`. |
| 5 | **Add Skill** opens a simple text editor, not a complex multi-step wizard. The default content is a single `SKILL.md` template. |
| 6 | Save in Add Skill creates a user skill in the vault. Cancel returns to the Skills list without persisting. |
| 7 | Existing import flows remain unchanged. Add Skill is an additional creation path, not a replacement for import. |
| 8 | Add Skill v1 requires `name` to already be a valid skill ID. No magical slug generation. |
| 9 | Dirty-state behavior must be identical for edit and create: Cancel and breadcrumb navigation may discard, but both modes must behave consistently. |
| 10 | Add Skill persistence uses existing `application.CreateSkill(...)`; the only backend surface addition in scope is a Wails UI binding, not a new REST/MCP action. |

## 4. Desired UX

### 4.1 Skills list

Header remains:

```txt
Settings › Skills
Procedural skills the agent can load on demand.
```

Toolbar actions should be:

```txt
[Add Skill] [Import Skill] [Import Folder]
```

Ordering rationale:

1. `Add Skill` = native/manual creation in AW.
2. `Import Skill` = bring one external skill into AW.
3. `Import Folder` = batch import external skills.

### 4.2 Edit existing skill

When editing `avell-health`, the main Settings header should show:

```txt
Settings › Skills › avell-health
```

Body should start directly with the editor content. Do not render a duplicate
local title like `Editing avell-health` unless it is visually part of the editor
body and not competing with the breadcrumb.

Remove this local button:

```txt
Back
```

The footer remains the standard `SaveCancelActions` pattern:

```txt
[Cancel] [Save]
```

Expected behavior:

- `Cancel` returns to the Skills list.
- Clicking the `Skills` breadcrumb returns to the Skills list.
- If edits are dirty and the app has an existing unsaved-changes pattern, reuse
  it. If not, keep current behavior and simply discard on Cancel/breadcrumb for
  this spec.

### 4.3 Add Skill

Clicking **Add Skill** opens a child detail state with breadcrumb:

```txt
Settings › Skills › Add Skill
```

Editor should be simple and text-first.

Initial state:

- One file draft: `SKILL.md`
- Pre-filled template:

```md
---
name: my-skill
description: Describe when the agent should use this skill.
---

# Skill: My Skill

Use this skill when...

## Procedure

1. ...
```

The user can edit the text directly.

Save behavior:

- Parse `name` and `description` from frontmatter.
- Create a user skill in the vault.
- In create mode, Save must call the create path; in edit mode, Save must call
  the existing update/save path. Never attempt to save create mode with `id: ''`.
- `name` is the skill ID in v1. It must already be valid; do not auto-slugify.
- Valid `name` pattern: `^[a-z0-9][a-z0-9-]*$`.
- Examples:
  - valid: `my-skill`, `gmail2`, `aw3-dev`
  - invalid: `My Skill`, `my_skill`, ` skill`, `my.skill`
- If `name` is missing or invalid, show a validation error.
- If the ID already exists, show a duplicate error.
- Duplicate detection must consider every skill visible to AW's skill store,
  including builtin, user/imported and package/bundled skills if that concept is
  present in the current implementation.
- On success, return to the Skills list and refresh it.

Cancel behavior:

- Return to Skills list.
- Do not persist anything.
- Behavior must match edit mode Cancel/breadcrumb discard semantics.

## 5. State model

Replace the current boolean-ish `editing` state concept with an explicit mode.

Suggested UI state shape:

```ts
type SkillEditorMode = 'edit' | 'create';

interface SkillEditorState {
  mode: SkillEditorMode;
  id: string;
  title: string;
  files: SkillFileDraft[];
  original: string;
}
```

Examples:

Existing skill:

```ts
{
  mode: 'edit',
  id: 'avell-health',
  title: 'avell-health',
  files: [...],
  original: JSON.stringify(files),
}
```

New skill:

```ts
{
  mode: 'create',
  id: '',
  title: 'Add Skill',
  files: [{ path: 'SKILL.md', content: DEFAULT_SKILL_TEMPLATE }],
  original: JSON.stringify([{ path: 'SKILL.md', content: DEFAULT_SKILL_TEMPLATE }]),
}
```

## 6. Settings breadcrumb integration

`SettingsModule` currently owns detail titles for some pages through callbacks,
for example provider/theme child titles.

Extend this pattern for Skills.

Suggested changes:

- Add state in `SettingsModule`:

```ts
const [skillsDetailTitle, setSkillsDetailTitle] = useState<string | null>(null);
```

- Pass callback to Skills page:

```tsx
{page === 'skills' && (
  <SkillsPage onSkillDetailTitleChange={setSkillsDetailTitle} />
)}
```

- Include skills child in `SettingsDetailTitle`:

```ts
child={
  page === 'providers' ? providerDetailTitle
  : page === 'theme' ? themeDetailTitle
  : page === 'skills' ? skillsDetailTitle
  : null
}
```

- When breadcrumb `Skills` is clicked while a skill editor is open, return to the
  Skills list instead of leaving Settings entirely.

Suggested callback shape:

```tsx
<SkillsPage
  onSkillDetailTitleChange={setSkillsDetailTitle}
  listRequest={skillsListRequest}
/>
```

Or a simpler approach:

- `SkillsPage` owns editor/list state.
- It calls `onSkillDetailTitleChange('avell-health')` when entering edit.
- It calls `onSkillDetailTitleChange('Add Skill')` when entering create.
- It calls `onSkillDetailTitleChange(null)` when returning to list/unmounting.
- It also clears the child title when the parent `Settings` breadcrumb is used to
  leave the Skills page entirely.

Acceptance requirement:

- There must not be two competing titles. The breadcrumb is the title.

## 7. Add Skill persistence path

Use the existing direct-create backend path:

```go
application.CreateSkill(store, id, name, description, enabled, files)
```

Current repo state:

- `application.CreateSkill(...)` exists and persists `origin=user` skills into
  the vault.
- `skill.create` already exists for the aw action gateway.
- The frontend currently lacks a Wails binding and service wrapper for create.

Required implementation shape:

```go
func (a *App) CreateSkill(id string, name string, description string, enabled bool, files []domain.SkillFile) dto.SkillResult
```

The method should:

1. Delegate to `application.CreateSkill(a.vault, id, name, description, enabled, files)`.
2. Refresh skills context on success, same as `SaveSkill`.
3. Return `dto.SkillResult` with `success/error`.
4. Not add or modify REST/MCP/`aw` actions.

Frontend service API:

```ts
skillsService.create({
  id,
  name,
  description,
  enabled: true,
  files,
}): Promise<SkillResult>
```

UI should display backend errors as inline message text using the existing
`message` pattern.

The UI may do cheap pre-validation for the v1 ID rule (`name` must match
`^[a-z0-9][a-z0-9-]*$`) before calling create, but backend validation remains the
source of truth.

Rules:

- Created skills are `origin=user`.
- Created skills default to enabled.
- Created skill ID comes from frontmatter `name` and must match
  `^[a-z0-9][a-z0-9-]*$`.
- Add Skill with a duplicate ID must fail even if the existing skill is builtin,
  imported/user or package/bundled.
- The source of truth after save is the AW vault/SQLite.
- Do not write to the original external skills folder.

## 8. Editor layout

Reuse the current file editor visual style:

```tsx
<div className="flex flex-col gap-1.5 rounded-lg border border-border bg-card p-3">
  <code className="text-xs text-muted-foreground">SKILL.md</code>
  <Textarea className="min-h-[...] resize-y font-mono text-xs" />
</div>
```

For Add Skill v1:

- Only `SKILL.md` is required.
- No separate file management UI is required.
- No multi-file add/remove UI in this spec.

Future enhancement, not part of this spec:

- Add auxiliary files to a skill from the UI.
- Preview parsed frontmatter.
- Live validation of name/description.

## 9. Copy changes

List page intro remains acceptable:

```txt
Skills live in your encrypted vault. Only each skill’s trigger is in the prompt; the agent loads the full procedure on demand.
```

Edit mode note should be moved out of the duplicate title area. If kept, render
as a small contextual note above the editor or near Save/Cancel:

```txt
Edits to a builtin skill mark it customized. Use Reset on the Skills list to restore the bundled version.
```

For user-created skill:

```txt
Create a skill by editing its SKILL.md. The name and description come from frontmatter.
```

## 10. Tests

### Component/unit tests

Add or update tests for `SkillsPage` and `SettingsModule`:

1. Opening edit for `avell-health` calls `onSkillDetailTitleChange('avell-health')`.
2. The edit screen does not render a `Back` button.
3. Cancel in edit calls `onSkillDetailTitleChange(null)` and returns to list.
4. Save in edit refreshes list and clears child breadcrumb title.
5. Add Skill button exists in list toolbar.
6. Clicking Add Skill calls `onSkillDetailTitleChange('Add Skill')`.
7. Add Skill editor starts with one `SKILL.md` draft containing frontmatter.
8. Cancel in Add Skill returns to list and does not call create/save.
9. Save in Add Skill calls the create service and refreshes the list.
10. Backend validation errors are displayed and keep the editor open.
11. Unmounting `SkillsPage` calls `onSkillDetailTitleChange(null)`.
12. Clicking the parent `Settings` breadcrumb while in edit/create mode clears the
    Skills child title.
13. Creating a skill with invalid `name` shows an error and does not call create.
14. Creating a duplicate skill keeps the editor open and displays the duplicate
    error.
15. Creating a skill successfully results in a list row with `origin=user` and
    `enabled=true`.
16. Editing a builtin skill may show the contextual customized-note, but does not
    render a duplicate title.

### Manual smoke

With app running and vault unlocked:

1. Open `Settings › Skills`.
2. Click `Edit` on `avell-health`.
3. Verify header is:

```txt
Settings › Skills › avell-health
```

4. Verify there is no local `Back` button.
5. Click breadcrumb `Skills`; verify list returns.
6. Click `Add Skill`.
7. Verify header is:

```txt
Settings › Skills › Add Skill
```

8. Verify editor contains default `SKILL.md` template.
9. Create a test skill.
10. Verify it appears in the list as `origin=user` and enabled.
11. Delete the test skill.

## 11. Definition of done

The implementation is complete when:

- Skills edit/create child states render only through the Settings breadcrumb.
- No local `Back` button or duplicate edit title appears in the Skills editor.
- `Add Skill` opens the `SKILL.md` text editor with the default template.
- Create mode validates `name` as the explicit skill ID and does not auto-slug.
- Duplicate IDs are rejected across all visible skill origins.
- Save in create mode uses the existing `application.CreateSkill(...)` path via
  the Wails UI binding.
- Save in edit mode continues to use the existing update/save path.
- Cancel, breadcrumb `Skills` and parent `Settings` navigation clear child title
  consistently.
- Component/unit tests and manual smoke pass.

## 12. Gates

Run standard frontend/backend checks:

```sh
go test ./...
cd frontend && npm run build:frontend
```

If touching generated Wails bindings or backend create APIs, also run the normal
AW build gate used by the repo.

## 13. Non-goals

- Do not implement new REST/MCP skill write actions in this spec.
- Do not redesign the entire Skills page.
- Do not add multi-file skill editing controls.
- Do not change import semantics.
- Do not add confirmation prompts.
- Do not add a separate Skills module outside Settings.

## 14. Risks / attention

1. **Breadcrumb ownership:** avoid duplicating navigation state between
   `SettingsModule` and `SkillsPage`. One source must drive the child title.
2. **Dirty editor state:** if breadcrumb navigation discards unsaved changes,
   make that behavior intentional and consistent with existing Settings pages.
3. **Create vs save:** do not overload existing save in a way that accidentally
   edits a skill with an empty ID.
4. **Frontmatter parsing:** Add Skill depends on valid frontmatter. Surface
   validation errors clearly.
5. **Pattern consistency:** no local Back button. Use breadcrumb + Save/Cancel.
6. **Wails bridge scope:** adding the Wails `App.CreateSkill(...)` binding is in
   scope only as a UI bridge to existing `application.CreateSkill(...)`. Do not
   add new REST/MCP actions or new persistence semantics.
7. **ID semantics:** v1 deliberately rejects pretty names like `My Skill` instead
   of guessing a slug. This avoids silent collisions and surprising IDs.
8. **Backend validation gap:** current `application.CreateSkill(...)` enforces
   non-empty and unique ID, but the v1 regex rule is stricter. UI must
   pre-validate `^[a-z0-9][a-z0-9-]*$`; if backend validation is strengthened,
   keep error messages consistent.
