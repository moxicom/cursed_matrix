import type { ReactNode } from 'react';

import { cn } from '@/shared/lib/cn';

export type BadgeTone = 'neutral' | 'green' | 'amber' | 'violet' | 'red' | 'cyan';

export interface BadgeProps {
  children: ReactNode;
  tone?: BadgeTone;
  /** Wrap the label in terminal brackets: [ ACTIVE ]. */
  bracketed?: boolean;
  className?: string;
  title?: string;
}

const TONES = {
  neutral: 'border-line text-txt-dim',
  green: 'border-green-border text-green-light',
  amber: 'border-amber-border-soft text-amber',
  violet: 'border-violet-border-on text-violet-light',
  red: 'border-red-border text-red-light',
  cyan: 'border-cyan-border text-cyan-chip',
} satisfies Record<BadgeTone, string>;

export function Badge({ children, tone = 'neutral', bracketed = false, className, title }: BadgeProps) {
  return (
    <span
      title={title}
      className={cn(
        'inline-flex items-center whitespace-nowrap border px-6 py-2 text-95 tracking-t6',
        TONES[tone],
        className,
      )}
    >
      {bracketed ? `[ ${String(children)} ]` : children}
    </span>
  );
}
