import type { ReactNode } from 'react';

import { cn } from '@/shared/lib/cn';

export interface PanelProps {
  children: ReactNode;
  /** Bold uppercase heading rendered in the panel's own header row. */
  title?: ReactNode;
  /** Muted text on the right side of the header. */
  aside?: ReactNode;
  /** Accent stripe: 2px colored border on top or left (callouts, plan cards). */
  accent?: string;
  accentSide?: 'top' | 'left';
  surface?: 'panel' | 'raised' | 'alt';
  className?: string;
  bodyClassName?: string;
}

const SURFACES = {
  panel: 'bg-bg-panel',
  raised: 'bg-bg-raised',
  alt: 'bg-bg-alt',
} satisfies Record<'panel' | 'raised' | 'alt', string>;

export function Panel({
  children,
  title,
  aside,
  accent,
  accentSide = 'top',
  surface = 'panel',
  className,
  bodyClassName,
}: PanelProps) {
  return (
    <section
      className={cn('border border-line', SURFACES[surface], className)}
      style={
        accent === undefined
          ? undefined
          : accentSide === 'top'
            ? { borderTop: `2px solid ${accent}` }
            : { borderLeft: `2px solid ${accent}` }
      }
    >
      {title !== undefined && (
        <div className="flex flex-wrap items-center justify-between gap-12 border-b border-line-subtle px-14 py-10">
          <div className="text-11 font-bold tracking-t9 text-txt">{title}</div>
          {aside !== undefined && <div className="text-95 text-txt-faint">{aside}</div>}
        </div>
      )}
      <div className={bodyClassName}>{children}</div>
    </section>
  );
}
