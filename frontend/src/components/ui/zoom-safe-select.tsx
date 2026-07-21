import * as React from 'react';
import { createPortal } from 'react-dom';

import { cn } from '@/lib/utils';
import { measureFixedScale } from '@/lib/app-zoom';

// ZoomSafeSelect — a shadcn-styled select whose dropdown is positioned with the
// app's own zoom-aware fixed-coordinate math (the same measureFixedScale()
// approach the context menus use). Radix/Floating-UI selects mis-position under
// the app-wide CSS `zoom` in WKWebView (the menu "flies" away from the field);
// this component avoids that entirely by computing the panel's fixed left/top
// from the trigger rect divided by the measured scale, so it stays glued to the
// trigger at any zoom level. Visual styling matches the shadcn Select.

export interface ZoomSafeSelectOption {
  value: string;
  label: React.ReactNode;
  disabled?: boolean;
}

interface ZoomSafeSelectProps {
  value: string;
  onValueChange: (value: string) => void;
  options: ZoomSafeSelectOption[];
  placeholder?: string;
  className?: string;
  disabled?: boolean;
  'aria-label'?: string;
}

interface PanelPos {
  left: number;
  width: number;
  top?: number;
  bottom?: number;
  maxHeight: number;
}

function computePanelPos(trigger: HTMLElement, optionCount: number): PanelPos {
  const scale = measureFixedScale();
  const rect = trigger.getBoundingClientRect();
  const gap = 4;
  // Convert the trigger rect (client coords) into the fixed-coordinate space;
  // the panel is position:fixed and gets visually scaled back by the zoom, so
  // it lands exactly aligned with the trigger.
  const left = rect.left / scale;
  const width = rect.width / scale;
  const viewportH = window.innerHeight / scale;
  const belowTop = rect.bottom / scale + gap;
  const spaceBelow = viewportH - belowTop;
  const spaceAbove = rect.top / scale - gap;
  const desired = Math.min(300, Math.max(optionCount, 1) * 34 + 8);
  if (spaceBelow >= Math.min(desired, 160) || spaceBelow >= spaceAbove) {
    return { left, width, top: belowTop, maxHeight: Math.max(120, Math.min(desired, spaceBelow)) };
  }
  return {
    left,
    width,
    bottom: viewportH - (rect.top / scale - gap),
    maxHeight: Math.max(120, Math.min(desired, spaceAbove)),
  };
}

