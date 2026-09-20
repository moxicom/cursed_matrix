import { cn } from '@/shared/lib/cn';

export interface LoadErrorProps {
  /** Already in the reader's language: see shared/api/messages. */
  message: string;
  onRetry?: () => void;
  retryLabel?: string;
  className?: string;
}

/**
 * Says that a screen's data did not arrive.
 *
 * Without it a failed read leaves the page looking merely empty, which reads
 * as "you have nothing here" rather than "we could not ask".
 */
export function LoadError({ message, onRetry, retryLabel, className }: LoadErrorProps) {
  return (
    <div
      role="alert"
      className={cn(
        'flex flex-wrap items-center gap-10 border border-red-border bg-bg-panel px-14 py-11',
        className,
      )}
    >
      <span className="text-11 tracking-t7 text-red-light">ERR</span>
      <span className="text-95 text-txt-dim">{message}</span>
      {onRetry !== undefined && (
        <button
          type="button"
          className="text-95 text-cyan underline-offset-4 hover:underline"
          onClick={onRetry}
        >
          {retryLabel ?? 'retry'}
        </button>
      )}
    </div>
  );
}
