import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { sidebarModeForWidth, useSidebar, useSidebarPrefs } from './sidebar.hooks';

describe('sidebarModeForWidth', () => {
  it('derives peek / icon / full from width', () => {
    expect(sidebarModeForWidth(12)).toBe('peek');
    expect(sidebarModeForWidth(84)).toBe('icon');
    expect(sidebarModeForWidth(108)).toBe('icon');
    expect(sidebarModeForWidth(260)).toBe('full');
  });
});

describe('useSidebar', () => {
  beforeEach(() => localStorage.clear());
  afterEach(() => localStorage.clear());

  it('collapse button toggles icon-width <-> full (vanilla parity)', () => {
    localStorage.setItem('aw-sidebar-width', '300');
    const { result } = renderHook(() => useSidebar());
    expect(result.current.width).toBe(300);
    expect(result.current.mode).toBe('full');

    act(() => result.current.toggleCollapse());
    expect(result.current.mode).toBe('icon');
    expect(localStorage.getItem('aw-sidebar-width')).toBe('84');

    act(() => result.current.toggleCollapse());
    expect(result.current.mode).toBe('full');
    expect(localStorage.getItem('aw-sidebar-width')).toBe('260');
  });

  it('expand restores the default full width', () => {
    localStorage.setItem('aw-sidebar-width', '12');
    const { result } = renderHook(() => useSidebar());
    expect(result.current.mode).toBe('peek');
    act(() => result.current.expand());
    expect(result.current.width).toBe(260);
    expect(result.current.mode).toBe('full');
  });
});

describe('useSidebarPrefs', () => {
  beforeEach(() => localStorage.clear());
  afterEach(() => localStorage.clear());

  it('normalizes unsupported top/bottom persisted positions back to left', () => {
    localStorage.setItem('aw-sidebar-position', 'top');
    const { result } = renderHook(() => useSidebarPrefs());
    expect(result.current.position).toBe('left');
  });

  it('does not persist unsupported top/bottom positions through the setter', () => {
    const { result } = renderHook(() => useSidebarPrefs());

    act(() => result.current.setPosition('bottom'));

    expect(result.current.position).toBe('left');
    expect(localStorage.getItem('aw-sidebar-position')).toBe('left');
  });
});
