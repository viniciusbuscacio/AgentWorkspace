import type { ComponentType } from 'react';

// The sidebar contract every workspace module must satisfy — enforced by the
// type system, not by code review. MODULE_VIEWS only accepts the branded
// result of defineModuleView(), and the brand cannot be forged outside this
// file, so a module that skips a rule (no close button, no right-click menu)
// does not compile.

declare const moduleViewBrand: unique symbol;

/** An extra action a module contributes to its sidebar right-click menu. */
export interface ModuleMenuExtra {
  id: string;
  label: string;
  /** Material symbol name. */
  icon: string;
  onSelect: () => void | Promise<void>;
}

/**
 * The sidebar rules. Every field is required and narrowly typed: omitting one
 * — or trying to opt out — is a compile error at the defineModuleView call.
 */
export interface ModuleSidebarRules {
  /**
   * The hover × button (AW2 parity). The literal `true` type means a module
   * cannot declare itself non-closable: AppShell always renders the button
   * and wires it to CLOSE — the module is hidden from the sidebar but stays
   * added, with its agent actions and prompt block untouched. Removing from
   * the workspace (the capability fence) lives in the context menu only.
   */
  closeButton: true;
  /**
   * The right-click context menu. 'standard' renders the built-in actions
   * (Close, Move up, Move down, Remove from workspace); extras are appended
   * above the destructive item. There is no shape that yields a module
   * without a menu.
   */
  contextMenu: 'standard' | { extras: readonly ModuleMenuExtra[] };
}

/**
 * LeaveGuard intercepts navigation away from a module that has unsaved work.
 * The module registers it upward; the shell calls it with the action to run if
 * the user confirms (or that runs immediately when there is nothing to lose).
 */
export type LeaveGuard = (proceed: () => void) => void;

/** What a module registers: its main view plus the sidebar rules. */
export interface ModuleRuntimeProps {
  createSpecChat?: (title: string, body: string) => Promise<void>;
  // Settings-compatible runtime props (only the Settings module reads these).
  onLocked?: () => Promise<void> | void;
  settingsInitialPage?: string;
  // registerLeaveGuard lets a module hand the shell an unsaved-changes guard
  // (null clears it) so every navigation/close path can confirm before
  // discarding the module's in-progress edits.
  registerLeaveGuard?: (guard: LeaveGuard | null) => void;
}

export interface ModuleViewSpec {
  /** Rendered by AppShell when the module is opened. */
  view: ComponentType<ModuleRuntimeProps>;
  /**
   * One sidebar item, one instance — closing hides it (AW2's
   * shouldHideModuleOnClose). The literal `true` keeps multi-instance
   * windows out of scope until a module actually needs them; relaxing this
   * type is the deliberate signal that the sidebar gained instance support.
   */
  singleton: true;
  sidebar: ModuleSidebarRules;
}

/**
 * A module accepted by MODULE_VIEWS. The unforgeable brand guarantees it came
 * through defineModuleView — a bare React component (or a hand-written object
 * literal) is rejected by the compiler.
 */
export type RegisteredModuleView = ModuleViewSpec & {
  readonly [moduleViewBrand]: true;
};

/** The only way to produce a RegisteredModuleView. */
export function defineModuleView(spec: ModuleViewSpec): RegisteredModuleView {
  return spec as RegisteredModuleView;
}

/** The extras of a module's context menu ([] for 'standard'). */
export function moduleMenuExtras(def: RegisteredModuleView): readonly ModuleMenuExtra[] {
  return def.sidebar.contextMenu === 'standard' ? [] : def.sidebar.contextMenu.extras;
}
