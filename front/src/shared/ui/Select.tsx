import { forwardRef, type SelectHTMLAttributes } from 'react';

import { cn } from '@/shared/lib/cn';

export interface SelectOption {
  value: string;
  label: string;
}

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  options: ReadonlyArray<SelectOption>;
  /** Non-selectable first entry, e.g. "+ link another task…". */
  placeholder?: string;
  variant?: 'solid' | 'dashed';
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { options, placeholder, variant = 'solid', className, ...rest },
  ref,
) {
  return (
    <select
      ref={ref}
      className={cn(
        'h-24 cursor-pointer bg-bg-input px-6 text-105 text-txt outline-none transition-colors',
        variant === 'dashed' ? 'border border-dashed border-line' : 'border border-line',
        className,
      )}
      {...rest}
    >
      {placeholder !== undefined && <option value="">{placeholder}</option>}
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  );
});
