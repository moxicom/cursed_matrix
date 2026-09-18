import { useEffect, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

import { cn } from '@/shared/lib/cn';

export type ModalSize = 'sm' | 'md' | 'lg';
export type ModalPlacement = 'center' | 'top';
export type ModalLayer = 'search' | 'modal' | 'pay';

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  size?: ModalSize;
  placement?: ModalPlacement;
  /** Controls both the backdrop opacity and the z-index pair, per the design. */
  layer?: ModalLayer;
  /** 2px colored stripe on top of the panel (paywall uses amber). */
  accent?: string;
  /** Override the panel border color (paywall uses a warm border). */
  borderColor?: string;
  surface?: 'raised' | 'panel';
  className?: string;
  /** Disable closing on backdrop click / Escape. */
  persistent?: boolean;
  /** Accessible name of the dialog — required for `role="dialog"` to be useful. */
  ariaLabel: string;
}

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/** Nested dialogs must not un-inert the app when only one of them closes. */
let inertDepth = 0;

const SIZES = {
  sm: 'w-[min(520px,94vw)]',
  md: 'w-[min(640px,92vw)] max-h-[66vh]',
  lg: 'w-[min(900px,95vw)] max-h-[88vh]',
} satisfies Record<ModalSize, string>;

const LAYERS = {
  search: { backdrop: 'z-search bg-overlay-search', panel: 'z-search-panel' },
  modal: { backdrop: 'z-modal bg-overlay-modal', panel: 'z-modal-panel' },
  pay: { backdrop: 'z-pay bg-overlay-pay', panel: 'z-pay-panel' },
} satisfies Record<ModalLayer, { backdrop: string; panel: string }>;

export function Modal({
  open,
  onClose,
  children,
  size = 'lg',
  placement = 'center',
  layer = 'modal',
  accent,
  borderColor,
  surface = 'raised',
  className,
  persistent = false,
  ariaLabel,
}: ModalProps) {
  const panelRef = useRef<HTMLDivElement>(null);

  // make the application behind the dialog inert, so Tab and screen-reader
  // navigation cannot reach the board underneath an "aria-modal" dialog
  useEffect(() => {
    if (!open) return undefined;
    const root = document.getElementById('root');
    inertDepth += 1;
    root?.setAttribute('inert', '');
    return () => {
      inertDepth -= 1;
      if (inertDepth === 0) root?.removeAttribute('inert');
    };
  }, [open]);

  // move focus into the dialog, keep Tab inside it, restore it on close
  useEffect(() => {
    if (!open) return undefined;
    const previous = document.activeElement as HTMLElement | null;
    const panel = panelRef.current;
    const first = panel?.querySelector<HTMLElement>(FOCUSABLE);
    (first ?? panel)?.focus();

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Tab' || !panel) return;
      const items = [...panel.querySelectorAll<HTMLElement>(FOCUSABLE)].filter(
        (element) => element.offsetParent !== null,
      );
      if (items.length === 0) return;
      const firstItem = items[0] as HTMLElement;
      const lastItem = items[items.length - 1] as HTMLElement;
      const active = document.activeElement;

      if (!event.shiftKey && active === lastItem) {
        event.preventDefault();
        firstItem.focus();
      } else if (event.shiftKey && (active === firstItem || active === panel)) {
        event.preventDefault();
        lastItem.focus();
      }
    };

    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      previous?.focus?.();
    };
  }, [open]);

  useEffect(() => {
    if (!open || persistent) return undefined;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, persistent, onClose]);

  if (!open) return null;

  const style: Record<string, string> = {};
  if (accent !== undefined) style['borderTop'] = `2px solid ${accent}`;
  if (borderColor !== undefined) style['borderColor'] = borderColor;

  return createPortal(
    <>
      <div
        aria-hidden="true"
        onClick={persistent ? undefined : onClose}
        className={cn('fixed inset-0', LAYERS[layer].backdrop)}
      />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label={ariaLabel}
        tabIndex={-1}
        className={cn(
          'fixed left-1/2 flex -translate-x-1/2 flex-col border border-line-strong',
          surface === 'raised' ? 'bg-bg-raised' : 'bg-bg-panel',
          placement === 'center' ? 'top-1/2 -translate-y-1/2' : 'top-54',
          SIZES[size],
          LAYERS[layer].panel,
          className,
        )}
        style={style}
      >
        {children}
      </div>
    </>,
    document.body,
  );
}

export interface ModalSectionProps {
  children: ReactNode;
  className?: string;
}

export function ModalHeader({ children, className }: ModalSectionProps) {
  return (
    <div
      className={cn(
        'flex flex-none flex-wrap items-center gap-10 border-b border-line bg-bg-input px-14 py-10',
        className,
      )}
    >
      {children}
    </div>
  );
}

export function ModalBody({ children, className }: ModalSectionProps) {
  return <div className={cn('min-h-0 flex-1 overflow-y-auto', className)}>{children}</div>;
}

export function ModalFooter({ children, className }: ModalSectionProps) {
  return (
    <div
      className={cn(
        'flex flex-none flex-wrap items-center gap-8 border-t border-line bg-bg-input px-14 py-11',
        className,
      )}
    >
      {children}
    </div>
  );
}
