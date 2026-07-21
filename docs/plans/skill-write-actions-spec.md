# Spec — AW skill write actions: skills are editable by the agent inside its workspace

> **Status:** implemented (2026-06-24) — skill.detail/save/create/set_enabled/import_file/import_folder/delete/reset exposed via the `aw` action gateway; frontmatter parser switched to yaml.v3 (block scalars). Backend-only, no UI. Go + frontend gates green; `wails build` packaged the app.

## 1. Objective

Expose full skill management through the existing **AW action surface** so any
agent connected to Agent Workspace can manage skills stored in the unlocked
vault.

The REST/MCP contract stays centered on one tool/action gateway:

```http
POST /api/aw
{
  "action": "skill.save",
  "args": { ... }
}
```

No separate `skills` tool is introduced. Skills are subactions of `aw`, just like
browser/app/provider actions.

## 2. Product principle

**Agent Workspace is the agent's workspace.**

If data/configuration lives inside the unlocked AW vault, the agent is allowed to
read and modify it. Users control agent authority by choosing what they place in
that vault/workspace, not by approving every internal workspace edit.

Therefore:

| Area | Rule |
|---|---|
| Skills inside unlocked AW vault | Agent can edit directly via `aw` actions. |
| External filesystem / external services | Keep existing sandbox/auth rules. |
| Destructive real-world actions outside AW | Still follow project safety rules. |
| Skill write actions | No extra confirmation/popup in backend. Bearer token + unlocked vault is enough. |
| Skill import actions | Import from a caller-supplied path; no local/default path is hardcoded. Imported content is copied into the AW vault, and SQLite/vault becomes the source after import. |

## 3. Current state

Already implemented/existing:

- Vault SQLite tables:
  - `skills`
  - `skill_files`
- Management use cases:
  - `ListSkillViews`
  - `GetSkillDetail`
  - `SaveSkillFiles`
  - `SetSkillEnabled`
  - `ImportSkillFromFile`
  - `ImportSkillsFromParent`
  - `DeleteSkill`
  - `ResetSkillToSeed`
- Runtime/progressive disclosure actions:
  - `skill.list`
  - `skill.read`
- Existing application bindings already support import/edit/delete/reset.

Gap:

- The `aw` action registry exposes only read/catalog actions.
- Agents cannot create/edit/import/delete skills through REST/MCP yet.
- Frontmatter multiline descriptions are currently parsed incorrectly in the
  catalog/management flows for block scalars like `>-`, `>` and `|`.

## 4. New AW actions

All actions are called through the existing `/api/aw` endpoint and action
registry.

Mutation actions should use a consistent response shape:

```json
{
  "success": true,
  "contextRefreshed": true,
  "restartRequired": false
}
```

Actions may include extra fields such as `skill`, `imported` or `skipped`.
Validation/persistence failures should return action errors using the existing
AW dispatcher conventions.

### 4.1 `skill.list`

Keep current behavior.

Returns the compact effective skill catalog using the existing response shape.
Do not wrap the result in a new `{ "skills": [...] }` object unless the current
`skill.list` contract is intentionally changed in a separate breaking-change
spec.

Current shape:

```json
[
  {
    "id": "gmail-web",
    "name": "gmail-web",
    "description": "...",
    "files": ["SKILL.md", "refs/example.md"]
  }
]
```

### 4.2 `skill.read`

Keep current behavior.

Args:

```json
{
  "id": "gmail-web",
  "path": "SKILL.md"
}
```

Rules:

- `path` defaults to `SKILL.md`.
- Reads only enabled/effective skills, matching current progressive disclosure.

### 4.3 `skill.detail`

Returns full persisted skill record, including disabled/deleted metadata where
appropriate for management flows.

Args:

```json
{
  "id": "gmail-web"
}
```

Result:

```json
{
  "skill": {
    "id": "gmail-web",
    "name": "gmail-web",
    "description": "Manage Gmail through an already-open browser tab...",
    "enabled": true,
    "origin": "builtin",
    "seedVersion": "1",
    "deleted": false,
    "files": [
      {
        "path": "SKILL.md",
        "content": "---\nname: gmail-web\ndescription: ...\n---\n\n# Skill...",
        "contentHash": "...",
        "seedHash": "..."
      }
    ]
  }
}
```

### 4.4 `skill.save`

Replaces the file set for an existing skill.

Args:

```json
{
  "id": "gmail-web",
  "files": [
    {
      "path": "SKILL.md",
      "content": "---\nname: gmail-web\ndescription: ...\n---\n\n# Skill..."
    },
    {
      "path": "refs/details.md",
      "content": "..."
    }
  ]
}
```

Rules:

- Reuse `SaveSkillFiles`.
- Must include `SKILL.md`.
- Normalize line endings/content exactly like the existing management path.
- Derive `name` and `description` from `SKILL.md` frontmatter.
- Builtin skills remain `origin=builtin`; saving a builtin marks it customized by
  changing content hash vs seed hash, as current use case already does.
- User skills remain `origin=user`.
- After save, refresh the runtime skills context.

Result:

```json
{
  "success": true,
  "contextRefreshed": true,
  "restartRequired": false,
  "skill": { "id": "gmail-web", "name": "gmail-web", "enabled": true }
}
```

### 4.5 `skill.create`

Creates a new user skill directly from supplied files. This avoids requiring a
filesystem import when an agent wants to author a skill from scratch.

Args:

```json
{
  "id": "my-skill",
  "name": "my-skill",
  "description": "Use when ...",
  "enabled": true,
  "files": [
    {
      "path": "SKILL.md",
      "content": "---\nname: my-skill\ndescription: Use when ...\n---\n\n# Skill: My Skill\n..."
    }
  ]
}
```

Rules:

- Reuse `ImportSkill` or an equivalent domain path suitable for direct creation.
- `id` must be unique.
- `origin` is always `user`.
- `enabled` defaults to `true`; if explicitly set to `false`, it must not be silently ignored.
- Must include `SKILL.md`.
- Derive/validate `name` and `description` from `SKILL.md`; explicit args are
  optional convenience and must not diverge silently from frontmatter.
- After create, refresh runtime skills context.

### 4.6 `skill.set_enabled`

Enables/disables a skill.

Args:

```json
{
  "id": "gmail-web",
  "enabled": false
}
```

Rules:

- Reuse `SetSkillEnabled`.
- Must return an error when the skill id does not exist.
- After mutation, refresh runtime skills context.

### 4.7 `skill.import_file`

Imports one skill from a `SKILL.md` path visible to the AW process.

Args:

```json
{
  "path": "<path-to-skill-md>"
}
```

Rules:

- Reuse `ImportSkillFromFile`.
- `path` is supplied by the caller; there is no backend default/hardcoded import path.
- Same duplicate behavior as the existing import path: duplicate IDs fail/skip
  according to current use case behavior.
- After import, refresh runtime skills context.

### 4.8 `skill.import_folder`

Imports every direct child folder containing `SKILL.md`.

Args:

```json
{
  "path": "<path-to-skills-parent-folder>"
}
```

Result mirrors the existing batch import summary:

```json
{
  "imported": ["gmail", "web-fetch"],
  "skipped": [
    { "id": "control-macos", "reason": "already exists" }
  ]
}
```

Rules:

- Reuse `ImportSkillsFromParent`.
- `path` is supplied by the caller; there is no backend default/hardcoded import path.
- If the root itself has `SKILL.md`, return the existing “use Import Skill” error.
- After successful partial/full import, refresh runtime skills context.

### 4.9 `skill.delete`

Deletes a skill.

Args:

```json
{
  "id": "my-skill"
}
```

Rules:

- Reuse `DeleteSkill`.
- Builtin skills are soft-deleted, so bootstrap does not resurrect them.
- User skills are physically removed.
- No confirmation in backend.
- After delete, refresh runtime skills context.

### 4.10 `skill.reset`

