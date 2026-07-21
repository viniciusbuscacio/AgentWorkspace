# Skills module — user guide

One-line: browse, create and manage the procedural skills the agent can load
mid-conversation.

## Purpose

- A skill is a folder with a `SKILL.md` (frontmatter: name + description,
  then instructions) and optional support files. The agent sees the catalog
  of names/descriptions in its prompt and loads a skill's full content only
  when relevant — knowledge on demand instead of a permanently long prompt.
- Two origins: **built-in** skills bundled with the app, and **user** skills
  created or imported here. Everything lives under the app's skills dir.

## What the user sees

- **List**: one row per skill with origin badge, an enable/disable switch,
  and actions: Edit, Reset (built-ins that were customized), Delete (red).
- **Editor**: full-file editing of SKILL.md and the skill's files; leaving
  with unsaved changes asks first. Editing a built-in marks it *customized* —
  Reset restores the bundled version.
- **New skill / Import**: create from scratch, import a single file or a
  folder of skills (invalid ones are skipped with a reason).
- If an app update ships a newer built-in that the user customized, the row
  shows an update-available hint; the agent can also detect and self-heal a
  stale customized built-in when asked.

## How the agent accesses it

The `skill.*` actions are core (always available, independent of this module
being added — the module is just the management UI):

- `skill.list` — the catalog (id, name, description, enabled).
- `skill.read { id, path? }` — load a skill's instructions or a support file.
- `skill.save { id, files }` and related management actions exist for
  self-editing workflows; edits to built-ins mark them customized.

Disabled skills stay installed but leave the agent's catalog.
