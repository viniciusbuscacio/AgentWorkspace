import DOMPurify from 'dompurify';
import { marked } from 'marked';

marked.setOptions({
  gfm: true,
  breaks: true,
});

export function renderMarkdown(markdown: string): string {
  const html = marked.parse(markdown || '', { async: false }) as string;
  const clean = DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
  });
  return stripUnsafeAttributes(clean);
}

export function renderMarkdownFragment(markdown: string): DocumentFragment {
  const html = renderMarkdown(markdown);
  const fragment = document.createDocumentFragment();
  const container = document.createElement('div');
  container.insertAdjacentHTML('afterbegin', html);
  fragment.append(...Array.from(container.childNodes));
  return fragment;
}

function stripUnsafeAttributes(html: string): string {
  if (typeof document === 'undefined') return html;
  const container = document.createElement('div');
  container.insertAdjacentHTML('afterbegin', html);
  for (const element of Array.from(container.querySelectorAll('*'))) {
    for (const attribute of Array.from(element.attributes)) {
      const name = attribute.name.toLowerCase();
      const value = attribute.value.trim().toLowerCase();
      if (name.startsWith('on') || value.startsWith('javascript:')) {
        element.removeAttribute(attribute.name);
      }
    }
    // Belt-and-suspenders for external links: the delegated click handler is
    // what actually opens them in the OS browser, but rel hardens against
    // tab-nabbing and target signals intent for the web-mode (new tab) path.
    if (element.tagName === 'A') {
      const href = (element.getAttribute('href') ?? '').trim();
      if (/^(https?:|mailto:)/i.test(href)) {
        element.setAttribute('rel', 'noopener noreferrer');
        element.setAttribute('target', '_blank');
      }
    }
  }
  return Array.from(container.childNodes).map(serializeNode).join('');
}

function serializeNode(node: ChildNode): string {
  if (node.nodeType === Node.TEXT_NODE) {
    return escapeHtml(node.textContent ?? '');
  }
  if (node instanceof Element) {
    return node.outerHTML;
  }
  return '';
}

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;');
}
