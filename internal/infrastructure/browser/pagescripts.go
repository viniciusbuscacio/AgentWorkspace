package browser

// The page-side scripts adapt frontend/src/lib/ui-automation.ts to arbitrary
// web pages: snapshot builds a pruned accessibility-ish tree with ephemeral
// refs (e1, e2, ...) kept on window.__awBrowserRefs; click/fill resolve a ref
// (preferred) or a CSS selector. Password values are always masked — page
// content is the caller's to see, secrets are not.

const snapshotJS = `function (max) {
  const ACTIONABLE = new Set(['button','link','textbox','searchbox','checkbox','radio','switch','slider','spinbutton','combobox','menuitem','menuitemcheckbox','tab','option']);
  const CONTAINER = new Set(['navigation','main','banner','contentinfo','complementary','region','group','list','listitem','dialog','heading','article','tablist','menu','form','table','iframe']);
  const IMPLICIT = {A:'link',BUTTON:'button',NAV:'navigation',MAIN:'main',HEADER:'banner',FOOTER:'contentinfo',ASIDE:'complementary',SECTION:'region',ARTICLE:'article',UL:'list',OL:'list',LI:'listitem',IMG:'img',TEXTAREA:'textbox',SELECT:'combobox',DIALOG:'dialog',FORM:'form',TABLE:'table',IFRAME:'iframe',H1:'heading',H2:'heading',H3:'heading',H4:'heading',H5:'heading',H6:'heading'};
  const INPUT_ROLE = {checkbox:'checkbox',radio:'radio',range:'slider',number:'spinbutton',button:'button',submit:'button',reset:'button',search:'searchbox'};
  const refs = new Map();
  window.__awBrowserRefs = refs;
  let count = 0;
  const clean = (t) => (t || '').replace(/\s+/g, ' ').trim().slice(0, 120);
  const visible = (el) => {
    if (el.hidden) return false;
    if (el.inert || (el.closest && el.closest('[inert]'))) return false;
    const style = el.ownerDocument.defaultView ? el.ownerDocument.defaultView.getComputedStyle(el) : null;
    if (style && (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0')) return false;
    if (el.getClientRects && el.getClientRects().length === 0) {
      const tag = el.tagName;
      if (!['HTML','BODY','HEAD','TITLE','SCRIPT','STYLE','META','LINK'].includes(tag)) return false;
    }
    return true;
  };
  const accessibleText = (el) => {
    let out = '';
    for (const node of Array.from(el.childNodes || [])) {
      if (node.nodeType === 3) {
        out += node.textContent || '';
      } else if (node.nodeType === 1) {
        const child = node;
        const childRole = child.getAttribute('role');
        if (child.getAttribute('aria-hidden') === 'true' || childRole === 'presentation' || childRole === 'none') continue;
        out += accessibleText(child);
      }
    }
    return out;
  };
  const labelledby = (el) => {
    const ids = (el.getAttribute('aria-labelledby') || '').split(/\s+/).filter(Boolean);
    return ids.map((id) => clean(el.ownerDocument.getElementById(id)?.textContent)).filter(Boolean).join(' ');
  };
  const role = (el) => {
    const explicit = el.getAttribute('role');
    if (explicit) return explicit;
    const tag = el.tagName;
    if (tag === 'INPUT') return INPUT_ROLE[(el.getAttribute('type') || 'text').toLowerCase()] || 'textbox';
    if (tag === 'A') return el.hasAttribute('href') ? 'link' : '';
    return IMPLICIT[tag] || '';
  };
  const name = (el, r) => {
    const aria = el.getAttribute('aria-label');
    if (aria) return clean(aria);
    const referenced = labelledby(el);
    if (referenced) return referenced;
    if (el.labels && el.labels[0]) return clean(el.labels[0].textContent);
    if (el.placeholder) return clean(el.placeholder);
    if (['button','link','heading','listitem','option','tab','menuitem'].includes(r)) {
      const text = clean(accessibleText(el));
      if (text) return text;
    }
    const title = el.getAttribute('title');
    if (title) return clean(title);
    if (el.tagName === 'IMG') return clean(el.getAttribute('alt'));
    return '';
  };
  const iframeDocument = (el) => {
    if (el.tagName !== 'IFRAME') return { available: false, doc: null };
    try {
      return el.contentDocument && el.contentDocument.body
        ? { available: true, doc: el.contentDocument }
        : { available: false, doc: null };
    } catch {
      return { available: false, doc: null };
    }
  };
  const states = (el, r) => {
    const out = [];
    if (['checkbox','radio','switch','menuitemcheckbox'].includes(r)) out.push(el.checked ? 'checked' : 'unchecked');
    if (el.disabled || el.getAttribute('aria-disabled') === 'true') out.push('disabled');
    const expanded = el.getAttribute('aria-expanded');
    if (expanded) out.push(expanded === 'true' ? 'expanded' : 'collapsed');
    return out;
  };
  const value = (el, r) => {
    if (!['textbox','searchbox','spinbutton','combobox'].includes(r)) return undefined;
    if (!('value' in el) || !el.value) return undefined;
    if (el.type === 'password') return '••••••';
    return String(el.value).slice(0, 80);
  };
  const lines = [];
  const visit = (el, depth) => {
    if (count >= max || !visible(el)) return;
    const r = role(el);
    const keep = ACTIONABLE.has(r) || CONTAINER.has(r) || el.hasAttribute('aria-label');
    let nextDepth = depth;
    const frame = iframeDocument(el);
    if (keep) {
      count++;
      const ref = 'e' + count;
      refs.set(ref, el);
      const parts = [r || 'generic'];
      const n = frame.available ? name(el, r) : (r === 'iframe' ? (name(el, r) || '<cross-origin>') : name(el, r));
      if (n) parts.push(JSON.stringify(n));
      parts.push('[' + ref + ']');
      parts.push(...states(el, r));
      const v = value(el, r);
      if (v !== undefined) parts.push('= ' + JSON.stringify(v));
      lines.push('  '.repeat(depth) + parts.join(' '));
      nextDepth = depth + 1;
    }
    if (frame.available) {
      for (const child of Array.from(frame.doc.body.children)) {
        if (count >= max) break;
        visit(child, nextDepth);
      }
      return;
    }
    if (r === 'iframe') return;
    for (const child of Array.from(el.children)) {
      if (count >= max) break;
      visit(child, nextDepth);
    }
  };
  for (const el of Array.from(document.body ? document.body.children : [])) visit(el, 0);
  return { url: location.href, title: document.title, tree: lines.join('\n'), nodes: count };
}`

