// open-view is the module-view twin of open-settings: an in-app deep link to
// any registered module view (e.g. the Settings › Servers hub cards opening
// the full REST/MCP/Web pages, which live as Apps modules). AppShell listens
// for this window event and switches the view through the same leave-guarded
// path backend ui:navigate uses.
export const OPEN_VIEW_EVENT = 'aw:open-view';

export interface OpenViewDetail {
  view: string;
}

export function openModuleView(view: string): void {
  window.dispatchEvent(new CustomEvent<OpenViewDetail>(OPEN_VIEW_EVENT, { detail: { view } }));
}
