import type { DragEvent } from 'react';

import { DeadlineBadge } from './DeadlineBadge';
import { TASK_COLOR_BY_ID } from '@/shared/config/domain';
import { useT } from '@/shared/i18n';
import { asciiBar } from '@/shared/lib/ascii';
import { cn } from '@/shared/lib/cn';
import { xpForTask } from '@/shared/lib/progression';
import type { Quadrant, Task } from '@/shared/types/domain';
import { Checkbox, TagChip } from '@/shared/ui';

export interface TaskCardProps {
  task: Task;
  quadrant: Quadrant;
  subtaskTotal: number;
  subtaskDone: number;
  linkCount: number;
  expanded: boolean;
  dragging?: boolean;
  onToggle: () => void;
  onOpen: () => void;
  onToggleSubtasks: () => void;
  onDragStart?: () => void;
  onDragEnd?: () => void;
  onDragOver?: (event: DragEvent<HTMLElement>) => void;
  onDrop?: (event: DragEvent<HTMLElement>) => void;
}

export function TaskCard({
  task,
  quadrant,
  subtaskTotal,
  subtaskDone,
  linkCount,
  expanded,
  dragging = false,
  onToggle,
  onOpen,
  onToggleSubtasks,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDrop,
}: TaskCardProps) {
  const t = useT();
  const done = task.status === 'COMPLETED';
  const xp = done ? (task.xpAwarded ?? 0) : xpForTask(quadrant, false);
  const stripe = TASK_COLOR_BY_ID[task.color].hex ?? '#23272b';

  return (
    <article
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onDragOver={onDragOver}
      onDrop={onDrop}
      onClick={onOpen}
      style={{ opacity: dragging ? 0.45 : done ? 0.62 : 1 }}
      className={cn(
        'flex cursor-grab border transition-colors hover:border-line-hover',
        dragging
          ? 'border-dashed border-cyan bg-[#101416]'
          : done
            ? 'border-node-done bg-bg-sunken'
            : 'border-line bg-bg-panel',
      )}
    >
      <span className="w-2 flex-none" style={{ background: stripe }} />

      <div className="min-w-0 flex-1 py-8 pl-5 pr-9">
        <div className="flex items-start gap-6">
          <Checkbox checked={done} onChange={onToggle} title={t.complete} className="-mt-px" />
          <h3
            className={cn(
              'm-0 min-w-0 flex-1 break-words text-115 font-medium leading-[1.45]',
              done ? 'text-txt-tag line-through' : 'text-txt',
            )}
          >
            {task.title}
          </h3>
          <span className="flex-none pt-2 text-9 text-txt-ghost">{task.id.toUpperCase()}</span>
        </div>

        <div className="mt-7 flex flex-wrap items-center gap-x-9 gap-y-4 pl-24">
          <DeadlineBadge task={task} />
          {task.tags.map((tag) => (
            <TagChip key={tag} name={tag} plain />
          ))}
          <span className="flex-1" />
          {linkCount > 0 && (
            <span title={t.links} className="whitespace-nowrap text-95 text-cyan">
              ◈{linkCount}
            </span>
          )}
          <span className="whitespace-nowrap text-95 text-violet-dim">
            {done ? '' : '+'}
            {xp} XP
          </span>
        </div>

        {subtaskTotal > 0 && (
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              onToggleSubtasks();
            }}
            className="mt-7 flex w-full items-center gap-7 border-0 border-t border-t-line-subtle bg-transparent pl-24 pt-5 text-left text-95 tracking-t5 text-txt-dim transition-colors hover:text-txt"
          >
            <span className="text-txt-faint">{expanded ? '▾' : '▸'}</span>
            {subtaskDone}/{subtaskTotal} {t.subsOf}
            <span className="flex-1" />
            <span className="tracking-tight text-[#6b8a75]">
              {asciiBar(subtaskTotal === 0 ? 0 : subtaskDone / subtaskTotal, 6)}
            </span>
          </button>
        )}
      </div>
    </article>
  );
}
