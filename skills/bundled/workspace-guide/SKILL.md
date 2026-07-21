---
name: workspace-guide
description: Explain Agent Workspace itself to the user — what a module or screen is, what a button does, what happens when they save/delete something, where data is stored. Use whenever the user asks "what is this module/screen/button", "how does X work", "o que é", "como funciona", "pra que serve" about Notes, Tasks, Passwords, Wallpaper, Logs, the Google Chrome / Microsoft Edge agent browsers, MCP Client or Settings. Load the module's doc BEFORE answering; never guess about the app's own UI.
---

# Workspace guide — the app explained, module by module

This skill holds the user-facing documentation of Agent Workspace's own
modules: what the user sees on screen, what each control does, what happens
under the hood, and how you (the agent) can act on that module.

## How to use it

1. Identify which module the user is asking about.
2. Load that module's doc: `skill.read { "id": "workspace-guide", "path": "<module>.md" }`.
3. Answer FROM the doc. If the doc does not cover the detail, say so honestly
   and offer to inspect (`ui.snapshot`, `system.selfcode`) — never invent
   behavior for buttons or screens you have not verified.

## Module docs available

| Module | File |
|---|---|
| Chat | `chat.md` |
| Notes | `notes.md` |
| Tasks | `tasks.md` |
| Passwords | `passwords.md` |
| Google Chrome / Microsoft Edge (agent browsers) | `agent-browser.md` |
| MCP Client | `mcp-client.md` |
| Skills | `skills.md` |
| MCP Server | `mcp-server.md` |
| REST API Server | `rest-server.md` |
| Web Access | `web-server.md` |
| Settings (all pages: Providers, Security, Permissions, Wallpaper, Logs…) | `settings.md` |
| Memory (Settings › Memory, user memory document) | `memory.md` |
| Obsidian | `obsidian.md` |

Every module has a guide. For details a guide does not cover, fall back to
`system.selfcode` (engineering map) and live inspection, and be explicit with
the user that you are inspecting rather than citing documentation.
