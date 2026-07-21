import { describe, expect, it } from 'vitest';
import { renderMarkdown } from './markdown';

describe('renderMarkdown', () => {
  it('sanitizes unsafe html', () => {
    const html = renderMarkdown('hello <img src=x onerror="alert(1)"> **world**');
    expect(html).toContain('<strong>world</strong>');
    expect(html).not.toContain('onerror');
  });

  it('adds rel and target to external links', () => {
    const html = renderMarkdown('see [Whisper](https://github.com/openai/whisper)');
    expect(html).toMatch(/<a [^>]*href="https:\/\/github\.com\/openai\/whisper"/);
    expect(html).toContain('rel="noopener noreferrer"');
    expect(html).toContain('target="_blank"');
  });

  it('leaves relative/anchor links without target', () => {
    const html = renderMarkdown('[top](#section) and [rel](./page)');
    expect(html).not.toContain('target="_blank"');
  });
});
