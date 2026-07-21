# Spec — Settings → Agent Instructions

> **Status:** v1 implemented (Phases 1–3) 2026-06-24. Phases 4 (project/compatibility discovery) and 5 (agentic-five polish) remain future work.
>
> Goal: make Agent Workspace's markdown instruction files visible, understandable, and editable from a dedicated **Settings → Agent Instructions** page. This closes the visibility gap for the `AGENTS.md` part of the agent stack while leaving Skills as the on-demand procedural layer.
>
> Review note: this spec was checked against the current repo shape on 2026-06-24. It is intentionally a product/architecture spec only; do not implement from this review pass.

## 1. Objective

Add a first-class **Agent Instructions** settings page that shows the markdown files and effective instruction context that shape how the Agent Workspace agent behaves.

The page should answer:

- What instruction files does this agent currently use?
- What is in `AGENTS.md`?
- What is in `USER.md`?
- Which sources are bundled, user-edited, imported, or discovered from a project?
- What effective instruction block is currently injected into the agent?
- Which instructions are editable, read-only, customized, or resettable?

This is the product surface for the first term from the "5 AI agent terms" roadmap:

```txt
AGENTS.md = project/workspace rules and instructions for the agent.
```

## 2. Product framing

Use this user-facing name:

```txt
Settings → Agent Instructions
```

Subtitle:

```txt
Markdown instruction files that shape how the agent behaves.
```

Avoid names like `MD Config Files`, `Markdown Config`, or `Prompt Config Files`. Markdown is the implementation detail; the product concept is **agent instructions**.

## 3. Current state

Already exists:

- A bundled runtime `AGENTS.md` file at `skills/bundled/AGENTS.md`.
- `domain.AppDocument` and `domain.AgentsDocumentID = "AGENTS.md"`.
- Vault persistence for app documents through the `app_documents` table.
- `skills.BundledAgentsDoc()` and bootstrap/update logic in `app.go` / `app_skills.go`.
- `Runtime.SetSkillsContext` injects the composed skills-slot context into the agent runtime.
- `application.RefreshSkillsContext` currently composes runtime `AGENTS.md` + the Skills index.
- `AW_SKILLS_DIR` can replace the effective skills catalog and optionally the effective runtime `AGENTS.md` during dev.
- Settings → Skills already manages procedural skill files, including search/filter and unsaved-change guard.
- Settings → Debug Mode / prompt visibility exists, but it is not a friendly instruction-file surface.

Gaps:

1. `AGENTS.md` is effectively invisible to the user.
2. There is no `USER.md` document for stable user preferences and behavior rules.
3. The user cannot inspect the effective instruction block without using debug/prompt tooling.
4. The user cannot distinguish bundled/default instruction text from a customized vault override.
5. There is no explicit source list or hierarchy for instruction documents.
6. Agents and external clients do not have a clean `aw` action group for instruction-document visibility.

## 4. Concepts and terminology

### 4.1 Instruction file

A markdown document that is treated as **trusted configuration** when it comes from the unlocked AW vault, bundled app seed, or an explicitly user-enabled project source.

Canonical file names:

| File | Purpose |
|---|---|
| `AGENTS.md` | Project/workspace/runtime rules for the agent. Commands, conventions, workflows, coding rules, safety rules. |
| `USER.md` | Stable user preferences, tone, profile, language, and personal operating rules. |
| `CLAUDE.md` | Compatibility/project instructions used by Claude-style projects, if discovered. |
| `GEMINI.md` | Compatibility/project instructions used by Gemini-style projects, if discovered. |
| `.github/copilot-instructions.md` | Compatibility/project instructions used by Copilot-style projects, if discovered. |
| Custom markdown instruction files | Future user-added instruction sources. |

V1 should only make vault/bundled `AGENTS.md` and vault `USER.md` editable. Compatibility/project files are a later phase unless the repo already has a trusted workspace model ready to use.

### 4.2 Effective Instructions

The composed instruction block currently injected into the agent runtime, excluding unrelated chat history, raw memory entries, tool outputs, or raw prompt-debug payloads.

Current implementation detail: the runtime has a dedicated skills/instructions slot that currently contains runtime `AGENTS.md` and the Skills index. This feature can keep that slot, but the UI must label sources clearly:

