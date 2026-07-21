// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { browserPageClick, browserPageFill, browserPageSnapshot } from './browser-page-snapshot';

beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, 'getClientRects').mockReturnValue({ length: 1, item: () => ({ width: 10, height: 10 }) as DOMRect, 0: { width: 10, height: 10 } as DOMRect } as unknown as DOMRectList);
  HTMLElement.prototype.scrollIntoView = vi.fn();
});

afterEach(() => {
  document.body.innerHTML = '';
  vi.restoreAllMocks();
});

function refFor(tree: string, name: string): string {
  const line = tree.split('\n').find((entry) => entry.includes(name));
  expect(line).toBeTruthy();
  return line!.match(/\[(e\d+)\]/)![1];
}

describe('browserPageSnapshot', () => {
  it('excludes decorative glyphs from names', () => {
    document.body.innerHTML = `<button><span aria-hidden="true">content_copy</span>Copiar</button>`;

    const { tree } = browserPageSnapshot();

    expect(tree).toContain('button "Copiar"');
    expect(tree).not.toContain('content_copy');
  });

  it('traverses same-origin iframes and keeps refs clickable', () => {
    document.body.innerHTML = `<iframe title="Frame"></iframe>`;
    const iframe = document.querySelector('iframe')!;
    iframe.contentDocument!.body.innerHTML = `<button id="inside">Inside</button>`;
    let clicked = false;
    iframe.contentDocument!.querySelector('#inside')!.addEventListener('click', () => {
      clicked = true;
    });

    const { tree } = browserPageSnapshot();
    const ref = refFor(tree, 'Inside');
    browserPageClick({ ref });

    expect(tree).toContain('iframe "Frame"');
    expect(tree).toContain('button "Inside"');
    expect(clicked).toBe(true);
  });

  it('marks inaccessible iframes as cross-origin leaves', () => {
    document.body.innerHTML = `<iframe></iframe><button>Outside</button>`;
    const iframe = document.querySelector('iframe')!;
    Object.defineProperty(iframe, 'contentDocument', {
      configurable: true,
      get() {
        throw new DOMException('Blocked', 'SecurityError');
      },
    });

    const { tree } = browserPageSnapshot();

    expect(tree).toContain('iframe "<cross-origin>"');
    expect(tree).toContain('button "Outside"');
  });

  it('filters hidden, transparent, inert and zero-size elements', () => {
    document.body.innerHTML = `
      <button style="opacity:0">Transparent</button>
      <div inert><button>Inert</button></div>
      <button id="zero" style="width:0;height:0">Zero</button>
      <button>Visible</button>
    `;
    vi.spyOn(document.querySelector('#zero') as HTMLElement, 'getClientRects').mockReturnValue({ length: 0, item: () => null } as unknown as DOMRectList);

    const { tree } = browserPageSnapshot();

    expect(tree).toContain('button "Visible"');
    expect(tree).not.toContain('Transparent');
    expect(tree).not.toContain('Inert');
    expect(tree).not.toContain('Zero');
  });

  it('masks password values and fill echoes', () => {
    document.body.innerHTML = `<input type="password" aria-label="Token" value="secret">`;

    const { tree } = browserPageSnapshot();
    const ref = refFor(tree, 'Token');
    const result = browserPageFill({ ref, value: 'new-secret' });

    expect(tree).toContain('= "••••••"');
    expect(tree).not.toContain('secret');
    expect(result.value).toBe('••••••');
  });

  it('caps nodes by max', () => {
    document.body.innerHTML = Array.from({ length: 20 }, (_, index) => `<button>B${index}</button>`).join('');

    const { nodes } = browserPageSnapshot(document, 5);

    expect(nodes).toBe(5);
  });
});