Restores a builtin skill to bundled seed.

Args:

```json
{
  "id": "gmail-web"
}
```

Rules:

- Reuse `ResetSkillToSeed`.
- Only works for bundled/builtin skills with a seed.
- Clears customization and undeletes if needed, matching current use case.
- After reset, refresh runtime skills context.

## 5. Runtime refresh requirement

Every mutation action must refresh the agent skills context after successful
persistence:

- `skill.save`
- `skill.create`
- `skill.set_enabled`
- `skill.import_file`
- `skill.import_folder`
- `skill.delete`
- `skill.reset`

Implementation should reuse the same application refresh path used after
existing skill mutations/unlock/bootstrap. The goal is:

1. SQLite/vault changes immediately.
2. Runtime skills index in prompt context updates for subsequent agent turns.
3. Subsequent agent turns see the new trigger list without app restart when
   possible.

If hot refresh is architecturally impossible for a specific client/session, the
action result must say so clearly:

```json
{
  "success": true,
  "contextRefreshed": false,
  "restartRequired": true
}
```

But preferred target is `contextRefreshed: true`.

If `AW_SKILLS_DIR` dev override is active, mutation actions may still persist to
the vault, but the effective runtime catalog is served from the dev override. In
that case, the action must not claim the effective catalog was refreshed. Return
an explicit status such as:

```json
{
  "success": true,
  "contextRefreshed": false,
  "restartRequired": false,
  "devOverrideActive": true,
  "warning": "AW_SKILLS_DIR is active; mutation was persisted to the vault but the effective runtime catalog is currently served from the dev override."
}
```

## 6. Frontmatter parser fix

Fix metadata parsing while implementing the write actions.

Problem examples currently catalog as raw scalar markers:

```yaml
description: >-
  Builda, roda e usa o AW3...
```

```yaml
description: >
  Executa um goal complexo...
```

```yaml
description: |
  Remove signs of AI-generated writing...
```

Required behavior:

- Parse frontmatter with a real YAML parser such as `gopkg.in/yaml.v3` or an
  equivalent parser that supports block scalars.
- Detect frontmatter delimiters as standalone `---` lines, not arbitrary `---`
  text inside the markdown body.
- `description: >-` and `description: >` fold multiline text into a single
  readable trigger paragraph.
- `description: |` preserves line breaks for stored content but catalog can
  normalize whitespace for display/prompt trigger.
- UTF-8 text and symbols (`→`, `—`, emoji, accents) must round-trip intact.

Tests should cover at least:

- single-line quoted description
- multiline folded `>`
- multiline strip-folded `>-`
- literal `|`
- missing description
- invalid/partial frontmatter

## 7. Suggested implementation points

### Backend

Files likely involved:

- `internal/infrastructure/tools/aw_skills.go`
- `internal/infrastructure/tools/aw_skills_test.go`
- `internal/infrastructure/tools/aw_registry.go`
- `internal/application/skills_manage.go`
- `internal/application/skills.go`
- `skills/*` parser/loader files
- `app_skills.go` only if existing application-level wrappers are useful

Implementation shape:

1. Extend the `tools.Options` / workspace callbacks with management functions,
   preferably as one grouped `SkillManage` callback set instead of many loose
   top-level callbacks.
2. Register write actions in `registerSkillActions` only when the required
   callbacks are non-nil.
3. Decode args with the existing `awRequiredStringArg` helpers where possible.
4. Return JSON through `awJSON` for consistency.
5. Preserve the existing `skill.list/read` behavior exactly.
6. If `AW_SKILLS_DIR` dev override is active, report `devOverrideActive: true`
   and `contextRefreshed: false` when the effective runtime catalog is still
   served from the override. Do not silently claim a hot refresh changed the
   effective catalog when it did not.

### UI

No UI work is part of this spec.

Do not add or change:

- buttons
- screens
- modals
- popups
- confirmation dialogs
- frontend routes/components
- Wails view behavior