- instruction documents are editable here;
- Skills index is read-only here and links users to Settings → Skills;
- full `SKILL.md` bodies are not shown in Effective Instructions unless they are actually injected by design.

The runtime should receive raw trusted instruction content. UI/action outputs should be safe to inspect and copy: secret-looking values must be scrubbed before display if there is any chance they came from user-edited content.

Implementation requirement: use one shared effective-instructions composer for both runtime injection and UI/actions. The composer should return structured data such as `{content, sources, metadata}`. The only difference between runtime and UI/action output is the output mode: raw trusted content for runtime, scrubbed content for UI/actions.

### 4.3 Agent Instructions vs Skills vs Memory

| Surface | Purpose |
|---|---|
| Agent Instructions | Always-on rules and stable instruction context. |
| Skills | On-demand procedural knowledge loaded only when relevant. |
| Memory | Curated facts about the user and past work. |
| Debug Mode | Developer-oriented raw prompt/turn visibility. |

## 5. Locked decisions

| # | Decision |
|---|---|
| 1 | The page name is **Agent Instructions**. |
| 2 | The page lives under **Settings** as `Settings → Agent Instructions`. |
| 3 | `AGENTS.md` remains the primary runtime instruction document. |
| 4 | Add support for `USER.md` as a separate first-class instruction document. |
| 5 | `USER.md` is for stable user preferences and behavior rules, not episodic memory. |
| 6 | Skills stay in Settings → Skills; do not merge Skills into Agent Instructions. |
| 7 | The page must show both individual instruction files and the effective composed instruction block. |
| 8 | Vault/bundled instruction files are trusted configuration. Discovered project files become trusted only after the user explicitly enables/imports them or when they are part of an opened trusted workspace policy. |
| 9 | External content inside instruction-looking markdown files is not automatically trusted unless explicitly added/enabled by the user. |
| 10 | Editing instruction documents refreshes the agent context live; no app restart required. |
| 11 | No secrets should be displayed or logged by instruction actions. Use the existing secret scrubbing conventions. |
| 12 | The first implementation should not require a full project/workspace hierarchy engine. It can start with vault app documents and add project discovery in a later phase. |
| 13 | Do not expose full raw prompt-debug payloads here. This page is focused on instruction documents and effective instruction composition only. |
| 14 | When `AW_SKILLS_DIR` is active, the UI/actions must report that the effective runtime `AGENTS.md` may come from the dev override and vault edits may not affect the current runtime until the override is disabled. |
| 15 | `USER.md` is vault-backed in v1. Do not load `USER.md` from `AW_SKILLS_DIR` unless a later spec explicitly changes dev-override semantics. |
| 16 | There must be exactly one effective-instructions composer shared by runtime injection and UI/aw-action inspection. It returns structured content plus source metadata. Runtime uses the raw trusted output; UI/actions use the same output after secret scrubbing. Do not create separate runtime and UI composers that can drift. |

## 6. UX design

### 6.1 Settings card

Add a Settings card:

```txt
Agent Instructions
Markdown instruction files that shape how the agent behaves.
Icon: rule | article | description | checklist_rtl
```

Suggested icon: `rule` or `article`.

### 6.2 Page layout

```txt
Settings › Agent Instructions
Markdown instruction files that shape how the agent behaves.

[Effective Instructions]
View the final instruction block currently loaded into the agent.

[Instruction Files]
AGENTS.md      Runtime/project rules for the agent.          Built-in / Customized
USER.md        User preferences and stable behavior rules.   User / Empty
CLAUDE.md      Compatibility instructions, if present.       Discovered / Disabled
GEMINI.md      Compatibility instructions, if present.       Discovered / Disabled
copilot...     Compatibility instructions, if present.       Discovered / Disabled

[Sources]
Bundled seed, vault override, project discovery, compatibility files.
```

### 6.3 Effective Instructions view

A read-only view with:

- composed instruction text;
- source breakdown;
- last refresh timestamp;
- token/character estimate;
- copy button;
- refresh button;
- warning that this is trusted configuration, not web/email content.

Do not include:

- chat messages;
- raw user memory entries unless they are part of the instruction composition by design;
- provider secrets;
- tool results;
- external content payloads.

### 6.4 Instruction file detail/editor

For each editable document:

```txt
Settings › Agent Instructions › AGENTS.md

Origin: built-in + vault override
Status: customized
Seed version: ...
Last updated: ...

[Markdown editor]

[Save] [Cancel] [Reset to built-in]
```

