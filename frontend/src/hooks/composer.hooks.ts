import { useCallback, useState } from 'react';

// Composer textarea resize, ported from vanilla AW2 (TEXTAREA_HEIGHT_KEY).
const HEIGHT_KEY = 'aw-chat-input-height';
const MIN_HEIGHT = 48;
const DEFAULT_HEIGHT = 96;

function readInitialHeight(): number {
  const saved = Number(localStorage.getItem(HEIGHT_KEY));
  return Number.isFinite(saved) && saved >= MIN_HEIGHT ? saved : DEFAULT_HEIGHT;
}

export interface UseComposerHeight {
  height: number;
  /** Pointer-down on the resize grip; drag up to grow, down to shrink. */
  beginResize: (event: React.PointerEvent) => void;
}

export function useComposerHeight(): UseComposerHeight {
  const [height, setHeight] = useState<number>(readInitialHeight);

  const beginResize = useCallback((event: React.PointerEvent) => {
    event.preventDefault();
    const startY = event.clientY;
    const startHeight = height;
    const maxHeight = window.innerHeight * 0.5;
    let latest = startHeight;
    document.body.style.userSelect = 'none';
    document.body.style.cursor = 'row-resize';

    const onMove = (ev: PointerEvent) => {
      const delta = startY - ev.clientY;
      latest = Math.min(maxHeight, Math.max(MIN_HEIGHT, startHeight + delta));
      setHeight(latest);
    };
    const onUp = () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      document.body.style.userSelect = '';
      document.body.style.cursor = '';
      localStorage.setItem(HEIGHT_KEY, String(Math.round(latest)));
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  }, [height]);

  return { height, beginResize };
}
