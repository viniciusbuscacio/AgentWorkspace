# Memory (Settings › Memory)

What the agent remembers about the user, across all chats. It is a single
living document ("memory document") stored in the encrypted vault and shown —
fully editable — in Settings › Memory.

## What it does

- The whole document is injected into the agent's context on every turn
  (capped at 8,000 characters), so the agent knows the user's name,
  preferences and recurring context in any chat, old or new.
- Lines follow the shape `- [category] key: content` with categories
  `profile`, `preference`, `context` and `reference`.

## How it gets populated

- **On request**: when the user says "remember this", "anote sobre mim",
  "meu nome é...", the agent saves it with the `memory.remember` action
  (upsert: repeating a key replaces the line). `memory.forget {key}` deletes
  the line(s) with that key — no UI needed to clean a stale fact.
- **Proactively**: when the agent notices a clearly durable preference (the
  language the user writes in, preferred answer style, tools, recurring
  context) it saves it without being asked — checking the document first so
  nothing is duplicated, and saving at most 1-2 facts per turn.
- **By hand**: the user can edit the document directly in Settings › Memory
  (Save/Cancel), including deleting anything they don't want kept.

## Special line: preferred-language

The line `preferred-language: <language>` is read by app features beyond the
agent itself: the module "?" help buttons ask their question in this language,
and the voice-dictation cleanup pass uses it as the language hint. Absent, the
app defaults to English.

## Maintenance

When the document grows past ~6,000 characters, the agent condenses it
(rewrites, merges duplicates, drops stale lines) on the next vault unlock, at
most once per day. A backup of the pre-condensation text is kept and can be
restored with "Undo condensation" in Settings › Memory.

## Privacy

The document lives only in the local encrypted vault. It is not synced
anywhere; deleting a line removes it from every future prompt.
