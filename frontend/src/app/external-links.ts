import { useEffect } from 'react';

/* eslint-disable @typescript-eslint/no-explicit-any */

// External links rendered in chat markdown (and anywhere else) are real
// <a href="https://…"> with no click handler. In the Wails webview a plain
// click navigates the window top-level, tearing the SPA out from under the user
// with no way back. This delegated handler intercepts clicks on absolute
// external links and routes them to the OS default browser via the Wails
// runtime (BrowserOpenURL). In web mode the runtime shim maps BrowserOpenURL to
// window.open(_blank), so a remote browser opens a new tab instead.

const EXTERNAL_HREF = /^(https?:|mailto:)/i;

// openExternalUrl hands a URL to the OS browser (desktop) / a new tab (web).
export function openExternalUrl(href: string): void {
  (window as any).runtime?.BrowserOpenURL?.(href);
}

// handleExternalLinkClick intercepts a click on an absolute external link and
// opens it outside the webview. It is a no-op (returns false) for non-primary
// or modified clicks, non-anchor targets, and internal/relative/non-http hrefs,
// leaving native behavior intact. Returns true when it handled the click.
export function handleExternalLinkClick(event: MouseEvent): boolean {
  // Let already-handled clicks, non-primary buttons, and modified clicks
  // (Ctrl/Cmd/Shift/Alt — "open in new tab/window") fall through to native
  // behavior so the OS / remote browser decides.
  if (event.defaultPrevented || event.button !== 0) return false;
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return false;

  const target = event.target as HTMLElement | null;
  const anchor = target?.closest?.('a');
  if (!anchor) return false;

  const href = anchor.getAttribute('href') ?? '';
  if (!EXTERNAL_HREF.test(href)) return false; // only absolute external links

  event.preventDefault();
  openExternalUrl(href);
  return true;
}

// useExternalLinks installs the delegated click handler on document for the
// lifetime of the app. Mount it once at the always-mounted root.
export function useExternalLinks(): void {
  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      handleExternalLinkClick(event);
    };
    // Capture phase so we run before any component that might stopPropagation,
    // and before the webview's default top-level navigation kicks in.
    document.addEventListener('click', onClick, true);
    return () => document.removeEventListener('click', onClick, true);
  }, []);
}