For `USER.md`:

```txt
Settings › Agent Instructions › USER.md

Origin: user
Status: empty | configured

[Markdown editor]

[Save] [Cancel]
```

### 6.5 Unsaved-change guard

Reuse the same UX pattern from Settings → Skills:

- leaving an edited instruction document prompts the user;
- `Save` persists and clears dirty state;
- `Discard` leaves and loses changes;
- `Keep editing` stays on the editor.

### 6.6 Search/filter

The page should support at least simple search/filter for instruction files:

```txt
[Search instruction files...] [All Sources ▼]
```

Source filter options:

- All Sources
- Built-in
- User
- Project
- Compatibility
- Dev Override
- Disabled

## 7. Instruction source model

### 7.1 Source types

```txt
builtin       Embedded app seed, e.g. skills/bundled/AGENTS.md
vault         User-customized document stored in encrypted vault
user          User-created document, e.g. USER.md
project       Discovered from an opened/repo workspace
compatibility CLAUDE.md, GEMINI.md, copilot-instructions.md, etc.
dev_override  Development override from AW_SKILLS_DIR
computed      Effective composed instructions
```

### 7.2 Suggested source precedence

For v1, keep it simple:

```txt
1. App base prompt (not shown as editable here; may be referenced)
2. Effective AGENTS.md
   - AW_SKILLS_DIR AGENTS.md when dev override is active and present
   - otherwise vault/bundled AGENTS.md
3. Vault USER.md
4. Enabled project/compatibility instruction files (future phase)
5. Skills index (visible in Skills, not edited here)
6. Memory/user facts (visible in Memory, not edited here)
```

If `AW_SKILLS_DIR` is active, vault mutations should still persist, but action/UI responses must be honest about effective impact:

- saving/resetting vault `AGENTS.md` does not change the live effective `AGENTS.md` while a dev-override `AGENTS.md` is active;
- saving vault `USER.md` can still affect the live context because `USER.md` remains vault-backed in v1;
- responses should include `devOverrideActive=true` and, when useful, an `effectiveChanged` boolean plus a warning for changes masked by the dev override.

If future project discovery supports nested files, use standard nearest-file-wins behavior for project instruction files, but do not implement that in v1 unless the repo already has the required workspace model.

### 7.3 Document statuses

Each instruction file should show:

```txt
missing | empty | active | disabled | customized | built-in | resettable | read-only | dev-override | error
```

Examples:

- `AGENTS.md`: built-in, active, customized, resettable.
- `AGENTS.md` with `AW_SKILLS_DIR`: dev-override, active, read-only.
- `USER.md`: user, active, empty.
- `CLAUDE.md`: compatibility, discovered, disabled.
- `.github/copilot-instructions.md`: compatibility, discovered, active.

## 8. New AW actions

All actions use the existing single `aw` gateway.

### 8.1 `instructions.list`

Returns visible instruction documents and metadata.

Args:

```json
{}
```

Response shape:

```json
{
  "documents": [
    {
      "id": "AGENTS.md",
      "title": "AGENTS.md",
      "description": "Runtime/project rules for the agent.",
      "origin": "builtin|vault|user|project|compatibility|dev_override|computed",
      "status": ["active", "customized"],
      "editable": true,
      "resettable": true,
      "enabled": true,
      "contentHash": "...",
      "seedHash": "...",
      "seedVersion": "...",
      "updatedAt": "..."
    }
  ]
}
```

### 8.2 `instructions.read`

Reads one instruction document.

Args:

```json
{
  "id": "AGENTS.md"
}
```

Response shape:

```json
{
  "document": {
    "id": "AGENTS.md",
    "content": "...",
    "origin": "vault",
    "editable": true,
    "status": ["active", "customized"],
    "contentHash": "...",
    "seedHash": "...",
    "seedVersion": "...",
    "updatedAt": "..."
  }
}
```

### 8.3 `instructions.save`

Saves an editable instruction document and refreshes runtime context.

Args:

```json
{
  "id": "USER.md",
  "content": "# User preferences\n...",
  "enabled": true
}
```

Response shape:

```json
{
  "success": true,
  "contextRefreshed": true,
  "effectiveChanged": true,
  "restartRequired": false,
  "devOverrideActive": false,
  "document": { "id": "USER.md", "status": ["active"] }
}
```

