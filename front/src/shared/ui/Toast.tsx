import { createPortal } from 'react-dom';

import { cn } from '@/shared/lib/cn';

export interface ToastProps {
  /** Machine code, e.g. QUEST_RESOLVED — stays language-independent. */
  code: string;
  detail?: string;
  tone?: 'green' | 'amber' | 'red';
  className?: string;
}

const TONES = {
  green: 'border-green-border text-green-light',
  amber: 'border-amber-border text-amber',
  red: 'border-red-border text-red-light',
} satisfies Record<'green' | 'amber' | 'red', string>;

export function Toast({ code, detail, tone = 'green', className }: ToastProps) {
  return createPortal(
    <div
      role="status"
      className={cn(
        'fixed bottom-18 left-1/2 z-toast flex -translate-x-1/2 items-center gap-10 border bg-bg-panel px-14 py-9',
        TONES[tone],
        className,
      )}
    >
      <span className="text-11 tracking-t7">{code}</span>
      {detail !== undefined && <span className="text-10 text-txt-dim">{detail}</span>}
    </div>,
    document.body,
  );
}
