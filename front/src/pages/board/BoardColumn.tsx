import { useState } from 'react';

import { SubtaskRow, TaskCard } from '@/entities/task';
import { useBoardStore } from '@/features/board/board.store';
import { usePreferencesStore } from '@/features/settings/preferences.store';
import type { QuadrantMeta } from '@/shared/config/domain';
import { useLocalized, useT } from '@/shared/i18n';
import { cn } from '@/shared/lib/cn';
import type { Task } from '@/shared/types/domain';
import { Button, EmptyState, IconButton } from '@/shared/ui';

const EMPTY_ART = '┌───────────────┐\n│               │\n│       ·       │\n│               │\n└───────────────┘';

export interface BoardColumnProps {
  meta: QuadrantMeta;
  tasks: readonly Task[];
  /** Total count shown in the header, before search narrowing. */
  count: number;
  /** False until the board has arrived: an empty column is not yet a fact. */
  loaded: boolean;
  /** Derived once per data change by the page, not re-scanned per row. */
  subtasksByParent: Map<string, Task[]>;
  linkCountById: Map<string, number>;
  draggingId: string | null;
  dropIndex: number | null;
  onOpenTask: (id: string) => void;
  onAdd: () => void;
  onDragStart: (id: string) => void;
  onDragEnd: () => void;
  onDragOverCard: (index: number) => void;
  onDragOverColumn: () => void;
  onDropOnCard: (beforeId: string) => void;
  onDropOnColumn: () => void;
}

export function BoardColumn({
  meta,
  tasks,
  count,
  loaded,
  subtasksByParent,
  linkCountById,
  draggingId,
  dropIndex,
  onOpenTask,
  onAdd,
  onDragStart,
  onDragEnd,
  onDragOverCard,
  onDragOverColumn,
  onDropOnCard,
  onDropOnColumn,
}: BoardColumnProps) {
  const t = useT();
  const localized = useLocalized();
  const toggleDone = useBoardStore((s) => s.toggleDone);
  const createSubtask = useBoardStore((s) => s.createSubtask);
  const expandedByDefault = usePreferencesStore((s) => s.subtasksExpandedByDefault);
  /** Per-card override of the "subtasks expanded by default" preference. */
  const [overrides, setOverrides] = useState<Record<string, boolean>>({});

  return (
    <section
      onDragOver={(event) => {
        event.preventDefault();
        onDragOverColumn();
      }}
      onDrop={(event) => {
        event.preventDefault();
        onDropOnColumn();
      }}
      className="flex min-h-0 min-w-272 flex-1 flex-col bg-bg-base"
    >
      <div className="flex-none border-b border-line bg-bg-panel px-12 pb-9 pt-10">
        <div className="flex items-center gap-8">
          <span className="text-105 font-bold tracking-t8" style={{ color: meta.accent }}>
            {meta.code}
          </span>
          <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-115 font-medium tracking-t7 text-txt">
            {localized(meta.name)}
          </span>
          <span className="text-105 text-txt-faint">{count}</span>
          <IconButton tone="green" title={t.newTask} onClick={onAdd} className="border-line-strong bg-bg-hover">
            +
          </IconButton>
        </div>
        <div className="mt-5 flex items-center justify-between gap-8">
          <span className="overflow-hidden text-ellipsis whitespace-nowrap text-95 tracking-t7 text-txt-faint">
            {localized(meta.subtitle)}
          </span>
          <span className="whitespace-nowrap text-95 text-violet">+{meta.xp} XP</span>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-8 pb-40">
        {loaded && tasks.length === 0 && (
          <EmptyState
            art={EMPTY_ART}
            message={localized(meta.empty)}
            actionLabel={t.emptyCta}
            onAction={onAdd}
          />
        )}

        {tasks.map((task, index) => {
          const subs = subtasksByParent.get(task.id) ?? [];
          const expanded = (overrides[task.id] ?? expandedByDefault) && subs.length > 0;
          return (
            <div key={task.id} className="mb-8">
              {dropIndex === index && draggingId !== null && draggingId !== task.id && (
                <div className="mb-6 h-2 bg-cyan" />
              )}

              <TaskCard
                task={task}
                quadrant={meta.id}
                subtaskTotal={subs.length}
                subtaskDone={subs.filter((s) => s.status === 'COMPLETED').length}
                linkCount={linkCountById.get(task.id) ?? 0}
                expanded={expanded}
                dragging={draggingId === task.id}
                onToggle={() => toggleDone(task.id)}
                onOpen={() => onOpenTask(task.id)}
                onToggleSubtasks={() =>
                  setOverrides((prev) => ({
                    ...prev,
                    [task.id]: !(prev[task.id] ?? expandedByDefault),
                  }))
                }
                onDragStart={() => onDragStart(task.id)}
                onDragEnd={onDragEnd}
                onDragOver={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  onDragOverCard(index);
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  onDropOnCard(task.id);
                }}
              />

              {expanded && (
                <div className={cn('pl-13')}>
                  {subs.map((sub, subIndex) => (
                    <SubtaskRow
                      key={sub.id}
                      subtask={sub}
                      quadrant={meta.id}
                      last={subIndex === subs.length - 1}
                      dragging={draggingId === sub.id}
                      onToggle={() => toggleDone(sub.id)}
                      onOpen={() => onOpenTask(sub.id)}
                      onDragStart={() => onDragStart(sub.id)}
                      onDragEnd={onDragEnd}
                    />
                  ))}
                  <div className="mt-3 flex items-stretch">
                    <span className="w-12 flex-none pt-4 text-center text-11 text-txt-branch">└</span>
                    <Button
                      variant="dashed"
                      size="xs"
                      className="mr-14 flex-1 justify-start text-95"
                      onClick={() => createSubtask(task.id, t.newSub)}
                    >
                      + {t.newSub}
                    </Button>
                  </div>
                </div>
              )}
            </div>
          );
        })}

        {dropIndex === null && draggingId !== null && <div className="h-2 bg-cyan" />}
      </div>
    </section>
  );
}
