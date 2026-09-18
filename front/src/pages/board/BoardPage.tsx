import { useMemo, useState } from 'react';

import { BoardColumn } from './BoardColumn';
import { useBoardStore } from '@/features/board/board.store';
import { useSessionStore } from '@/features/session/session.store';
import { TagFilter } from '@/features/filters/TagFilter';
import {
  matchesFilters,
  useFiltersStore,
  type DeadlineFilter,
  type StatusFilter,
} from '@/features/filters/filters.store';
import { FREE_TASK_CAP, QUADRANTS, TASK_COLORS } from '@/shared/config/domain';
import { useLang, useLocalized, useT } from '@/shared/i18n';
import { mockTagStats } from '@/shared/mocks';
import type { Quadrant, Task } from '@/shared/types/domain';
import { Button, Chip, ColorSwatch, Select } from '@/shared/ui';
import { BoardToolbar, BoardToolbarSection, BoardToolbarTail } from '@/widgets';

export interface BoardPageProps {
  onOpenTask: (id: string) => void;
  onQuotaHit: () => void;
}

export function BoardPage({ onOpenTask, onQuotaHit }: BoardPageProps) {
  const t = useT();
  const lang = useLang();
  const localized = useLocalized();
  // select state slices, not the whole store: actions keep a stable identity
  const tasks = useBoardStore((s) => s.tasks);
  const links = useBoardStore((s) => s.links);
  const createTask = useBoardStore((s) => s.createTask);
  const moveTask = useBoardStore((s) => s.moveTask);
  const activeTaskCount = useBoardStore((s) => s.activeTaskCount());
  const filters = useFiltersStore();
  const plan = useSessionStore((s) => s.user.plan);

  const [draggingId, setDraggingId] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<{ quadrant: Quadrant; index: number | null } | null>(
    null,
  );

  const criteria = useMemo(
    () => ({
      status: filters.status,
      tags: filters.tags,
      colors: filters.colors,
      quadrants: filters.quadrants,
      deadline: filters.deadline,
      topology: filters.topology,
      query: filters.query,
    }),
    [
      filters.status,
      filters.tags,
      filters.colors,
      filters.quadrants,
      filters.deadline,
      filters.topology,
      filters.query,
    ],
  );

  /**
   * One pass over the data per change instead of a nested scan per rendered row:
   * subtasks are grouped by parent and link counts are tallied up front.
   */
  const { subtasksByParent, linkCountById, byId } = useMemo(() => {
    const subtasks = new Map<string, Task[]>();
    const counts = new Map<string, number>();
    const index = new Map<string, Task>();

    tasks.forEach((task) => {
      index.set(task.id, task);
      if (task.parentTaskId !== null) {
        const bucket = subtasks.get(task.parentTaskId);
        if (bucket) bucket.push(task);
        else subtasks.set(task.parentTaskId, [task]);
      }
    });
    subtasks.forEach((bucket) => bucket.sort((a, b) => a.position - b.position));
    links.forEach((link) => {
      counts.set(link.sourceTaskId, (counts.get(link.sourceTaskId) ?? 0) + 1);
      counts.set(link.targetTaskId, (counts.get(link.targetTaskId) ?? 0) + 1);
    });

    return { subtasksByParent: subtasks, linkCountById: counts, byId: index };
  }, [tasks, links]);

  const { visibleParents, visibleSubCount } = useMemo(() => {
    const passes = (task: Task): boolean => {
      const parent = task.parentTaskId === null ? undefined : byId.get(task.parentTaskId);
      return matchesFilters(task, criteria, {
        quadrant: task.quadrant ?? parent?.quadrant ?? null,
        linkCount: linkCountById.get(task.id) ?? 0,
      });
    };

    const parents = tasks.filter((task) => task.parentTaskId === null);
    return {
      visibleParents: parents.filter(
        (task) => passes(task) || (subtasksByParent.get(task.id) ?? []).some(passes),
      ),
      visibleSubCount: tasks.filter((task) => task.parentTaskId !== null && passes(task)).length,
    };
  }, [tasks, criteria, byId, linkCountById, subtasksByParent]);

  const statusOptions: Array<{ value: StatusFilter; label: string }> = [
    { value: 'active', label: t.active },
    { value: 'completed', label: t.completed },
    { value: 'all', label: t.all },
  ];

  const deadlineOptions: Array<{ value: DeadlineFilter; label: string }> = [
    { value: 'any', label: t.dlAny },
    { value: 'overdue', label: t.dlOverdue },
    { value: 'today', label: t.dlToday },
    { value: 'week', label: t.dlWeek },
    { value: 'none', label: t.dlNone },
  ];

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <BoardToolbar>
        <BoardToolbarSection label={t.state} gap="tight">
          {statusOptions.map((option) => (
            <Chip
              key={option.value}
              active={filters.status === option.value}
              onClick={() => filters.setStatus(option.value)}
            >
              {option.label}
            </Chip>
          ))}
        </BoardToolbarSection>

        <BoardToolbarSection label={t.tags} grow>
          <TagFilter tags={mockTagStats} />
        </BoardToolbarSection>

        <BoardToolbarSection label={t.color}>
          {TASK_COLORS.map((color) => (
            <ColorSwatch
              key={color.id}
              color={color.hex}
              title={localized(color.name)}
              selected={filters.colors.includes(color.id)}
              onClick={() => filters.toggleColor(color.id)}
            />
          ))}
        </BoardToolbarSection>

        <BoardToolbarSection label={t.deadline} gap="normal">
          <Select
            options={deadlineOptions}
            value={filters.deadline}
            onChange={(event) => filters.setDeadline(event.target.value as DeadlineFilter)}
          />
        </BoardToolbarSection>

        <BoardToolbarTail>
          <span
            className={filters.status === 'completed' ? 'text-105 text-green' : 'text-105 text-txt-faint'}
          >
            {filters.status === 'completed'
              ? t.archiveMode
              : `${visibleParents.length}${lang === 'RU' ? ' задач · ' : ' tasks · '}${visibleSubCount}${
                  lang === 'RU' ? ' подзадач' : ' subtasks'
                }`}
          </span>
          <Button variant="danger" onClick={filters.reset}>
            {t.reset}
          </Button>
        </BoardToolbarTail>
      </BoardToolbar>

      <div className="flex min-h-0 flex-1 gap-px overflow-x-auto bg-line-subtle">
        {QUADRANTS.map((meta) => {
          const columnTasks = visibleParents
            .filter((task) => task.quadrant === meta.id)
            .sort((a, b) => a.position - b.position);

          return (
            <BoardColumn
              key={meta.id}
              meta={meta}
              tasks={columnTasks}
              count={columnTasks.length}
              subtasksByParent={subtasksByParent}
              linkCountById={linkCountById}
              draggingId={draggingId}
              dropIndex={dropTarget?.quadrant === meta.id ? dropTarget.index : null}
              onOpenTask={onOpenTask}
              onAdd={() => {
                if (plan === 'FREE' && activeTaskCount >= FREE_TASK_CAP) {
                  onQuotaHit();
                  return;
                }
                createTask(meta.id, lang === 'RU' ? 'Новая задача' : 'New task');
              }}
              onDragStart={setDraggingId}
              onDragEnd={() => {
                setDraggingId(null);
                setDropTarget(null);
              }}
              onDragOverCard={(index) => setDropTarget({ quadrant: meta.id, index })}
              onDragOverColumn={() => setDropTarget({ quadrant: meta.id, index: null })}
              onDropOnCard={(beforeId) => {
                if (draggingId !== null && draggingId !== beforeId) {
                  moveTask(draggingId, meta.id, beforeId);
                }
                setDraggingId(null);
                setDropTarget(null);
              }}
              onDropOnColumn={() => {
                if (draggingId !== null) moveTask(draggingId, meta.id, null);
                setDraggingId(null);
                setDropTarget(null);
              }}
            />
          );
        })}
      </div>
    </div>
  );
}
