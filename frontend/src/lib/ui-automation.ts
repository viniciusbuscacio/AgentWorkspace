// Raw UI-automation primitives backing the aw ui.* actions. The backend emits
// a ui:command round-trip; this module executes it against the live DOM and
// returns a JSON-serializable result (or throws, which the bridge reports as
// an error). Kept framework-free so it can be unit-tested under happy-dom.
//
// ui.snapshot returns a pruned accessibility tree (role + accessible name +
// state) in a compact YAML-like form, with an ephemeral ref (e1, e2, ...) on
// every node. ui.click / ui.fill take that ref (preferred) or a CSS selector.

export interface UiCommandParams {
  selector?: string;
  ref?: string;
  value?: string;
  max?: number;
}

interface AxNode {
  role: string;
  name?: string;
  ref: string;
  states: string[];
  value?: string;
  children: AxNode[];
}

// refMap maps the refs emitted by the latest snapshot to their elements, so a
// follow-up click/fill resolves the exact element instead of a fragile
// selector. Reset on every snapshot.
let refMap = new Map<string, Element>();
const SNAPSHOT_DEFAULT_MAX = 500;

const IMPLICIT_ROLE: Record<string, string> = {
  A: 'link', BUTTON: 'button', NAV: 'navigation', MAIN: 'main',
  HEADER: 'banner', FOOTER: 'contentinfo', ASIDE: 'complementary',
  SECTION: 'region', ARTICLE: 'article', UL: 'list', OL: 'list',
  LI: 'listitem', IMG: 'img', TEXTAREA: 'textbox', SELECT: 'combobox',
  DIALOG: 'dialog', H1: 'heading', H2: 'heading', H3: 'heading',
  H4: 'heading', H5: 'heading', H6: 'heading',
};
const INPUT_ROLE: Record<string, string> = {
  checkbox: 'checkbox', radio: 'radio', range: 'slider', number: 'spinbutton',
  button: 'button', submit: 'button', reset: 'button', search: 'searchbox',
};
const DATA_SLOT_ROLE: Record<string, string> = {
  card: 'group', 'card-title': 'heading', switch: 'switch',
};
const ACTIONABLE = new Set([
  'button', 'link', 'textbox', 'searchbox', 'checkbox', 'radio', 'switch',
  'slider', 'spinbutton', 'combobox', 'menuitem', 'menuitemcheckbox', 'tab', 'option',
]);
const KEPT_CONTAINER = new Set([
  'navigation', 'main', 'banner', 'contentinfo', 'complementary', 'region',
  'group', 'list', 'listitem', 'dialog', 'heading', 'article', 'tablist', 'menu',
]);
const CHECKABLE = new Set(['checkbox', 'radio', 'switch', 'menuitemcheckbox']);
const TEXT_INPUT = new Set(['textbox', 'searchbox', 'spinbutton', 'combobox']);

