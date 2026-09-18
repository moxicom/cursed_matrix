import { DeadlineBadge } from './DeadlineBadge';
import { TASK_COLOR_BY_ID } from '@/shared/config/domain';
import { useT } from '@/shared/i18n';
import { cn } from '@/shared/lib/cn';
import { xpForTask } from '@/shared/lib/progression';
import type { Quadrant, Task } from '@/shared/types/domain';
import { Checkbox } from '@/shared/ui';

export interface SubtaskRowProps {
  subtask: Task;
  /** Quadrant inherited from the parent — drives the XP figure. */
  quadrant: Quadrant;
  last: boolean;
  dragging?: boolean;
  onToggle: () => void;
  onOpen: () => void;
  onDragStart?: () => void;
  onDragEnd?: () => void;
}

export function SubtaskRow({
  subtask,
  quadrant,
  last,
  dragging = false,
  onToggle,
  onOpen,
  onDragStart,
  onDragEnd,
}: SubtaskRowProps) {
  const t = useT();
  const done = subtask.status === 'COMPLETED';
  const xp = done ? (subtask.xpAwarded ?? 0) : xpForTask(quadrant, true);
  const color = TASK_COLOR_BY_ID[subtask.color].hex ?? '#2e343a';

  return (
    <div className="mt-3 flex items-stretch">
      <span className="w-12 flex-none pt-7 text-center text-11 text-txt-branch">
        {last ? '└' : '├'}
      </span>
      {/* clicking the row is a mouse shortcut; the keyboard path is the title
          button inside, so the row itself must not become a second tab stop */}
      {/* eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-noninteractive-element-interactions */}
      <article
        draggable
        title={t.subDragHint}
        onDragStart={onDragStart}
        onDragEnd={onDragEnd}
        onClick={onOpen}
        style={{ borderLeft: `2px solid ${color}`, opacity: dragging ? 0.45 : done ? 0.6 : 1 }}
        className={cn(
          'mr-14 flex min-w-0 flex-1 cursor-grab items-center gap-5 border bg-bg-input py-4 pl-4 pr-7 transition-colors hover:border-line-hover',
          dragging ? 'border-cyan' : 'border-line-subtle',
        )}
      >
        <Checkbox checked={done} size="sm" onChange={onToggle} />
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            onOpen();
          }}
          className={cn(
            'min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap border-0 bg-transparent p-0 text-left text-105',
            done ? 'text-txt-tag line-through' : 'text-txt-soft',
          )}
        >
          {subtask.title}
        </button>
        {!done && <DeadlineBadge task={subtask} bare />}
        <span className="flex-none whitespace-nowrap text-9 text-violet-sub">
          {done ? '' : '+'}
          {xp} XP
        </span>
      </article>
    </div>
  );
}