### 8.4 `instructions.reset`

Resets a built-in instruction document to its seed.

Args:

```json
{
  "id": "AGENTS.md"
}
```

Response shape:

```json
{
  "success": true,
  "contextRefreshed": true,
  "effectiveChanged": true,
  "restartRequired": false,
  "devOverrideActive": false,
  "document": { "id": "AGENTS.md", "status": ["active", "built-in"] }
}
```

### 8.5 `instructions.effective`

Returns the composed effective instruction block and source breakdown.

Args:

```json
{
  "includeContent": true
}
```

Response shape:

```json
{
  "content": "...",
  "sources": [
    { "id": "AGENTS.md", "origin": "vault", "enabled": true, "chars": 1234 },
    { "id": "USER.md", "origin": "user", "enabled": true, "chars": 456 }
  ],
  "generatedAt": "...",
  "redacted": true,
  "charCount": 1690
}
```

If `includeContent=false`, return only metadata/sources.

### 8.6 `instructions.sources`

Returns source inventory and discovery status.

Args:

```json
{}
```

Response shape:

```json
{
  "sources": [
    { "kind": "builtin", "id": "AGENTS.md", "active": true },
    { "kind": "dev_override", "id": "AGENTS.md", "active": false, "reason": "AW_SKILLS_DIR_not_set" },
    { "kind": "user", "id": "USER.md", "active": true },
    { "kind": "compatibility", "id": "CLAUDE.md", "active": false, "reason": "not_found" }
  ]
}
```

## 9. Backend architecture

### 9.1 Domain

Extend `domain.AppDocument` support beyond only `AGENTS.md`.

Suggested constants:

```go
const AgentsDocumentID = "AGENTS.md"
const UserDocumentID = "USER.md"
```

`app_documents.updated_at` already exists in the schema but is not currently exposed through `domain.AppDocument`; add an `UpdatedAt` field if the UI needs to show last-updated metadata.

Add instruction-facing DTO/domain types rather than overloading skill view types:

```go
type InstructionDocument struct { ... }
type InstructionSource struct { ... }
type EffectiveInstructions struct { ... }
```

Validate ids with a v1 allowlist (`AGENTS.md`, `USER.md`) instead of accepting arbitrary paths. Compatibility/custom ids can be added with explicit validation in later phases.

### 9.2 Application

Create or extend use cases:

```txt
internal/application/instructions.go
```

Responsibilities:

- list instruction documents;
- get one document;
- save editable document;
- reset seed-backed document;
- compose effective instructions;
- validate document ids;
- apply secret scrubbing for UI/action output where needed;
- refresh runtime context after mutation.

### 9.3 Infrastructure / vault

Extend vault app-document persistence to support multiple document ids:

- `AGENTS.md` existing;
- `USER.md` new;
- future compatibility/custom docs.

The table already supports multiple ids. The port/use-case layer needs any missing read/list metadata required by the UI/actions, for example:

```go
ListAppDocuments() ([]domain.AppDocument, error)
GetAppDocument(id string) (domain.AppDocument, bool, error)
UpsertAppDocument(doc domain.AppDocument) error
```

Keep seed-update mechanics for built-in documents:

- built-in seed hash;
- content hash;
- customized detection;
- reset to seed.

`USER.md` is user-origin and not seed-backed unless this spec later chooses an explicit template seed. Reset should fail for non-seed-backed documents.

### 9.4 Runtime context refresh

Today `RefreshSkillsContext` / `SetSkillsContext` inject runtime `AGENTS.md` + the Skills index into the agent's dedicated skills/instructions slot.

Update the composition to include:

```txt
AGENTS.md
USER.md
Skills index
```

Maintain clear delimiters in the runtime prompt:

```txt
### Agent Instructions: AGENTS.md
Trusted configuration from: <source>
...

### User Instructions: USER.md
Trusted configuration from: vault
...

### Skills
Read-only skills index. Full skill bodies are loaded on demand through skill.read.
...
```

The injected labels must clearly say these are trusted configuration documents. The user-facing Effective Instructions action/view should use the same composer but scrub display output with the existing secret scrubber.

Implementation naming can either keep `RefreshSkillsContext` for compatibility and add a narrower composer under it, or rename it in a dedicated refactor. Do not leave two divergent composition paths.

