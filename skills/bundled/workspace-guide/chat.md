# Chat module — user guide

One-line: the core conversation surface with the agent; every other module can
be driven from here by just asking.

## Purpose

- Chat is the only Core module: always added, cannot be removed. The workspace
  exists around it — modules add capabilities (actions + context) that the
  agent uses inside this conversation.
- Each chat is a persistent session stored in the encrypted vault (messages,
  titles, compaction summaries). Chats survive restarts and locks.

## What the user sees

- **Sidebar**: "New Chat" on top; recent chats listed below the modules; a
  mini status bar at the bottom (hide sidebar, lock vault, Archived Chats
  toggle, server dot when listening) — the archive icon opens the archived
  drawer upward, with its own search. Right-click a chat for
  rename/archive/delete; the X on hover archives it.
- **Message list**: markdown rendering (headings, code blocks, tables),
  streaming replies, live subagent cards when parallel workers run, and
  compact system markers (e.g. compaction notices).
- **Composer**: multiline box (drag the top edge to resize), attachment
  button (images/PDFs — they are read through the safe attachment pipeline
  and treated as untrusted content), and a picture-in-picture button that
  opens the same chat in a small always-on-top window.
- Sending while a reply is streaming queues the message; queued items can be
  edited before they are sent.

## Behavior worth knowing

- **Titles**: new chats are auto-named after the first exchange; rename via
  right-click. Titles/summaries are treated as descriptive data, never as
  instructions to the agent.
- **Compaction**: long chats are summarized automatically so the context stays
  within model limits; a marker shows where compaction happened.
- **Locking**: the vault locking (manual or auto-lock) stops any in-flight
  turn; the chat itself is preserved and reopens after unlock.

## How the agent uses it

- Core chat control actions (always available): `chat.list`, `chat.create`,
  `chat.send`, `chat.messages`, plus chat-memory recall
  (`memory.chat.search/open/recent`) over past conversations.
- Deleting a chat from the UI moves it to Archived first (trash-bin
  semantics); permanent delete happens from the archived list, behind a
  confirmation modal (it is irreversible).
