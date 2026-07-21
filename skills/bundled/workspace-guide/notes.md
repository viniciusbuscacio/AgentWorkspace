# Notes module — user guide

One-line: durable free-text notes the user keeps and the agent can read and
write.

## Purpose

- Notes hold text the user wants to KEEP (plans, drafts, reference) — unlike
  chat messages, they are meant to be stable documents.
- Stored in the encrypted vault; readable only while unlocked.

## What the user sees

Two panes:

- **Left — list**: refresh and "New note" icon buttons, a "Show archived"
  checkbox, and one card per note (pinned notes sort first, then newest).
  Hover icons on each card: pin/unpin, archive/unarchive, delete.
- **Right — editor**: title + content, action row with Pin/Unpin,
  Archive/Unarchive, an "Insert into Agent prompt" checkbox and Save/Cancel.

Controls worth explaining:

- **Pin**: sorts the note first in the list. **Archive**: hides it from the
  default list without deleting (visible with "Show archived").
- **Insert into Agent prompt**: when checked (the default for new notes), the
  note's content is injected into the agent's context on every turn — good
  for standing instructions or reference the agent should always see. Uncheck
  for long notes that would waste context.
- **Delete**: permanent, with confirmation.

## How the agent accesses it

Actions exist only while the module is added:

- `notes.list { includeArchived? }` — pinned first, then newest.
- `notes.get { id }` — full content of one note.
- `notes.create { title, content?, inPrompt? }` — inPrompt defaults to true.
- `notes.update { id, title?, content?, pinned?, archived?, inPrompt? }` —
  partial update; omitted fields keep their values.
- `notes.delete { id }` — the result echoes what was deleted.

The agent prefers updating an existing note over creating near-duplicates.
An open Notes view refreshes automatically when the agent changes notes.
