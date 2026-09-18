import type { ReactNode } from 'react';

import { cn } from '@/shared/lib/cn';

export type PageWidth = 'activity' | 'page' | 'settings' | 'upgrade' | 'landing';

export interface PageContainerProps {
  children: ReactNode;
  width?: PageWidth;
  /** Vertical rhythm between sections (18px in the design). */
  stacked?: boolean;
  className?: string;
  innerClassName?: string;
}

const WIDTHS = {
  activity: 'max-w-activity',
  page: 'max-w-page',
  settings: 'max-w-settings',
  upgrade: 'max-w-upgrade',
  landing: 'max-w-landing',
} satisfies Record<PageWidth, string>;

/** Scrollable page body used by Activity, Leaderboard, Profile, Settings and Pricing. */
export function PageContainer({
  children,
  width = 'page',
  stacked = true,
  className,
  innerClassName,
}: PageContainerProps) {
  return (
    <div className={cn('min-h-0 flex-1 overflow-y-auto', className)}>
      <div
        className={cn(
          'mx-auto px-22 pb-48 pt-20',
          WIDTHS[width],
          stacked && 'flex flex-col gap-18',
          innerClassName,
        )}
      >
        {children}
      </div>
    </div>
  );
}