export function ZoomSafeSelect({
  value,
  onValueChange,
  options,
  placeholder,
  className,
  disabled,
  'aria-label': ariaLabel,
}: ZoomSafeSelectProps) {
  const [open, setOpen] = React.useState(false);
  const [pos, setPos] = React.useState<PanelPos | null>(null);
  const [activeIndex, setActiveIndex] = React.useState(-1);
  const triggerRef = React.useRef<HTMLButtonElement>(null);
  const panelRef = React.useRef<HTMLDivElement>(null);
  const listboxId = React.useId();

  const selectedIndex = options.findIndex((opt) => opt.value === value);
  const selected = selectedIndex >= 0 ? options[selectedIndex] : undefined;

  const close = React.useCallback((focusTrigger = false) => {
    setOpen(false);
    setPos(null);
    if (focusTrigger) triggerRef.current?.focus();
  }, []);

  const openPanel = React.useCallback(() => {
    if (disabled || !triggerRef.current) return;
    setPos(computePanelPos(triggerRef.current, options.length));
    setActiveIndex(selectedIndex >= 0 ? selectedIndex : firstEnabled(options));
    setOpen(true);
  }, [disabled, options, selectedIndex]);

  // Close on outside interaction, scroll, and resize.
  React.useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: MouseEvent) => {
      const target = event.target as Node;
      if (panelRef.current?.contains(target) || triggerRef.current?.contains(target)) return;
      close();
    };
    // Close only on scrolls that can move the trigger (an ancestor scrolling),
    // because those detach the fixed panel from the field. Scrolls inside the
    // panel's own list — or anywhere else — must not close it.
    const onScroll = (event: Event) => {
      const trigger = triggerRef.current;
      if (trigger && event.target instanceof Node && event.target.contains(trigger)) close();
    };
    const onResize = () => close();
    document.addEventListener('mousedown', onPointerDown, true);
    window.addEventListener('scroll', onScroll, true);
    window.addEventListener('resize', onResize);
    return () => {
      document.removeEventListener('mousedown', onPointerDown, true);
      window.removeEventListener('scroll', onScroll, true);
      window.removeEventListener('resize', onResize);
    };
  }, [open, close]);

  // Focus the listbox when it opens and keep the active option in view.
  // Both must never scroll anything outside the panel: under the app zoom,
  // WKWebView's rect math can make focus()/scrollIntoView() scroll an ancestor
  // "to reveal" the panel, which the close-on-page-scroll listener would treat
  // as the page moving and close the panel right after it opened.
  React.useEffect(() => {
    if (open) panelRef.current?.focus({ preventScroll: true });
  }, [open]);
  React.useEffect(() => {
    if (!open || activeIndex < 0) return;
    const panel = panelRef.current;
    const node = panel?.querySelector<HTMLElement>(`[data-index="${activeIndex}"]`);
    if (!panel || !node) return;
    // Manual panel-relative scrolling (the panel is the options' offsetParent).
    const top = node.offsetTop;
    const bottom = top + node.offsetHeight;
    if (top < panel.scrollTop) panel.scrollTop = top;
    else if (bottom > panel.scrollTop + panel.clientHeight) panel.scrollTop = bottom - panel.clientHeight;
  }, [open, activeIndex]);

  function moveActive(delta: number) {
    setActiveIndex((current) => {
      const n = options.length;
      let next = current;
      for (let i = 0; i < n; i += 1) {
        next = (next + delta + n) % n;
        if (!options[next]?.disabled) return next;
      }
      return current;
    });
  }

  function commit(index: number) {
    const opt = options[index];
    if (!opt || opt.disabled) return;
    onValueChange(opt.value);
    close(true);
  }

  function onTriggerKeyDown(event: React.KeyboardEvent) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp' || event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      openPanel();
    }
  }

  function onListKeyDown(event: React.KeyboardEvent) {
    switch (event.key) {
      case 'ArrowDown': event.preventDefault(); moveActive(1); break;
      case 'ArrowUp': event.preventDefault(); moveActive(-1); break;
      case 'Home': event.preventDefault(); setActiveIndex(firstEnabled(options)); break;
      case 'End': event.preventDefault(); setActiveIndex(lastEnabled(options)); break;
      case 'Enter':
      case ' ': event.preventDefault(); commit(activeIndex); break;
      case 'Escape': event.preventDefault(); close(true); break;
      case 'Tab': close(); break;
      default: break;
    }
  }

  const panel = open && pos ? createPortal(
    <div
      ref={panelRef}
      role="listbox"
      id={listboxId}
      aria-label={ariaLabel}
      tabIndex={-1}
      onKeyDown={onListKeyDown}
      className="fixed z-[10000] min-w-[8rem] overflow-x-hidden overflow-y-auto overscroll-contain rounded-md border bg-popover p-1 text-popover-foreground shadow-md outline-none animate-in fade-in-0 zoom-in-95"
      style={{
        left: pos.left,
        width: pos.width,
        top: pos.top,
        bottom: pos.bottom,
        maxHeight: pos.maxHeight,
      }}
    >
      {options.map((opt, index) => {
        const isSelected = opt.value === value;
        const isActive = index === activeIndex;
        return (
          <div
            key={opt.value}
            role="option"
            data-index={index}
            aria-selected={isSelected}
            aria-disabled={opt.disabled || undefined}
            onMouseEnter={() => !opt.disabled && setActiveIndex(index)}
            onClick={() => commit(index)}
            className={cn(
              'relative flex w-full cursor-default items-center gap-2 rounded-sm py-1.5 pr-8 pl-2 text-sm outline-hidden select-none',
              isActive && !opt.disabled && 'bg-accent text-accent-foreground',
              opt.disabled && 'pointer-events-none opacity-50',
            )}
          >
            {isSelected && (
              <span className="absolute right-2 flex size-3.5 items-center justify-center">
                <span className="material-symbols-outlined text-[16px]" aria-hidden="true">check</span>
              </span>
            )}
            <span className="line-clamp-1">{opt.label}</span>
          </div>
        );
      })}
    </div>,
    document.body,
  ) : null;

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listboxId : undefined}
        aria-label={ariaLabel}
        disabled={disabled}
        onClick={() => (open ? close() : openPanel())}
        onKeyDown={onTriggerKeyDown}
        className={cn(
          "flex h-9 w-full items-center justify-between gap-2 rounded-md border border-input bg-input px-3 py-2 text-sm whitespace-nowrap shadow-xs transition-[color,box-shadow] outline-none hover:bg-muted focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
          !selected && 'text-muted-foreground',
          className,
        )}
      >
        <span className="line-clamp-1 text-left">{selected ? selected.label : (placeholder ?? '')}</span>
        <span className="material-symbols-outlined text-[16px] opacity-50" aria-hidden="true">expand_more</span>
      </button>
      {panel}
    </>
  );
}

function firstEnabled(options: ZoomSafeSelectOption[]): number {
  const i = options.findIndex((opt) => !opt.disabled);
  return i;
}

function lastEnabled(options: ZoomSafeSelectOption[]): number {
  for (let i = options.length - 1; i >= 0; i -= 1) {
    if (!options[i]?.disabled) return i;
  }
  return -1;
}
