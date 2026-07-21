export interface BrowserSnapshotResult {
  url: string;
  title: string;
  tree: string;
  nodes: number;
}

export interface BrowserTargetParams {
  ref?: string;
  selector?: string;
  value?: string;
}

const ACTIONABLE = new Set(['button', 'link', 'textbox', 'searchbox', 'checkbox', 'radio', 'switch', 'slider', 'spinbutton', 'combobox', 'menuitem', 'menuitemcheckbox', 'tab', 'option']);
const CONTAINER = new Set(['navigation', 'main', 'banner', 'contentinfo', 'complementary', 'region', 'group', 'list', 'listitem', 'dialog', 'heading', 'article', 'tablist', 'menu', 'form', 'table', 'iframe']);
const IMPLICIT: Record<string, string> = {
  A: 'link', BUTTON: 'button', NAV: 'navigation', MAIN: 'main',
  HEADER: 'banner', FOOTER: 'contentinfo', ASIDE: 'complementary',
  SECTION: 'region', ARTICLE: 'article', UL: 'list', OL: 'list',
  LI: 'listitem', IMG: 'img', TEXTAREA: 'textbox', SELECT: 'combobox',
  DIALOG: 'dialog', FORM: 'form', TABLE: 'table', IFRAME: 'iframe',
  H1: 'heading', H2: 'heading', H3: 'heading', H4: 'heading',
  H5: 'heading', H6: 'heading',
};
const INPUT_ROLE: Record<string, string> = {
  checkbox: 'checkbox', radio: 'radio', range: 'slider', number: 'spinbutton',
  button: 'button', submit: 'button', reset: 'button', search: 'searchbox',
};

let browserRefMap = new Map<string, Element>();

export function browserPageSnapshot(doc: Document = document, max = 500): BrowserSnapshotResult {
  browserRefMap = new Map();
  let count = 0;
  const lines: string[] = [];

  const visit = (el: Element, depth: number) => {
    if (count >= max || !isVisible(el)) return;
    const currentRole = role(el);
    const keep = ACTIONABLE.has(currentRole) || CONTAINER.has(currentRole) || el.hasAttribute('aria-label');
    let nextDepth = depth;
    const frame = iframeDocument(el);

    if (keep) {
      count += 1;
      const ref = `e${count}`;
      browserRefMap.set(ref, el);
      const parts = [currentRole || 'generic'];
      const computedName = frame.available ? name(el, currentRole) : (currentRole === 'iframe' ? (name(el, currentRole) || '<cross-origin>') : name(el, currentRole));
      if (computedName) parts.push(JSON.stringify(computedName));
      parts.push(`[${ref}]`);
      parts.push(...states(el, currentRole));
      const computedValue = value(el, currentRole);
      if (computedValue !== undefined) parts.push(`= ${JSON.stringify(computedValue)}`);
      lines.push(`${'  '.repeat(depth)}${parts.join(' ')}`);
      nextDepth = depth + 1;
    }

    if (frame.available && frame.doc?.body) {
      for (const child of Array.from(frame.doc.body.children)) {
        if (count >= max) break;
        visit(child, nextDepth);
      }
      return;
    }
    if (currentRole === 'iframe') return;
    for (const child of Array.from(el.children)) {
      if (count >= max) break;
      visit(child, nextDepth);
    }
  };

  for (const el of Array.from(doc.body ? doc.body.children : [])) visit(el, 0);
  return { url: doc.location.href, title: doc.title, tree: lines.join('\n'), nodes: count };
}

export function browserPageClick(params: BrowserTargetParams): { clicked: boolean; ref?: string; selector?: string } {
  const el = resolveTarget(params) as HTMLElement;
  el.scrollIntoView({ block: 'center' });
  el.click();
  return { clicked: true, ref: params.ref, selector: params.selector };
}

export function browserPageFill(params: BrowserTargetParams): { filled: boolean; value: string } {
  const el = resolveTarget(params);
  const nextValue = params.value ?? '';
  if (!(el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement)) {
    throw new Error('target element is not fillable');
  }
  const proto = Object.getPrototypeOf(el);
  const setter = Object.getOwnPropertyDescriptor(proto, 'value');
  if (setter?.set) {
    setter.set.call(el, nextValue);
  } else {
    el.value = nextValue;
  }
  el.dispatchEvent(new Event('input', { bubbles: true }));
  el.dispatchEvent(new Event('change', { bubbles: true }));
  return { filled: true, value: el instanceof HTMLInputElement && el.type === 'password' ? '••••••' : nextValue };
}

