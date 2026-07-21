import { render } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useUiControlBridge } from './ui-control.hooks';

const handlers = new Map<string, (payload: never) => void>();
const reportUiState = vi.fn();

vi.mock('@services/events', () => ({
  onAwEvent: (name: string, callback: (payload: never) => void) => {
    handlers.set(name, callback);
    return () => handlers.delete(name);
  },
}));

vi.mock('@services/ui.service', () => ({
  uiService: {
    reportUiState: (...args: unknown[]) => reportUiState(...args),
  },
}));

function Probe() {
  useUiControlBridge();
  return null;
}

beforeEach(() => {
  handlers.clear();
  reportUiState.mockReset();
  localStorage.clear();
  document.documentElement.dataset.theme = '';
});

describe('useUiControlBridge', () => {
  it('reports the saved theme and font on mount', () => {
    render(<Probe />);
    expect(reportUiState).toHaveBeenCalledWith({
      theme: 'midnight',
      fontFamily: 'system',
      fontSize: 14,
    });
  });

  it('applies ui:set-theme and reports the new theme', () => {
    render(<Probe />);
    handlers.get('ui:set-theme')!({ theme: 'ocean' } as never);
    expect(document.documentElement.dataset.theme).toBe('ocean');
    expect(localStorage.getItem('aw.theme')).toBe('ocean');
    expect(reportUiState).toHaveBeenLastCalledWith({ theme: 'ocean' });
  });

  it('applies ui:set-font keeping the saved family when only size changes', () => {
    localStorage.setItem('aw.font.family', 'jetbrains-mono');
    render(<Probe />);
    handlers.get('ui:set-font')!({ family: '', size: 18 } as never);
    expect(localStorage.getItem('aw.font.family')).toBe('jetbrains-mono');
    expect(localStorage.getItem('aw.font.size')).toBe('18');
    expect(reportUiState).toHaveBeenLastCalledWith({ fontFamily: 'jetbrains-mono', fontSize: 18 });
  });

  it('unsubscribes from ui events on unmount', () => {
    const view = render(<Probe />);
    expect(handlers.size).toBe(2);
    view.unmount();
    expect(handlers.size).toBe(0);
  });
});
