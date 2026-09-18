import { forwardRef, type InputHTMLAttributes } from 'react';

import { cn } from '@/shared/lib/cn';

export type InputVariant = 'solid' | 'dashed' | 'bare';
export type InputSize = 'sm' | 'md' | 'lg';

export interface InputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> {
  variant?: InputVariant;
  inputSize?: InputSize;
}

const VARIANTS = {
  solid: 'bg-bg-input border border-line focus:border-cyan',
  dashed: 'bg-bg-input border border-dashed border-line focus:border-green',
  bare: 'bg-transparent border-0',
} satisfies Record<InputVariant, string>;

const SIZES = {
  sm: 'px-7 py-6 text-105',
  md: 'px-9 py-7 text-105',
  lg: 'px-10 py-8 text-13',
} satisfies Record<InputSize, string>;

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { variant = 'solid', inputSize = 'md', className, ...rest },
  ref,
) {
  return (
    <input
      ref={ref}
      className={cn(
        'w-full min-w-0 text-txt outline-none transition-colors',
        VARIANTS[variant],
        SIZES[inputSize],
        className,
      )}
      {...rest}
    />
  );
});