function resolveTarget(params: BrowserTargetParams): Element {
  if (params.ref) {
    const el = browserRefMap.get(params.ref);
    if (!el) throw new Error(`unknown ref ${params.ref} - run browser.snapshot first`);
    if (!el.isConnected) throw new Error(`ref ${params.ref} is no longer in the DOM - re-run browser.snapshot`);
    return el;
  }
  if (params.selector) {
    const el = document.querySelector(params.selector);
    if (!el) throw new Error(`no element matches selector: ${params.selector}`);
    return el;
  }
  throw new Error('ref (from browser.snapshot) or selector is required');
}

function isVisible(el: Element): boolean {
  const html = el as HTMLElement;
  if (html.hidden) return false;
  if (html.inert || html.closest('[inert]')) return false;
  const style = el.ownerDocument.defaultView?.getComputedStyle(html);
  if (style && (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0')) return false;
  if (html.getClientRects && html.getClientRects().length === 0) {
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

function accessibleText(el: Element): string {
  let out = '';
  for (const node of Array.from(el.childNodes)) {
    if (node.nodeType === 3) {
      out += node.textContent || '';
    } else if (node.nodeType === 1) {
      const child = node as Element;
      const childRole = child.getAttribute('role');
      if (child.getAttribute('aria-hidden') === 'true' || childRole === 'presentation' || childRole === 'none') continue;
      out += accessibleText(child);
    }
  }
  return out;
}

function referencedText(el: Element): string {
  const ids = (el.getAttribute('aria-labelledby') || '').split(/\s+/).filter(Boolean);
  return ids.map((id) => clean(el.ownerDocument.getElementById(id)?.textContent)).filter(Boolean).join(' ');
}

function role(el: Element): string {
  const explicit = el.getAttribute('role');
  if (explicit) return explicit;
  const tag = el.tagName;
  if (tag === 'INPUT') return INPUT_ROLE[(el.getAttribute('type') || 'text').toLowerCase()] || 'textbox';
  if (tag === 'A') return el.hasAttribute('href') ? 'link' : '';
  return IMPLICIT[tag] || '';
}

function name(el: Element, currentRole: string): string {
  const aria = el.getAttribute('aria-label');
  if (aria) return clean(aria);
  const referenced = referencedText(el);
  if (referenced) return referenced;
  if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement) {
    if (el.labels?.[0]) return clean(el.labels[0].textContent);
  }
  if ('placeholder' in el && typeof el.placeholder === 'string') return clean(el.placeholder);
  if (['button', 'link', 'heading', 'listitem', 'option', 'tab', 'menuitem'].includes(currentRole)) {
    const text = clean(accessibleText(el));
    if (text) return text;
  }
  const title = el.getAttribute('title');
  if (title) return clean(title);
  if (el.tagName === 'IMG') return clean(el.getAttribute('alt'));
  return '';
}

function iframeDocument(el: Element): { available: boolean; doc: Document | null } {
  if (el.tagName !== 'IFRAME') return { available: false, doc: null };
  try {
    const frame = el as HTMLIFrameElement;
    return frame.contentDocument?.body ? { available: true, doc: frame.contentDocument } : { available: false, doc: null };
  } catch {
    return { available: false, doc: null };
  }
}

function states(el: Element, currentRole: string): string[] {
  const out: string[] = [];
  if (['checkbox', 'radio', 'switch', 'menuitemcheckbox'].includes(currentRole)) {
    out.push(el instanceof HTMLInputElement && el.checked ? 'checked' : 'unchecked');
  }
  if ('disabled' in el && el.disabled || el.getAttribute('aria-disabled') === 'true') out.push('disabled');
  const expanded = el.getAttribute('aria-expanded');
  if (expanded) out.push(expanded === 'true' ? 'expanded' : 'collapsed');
  return out;
}

function value(el: Element, currentRole: string): string | undefined {
  if (!['textbox', 'searchbox', 'spinbutton', 'combobox'].includes(currentRole)) return undefined;
  if (!(el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement) || !el.value) return undefined;
  if (el instanceof HTMLInputElement && el.type === 'password') return '••••••';
  return String(el.value).slice(0, 80);
}
