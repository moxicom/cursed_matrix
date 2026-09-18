import type { ReactNode } from 'react';

import { Button } from './Button';
import { cn } from '@/shared/lib/cn';

export interface EmptyStateProps {
  /** Multiline ASCII art shown above the message. */
  art?: string;
  message: string;
  actionLabel?: string;
  onAction?: () => void;
  children?: ReactNode;
  className?: string;
}

export function EmptyState({
  art,
  message,
  actionLabel,
  onAction,
  children,
  className,
}: EmptyStateProps) {
  return (
    <div className={cn('mt-20 flex flex-col items-center gap-10', className)}>
      {art !== undefined && (
        <pre className="m-0 text-10 leading-[1.35] text-[#2a2f33]">{art}</pre>
      )}
      <span className="text-center text-95 tracking-t6 text-txt-faint">{message}</span>
      {actionLabel !== undefined && (
        <Button variant="dashed" size="md" onClick={onAction}>
          {actionLabel}
        </Button>
      )}
      {children}
    </div>
  );
}
