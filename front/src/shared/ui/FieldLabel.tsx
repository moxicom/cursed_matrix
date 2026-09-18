import type { ReactNode } from 'react';

import { cn } from '@/shared/lib/cn';

export interface FieldLabelProps {
  children: ReactNode;
  /** Right-aligned counter or summary, e.g. "2 / 3 done". */
  aside?: ReactNode;
  /** Id of the control this caption names — renders a real <label>. */
  htmlFor?: string;
  className?: string;
}

/** Small uppercase caption above a control: TITLE, DEADLINE, TAGS, LEGEND… */
export function FieldLabel({ children, aside, htmlFor, className }: FieldLabelProps) {
  const caption =
    htmlFor === undefined ? (
      <span className="text-9 tracking-t8 text-txt-label">{children}</span>
    ) : (
      <label htmlFor={htmlFor} className="text-9 tracking-t8 text-txt-label">
        {children}
      </label>
    );

  if (aside === undefined) {
    return <div className={cn('text-9 tracking-t8 text-txt-label', className)}>{caption}</div>;
  }

  return (
    <div className={cn('flex items-center justify-between gap-10', className)}>
      {caption}
      <span className="text-95 text-txt-faint">{aside}</span>
    </div>
  );
}
