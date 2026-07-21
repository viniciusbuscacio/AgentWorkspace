// @vitest-environment happy-dom
import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import React from 'react';
import { Switch } from './switch';

describe('Switch', () => {
  it('keeps a stable centered track/thumb geometry', () => {
    const { container } = render(<Switch checked={false} onCheckedChange={() => {}} />);
    const track = container.querySelector('[data-slot="switch"]') as HTMLElement;
    const thumb = container.querySelector('[data-slot="switch-thumb"]') as HTMLElement;

    expect(track).not.toBeNull();
    expect(thumb).not.toBeNull();
    expect(track.className).toContain('h-6');
    expect(track.className).toContain('w-11');
    expect(track.className).toContain('p-0.5');
    expect(thumb.className).toContain('size-5');
    expect(thumb.className).toContain('data-[state=checked]:group-data-[size=default]/switch:translate-x-5');
  });

  it('does not regress to the cramped ReUI generated geometry', () => {
    const { container } = render(<Switch checked={false} onCheckedChange={() => {}} />);
    const track = container.querySelector('[data-slot="switch"]') as HTMLElement;
    const thumb = container.querySelector('[data-slot="switch-thumb"]') as HTMLElement;

    expect(track.className).not.toContain('h-[1.15rem]');
    expect(track.className).not.toContain('w-8');
    expect(thumb.className).not.toContain('data-[state=checked]:translate-x-[calc(100%-2px)]');
  });
});
