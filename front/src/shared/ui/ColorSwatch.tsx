import type { ButtonHTMLAttributes } from 'react';

import { cn } from '@/shared/lib/cn';

export type ColorSwatchSize = 'sm' | 'md' | 'lg';

export interface ColorSwatchProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'color'> {
  /** Hex value, or null for the NONE option (rendered transparent with a ∅ mark). */
  color: string | null;
  selected?: boolean;
  size?: ColorSwatchSize;
}

const SIZES = {
  sm: 'h-16 w-16 text-9', // board toolbar
  md: 'h-18 w-18 text-9', // graph sidebar
  lg: 'h-24 w-24 text-10', // task modal
} satisfies Record<ColorSwatchSize, string>;

export function ColorSwatch({
  color,
  selected = false,
  size = 'sm',
  className,
  type = 'button',
  style,
  ...rest
}: ColorSwatchProps) {
  return (
    <button
      // eslint-disable-next-line react/button-has-type
      type={type}
      aria-pressed={selected}
      className={cn(
        'flex flex-none items-center justify-center border p-0 leading-none text-txt-faint transition-colors hover:border-txt',
        selected ? 'border-txt' : 'border-line-strong',
        SIZES[size],
        className,
      )}
      style={{ background: color ?? 'transparent', ...style }}
      {...rest}
    >
      {color === null ? '∅' : ''}
    </button>
  );
}
