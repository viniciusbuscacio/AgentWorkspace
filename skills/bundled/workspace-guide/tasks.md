# Tasks module — user guide

One-line: a simple shared task list that the user and the agent both
maintain.

## Purpose

- Tracks actionable items in order. The agent adds things the user wants done
  later, moves statuses forward as work progresses, and consults it when
  asked "what is pending?".
- Stored in the encrypted vault (legacy table names `backlog_items` +
  `backlog_attachments` — a storage detail only).

## What the user sees

- **Top**: an "Add a task..." input with an Add button (Enter also adds).
- **List**: one card per item, in order. Completed items show struck-through.
  Each card has a quick complete/reopen toggle and a delete button on hover.
- **Detail** (click a card): title, a long **body** (description/spec — when
  the user asks the agent to "write a spec" for an item, it goes here),
  a **status** selector and image **attachments** with preview.

Statuses: `open` → `in-progress` → `needs-validation` → `completed`.
(Old vaults with `todo`/`done` are migrated automatically on unlock.)

Attachments: images only, up to 5 per item / 5 MB each; stored as encrypted
blobs in the vault.

## How the agent accesses it

Actions exist only while the module is added:

- `tasks.list {}` — everything in order, with attachment metadata.
- `tasks.get { id }` — one item in full (attachment names/sizes only —
  never the binary content).
- `tasks.add { title, body?, status? }` — appends; status defaults to open.
- `tasks.update { id, title?, body?, status?, position? }`.
- `tasks.delete { id }` — removes the item and its attachments.

An open Tasks view refreshes automatically when the agent changes items.
