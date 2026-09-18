import type { ReactNode } from 'react';

import { cn } from '@/shared/lib/cn';

export interface GraphSidebarProps {
  children: ReactNode;
  className?: string;
}

/** 238px filter rail on the left of the graph canvas. */
export function GraphSidebar({ children, className }: GraphSidebarProps) {
  return (
    <aside
      className={cn(
        'flex w-238 flex-none flex-col overflow-y-auto border-r border-line bg-bg-sunken',
        className,
      )}
    >
      {children}
    </aside>
  );
}

export interface GraphSidebarSectionProps {
  label?: string;
  children: ReactNode;
  /** Drops the bottom divider — used by the last section. */
  last?: boolean;
  className?: string;
}

export function GraphSidebarSection({
  label,
  children,
  last = false,
  className,
}: GraphSidebarSectionProps) {
  return (
    <div className={cn('px-13 py-11', !last && 'border-b border-line-subtle', className)}>
      {label !== undefined && (
        <div className="mb-7 text-95 tracking-t8 text-txt-label">{label}</div>
      )}
      {children}
    </div>
  );
}

export interface GraphSidebarHeaderProps {
  title: string;
  stats: string;
}

export function GraphSidebarHeader({ title, stats }: GraphSidebarHeaderProps) {
  return (
    <div className="border-b border-line-subtle px-13 py-11">
      <div className="text-11 font-bold tracking-t9 text-txt">{title}</div>
      <div className="mt-4 text-95 text-txt-faint">{stats}</div>
    </div>
  );
}
