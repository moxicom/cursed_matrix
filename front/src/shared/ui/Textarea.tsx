import { forwardRef, type TextareaHTMLAttributes } from 'react';

import { cn } from '@/shared/lib/cn';

export type TextareaProps = TextareaHTMLAttributes<HTMLTextAreaElement>;

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { className, rows = 4, ...rest },
  ref,
) {
  return (
    <textarea
      ref={ref}
      rows={rows}
      className={cn(
        'w-full resize-y border border-line bg-bg-input px-10 py-8 text-11 leading-[1.55] text-txt-soft outline-none transition-colors focus:border-cyan',
        className,
      )}
      {...rest}
    />
  );
});
