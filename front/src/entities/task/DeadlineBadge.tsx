import { useT } from '@/shared/i18n';
import { DEADLINE_TONE_CLASS, deadlineInfo } from '@/shared/lib/deadline';
import { cn } from '@/shared/lib/cn';
import type { Task } from '@/shared/types/domain';

export interface DeadlineBadgeProps {
  task: Task;
  /** Compact form drops the border — used inside subtask rows. */
  bare?: boolean;
  className?: string;
}

export function DeadlineBadge({ task, bare = false, className }: DeadlineBadgeProps) {
  const t = useT();
  const info = deadlineInfo(task, t.completed);

  if (bare) {
    return (
      <span className={cn('whitespace-nowrap text-9', DEADLINE_TONE_CLASS[info.tone], className)}>
        {info.text === '—' ? '' : info.text}
      </span>
    );
  }

  return (
    <span
      className={cn(
        'whitespace-nowrap border px-5 py-px text-95 tracking-t5',
        DEADLINE_TONE_CLASS[info.tone],
        className,
      )}
    >
      {info.text}
    </span>
  );
}
