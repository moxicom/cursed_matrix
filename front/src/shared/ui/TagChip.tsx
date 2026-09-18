import { cn } from '@/shared/lib/cn';

export interface TagChipProps {
  name: string;
  /** Rendered as a bordered chip with a remove button (task modal). */
  onRemove?: () => void;
  /** Plain `#tag` text, as shown on board cards. */
  plain?: boolean;
  className?: string;
}

export function TagChip({ name, onRemove, plain = false, className }: TagChipProps) {
  if (plain) {
    return <span className={cn('text-95 text-txt-tag', className)}>#{name}</span>;
  }

  return (
    <span
      className={cn(
        'inline-flex items-center gap-5 border border-line bg-bg-input px-6 py-3 text-10 text-txt-soft',
        className,
      )}
    >
      #{name}
      {onRemove !== undefined && (
        <button
          type="button"
          onClick={onRemove}
          aria-label={`remove ${name}`}
          className="border-0 bg-transparent p-0 text-10 leading-none text-txt-faint transition-colors hover:text-red"
        >
          ×
        </button>
      )}
    </span>
  );
}
