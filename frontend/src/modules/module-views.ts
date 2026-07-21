import { createElement } from 'react';
import { defineModuleView, type ModuleRuntimeProps, type RegisteredModuleView } from './module-contract';
import { TasksModule } from './tasks/TasksModule';
import { BrowserModule } from './browser/BrowserModule';
import { NotesModule } from './notes/NotesModule';
import { PasswordsModule } from './passwords/PasswordsModule';
import { ObsidianModule } from './obsidian/ObsidianModule';
import { McpClientModule } from './mcp-client/McpClientModule';
import { SkillsModule } from './skills/SkillsModule';
import { McpServerModule, RestServerModule, WebServerModule } from './servers/ServerModules';
import { SettingsModule, type SettingsPage } from './settings/SettingsModule';

const BrowserChrome = () => createElement(BrowserModule, { browserId: 'browser-chrome', title: 'Google Chrome' });
const BrowserEdge = () => createElement(BrowserModule, { browserId: 'browser-edge', title: 'Microsoft Edge' });

// SettingsModuleView adapts the generic module runtime props to SettingsModule.
// Settings is a fixed built-in module (opened/closed from the sidebar) rather
// than a special AppShell surface.
const SettingsModuleView = (props: ModuleRuntimeProps) =>
  createElement(SettingsModule, {
    onLocked: props.onLocked ?? (() => {}),
    initialPage: props.settingsInitialPage as SettingsPage | undefined,
    onRegisterLeaveGuard: props.registerLeaveGuard,
  });

// MODULE_VIEWS maps a workspace-module id to its registered view. Adding a
// module to aw touches the Go registry (domain.ModuleCatalog) plus one folder
// under modules/<id>/ registered here through defineModuleView — which is the
// compile-time gate: a module that does not declare the sidebar rules (close
// button, right-click menu) does not build. chat/home are fixed surfaces
// rendered directly by AppShell; settings is a fixed built-in module here.
export const MODULE_VIEWS: Readonly<Record<string, RegisteredModuleView>> = {
  notes: defineModuleView({
    view: NotesModule,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
  obsidian: defineModuleView({
    view: ObsidianModule,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
  tasks: defineModuleView({
    view: TasksModule,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
  passwords: defineModuleView({
    view: PasswordsModule,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
  'mcp-client': defineModuleView({
    view: McpClientModule,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
  'browser-chrome': defineModuleView({
    view: BrowserChrome,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
  'browser-edge': defineModuleView({
    view: BrowserEdge,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
  skills: defineModuleView({
    view: SkillsModule,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
  'mcp-server': defineModuleView({
    view: McpServerModule,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
  'rest-server': defineModuleView({
    view: RestServerModule,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
  'web-server': defineModuleView({
    view: WebServerModule,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
  settings: defineModuleView({
    view: SettingsModuleView,
    singleton: true,
    sidebar: { closeButton: true, contextMenu: 'standard' },
  }),
};

export function hasModuleView(id: string): boolean {
  return Object.prototype.hasOwnProperty.call(MODULE_VIEWS, id);
}

export function moduleView(id: string): RegisteredModuleView | undefined {
  return hasModuleView(id) ? MODULE_VIEWS[id] : undefined;
}
