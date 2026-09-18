import { cn } from '@/shared/lib/cn';

export interface KeyHintProps {
  children: string;
  /** Bordered variant — the ESC button inside the search palette. */
  bordered?: boolean;
  className?: string;
}

export function KeyHint({ children, bordered = false, className }: KeyHintProps) {
  return (
    <kbd
      className={cn(
        'whitespace-nowrap font-mono not-italic',
        bordered ? 'border border-line px-6 py-3 text-9 text-txt-faint' : 'text-10 text-txt-label',
        className,
      )}
    >
      {children}
    </kbd>
  );
}
