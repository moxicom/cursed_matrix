import type { ButtonHTMLAttributes } from 'react';

import { cn } from '@/shared/lib/cn';

export interface CheckboxProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onChange'> {
  checked: boolean;
  onChange?: (next: boolean) => void;
  size?: 'sm' | 'md';
}

/** Terminal checkbox: `[×]` / `[ ]`. Glyphs come straight from the design. */
export function Checkbox({
  checked,
  onChange,
  size = 'md',
  className,
  type = 'button',
  onClick,
  ...rest
}: CheckboxProps) {
  return (
    <button
      type={type}
      role="checkbox"
      aria-checked={checked}
      onClick={(event) => {
        event.stopPropagation();
        onClick?.(event);
        onChange?.(!checked);
      }}
      className={cn(
        'flex flex-none items-center justify-center whitespace-nowrap border-0 bg-transparent leading-none transition-colors hover:text-green',
        size === 'md' ? 'h-20 w-24 text-115' : 'h-19 w-22 text-11',
        checked ? 'text-green' : 'text-txt-faint',
        className,
      )}
      {...rest}
    >
      {checked ? '[×]' : '[ ]'}
    </button>
  );
}
