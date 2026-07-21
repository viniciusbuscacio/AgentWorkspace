import { describe, expect, it } from 'vitest';
import type { ComponentType } from 'react';
import {
  defineModuleView,
  moduleMenuExtras,
  type ModuleViewSpec,
  type RegisteredModuleView,
} from './module-contract';
import { MODULE_VIEWS } from './module-views';

const SomeView: ComponentType = () => null;

// ─── Compile-time gate ────────────────────────────────────────────
// These blocks are the actual enforcement test: each @ts-expect-error marks a
// way of registering a non-compliant module that MUST fail to compile. If the
// contract is ever weakened, the now-unused @ts-expect-error itself becomes a
// build error in `npm run build:frontend` (tsc covers test files).

// A bare React component is not a registered module view.
// @ts-expect-error -- raw components are rejected; use defineModuleView
const bareComponent: RegisteredModuleView = SomeView;

// A hand-written object literal cannot forge the registration brand.
// @ts-expect-error -- the brand is unforgeable outside module-contract.ts
const forged: RegisteredModuleView = {
  view: SomeView,
  singleton: true,
  sidebar: { closeButton: true, contextMenu: 'standard' },
};

// The sidebar rules cannot be omitted.
// @ts-expect-error -- sidebar rules are required
const noSidebar = defineModuleView({ view: SomeView, singleton: true });

// The close button cannot be skipped...
// @ts-expect-error -- closeButton is required
const noCloseButton = defineModuleView({ view: SomeView, singleton: true, sidebar: { contextMenu: 'standard' } });

// ...nor opted out of.
// @ts-expect-error -- a module cannot declare itself non-closable
const optedOut = defineModuleView({ view: SomeView, singleton: true, sidebar: { closeButton: false, contextMenu: 'standard' } });

// The right-click menu cannot be skipped.
// @ts-expect-error -- contextMenu is required
const noMenu = defineModuleView({ view: SomeView, singleton: true, sidebar: { closeButton: true } });

// The singleton declaration cannot be skipped...
// @ts-expect-error -- singleton is required
const noSingleton = defineModuleView({ view: SomeView, sidebar: { closeButton: true, contextMenu: 'standard' } });

// ...and multi-instance does not exist until the sidebar supports it.
// @ts-expect-error -- singleton: false is not a registrable shape yet
const multiInstance = defineModuleView({ view: SomeView, singleton: false, sidebar: { closeButton: true, contextMenu: 'standard' } });

// Keep the rejected values "used" so lint stays quiet; they are type-level.
void [bareComponent, forged, noSidebar, noCloseButton, optedOut, noMenu, noSingleton, multiInstance];

// ─── Runtime behavior ─────────────────────────────────────────────

describe('module contract', () => {
  it('defineModuleView returns the spec for AppShell to consume', () => {
    const spec: ModuleViewSpec = {
      view: SomeView,
      singleton: true,
      sidebar: { closeButton: true, contextMenu: 'standard' },
    };
    const registered = defineModuleView(spec);
    expect(registered.view).toBe(SomeView);
    expect(registered.sidebar.closeButton).toBe(true);
    expect(moduleMenuExtras(registered)).toEqual([]);
  });

  it('exposes context-menu extras when declared', () => {
    const onSelect = () => undefined;
    const registered = defineModuleView({
      view: SomeView,
      singleton: true,
      sidebar: {
        closeButton: true,
        contextMenu: { extras: [{ id: 'x', label: 'Do X', icon: 'bolt', onSelect }] },
      },
    });
    expect(moduleMenuExtras(registered).map((extra) => extra.id)).toEqual(['x']);
  });

  it('every registered module declares the full sidebar contract', () => {
    for (const [id, def] of Object.entries(MODULE_VIEWS)) {
      expect(def.view, `module ${id} must have a view`).toBeTypeOf('function');
      expect(def.singleton, `module ${id} must declare singleton`).toBe(true);
      expect(def.sidebar.closeButton, `module ${id} must keep the close button`).toBe(true);
      expect(def.sidebar.contextMenu, `module ${id} must declare a context menu`).toBeTruthy();
    }
  });
});