function isVisible(el: Element): boolean {
  const html = el as HTMLElement;
  if (html.hidden) return false;
  if (html.inert || html.closest('[inert]')) return false;
  const style = el.ownerDocument.defaultView?.getComputedStyle(html);
  if (style && (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0')) return false;
  if (typeof html.getClientRects === 'function' && html.getClientRects().length === 0) {
    const tag = el.tagName;
    if (!['HTML', 'BODY', 'HEAD', 'TITLE', 'SCRIPT', 'STYLE', 'META', 'LINK'].includes(tag) && hasExplicitlyNoBox(html, style)) return false;
  }
  return true;
}

function hasExplicitlyNoBox(el: HTMLElement, style: CSSStyleDeclaration | undefined): boolean {
  const inline = el.getAttribute('style') || '';
  if (style && (style.width === '0px' || style.height === '0px')) return true;
  if (/\b(width|height)\s*:\s*0(?:px)?\b/i.test(inline)) return true;
  if (style?.position === 'absolute' || style?.position === 'fixed') {
    const rect = el.getBoundingClientRect();
    if (rect.right <= 0 || rect.bottom <= 0) return true;
  }
  return false;
}

function clean(text: string | null | undefined): string {
  return (text || '').replace(/\s+/g, ' ').trim().slice(0, 120);
}

// accessibleText is the visible text of an element with decorative subtrees
// removed — aria-hidden and role=presentation/none — matching how the
// accessible name is computed (so icon glyphs like "content_copy" don't leak
// into a button's name). Plain textContent would include them.
function accessibleText(el: Element): string {
  let out = '';
  for (const node of Array.from(el.childNodes)) {
    if (node.nodeType === 3) {
      out += node.textContent || '';
    } else if (node.nodeType === 1) {
      const child = node as Element;
      const role = child.getAttribute('role');
      if (child.getAttribute('aria-hidden') === 'true' || role === 'presentation' || role === 'none') continue;
      out += accessibleText(child);
    }
  }
  return out;
}

function computeRole(el: Element): string {
  const explicit = el.getAttribute('role');
  if (explicit) return explicit;
  const slot = el.getAttribute('data-slot');
  if (slot && DATA_SLOT_ROLE[slot]) return DATA_SLOT_ROLE[slot];
  const tag = el.tagName;
  if (tag === 'INPUT') {
    const type = (el.getAttribute('type') || 'text').toLowerCase();
    return INPUT_ROLE[type] || 'textbox';
  }
  if (tag === 'A') return el.hasAttribute('href') ? 'link' : '';
  return IMPLICIT_ROLE[tag] || '';
}

function referencedText(el: Element, attr: string): string {
  const ids = (el.getAttribute(attr) || '').split(/\s+/).filter(Boolean);
  return ids.map((id) => clean(el.ownerDocument.getElementById(id)?.textContent)).filter(Boolean).join(' ');
}

// fieldLabel finds the app's Field label for a control (Field wraps a
// FieldLabel + the control without a `for`/`id` association).
function fieldLabel(el: Element): string {
  const field = el.closest('[data-slot="field"], [data-slot="field-set"]');
  if (!field) return '';
  const label = field.querySelector('[data-slot="field-label"], [data-slot="field-legend"]');
  return clean(label?.textContent);
}

function computeName(el: Element, role: string): string {
  const aria = el.getAttribute('aria-label');
  if (aria) return clean(aria);
  const labelledby = referencedText(el, 'aria-labelledby');
  if (labelledby) return labelledby;

  if (role === 'group' && el.getAttribute('data-slot') === 'card') {
    const title = el.querySelector('[data-slot="card-title"]');
    return title ? clean(accessibleText(title)) : '';
  }
  if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement) {
    const label = el.labels?.[0]?.textContent;
    if (label) return clean(label);
    const field = fieldLabel(el);
    if (field) return field;
    if (el instanceof HTMLInputElement && el.placeholder) return clean(el.placeholder);
  }
  if (['button', 'link', 'heading', 'listitem', 'option', 'switch', 'tab', 'menuitem'].includes(role)) {
    const text = clean(accessibleText(el));
    if (text) return text;
  }
  const title = el.getAttribute('title');
  if (title) return clean(title);
  if (el.tagName === 'IMG') return clean(el.getAttribute('alt'));
  const awid = el.getAttribute('data-awid');
  return awid ? clean(awid) : '';
}

function computeStates(el: Element, role: string): string[] {
  const states: string[] = [];
  const ariaChecked = el.getAttribute('aria-checked');
  const checked = ariaChecked ? ariaChecked === 'true'
    : el instanceof HTMLInputElement ? el.checked : false;
  if (CHECKABLE.has(role)) states.push(checked ? 'checked' : 'unchecked');
  const disabled = (el as HTMLButtonElement).disabled || el.getAttribute('aria-disabled') === 'true';
  if (disabled) states.push('disabled');
  const expanded = el.getAttribute('aria-expanded');
  if (expanded) states.push(expanded === 'true' ? 'expanded' : 'collapsed');
  if (el.getAttribute('aria-selected') === 'true') states.push('selected');
  if (role === 'heading') {
    const level = el.getAttribute('aria-level') || el.tagName.match(/^H([1-6])$/)?.[1];
    if (level) states.push(`level=${level}`);
  }
  return states;
}

function computeValue(el: Element, role: string): string | undefined {
  if (!TEXT_INPUT.has(role)) return undefined;
  if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement) {
    if (!el.value) return undefined;
    // Never echo secrets: password fields and explicit sensitive controls are masked.
    if (el instanceof HTMLInputElement && el.type === 'password') return '••••••';
    if (el.hasAttribute('data-sensitive')) return '••••••';
    return el.value.slice(0, 80);
  }
  return undefined;
}

function shouldKeep(el: Element, role: string): boolean {
  if (ACTIONABLE.has(role) || KEPT_CONTAINER.has(role)) return true;
  return el.hasAttribute('data-awid') || el.hasAttribute('aria-label');
}

function buildTree(max: number): AxNode[] {
  refMap = new Map();
  let count = 0;
  const nextRef = () => `e${++count}`;

  const visitChildren = (el: Element): AxNode[] => {
    const children: AxNode[] = [];
    for (const child of Array.from(el.children)) {
      if (count >= max) break;
      children.push(...visit(child));
    }
    return children;
  };

  // Pre-order: a kept node takes its ref before its children, so refs read
  // top-to-bottom the way the tree prints.
  const visit = (el: Element): AxNode[] => {
    if (count >= max || !isVisible(el)) return [];
    const role = computeRole(el);
    if (!shouldKeep(el, role)) return visitChildren(el);
    const ref = nextRef();
    refMap.set(ref, el);
    const node: AxNode = {
      role: role || 'generic',
      name: computeName(el, role) || undefined,
      ref,
      states: computeStates(el, role),
      value: computeValue(el, role),
      children: [],
    };
    node.children = visitChildren(el);
    return [node];
  };

  return Array.from(document.body.children).flatMap(visit);
}

function serialize(nodes: AxNode[], depth: number): string[] {
  const lines: string[] = [];
  for (const node of nodes) {
    const parts = [node.role];
    if (node.name) parts.push(JSON.stringify(node.name));
    parts.push(`[${node.ref}]`);
    parts.push(...node.states);
    if (node.value !== undefined) parts.push(`= ${JSON.stringify(node.value)}`);
    lines.push('  '.repeat(depth) + parts.join(' '));
    lines.push(...serialize(node.children, depth + 1));
  }
  return lines;
}

