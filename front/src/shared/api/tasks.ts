import type { LinkType, Quadrant, Task, TaskColorId, TaskLink } from '@/shared/types/domain';

import { api } from './client';

/**
 * The wire shape of a task.
 *
 * Every nullable field is optional here because that is how it arrives: the
 * contract declares them nullable and not required, and the server leaves an
 * absent one out rather than writing null. The domain type promises null, so
 * this is where the two are reconciled — once, instead of every screen
 * remembering to treat undefined as null.
 */
type TaskWire = Omit<
  Task,
  | 'parentTaskId'
  | 'quadrant'
  | 'deadlineAt'
  | 'completedAt'
  | 'xpAwarded'
  | 'quadrantAtCompletion'
  | 'completedVia'
> & {
  parentTaskId?: string | null;
  quadrant?: Quadrant | null;
  deadlineAt?: string | null;
  completedAt?: string | null;
  xpAwarded?: number | null;
  quadrantAtCompletion?: Quadrant | null;
  completedVia?: Task['completedVia'];
  deletedAt?: string | null;
};

function toTask(wire: TaskWire): Task {
  return {
    ...wire,
    parentTaskId: wire.parentTaskId ?? null,
    quadrant: wire.quadrant ?? null,
    deadlineAt: wire.deadlineAt ?? null,
    completedAt: wire.completedAt ?? null,
    xpAwarded: wire.xpAwarded ?? null,
    quadrantAtCompletion: wire.quadrantAtCompletion ?? null,
    completedVia: wire.completedVia ?? null,
    tags: wire.tags ?? [],
  };
}

/** The board and the graph both want the whole working set, so it is one call. */
export interface Board {
  tasks: Task[];
  links: TaskLink[];
  /** True when the result hit the server's cap and more exist. */
  truncated: boolean;
}

export type StatusFilter = 'ACTIVE' | 'COMPLETED' | 'ALL';

export interface BoardQuery {
  status?: StatusFilter;
  quadrants?: Quadrant[];
  colors?: TaskColorId[];
  tags?: string[];
  query?: string;
}

export async function board(query: BoardQuery = {}): Promise<Board> {
  const answer = await api.get<{ tasks: TaskWire[]; links: TaskLink[]; truncated: boolean }>(
    '/tasks',
    {
      query: {
        status: query.status ?? 'ALL',
        quadrants: query.quadrants,
        colors: query.colors,
        tags: query.tags,
        query: query.query,
      },
    },
  );

  return {
    tasks: answer.tasks.map(toTask),
    links: answer.links,
    truncated: answer.truncated,
  };
}

export interface TaskDraft {
  title: string;
  description?: string;
  quadrant: Quadrant;
  color?: TaskColorId;
  deadlineAt?: string | null;
  deadlineHasTime?: boolean;
}

export async function create(draft: TaskDraft) {
  return toTask(await api.post<TaskWire>('/tasks', { body: draft }));
}

export async function createSubtask(parentId: string, title: string) {
  return toTask(await api.post<TaskWire>(`/tasks/${parentId}/subtasks`, { body: { title } }));
}

/**
 * The fields a task's own form may change.
 *
 * Quadrant and position are not among them: where a task sits is a move, and
 * a completed task is frozen with its XP snapshot.
 */
export interface TaskPatch {
  title?: string;
  description?: string;
  color?: TaskColorId;
  deadlineAt?: string | null;
  deadlineHasTime?: boolean;
}

export async function patch(id: string, change: TaskPatch) {
  return toTask(await api.patch<TaskWire>(`/tasks/${id}`, { body: change }));
}

export async function remove(id: string) {
  return api.delete<{ deletedIds: string[] }>(`/tasks/${id}`);
}

export interface LevelUp {
  fromLevel: number;
  toLevel: number;
}

/** What one completion or reopening changed, so the board reconciles itself. */
type ProgressWire = Omit<Progress, 'tasks' | 'levelUp'> & {
  tasks: TaskWire[];
  levelUp?: LevelUp | null;
};

function toProgress(wire: ProgressWire): Progress {
  return {
    tasks: wire.tasks.map(toTask),
    xpAwarded: wire.xpAwarded,
    levelUp: wire.levelUp ?? null,
    unlockedAchievements: wire.unlockedAchievements ?? [],
  };
}

export interface Progress {
  tasks: Task[];
  xpAwarded: number;
  /** Null rather than absent: normalised on arrival, like every nullable. */
  levelUp: LevelUp | null;
  unlockedAchievements: string[];
}

export async function complete(id: string) {
  return toProgress(await api.post<ProgressWire>(`/tasks/${id}/complete`));
}

export async function reopen(id: string) {
  return toProgress(await api.post<ProgressWire>(`/tasks/${id}/reopen`));
}

/**
 * Where a task should sit, named by a neighbour rather than a number: two
 * clients dragging at once would compute the same number from different
 * starting points.
 */
export interface Move {
  targetQuadrant?: Quadrant;
  /** Reorders a subtask among its siblings; must be the parent it has. */
  parentTaskId?: string;
  beforeTaskId?: string;
  afterTaskId?: string;
}

export async function move(id: string, where: Move) {
  const answer = await api.post<{ tasks: TaskWire[] }>(`/tasks/${id}/move`, { body: where });
  return { tasks: answer.tasks.map(toTask) };
}

export async function promote(id: string, quadrant?: Quadrant) {
  const answer = await api.post<{ tasks: TaskWire[] }>(`/tasks/${id}/promote`, {
    body: quadrant ? { quadrant } : {},
  });
  return { tasks: answer.tasks.map(toTask) };
}

export interface Tag {
  id: string;
  name: string;
  taskCount: number;
}

export async function tags() {
  const answer = await api.get<{ tags: Tag[] }>('/tags');
  return answer.tags;
}

export async function attachTag(taskId: string, name: string) {
  return api.post<Tag>(`/tasks/${taskId}/tags`, { body: { name } });
}

export async function detachTag(taskId: string, tagId: string) {
  await api.delete<void>(`/tasks/${taskId}/tags/${tagId}`);
}

/**
 * One row the server matched, and why.
 *
 * The board holds only what one screen needs, so searching it would miss the
 * archive; and the snippet is cut where the term is, which is something only
 * the side that did the matching knows.
 */
export interface SearchHit {
  id: string;
  title: string;
  quadrant: Quadrant | null;
  isSubtask: boolean;
  status: 'ACTIVE' | 'COMPLETED';
  matchedField: 'TITLE' | 'DESCRIPTION' | 'TAG';
  matchedText: string;
}

export async function search(query: string, limit = 14, signal?: AbortSignal) {
  const options: { query: Record<string, string | number>; signal?: AbortSignal } = {
    query: { query, limit },
  };
  if (signal) options.signal = signal;

  const answer = await api.get<{ items: Array<Omit<SearchHit, 'quadrant'> & { quadrant?: Quadrant | null }> }>(
    '/search',
    options,
  );
  return answer.items.map((hit) => ({ ...hit, quadrant: hit.quadrant ?? null }) satisfies SearchHit);
}

export async function createLink(sourceTaskId: string, targetTaskId: string, type?: LinkType) {
  return api.post<TaskLink>('/links', {
    body: type ? { sourceTaskId, targetTaskId, type } : { sourceTaskId, targetTaskId },
  });
}

export async function setLinkType(linkId: string, type: LinkType) {
  return api.patch<TaskLink>(`/links/${linkId}`, { body: { type } });
}

export async function removeLink(linkId: string) {
  await api.delete<void>(`/links/${linkId}`);
}