This task is backend/action-surface only: expose `skill.*` through `/api/aw`,
persist in the vault, refresh runtime context, return JSON, and test through
REST/MCP.

## 8. Tests

### Go unit tests

Add/extend `internal/infrastructure/tools/aw_skills_test.go`:

- registers all new actions when management callbacks are present
- `skill.detail` requires `id`
- `skill.save` requires `id` and `files`
- `skill.create` requires `SKILL.md`
- `skill.set_enabled` parses boolean
- `skill.import_file` requires `path`
- `skill.import_folder` requires `path`
- `skill.delete` requires `id`
- `skill.reset` requires `id`
- callbacks are invoked with the expected args
- actions are absent or return unavailable when callbacks are nil, matching
  existing registry style
- mutation responses include `contextRefreshed` and `restartRequired`

Add/extend application tests:

- create user skill direct from files
- create user skill with `enabled=false` preserves disabled state
- reject `skill.create` when explicit `name`/`description` diverge from `SKILL.md` frontmatter
- save existing user skill
- save customized builtin skill
- delete builtin soft vs user hard
- reset builtin from seed
- import folder partial success preserves skipped list
- `skill.set_enabled` on an unknown id returns an error
- invalid skill file paths are rejected (`..`, absolute paths, empty paths, duplicates)
- runtime refresh hook called once after each successful mutation
- runtime refresh hook not called on validation/persistence errors

Add parser tests:

- YAML block scalar descriptions parse correctly
- quoted descriptions containing `:` parse correctly
- frontmatter parsing is not confused by `---` inside the markdown body
- UTF-8 survives parse/save/read/list

### Integration/manual smoke

After build, with AW running and vault unlocked:

```sh
EP=http://127.0.0.1:9301/api/aw
TOK=<bearer>

curl -s -X POST "$EP" -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
  -d '{"action":"skill.create","args":{"id":"smoke-skill","files":[{"path":"SKILL.md","content":"---\nname: smoke-skill\ndescription: Smoke test skill\n---\n\n# Smoke\n"}]}}'

curl -s -X POST "$EP" -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
  -d '{"action":"skill.list","args":{}}' | grep smoke-skill

curl -s -X POST "$EP" -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
  -d '{"action":"skill.save","args":{"id":"smoke-skill","files":[{"path":"SKILL.md","content":"---\nname: smoke-skill\ndescription: Updated smoke test skill\n---\n\n# Smoke updated\n"}]}}'

curl -s -X POST "$EP" -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
  -d '{"action":"skill.delete","args":{"id":"smoke-skill"}}'
```

Expected:

- create returns success
- list shows the new skill
- save updates description/body
- read/detail returns updated body
- delete removes user skill from list
- no app restart required for the catalog to update

## 9. Gates

Run the standard AW gates:

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

On Windows, use the equivalent available Go/npm commands if `golangci-lint` is
not installed locally.

## 10. Non-goals

- No separate REST endpoint such as `/api/skills`.
- No separate MCP tool outside the `aw` action gateway.
- No backend confirmation prompts for skill writes.
- No UI/frontend work: no buttons, screens, modals, popups, Wails view changes,
  or confirmation dialogs.
- No external filesystem synchronization back to the original imported skill
  folders. SQLite/vault is the source after import.
- No version history feature in this spec.
- No collaborative conflict resolution.

## 11. Risks / attention

1. **Prompt persistence:** skills alter future agent behavior. This is intended
   inside AW, but parser/validation bugs can make bad triggers hard to notice.
   Keep list/detail/read reliable and test block scalar parsing.
2. **Context staleness:** writing SQLite without refreshing runtime context would
   confuse agents. Treat refresh as part of the mutation contract.
3. **Builtin lifecycle:** deleting builtins must remain soft; otherwise bootstrap
   behavior can surprise users.
4. **Duplicate imports:** batch import should remain partial-success, not all-or-
   nothing, matching existing import behavior.
5. **UTF-8:** current Windows console output can look mojibake, but stored JSON
   and SQLite content must remain UTF-8 correct.
