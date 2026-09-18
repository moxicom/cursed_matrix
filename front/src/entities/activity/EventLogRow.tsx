import { cn } from '@/shared/lib/cn';
import type { ActivityEvent } from '@/shared/types/domain';

export interface EventLogRowProps {
  event: ActivityEvent;
}

const TONE: Record<string, string> = {
  TASK_COMPLETED: 'text-green',
  SUBTASK_COMPLETED: 'text-green',
  TASK_LINKED: 'text-cyan',
  STREAK_EXTENDED: 'text-amber',
  ACHIEVEMENT_UNLOCKED: 'text-amber',
  LEVEL_UP: 'text-violet',
};

export function EventLogRow({ event }: EventLogRowProps) {
  const time = event.occurredAt.slice(11, 19);

  return (
    <div className="flex flex-wrap items-baseline gap-11 border-b border-line-faint px-14 py-7">
      <span className="w-86 flex-none text-95 text-txt-label">{time}</span>
      <span className={cn('w-186 flex-none text-10 tracking-t5', TONE[event.type] ?? 'text-txt-dim')}>
        {event.type}
      </span>
      <span className="min-w-120 flex-1 break-words text-10 text-txt-dim">{event.detail}</span>
      <span className="text-95 text-violet-dim">{event.xp === null ? '' : `+${event.xp} XP`}</span>
    </div>
  );
}
