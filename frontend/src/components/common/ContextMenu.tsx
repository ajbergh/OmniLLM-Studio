import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { createPortal } from 'react-dom';

export interface ContextMenuItem {
  label: string;
  action: () => void;
  icon?: ReactNode;
  shortcut?: string;
  disabled?: boolean;
  danger?: boolean;
}

export type ContextMenuEntry = ContextMenuItem | 'divider';

export interface ContextMenuPosition {
  x: number;
  y: number;
}

/**
 * Tracks open/close state for a ContextMenu and carries a per-invocation
 * context value (e.g. the right-clicked asset or clip id).
 *
 * `openAt(x, y, context)` supports keyboard invocation (Shift+F10) where no
 * pointer event exists — pass the target element's bounding-rect corner.
 */
export function useContextMenu<T = undefined>() {
  const [state, setState] = useState<{ position: ContextMenuPosition; context: T } | null>(null);
  const open = useCallback((event: { clientX: number; clientY: number; preventDefault: () => void; stopPropagation?: () => void }, context: T) => {
    event.preventDefault();
    event.stopPropagation?.();
    setState({ position: { x: event.clientX, y: event.clientY }, context });
  }, []);
  const openAt = useCallback((x: number, y: number, context: T) => {
    setState({ position: { x, y }, context });
  }, []);
  const close = useCallback(() => setState(null), []);
  return { menu: state, open, openAt, close };
}

/**
 * Portal-rendered context menu: viewport-aware placement, keyboard navigation
 * (arrows/Home/End/Enter/Escape), click-outside/scroll/resize close, and
 * `menu`/`menuitem` roles. Safe inside scroll and overflow containers.
 */
export function ContextMenu({ position, items, onClose }: {
  position: ContextMenuPosition;
  items: ContextMenuEntry[];
  onClose: () => void;
}) {
  const menuRef = useRef<HTMLDivElement | null>(null);
  const [placed, setPlaced] = useState<ContextMenuPosition>(position);
  const [activeIndex, setActiveIndex] = useState<number>(-1);

  const actionable = items
    .map((item, index) => ({ item, index }))
    .filter((entry): entry is { item: ContextMenuItem; index: number } => entry.item !== 'divider' && !(entry.item as ContextMenuItem).disabled);

  // Clamp to the viewport once the menu has a measurable size; flip above /
  // left of the cursor when it would overflow.
  useLayoutEffect(() => {
    const node = menuRef.current;
    if (!node) return;
    const rect = node.getBoundingClientRect();
    let x = position.x;
    let y = position.y;
    if (x + rect.width > window.innerWidth - 8) x = Math.max(8, position.x - rect.width);
    if (y + rect.height > window.innerHeight - 8) y = Math.max(8, Math.min(position.y - rect.height, window.innerHeight - rect.height - 8));
    setPlaced({ x, y });
  }, [position]);

  useEffect(() => {
    const onPointerDown = (event: PointerEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) onClose();
    };
    const onScroll = (event: Event) => {
      if (!menuRef.current?.contains(event.target as Node)) onClose();
    };
    const onResize = () => onClose();
    window.addEventListener('pointerdown', onPointerDown, true);
    window.addEventListener('scroll', onScroll, true);
    window.addEventListener('resize', onResize);
    return () => {
      window.removeEventListener('pointerdown', onPointerDown, true);
      window.removeEventListener('scroll', onScroll, true);
      window.removeEventListener('resize', onResize);
    };
  }, [onClose]);

  useEffect(() => {
    menuRef.current?.focus();
  }, []);

  const onKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      onClose();
      return;
    }
    if (actionable.length === 0) return;
    const currentPos = actionable.findIndex((entry) => entry.index === activeIndex);
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setActiveIndex(actionable[(currentPos + 1) % actionable.length].index);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActiveIndex(actionable[(currentPos - 1 + actionable.length) % actionable.length].index);
    } else if (event.key === 'Home') {
      event.preventDefault();
      setActiveIndex(actionable[0].index);
    } else if (event.key === 'End') {
      event.preventDefault();
      setActiveIndex(actionable[actionable.length - 1].index);
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      const entry = actionable.find((candidate) => candidate.index === activeIndex);
      if (entry) {
        onClose();
        entry.item.action();
      }
    }
  };

  return createPortal(
    <div
      ref={menuRef}
      role="menu"
      tabIndex={-1}
      className="fixed z-[100] min-w-44 max-w-64 rounded-md border border-border bg-surface p-1 shadow-xl outline-none"
      style={{ left: placed.x, top: placed.y }}
      onKeyDown={onKeyDown}
      onPointerDown={(event) => event.stopPropagation()}
      onContextMenu={(event) => event.preventDefault()}
    >
      {items.map((item, index) =>
        item === 'divider' ? (
          <div key={`divider-${index}`} role="separator" className="my-1 h-px bg-border" />
        ) : (
          <button
            key={`${item.label}-${index}`}
            role="menuitem"
            disabled={item.disabled}
            aria-disabled={item.disabled || undefined}
            aria-label={item.label}
            aria-keyshortcuts={item.shortcut}
            className={`flex w-full items-center gap-2 rounded px-2 py-1 text-left text-[11px] ${
              item.danger ? 'text-red-400 hover:bg-red-500/10 hover:text-red-300' : 'text-text-secondary hover:bg-surface-alt hover:text-text'
            } ${index === activeIndex ? (item.danger ? 'bg-red-500/10 text-red-300' : 'bg-surface-alt text-text') : ''} disabled:cursor-not-allowed disabled:opacity-40`}
            onPointerEnter={() => setActiveIndex(index)}
            onClick={() => {
              onClose();
              item.action();
            }}
          >
            {item.icon && <span className="shrink-0 text-text-muted" aria-hidden="true">{item.icon}</span>}
            <span className="min-w-0 flex-1 truncate">{item.label}</span>
            {item.shortcut && <span className="shrink-0 font-mono text-[9px] text-text-muted" aria-hidden="true">{item.shortcut}</span>}
          </button>
        ),
      )}
    </div>,
    document.body,
  );
}