function countNodes(nodes: AxNode[]): number {
  return nodes.reduce((sum, n) => sum + 1 + countNodes(n.children), 0);
}

function snapshot(params: UiCommandParams): { tree: string; nodes: number } {
  const max = typeof params.max === 'number' && params.max > 0 ? params.max : SNAPSHOT_DEFAULT_MAX;
  const tree = buildTree(max);
  return { tree: serialize(tree, 0).join('\n'), nodes: countNodes(tree) };
}

function resolveTarget(params: UiCommandParams): HTMLElement {
  if (params.ref) {
    const el = refMap.get(params.ref);
    if (!el) throw new Error(`unknown ref ${params.ref} — run ui.snapshot first`);
    if (!el.isConnected) throw new Error(`ref ${params.ref} is no longer in the DOM — re-run ui.snapshot`);
    return el as HTMLElement;
  }
  if (params.selector) {
    let el: Element | null;
    try {
      el = document.querySelector(params.selector);
    } catch {
      throw new Error(`invalid selector: ${params.selector}`);
    }
    if (!el) throw new Error(`no element matches selector: ${params.selector}`);
    return el as HTMLElement;
  }
  throw new Error('ref or selector is required');
}

function click(params: UiCommandParams): { clicked: boolean; ref?: string; selector?: string } {
  const el = resolveTarget(params);
  el.scrollIntoView({ block: 'center' });
  el.click();
  return { clicked: true, ref: params.ref, selector: params.selector };
}

// fill uses the native value setter so React's onChange fires (assigning
// .value directly is invisible to React's synthetic event system).
function fill(params: UiCommandParams): { filled: boolean; value: string } {
  const el = resolveTarget(params);
  const value = params.value ?? '';
  if (!(el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement)) {
    throw new Error('target element is not fillable');
  }
  const proto = Object.getPrototypeOf(el);
  const setter = Object.getOwnPropertyDescriptor(proto, 'value')?.set;
  if (setter) {
    setter.call(el, value);
  } else {
    el.value = value;
  }
  el.dispatchEvent(new Event('input', { bubbles: true }));
  el.dispatchEvent(new Event('change', { bubbles: true }));
  return { filled: true, value };
}

// Screenshot uses modern-screenshot (SVG foreignObject) rather than a JS CSS
// parser, so WebKit renders the page itself — modern color functions like
// oklch (emitted by Tailwind v4) and web fonts work.
//
// The raw canvas is retina-resolution (often ~2880px wide), whose PNG data URI
// is several megabytes. When that gets handed to an LLM as a tool result it
// tokenizes to ~1M tokens and overflows the model's context window. We downscale
// the longest side to SCREENSHOT_MAX_EDGE and emit JPEG so a self-screenshot is
// a lightweight, readable vision input.
const SCREENSHOT_MAX_EDGE = 1280;
const SCREENSHOT_JPEG_QUALITY = 0.7;

async function screenshot(): Promise<{ dataUri: string; width: number; height: number }> {
  const { domToCanvas } = await import('modern-screenshot');
  const source = await domToCanvas(document.body);
  const longest = Math.max(source.width, source.height);
  const scale = longest > SCREENSHOT_MAX_EDGE ? SCREENSHOT_MAX_EDGE / longest : 1;
  const width = Math.max(1, Math.round(source.width * scale));
  const height = Math.max(1, Math.round(source.height * scale));
  let out: HTMLCanvasElement = source;
  if (scale < 1) {
    const scaled = document.createElement('canvas');
    scaled.width = width;
    scaled.height = height;
    const ctx = scaled.getContext('2d');
    if (ctx) {
      ctx.imageSmoothingQuality = 'high';
      ctx.drawImage(source, 0, 0, width, height);
      out = scaled;
    }
  }
  return {
    dataUri: out.toDataURL('image/jpeg', SCREENSHOT_JPEG_QUALITY),
    width: out.width,
    height: out.height,
  };
}

export async function executeUiCommand(command: string, params: UiCommandParams): Promise<unknown> {
  switch (command) {
    case 'snapshot':
      return snapshot(params);
    case 'click':
      return click(params);
    case 'fill':
      return fill(params);
    case 'screenshot':
      return screenshot();
    case 'settle':
      return settle();
    default:
      throw new Error(`unknown ui command: ${command}`);
  }
}

// settle resolves only after the browser has laid out and painted a fresh
// frame. The native screenshot reads the window's composited pixels, so after a
// navigation (e.g. clicking "Apps") we must wait for the new view to actually
// paint — otherwise the capture grabs the previous frame. Forcing a synchronous
// layout read plus two animation frames guarantees a compositor paint landed.
function settle(): Promise<{ settled: true }> {
  return new Promise((resolve) => {
    void document.body.offsetHeight;
    requestAnimationFrame(() => {
      requestAnimationFrame(() => resolve({ settled: true }));
    });
  });
}
