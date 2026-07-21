import { useCallback, useEffect, useRef, useState, type DependencyList } from 'react';

const BOTTOM_THRESHOLD_PX = 96;

function isNearBottom(scrollEl: HTMLDivElement) {
  return scrollEl.scrollHeight - scrollEl.scrollTop - scrollEl.clientHeight <= BOTTOM_THRESHOLD_PX;
}

function isAtHardBottom(scrollEl: HTMLDivElement) {
  return scrollEl.scrollHeight - scrollEl.scrollTop - scrollEl.clientHeight <= 1;
}

/**
 * Keeps chat pinned to the bottom while the user is following the latest
 * messages, but lets them scroll up during streaming without being pulled back
 * down on every chunk. scrollToBottom({ force: true }) re-pins the chat (used
 * when the user sends a message or clicks the down button). Ported from AW2.
 */
export function useChatAutoScroll(deps: DependencyList) {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const frameRef = useRef<number | null>(null);
  const shouldPinToBottomRef = useRef(true);
  const lastScrollTopRef = useRef(0);
  const lastScrollHeightRef = useRef(0);
  const [isAtBottom, setIsAtBottom] = useState(true);

  const cancelPendingScroll = useCallback(() => {
    if (frameRef.current !== null) {
      cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
    }
  }, []);

  const updateBottomState = useCallback((scrollEl: HTMLDivElement) => {
    const atBottom = isNearBottom(scrollEl);
    shouldPinToBottomRef.current = atBottom;
    setIsAtBottom(atBottom);
    return atBottom;
  }, []);

  const scrollToBottom = useCallback((options?: { force?: boolean }) => {
    if (options?.force) {
      shouldPinToBottomRef.current = true;
      setIsAtBottom(true);
    }
    cancelPendingScroll();
    frameRef.current = requestAnimationFrame(() => {
      frameRef.current = null;
      const scrollEl = scrollRef.current;
      // Re-check the pin: the user may have scrolled up between scheduling
      // this frame and it firing (frequent while streaming).
      if (scrollEl && shouldPinToBottomRef.current) {
        scrollEl.scrollTop = scrollEl.scrollHeight;
        updateBottomState(scrollEl);
      }
    });
  }, [cancelPendingScroll, updateBottomState]);

  useEffect(() => {
    const scrollEl = scrollRef.current;
    if (!scrollEl) return;
    lastScrollTopRef.current = scrollEl.scrollTop;
    lastScrollHeightRef.current = scrollEl.scrollHeight;
    updateBottomState(scrollEl);
    const handleScroll = () => {
      const prevTop = lastScrollTopRef.current;
      const prevHeight = lastScrollHeightRef.current;
      lastScrollTopRef.current = scrollEl.scrollTop;
      lastScrollHeightRef.current = scrollEl.scrollHeight;
      // Auto-scroll only ever moves the chat down, so upward movement (while
      // the content didn't shrink and we're not clamped at the hard bottom,
      // e.g. by a window resize) is always the user scrolling back: unpin
      // immediately instead of waiting for the position to leave the bottom
      // threshold — otherwise the next streaming chunk drags the user back
      // down before they can escape it.
      const movedUp =
        scrollEl.scrollTop < prevTop &&
        scrollEl.scrollHeight >= prevHeight &&
        !isAtHardBottom(scrollEl);
      if (movedUp) {
        shouldPinToBottomRef.current = false;
        setIsAtBottom(false);
        cancelPendingScroll();
        return;
      }
      updateBottomState(scrollEl);
    };
    // The scroll-direction check above can miss the gesture in WKWebView:
    // WebKit coalesces scroll events per frame, so the user's upward delta and
    // our programmatic pin-to-bottom can merge into one net-downward event.
    // The wheel event carries the user's intent before the position updates,
    // so it unpins reliably regardless of how scroll events are delivered.
    const handleWheel = (event: WheelEvent) => {
      if (event.deltaY >= 0) return;
      if (scrollEl.scrollHeight <= scrollEl.clientHeight) return;
      shouldPinToBottomRef.current = false;
      setIsAtBottom(false);
      cancelPendingScroll();
    };
    scrollEl.addEventListener('scroll', handleScroll, { passive: true });
    scrollEl.addEventListener('wheel', handleWheel, { passive: true });
    return () => {
      scrollEl.removeEventListener('scroll', handleScroll);
      scrollEl.removeEventListener('wheel', handleWheel);
    };
  }, [cancelPendingScroll, updateBottomState]);

  useEffect(() => {
    if (!shouldPinToBottomRef.current) return;
    scrollToBottom();
    return () => {
      cancelPendingScroll();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { scrollRef, isAtBottom, scrollToBottom };
}
