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
  const xp = typeof event.detail.xp === 'number' ? event.detail.xp : null;

  return (
    <div className="flex flex-wrap items-baseline gap-11 border-b border-line-faint px-14 py-7">
      <span className="w-86 flex-none text-95 text-txt-label">{time}</span>
      <span className={cn('w-186 flex-none text-10 tracking-t5', TONE[event.type] ?? 'text-txt-dim')}>
        {event.type}
      </span>
      <span className="min-w-120 flex-1 break-words text-10 text-txt-dim">{describe(event)}</span>
      <span className="text-95 text-violet-dim">{xp === null ? '' : `+${xp} XP`}</span>
    </div>
  );
}

/**
 * Says what happened from the parameters the server sent.
 *
 * The server never sends a sentence — it names the task and gives values — so
 * whatever appears here is the client's, and it changes with the language
 * without the server knowing which one is read.
 */
function describe(event: ActivityEvent): string {
  if (event.taskTitle) return event.taskTitle;

  const { detail } = event;
  if (typeof detail.code === 'string') return detail.code;
  // Both halves are checked: the pair is written together, but this reads a
  // loose bag of values and one missing would print "LVL undefined".
  if (typeof detail.toLevel === 'number') {
    return typeof detail.fromLevel === 'number'
      ? `LVL ${String(detail.fromLevel)} → ${String(detail.toLevel)}`
      : `LVL ${String(detail.toLevel)}`;
  }
  if (typeof detail.currentStreak === 'number') return `${String(detail.currentStreak)}D`;
  return '';
}
