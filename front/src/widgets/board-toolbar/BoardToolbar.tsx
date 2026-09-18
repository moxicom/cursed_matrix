import type { ReactNode } from 'react';

import { cn } from '@/shared/lib/cn';

export interface BoardToolbarProps {
  children: ReactNode;
  className?: string;
}

/**
 * 38px filter strip above the four board columns.
 * Never wider than the viewport: the flexible section inside it collapses its
 * own content instead of letting the strip scroll sideways.
 */
export function BoardToolbar({ children, className }: BoardToolbarProps) {
  return (
    <div
      className={cn(
        'flex h-38 w-full flex-none items-stretch overflow-hidden border-b border-line bg-bg-sunken',
        className,
      )}
    >
      {children}
    </div>
  );
}

export interface BoardToolbarSectionProps {
  /** Small uppercase caption: STATE, TAGS, COLOR, DEADLINE. */
  label?: string;
  children: ReactNode;
  gap?: 'tight' | 'normal';
  /** Takes the leftover width and clips its own content (the tags section). */
  grow?: boolean;
  className?: string;
}

export function BoardToolbarSection({
  label,
  children,
  gap = 'normal',
  grow = false,
  className,
}: BoardToolbarSectionProps) {
  return (
    <div
      className={cn(
        'flex items-center overflow-hidden border-r border-line-subtle px-12',
        grow ? 'min-w-0 flex-1' : 'flex-none',
        gap === 'tight' ? 'gap-2' : 'gap-5',
        className,
      )}
    >
      {label !== undefined && (
        <span className="mr-7 whitespace-nowrap text-10 tracking-t8 text-txt-label">{label}</span>
      )}
      {children}
    </div>
  );
}

/** Right-aligned tail of the toolbar: counters and the reset button. */
export function BoardToolbarTail({ children, className }: BoardToolbarProps) {
  return (
    <div
      className={cn(
        'flex flex-none items-center justify-end gap-12 whitespace-nowrap px-14',
        className,
      )}
    >
      {children}
    </div>
  );
}
