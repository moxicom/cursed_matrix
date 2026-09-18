import type { ButtonHTMLAttributes, ReactNode } from 'react';

import { cn } from '@/shared/lib/cn';

export type ButtonVariant = 'ghost' | 'primary' | 'violet' | 'danger' | 'solid' | 'dashed' | 'plain';
export type ButtonSize = 'xs' | 'sm' | 'md' | 'lg';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** Stretch to the container width (plan cards, sidebar reset). */
  block?: boolean;
  children?: ReactNode;
}

const VARIANTS = {
  ghost: 'bg-transparent border border-line text-txt-dim hover:border-line-hover hover:text-txt',
  primary:
    'bg-green-bg border border-green-border text-green-light hover:bg-green-bg-hover hover:border-green',
  violet: 'bg-violet-bg border border-violet-border text-violet-pale hover:border-violet',
  danger: 'bg-transparent border border-line text-txt-dim hover:text-red hover:border-red-border',
  solid: 'bg-bg-hover border border-line-strong text-txt hover:border-line-hover',
  dashed:
    'bg-transparent border border-dashed border-line text-txt-faint hover:text-green hover:border-green-border-dim',
  plain: 'bg-transparent border-0 text-txt-dim hover:text-txt',
} satisfies Record<ButtonVariant, string>;

const SIZES = {
  xs: 'px-6 py-3 text-9 tracking-t5',
  sm: 'px-9 py-4 text-105 tracking-t6',
  md: 'px-11 py-6 text-105 tracking-t5',
  lg: 'px-14 py-8 text-11 tracking-t7',
} satisfies Record<ButtonSize, string>;

export function Button({
  variant = 'ghost',
  size = 'sm',
  block = false,
  className,
  type = 'button',
  children,
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(
        'inline-flex items-center justify-center gap-7 whitespace-nowrap transition-colors',
        'disabled:cursor-not-allowed disabled:opacity-50',
        VARIANTS[variant],
        SIZES[size],
        block && 'w-full',
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  );
}