Required composer contract:

```go
type EffectiveInstructionsOutputMode string

const (
    EffectiveInstructionsRaw     EffectiveInstructionsOutputMode = "raw"     // runtime injection
    EffectiveInstructionsScrubbed EffectiveInstructionsOutputMode = "scrubbed" // UI/actions
)

type EffectiveInstructions struct {
    Content   string
    Sources   []InstructionSource
    Metadata  EffectiveInstructionsMetadata
    Redacted  bool
}
```

The runtime path and `instructions.effective` must call the same composer and differ only by output mode. Tests should fail if runtime and UI composition drift in source order or section labels.

### 9.5 Tools registry

Add:

```txt
internal/infrastructure/tools/aw_instructions.go
internal/infrastructure/tools/aw_instructions_test.go
```

Register actions when the instruction funcs are wired, mirroring the `skillManageFuncs` pattern so unavailable callbacks return a clear `errUnavailable("instructions")`:

```txt
instructions.list
instructions.read
instructions.save
instructions.reset
instructions.effective
instructions.sources
```

Update:

- `aw.actions` expected list tests;
- `awToolDescription()`;
- `docs/SELFCODE.md`.

### 9.6 Wails UI bindings/services

Add Wails methods or reuse action dispatcher internally:

```txt
ListInstructionDocuments
GetInstructionDocument
SaveInstructionDocument
ResetInstructionDocument
GetEffectiveInstructions
GetInstructionSources
```

Frontend service:

```txt
frontend/src/services/instructions.service.ts
```

The service wrapper should be the only frontend import point for generated Wails bindings, matching the existing `skills.service.ts` pattern. Add/update generated `frontend/wailsjs` bindings as part of the implementation workflow.

Settings page:

```txt
frontend/src/modules/settings/pages/AgentInstructionsPage.tsx
```

Add to `SettingsModule.tsx`:

```txt
SettingsPage union: 'instructions'
Card: Agent Instructions
Description: Markdown instruction files that shape how the agent behaves.
```

## 10. Security and safety

### 10.1 Trust boundary

Instruction documents are powerful. Treat them as trusted configuration only when they come from:

- bundled seed;
- encrypted vault document;
- explicit user edit/import;
- explicitly enabled trusted project source.

Do not auto-trust arbitrary markdown files found in a random folder.

### 10.2 Prompt injection

Instruction-looking files from external/untrusted locations can contain prompt injection. The UI must make enable/import an explicit user action.

If project discovery is added later:

- show discovered files as disabled by default unless the workspace trust model says otherwise;
- show source path with user-safe redaction when needed;
- require explicit enable/import for external folders.

### 10.3 Secret handling

Do not display or log secrets.

- Scrub secret-looking values before returning `instructions.effective` content if necessary.
- Prefer the existing `application.ScrubChatSecrets` behavior for display/log scrubbing unless a stronger shared scrubber exists by implementation time.
- Tool-call logs should include only action name, doc id, content length/hash, status, and duration.
- Do not log document content.

### 10.4 Destructive actions

Resetting `AGENTS.md` to seed discards user changes. It needs UI confirmation.

The `instructions.reset` action should be available through `aw`, but the agent must follow the general destructive-action rule and confirm with the user before calling it unless the user explicitly asked to reset.

### 10.5 Locked vault and dev override behavior

- If the vault is locked, instruction actions should return a clear locked/unavailable error; the UI should show a locked state instead of an empty list.
- If `AW_SKILLS_DIR` is active, `instructions.list` / `instructions.effective` should identify the dev-override `AGENTS.md` source when applicable.
- If `AW_SKILLS_DIR` is active and the user saves/resets vault `AGENTS.md`, persist the change but report that the live runtime may not reflect vault `AGENTS.md` until the override is disabled.
- If `AW_SKILLS_DIR` is active and the user saves vault `USER.md`, refresh the context normally because `USER.md` remains vault-backed in v1.

## 11. Phases

### Phase 1 — Vault documents + actions

1. Generalize app document support for multiple ids and expose needed metadata (`updatedAt` if used by the UI).
2. Add `USER.md` as an editable user document.
3. Implement application use cases.
4. Add `instructions.*` `aw` actions.
5. Refresh runtime context after save/reset.
6. Handle locked vault and `AW_SKILLS_DIR` dev-override reporting.
7. Update `docs/SELFCODE.md` and action description.

