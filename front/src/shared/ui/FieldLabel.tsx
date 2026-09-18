import type { ReactNode } from 'react';

import { cn } from '@/shared/lib/cn';

export interface FieldLabelProps {
  children: ReactNode;
  /** Right-aligned counter or summary, e.g. "2 / 3 done". */
  aside?: ReactNode;
  className?: string;
}

/** Small uppercase caption above a control: TITLE, DEADLINE, TAGS, LEGEND… */
export function FieldLabel({ children, aside, className }: FieldLabelProps) {
  if (aside === undefined) {
    return (
      <div className={cn('text-9 tracking-t8 text-txt-label', className)}>{children}</div>
    );
  }
  return (
    <div className={cn('flex items-center justify-between gap-10', className)}>
      <span className="text-9 tracking-t8 text-txt-label">{children}</span>
      <span className="text-95 text-txt-faint">{aside}</span>
    </div>
  );
}
