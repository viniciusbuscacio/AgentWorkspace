# Obsidian

Connects Agent Workspace to the user's Obsidian vault — a folder of Markdown
notes on disk. The agent can search and read notes, and (only when the user
allows it) create or update them.

## Setup (in the module page)

- **Vault folder** — the user picks their Obsidian vault folder. This folder
  is a jail: every agent action is confined to it; paths outside it (and the
  `.obsidian` configuration folder) are rejected.
- **Create/Update Notes/Folders** — OFF by default. While off,
  `obsidian.write` and `obsidian.append` are refused; reading always works.
- **Delete Notes/Folders** — OFF by default, separate from create/update.
  While off, `obsidian.delete` is refused.
- **Always-read notes** — notes injected into the agent context on every
  message, in full (the page warns when they get heavy). Good for a
  dashboard note or standing instructions.

## Agent actions

- `obsidian.list {folder?}` — entries under a folder (vault root by default).
- `obsidian.search {query, max?}` — match note names and contents; returns
  paths plus a snippet around content matches.
- `obsidian.read {path}` — full note text (capped at ~24,000 characters).
- `obsidian.write {path, content}` — create or overwrite a note (Write access
  required; missing folders are created).
- `obsidian.append {path, content}` — add to the end of a note, creating it
  when absent (Create/Update required).
- `obsidian.delete {path}` — remove a note or folder (Delete required; the
  vault root and `.obsidian` are always refused).

## Notes for the agent

- Paths are vault-relative, slash-separated (`Projects/aw.md`).
- Prefer `obsidian.search` before reading; prefer `obsidian.append` for logs
  and running notes; use `obsidian.write` only when replacing content is the
  explicit intent.
- Obsidian itself picks up file changes automatically; no sync step needed.
