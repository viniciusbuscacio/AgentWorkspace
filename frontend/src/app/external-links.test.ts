import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { handleExternalLinkClick } from './external-links';

/* eslint-disable @typescript-eslint/no-explicit-any */

describe('handleExternalLinkClick', () => {
  let openSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    openSpy = vi.fn();
    (window as any).runtime = { BrowserOpenURL: openSpy };
  });

  afterEach(() => {
    delete (window as any).runtime;
    document.body.innerHTML = '';
  });

  function clickOn(html: string, init: Partial<MouseEventInit> = {}) {
    document.body.innerHTML = html;
    const anchor = document.querySelector('a, span, img')!;
    const event = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, ...init });
    Object.defineProperty(event, 'target', { value: anchor, enumerable: true });
    const handled = handleExternalLinkClick(event);
    return { event, handled };
  }

  it('opens absolute https links in the OS browser and prevents navigation', () => {
    const { event, handled } = clickOn('<a href="https://example.com/x">link</a>');
    expect(handled).toBe(true);
    expect(event.defaultPrevented).toBe(true);
    expect(openSpy).toHaveBeenCalledWith('https://example.com/x');
  });

  it('handles mailto links', () => {
    const { handled } = clickOn('<a href="mailto:a@b.com">mail</a>');
    expect(handled).toBe(true);
    expect(openSpy).toHaveBeenCalledWith('mailto:a@b.com');
  });

  it('resolves the anchor from a nested click target', () => {
    document.body.innerHTML = '<a href="https://example.com"><img src="x" /></a>';
    const img = document.querySelector('img')!;
    const event = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 });
    Object.defineProperty(event, 'target', { value: img, enumerable: true });
    expect(handleExternalLinkClick(event)).toBe(true);
    expect(openSpy).toHaveBeenCalledWith('https://example.com');
  });

  it('ignores anchor (#) and relative links', () => {
    expect(clickOn('<a href="#section">x</a>').handled).toBe(false);
    expect(clickOn('<a href="./page">x</a>').handled).toBe(false);
    expect(clickOn('<a href="">x</a>').handled).toBe(false);
    expect(openSpy).not.toHaveBeenCalled();
  });

  it('ignores javascript:/data: schemes', () => {
    expect(clickOn('<a href="javascript:alert(1)">x</a>').handled).toBe(false);
    expect(clickOn('<a href="data:text/html,x">x</a>').handled).toBe(false);
    expect(openSpy).not.toHaveBeenCalled();
  });

  it('ignores non-anchor clicks', () => {
    expect(clickOn('<span>plain</span>').handled).toBe(false);
    expect(openSpy).not.toHaveBeenCalled();
  });

  it('respects modifier keys and non-primary buttons (native behavior)', () => {
    expect(clickOn('<a href="https://example.com">x</a>', { metaKey: true }).handled).toBe(false);
    expect(clickOn('<a href="https://example.com">x</a>', { ctrlKey: true }).handled).toBe(false);
    expect(clickOn('<a href="https://example.com">x</a>', { shiftKey: true }).handled).toBe(false);
    expect(clickOn('<a href="https://example.com">x</a>', { button: 1 }).handled).toBe(false);
    expect(openSpy).not.toHaveBeenCalled();
  });
});
