import { EventsOn } from '@wails/runtime/runtime';
import type { PromptDebugSnapshot } from '@modules/chat/prompt-debug-messages';

// Every chat event carries chatId AND the per-turn runId (AW2 parity): the
// frontend routes strictly by chatId and uses runId to drop late chunks from
// stopped or stale runs, so messages never land in the wrong chat.
export type ChatStartPayload = { chatId: string; runId?: string };
export type ChatDeltaPayload = { chatId: string; runId?: string; seq: number; delta: string };
export type ChatDonePayload = { chatId: string; runId?: string; messageId?: string };
export type ChatErrorPayload = { chatId: string; runId?: string; error: string };
export type ChatRenamedPayload = { chatId: string; title: string };
export type ChatCompactedPayload = { chatId: string; messageCount: number };
export type ChatRefreshPayload = { chatId?: string };
export type SubagentResult = { id: string; status: 'success' | 'error' | 'timeout'; output: string; elapsedMs: number };
export type SubagentPayload = {
  chatId: string;
  runId?: string;
  phase: 'start' | 'task-done' | 'end';
  tasks?: Array<{ id: string; task: string }>;
  taskId?: string;
  result?: SubagentResult;
  results?: SubagentResult[];
};
export type PromptDebugPayload = { snapshot: PromptDebugSnapshot };
export type InlineImagePayload = { id?: string; chatId: string; dataUri: string; width?: number; height?: number; caption?: string };
export type ToolConfirmPayload = {
  id: string;
  tool: string;
  summary: string;
  args?: unknown;
  moduleId?: string;
  moduleName?: string;
  modulePolicy?: string;
};
export type VaultAutoLockedPayload = { reason?: string };
export type VaultDevUnlockedPayload = { profile?: string };
// ui:* events are emitted by the backend aw control actions (app.theme.set,
// app.navigate, ...) so the agent can drive the UI; the frontend applies them
// and reports the resulting state back via ReportUIState.
export type UiSetThemePayload = { theme: string };
export type UiSetWallpaperPayload = { id: string };
export type UiSetWallpaperGlassPayload = { opacity: number };
export type UiSetFontPayload = { family?: string; size?: number };
export type UiSetZoomPayload = { percent: number };
// view is a module id or a fixed surface (home/settings) — validated against
// the registry-derived list on the backend before the event is emitted.
export type UiNavigatePayload = { view: string; chatId?: string };
// modules:changed broadcasts the full catalog after an add/remove (from the
// Settings UI, the Home grid or the agent's module.* actions).
export type ModulesChangedPayload = { modules: Array<{ id: string; added: boolean }> };
// provider:device-code carries the GitHub device-flow user code so the
// Providers page can show it while the browser sign-in is pending.
export type ProviderDeviceCodePayload = { userCode: string; verificationUri: string };
// ui:command is a request/response round-trip for raw UI automation (the aw
// ui.* actions). The frontend executes the command against the DOM and replies
// via ResolveUICommand(id, resultJSON, errMsg).
export type UiCommandPayload = {
  id: string;
  command: 'snapshot' | 'click' | 'fill' | 'screenshot' | 'settle';
  params: { selector?: string; value?: string; filter?: string; max?: number };
};

// app:notify is the single notification channel: backend callers (agent
// app.notify, provider fallback, sign-in prompts) emit it and the shell shows
// the standard in-app toast. There is no OS notification path anymore.
export type AppNotifyPayload = { title?: string; body?: string };

// servers:state mirrors the three network servers' live listening state
// (REST, MCP, Web) — pushed by the backend after every start/stop, never
// polled. Drives the sidebar exposure dot and the Settings › Servers hub.
export type ServerIndicatorEntry = { running: boolean; port: number };
export type ServersStatePayload = { rest: ServerIndicatorEntry; mcp: ServerIndicatorEntry; web: ServerIndicatorEntry };

export type AwEventPayloads = {
  'chat:start': ChatStartPayload;
  'chat:delta': ChatDeltaPayload;
  'chat:done': ChatDonePayload;
  'chat:error': ChatErrorPayload;
  'chat:renamed': ChatRenamedPayload;
  'chat:compacted': ChatCompactedPayload;
  'chat:refresh': ChatRefreshPayload;
  'chat:subagent': SubagentPayload;
  'prompt:debug': PromptDebugPayload;
  'chat:inline-image': InlineImagePayload;
  'tool:confirm': ToolConfirmPayload;
  'vault:auto-locked': VaultAutoLockedPayload;
  'vault:dev-unlocked': VaultDevUnlockedPayload;
  'ui:set-theme': UiSetThemePayload;
  'ui:set-wallpaper': UiSetWallpaperPayload;
  'ui:set-wallpaper-glass': UiSetWallpaperGlassPayload;
  'ui:set-font': UiSetFontPayload;
  'ui:set-zoom': UiSetZoomPayload;
  'ui:navigate': UiNavigatePayload;
  'ui:command': UiCommandPayload;
  'modules:changed': ModulesChangedPayload;
  'provider:device-code': ProviderDeviceCodePayload;
  // Agent-side vault mutations (notes.*/tasks.* aw actions): an open module
  // view refreshes its list on these. Payload is empty by design.
  'notes:changed': Record<string, never>;
  'tasks:changed': Record<string, never>;
  'memory:changed': Record<string, never>;
  'app:notify': AppNotifyPayload;
  'servers:state': ServersStatePayload;
};

export type AwEventName = keyof AwEventPayloads;

export function onAwEvent<TName extends AwEventName>(
  eventName: TName,
  callback: (payload: AwEventPayloads[TName]) => void,
): () => void {
  return EventsOn(eventName, (payload: AwEventPayloads[TName]) => callback(payload));
}