const resolveTargetJS = `function (ref, selector) {
  if (ref) {
    const refs = window.__awBrowserRefs;
    const el = refs ? refs.get(ref) : null;
    if (!el) throw new Error('unknown ref ' + ref + ' - run browser.snapshot first');
    if (!el.isConnected) throw new Error('ref ' + ref + ' is no longer in the DOM - re-run browser.snapshot');
    return el;
  }
  const el = document.querySelector(selector);
  if (!el) throw new Error('no element matches selector: ' + selector);
  return el;
}`

const clickJS = `function (ref, selector) {
  const resolve = ` + resolveTargetJS + `;
  const el = resolve(ref, selector);
  el.scrollIntoView({ block: 'center' });
  el.click();
  return { clicked: true, ref: ref || undefined, selector: selector || undefined };
}`

const navigationTargetJS = `function (ref, selector) {
  const resolve = ` + resolveTargetJS + `;
  const el = resolve(ref, selector);
  const anchor = el.closest ? el.closest('a[href], area[href]') : null;
  if (!anchor) return { url: '' };
  return { url: anchor.href || '' };
}`

const currentURLJS = `function () {
  return { url: location.href || '' };
}`

const fillJS = `function (ref, selector, value) {
  const resolve = ` + resolveTargetJS + `;
  const el = resolve(ref, selector);
  const tag = el.tagName;
  if (tag !== 'INPUT' && tag !== 'TEXTAREA' && tag !== 'SELECT') throw new Error('target element is not fillable');
  const proto = Object.getPrototypeOf(el);
  const setter = Object.getOwnPropertyDescriptor(proto, 'value');
  if (setter && setter.set) { setter.set.call(el, value); } else { el.value = value; }
  el.dispatchEvent(new Event('input', { bubbles: true }));
  el.dispatchEvent(new Event('change', { bubbles: true }));
  const echo = el.type === 'password' ? '••••••' : value;
  return { filled: true, value: echo };
}`
