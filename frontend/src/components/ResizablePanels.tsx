import { useState, useRef, useEffect, useCallback } from 'react';

interface DragHandleProps {
  onMouseDown: (e: React.MouseEvent) => void;
  /** Tailwind visibility + display classes. Defaults to 'hidden xl:flex'. */
  visibilityClass?: string;
}

/** Thin vertical bar rendered between two resizable panels. */
export function DragHandle({ onMouseDown, visibilityClass = 'hidden xl:flex' }: DragHandleProps) {
  return (
    <div
      className={`${visibilityClass} w-1.5 shrink-0 cursor-col-resize select-none items-center justify-center hover:bg-primary/20 active:bg-primary/30 transition-colors group`}
      onMouseDown={onMouseDown}
      title="Drag to resize"
    >
      <div className="h-8 w-px rounded-full bg-border/60 group-hover:bg-primary/60 transition-colors" />
    </div>
  );
}

interface UseResizablePanelsOptions {
  defaultLeft: number;
  defaultRight: number;
  /** Minimum side-panel width in px. @default 160 */
  minWidth?: number;
  /**
   * Viewport width threshold (px) at which the horizontal layout is active.
   * Should match the Tailwind breakpoint used on the container's flex-row class.
   * @default 1280  (xl)
   */
  breakpoint?: number;
}

export interface UseResizablePanelsResult {
  leftStyle: React.CSSProperties | undefined;
  rightStyle: React.CSSProperties | undefined;
  /** True when the viewport is wide enough for the side-by-side layout. */
  isWide: boolean;
  startLeft: (e: React.MouseEvent) => void;
  startRight: (e: React.MouseEvent) => void;
}

/**
 * Manages drag-to-resize state for a 3-column studio layout.
 *
 * Usage:
 *   const { leftStyle, rightStyle, startLeft, startRight } = useResizablePanels({ defaultLeft: 360, defaultRight: 320 });
 *
 *   // Change the container from a CSS grid to a flex row:
 *   <div className="flex min-h-0 flex-1 flex-col xl:flex-row">
 *     <aside style={leftStyle}>...</aside>
 *     <DragHandle onMouseDown={startLeft} />
 *     <main className="flex-1 min-w-0">...</main>
 *     <DragHandle onMouseDown={startRight} />
 *     <aside style={rightStyle}>...</aside>
 *   </div>
 */
export function useResizablePanels({
  defaultLeft,
  defaultRight,
  minWidth = 160,
  breakpoint = 1280,
}: UseResizablePanelsOptions): UseResizablePanelsResult {
  const [leftW, setLeftW] = useState(defaultLeft);
  const [rightW, setRightW] = useState(defaultRight);
  const [isWide, setIsWide] = useState(
    () => typeof window !== 'undefined' && window.innerWidth >= breakpoint,
  );

  const dragRef = useRef<{ side: 'left' | 'right'; startX: number; startW: number } | null>(null);

  useEffect(() => {
    const check = () => setIsWide(window.innerWidth >= breakpoint);
    window.addEventListener('resize', check);
    return () => window.removeEventListener('resize', check);
  }, [breakpoint]);

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      const drag = dragRef.current;
      if (!drag) return;
      const dx = e.clientX - drag.startX;
      if (drag.side === 'left') {
        setLeftW(Math.max(minWidth, drag.startW + dx));
      } else {
        setRightW(Math.max(minWidth, drag.startW - dx));
      }
    };
    const onUp = () => { dragRef.current = null; };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    return () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
  }, [minWidth]);

  const startLeft = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    dragRef.current = { side: 'left', startX: e.clientX, startW: leftW };
  }, [leftW]);

  const startRight = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    dragRef.current = { side: 'right', startX: e.clientX, startW: rightW };
  }, [rightW]);

  return {
    leftStyle: isWide ? { width: leftW, flexShrink: 0 } : undefined,
    rightStyle: isWide ? { width: rightW, flexShrink: 0 } : undefined,
    isWide,
    startLeft,
    startRight,
  };
}
