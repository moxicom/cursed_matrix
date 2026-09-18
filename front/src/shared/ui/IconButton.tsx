import type { ButtonHTMLAttributes, ReactNode } from 'react';

import { cn } from '@/shared/lib/cn';

export type IconButtonSize = 'sm' | 'md' | 'lg';
export type IconButtonTone = 'default' | 'green' | 'danger' | 'cyan';

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  size?: IconButtonSize;
  tone?: IconButtonTone;
  /** Drop the border — used for inline glyph buttons inside cards. */
  bare?: boolean;
  children?: ReactNode;
}

const SIZES = {
  sm: 'h-22 w-22 text-9',
  md: 'h-24 w-24 text-11',
  lg: 'h-26 w-26 text-11',
} satisfies Record<IconButtonSize, string>;

const TONES = {
  default: 'text-txt-faint hover:text-txt hover:border-line-hover',
  green: 'text-txt hover:text-green hover:border-green',
  danger: 'text-txt-dim hover:text-red hover:border-red-border',
  cyan: 'text-cyan hover:border-cyan',
} satisfies Record<IconButtonTone, string>;

export function IconButton({
  size = 'sm',
  tone = 'default',
  bare = false,
  className,
  type = 'button',
  children,
  ...rest
}: IconButtonProps) {
  return (
    <button
      // eslint-disable-next-line react/button-has-type
      type={type}
      className={cn(
        'inline-flex flex-none items-center justify-center leading-none transition-colors',
        bare ? 'border-0 bg-transparent' : 'border border-line-subtle bg-transparent',
        SIZES[size],
        TONES[tone],
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  );
}