**Accept:** an agent can call `instructions.list`, `instructions.read {AGENTS.md}`, `instructions.save {USER.md}`, and `instructions.effective`.

### Phase 2 — Settings → Agent Instructions UI

1. Add Settings card and page.
2. Show Effective Instructions, Instruction Files, and Sources sections.
3. Add editor/detail view for `AGENTS.md` and `USER.md`.
4. Add unsaved-change guard.
5. Add reset-to-seed for `AGENTS.md` with confirmation.
6. Add search/filter.

**Accept:** the user can inspect and edit `AGENTS.md` / `USER.md` without touching debug mode or filesystem manually.

### Phase 3 — Effective source breakdown

1. Show source order and composition.
2. Show active/customized/read-only/resettable statuses.
3. Show char/token estimates.
4. Add copy effective instructions.
5. Add refresh button.

**Accept:** the user can understand exactly what instruction files are shaping the agent.

### Phase 4 — Compatibility/project discovery

1. Detect common instruction files in a trusted/opened project context:
   - `AGENTS.md`;
   - `CLAUDE.md`;
   - `GEMINI.md`;
   - `.github/copilot-instructions.md`;
   - future `USER.md` if deliberately supported.
2. Show discovered files under Sources.
3. Do not enable untrusted discovered files automatically unless the workspace trust policy allows it.
4. Add explicit enable/import flow.
5. Add precedence/hierarchy tests.

**Accept:** project instruction files become visible and can be enabled/imported intentionally.

### Phase 5 — Polish and integration with the agentic-five surface

1. Link Agent Instructions from any future Agent Capabilities / Agent Standards page.
2. Add status to show `AGENTS.md` is configured.
3. Add help text explaining Agent Instructions vs Skills vs Memory.
4. Add tests for no drift between UI, actions, and docs.

**Accept:** AGENTS.md visibility is complete enough to mark term #1 as product-ready.

## 12. Tests

Backend:

- list returns `AGENTS.md` and `USER.md`;
- read returns content and metadata;
- save validates ids and refreshes context;
- reset works only for seed-backed docs;
- effective instructions include expected source order;
- runtime and UI/actions use the same composer with different output modes only;
- effective output scrubs secret-looking content while runtime composition remains raw trusted config;
- `AW_SKILLS_DIR` reports dev-override source and honest `effectiveChanged` / refresh status;
- locked vault returns a clear error/state;
- action registry includes all `instructions.*` actions;
- action logs do not store content;
- architecture tests remain green.

Frontend:

- Settings card appears and search finds it;
- page renders three sections;
- editor save/cancel works;
- dirty editor warns before leaving;
- reset confirmation is required;
- Effective Instructions view renders source breakdown;
- service wrappers are the only Wails imports.

Security:

- secret-looking values are scrubbed in effective output where applicable;
- untrusted discovered files are disabled by default;
- no raw document content appears in app logs.

## 13. Gates

Run after each implementation phase:

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

On Windows/PowerShell:

```powershell
$env:PATH = "C:\Program Files\Go\bin;$env:USERPROFILE\go\bin;$env:PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend; npm run build:frontend
```

## 14. Non-goals

- Do not merge Skills into Agent Instructions.
- Do not replace Settings → Memory.
- Do not expose raw full prompt-debug payloads here.
- Do not implement A2A in this spec.
- Do not implement subagent visibility in this spec.
- Do not auto-enable arbitrary markdown files from untrusted folders.
- Do not create a general markdown editor for every file in the filesystem.

## 15. Open questions

1. Should `USER.md` be seeded with an empty template or created only on first save?
   - Recommendation: seed an empty/template document so the UI always shows it.
2. Should compatibility files be read-only previews or importable into the vault?
   - Recommendation: preview first, import/enable explicitly.
3. Should `instructions.effective` show the scrubbed content by default?
   - Recommendation: yes for the UI, but allow `includeContent=false` for metadata-only calls.
4. Should project `AGENTS.md` hierarchy be implemented before or after A2A/subagent visibility?
   - Recommendation: after the basic vault-level page is complete.
5. Should `AW_SKILLS_DIR` eventually support `USER.md`?
   - Recommendation: no for v1. Keep `USER.md` vault-backed so the dev override continues to mean "skills catalog + runtime AGENTS.md override", not full user profile override.
