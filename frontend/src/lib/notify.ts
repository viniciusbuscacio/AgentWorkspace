import { createElement } from 'react';
import { toast } from 'sonner';

// The ONE way to surface a transient notice in aw: the themed Sonner toast
// (top-right, 4s) mounted at the App root, styled like the shadcn Alert —
// leading icon, title line, optional muted description line. Pages route
// their old setMessage strings through notify(), which picks the error style
// by text shape — the codebase's failure messages consistently match this
// pattern.
const ERRORISH = /could not|failed|error|invalid|denied|cannot|unable to|not allowed|refused/i;

function glyph(name: string) {
  return createElement(
    'span',
    { className: 'material-symbols-outlined text-[18px] leading-none', 'aria-hidden': true },
    name,
  );
}

export function notify(text: string, description?: string): void {
  const trimmed = (text ?? '').trim();
  if (!trimmed) return; // legacy setMessage('') clears — nothing to show
  const body = (description ?? '').trim() || undefined;
  if (ERRORISH.test(trimmed) || (body && ERRORISH.test(body))) {
    toast.error(trimmed, { description: body, icon: glyph('error') });
  } else {
    toast(trimmed, { description: body, icon: glyph('check_circle') });
  }
}
