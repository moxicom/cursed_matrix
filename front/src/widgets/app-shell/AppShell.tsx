import type { ReactNode } from 'react';

import { cn } from '@/shared/lib/cn';

export interface AppShellProps {
  /** The 46px AppHeader, or nothing on the landing page. */
  header?: ReactNode;
  children: ReactNode;
  /** Overlays rendered above the shell: task modal, search palette, paywall, toast. */
  overlays?: ReactNode;
  className?: string;
}

/**
 * Full-viewport application frame: fixed header, a single scroll-owning body.
 * Matches the design's root container (100vh, min-height 640px, no page scroll).
 */
export function AppShell({ header, children, overlays, className }: AppShellProps) {
  return (
    <div
      className={cn(
        'flex h-screen min-h-[640px] flex-col overflow-hidden bg-bg-base text-12 tracking-base',
        className,
      )}
    >
      {header}
      {children}
      {overlays}
    </div>
  );
}
