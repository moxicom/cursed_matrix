import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

import { cn } from '@/shared/lib/cn';

export interface PopoverProps {
  open: boolean;
  onClose: () => void;
  /** Element the panel is anchored to. */
  anchor: HTMLElement | null;
  children: ReactNode;
  /** Panel width in px. */
  width?: number;
  align?: 'start' | 'end';
  className?: string;
}

/**
 * Anchored panel rendered in a portal — toolbars use `overflow-x: auto`, which
 * would clip an absolutely positioned child, so the panel is positioned fixed
 * from the anchor's rect and flipped when it would leave the viewport.
 */
export function Popover({
  open,
  onClose,
  anchor,
  children,
  width = 260,
  align = 'start',
  className,
}: PopoverProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<{ top: number; left: number } | null>(null);

  useLayoutEffect(() => {
    // nothing to place while closed; the component renders null anyway
    if (!open || !anchor) return undefined;

    const place = () => {
      const rect = anchor.getBoundingClientRect();
      const panelHeight = panelRef.current?.offsetHeight ?? 320;
      const gap = 4;

      let left = align === 'end' ? rect.right - width : rect.left;
      left = Math.max(8, Math.min(left, window.innerWidth - width - 8));

      const below = rect.bottom + gap;
      const top =
        below + panelHeight > window.innerHeight - 8
          ? Math.max(8, rect.top - gap - panelHeight)
          : below;

      setPosition({ top, left });
    };

    place();
    window.addEventListener('resize', place);
    window.addEventListener('scroll', place, true);
    return () => {
      window.removeEventListener('resize', place);
      window.removeEventListener('scroll', place, true);
    };
  }, [open, anchor, width, align]);

  useEffect(() => {
    if (!open) return undefined;

    const onPointerDown = (event: MouseEvent) => {
      const target = event.target as Node;
      if (panelRef.current?.contains(target)) return;
      if (anchor?.contains(target)) return;
      onClose();
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation();
        onClose();
      }
    };

    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open, anchor, onClose]);

  if (!open) return null;

  return createPortal(
    <div
      ref={panelRef}
      className={cn(
        'fixed z-pay-panel flex flex-col border border-line-strong bg-bg-panel',
        position === null && 'invisible',
        className,
      )}
      style={{ width, top: position?.top ?? 0, left: position?.left ?? 0 }}
    >
      {children}
    </div>,
    document.body,
  );
}
