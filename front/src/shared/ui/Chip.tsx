import type { ButtonHTMLAttributes, ReactNode } from 'react';

import { cn } from '@/shared/lib/cn';

/**
 * Filter chip. The design uses two active palettes:
 *  - `green` for state filters and on/off toggles (chip() in the source);
 *  - `cyan` for tags, quadrants, topology and ranking periods (chipB()).
 */
export type ChipTone = 'green' | 'cyan';
export type ChipSize = 'xs' | 'sm' | 'md';

export interface ChipProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  active?: boolean;
  tone?: ChipTone;
  size?: ChipSize;
  children?: ReactNode;
}

const ACTIVE = {
  green: 'bg-green-bg border-green-border text-green-light',
  cyan: 'bg-cyan-bg border-cyan-border text-cyan-chip',
} satisfies Record<ChipTone, string>;

const SIZES = {
  xs: 'px-6 py-3 text-10',
  sm: 'px-7 py-3 text-105',
  md: 'px-9 py-4 text-105 tracking-t6',
} satisfies Record<ChipSize, string>;

export function Chip({
  active = false,
  tone = 'green',
  size = 'sm',
  className,
  type = 'button',
  children,
  ...rest
}: ChipProps) {
  return (
    <button
      type={type}
      aria-pressed={active}
      className={cn(
        'inline-flex items-center gap-7 whitespace-nowrap border transition-colors',
        active
          ? ACTIVE[tone]
          : 'border-line bg-transparent text-txt-dim hover:border-line-hover',
        SIZES[size],
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  );
}
