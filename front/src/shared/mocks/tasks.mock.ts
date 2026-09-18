import { currentUser } from './user.mock';
import { LINK_PAIRS, TASK_SEEDS } from './seed';
import { LINK_TYPES } from '@/shared/config/domain';
import { pad2 } from '@/shared/lib/ascii';
import { xpForTask } from '@/shared/lib/progression';
import type { Task, TaskLink } from '@/shared/types/domain';

/**
 * Extra tags sprinkled over the seed so the tag list is long enough to exercise
 * the collapsing filter. Mock-only — real tags come from the backend.
 */
const EXTRA_TAGS = ['ops', 'research', 'finance', 'legal', 'growth', 'infra', 'personal', 'review'];

const HOUR = 3_600_000;
const DAY = 86_400_000;
const now = Date.now();
const iso = (ms: number) => new Date(ms).toISOString();

interface BuiltBoard {
  tasks: Task[];
  links: TaskLink[];
}

function build(): BuiltBoard {
  const tasks: Task[] = [];
  /** seed index → task id, so LINK_PAIRS can be resolved afterwards. */
  const idBySeed = new Map<number, string>();
  let counter = 1;

  TASK_SEEDS.forEach((seed, seedIndex) => {
    const id = `t${pad2(counter)}`;
    counter += 1;
    idBySeed.set(seedIndex, id);

    const deadlineAt = seed.deadlineHours === null ? null : iso(now + seed.deadlineHours * HOUR);
    const completedAt = seed.done ? iso(now - DAY * (1 + (seedIndex % 9))) : null;

    tasks.push({
      id,
      userId: currentUser.id,
      parentTaskId: null,
      title: seed.title,
      description: `${seed.title}. ${
        seed.tags[0] === 'backend'
          ? 'Touches the sync layer; keep the migration reversible.'
          : 'Keep the scope tight and close it in one session.'
      }`,
      quadrant: seed.quadrant,
      position: (seedIndex + 1) * 1024,
      color: seed.color,
      deadlineAt,
      deadlineHasTime: deadlineAt !== null,
      status: seed.done ? 'COMPLETED' : 'ACTIVE',
      createdAt: iso(now - DAY * (3 + (seedIndex % 40))),
      updatedAt: iso(now - DAY),
      completedAt,
      xpAwarded: seed.done ? xpForTask(seed.quadrant, false) : null,
      quadrantAtCompletion: seed.done ? seed.quadrant : null,
      completedVia: seed.done ? 'DIRECT' : null,
      tags:
        seedIndex % 3 === 0
          ? [...seed.tags, EXTRA_TAGS[seedIndex % EXTRA_TAGS.length] as string]
          : [...seed.tags],
    });

    seed.subs.forEach((sub, subIndex) => {
      const subId = `t${pad2(counter)}`;
      counter += 1;
      const done = seed.done || sub.done;
      const subDeadline =
        seed.deadlineHours === null
          ? null
          : iso(now + (seed.deadlineHours + 6 * (subIndex + 1)) * HOUR);

      tasks.push({
        id: subId,
        userId: currentUser.id,
        parentTaskId: id,
        title: sub.title,
        description: '',
        // subtasks have no quadrant of their own — they inherit the parent's
        quadrant: null,
        position: (subIndex + 1) * 1024,
        color: seed.color,
        deadlineAt: subDeadline,
        deadlineHasTime: subDeadline !== null,
        status: done ? 'COMPLETED' : 'ACTIVE',
        createdAt: iso(now - DAY * (2 + subIndex)),
        updatedAt: iso(now - DAY),
        completedAt: done ? iso(now - DAY) : null,
        xpAwarded: done ? xpForTask(seed.quadrant, true) : null,
        quadrantAtCompletion: done ? seed.quadrant : null,
        completedVia: done ? (seed.done ? 'PARENT_CASCADE' : 'DIRECT') : null,
        tags: [],
      });
    });
  });

  const links: TaskLink[] = [];
  LINK_PAIRS.forEach(([a, b], index) => {
    const source = idBySeed.get(a);
    const target = idBySeed.get(b);
    if (source === undefined || target === undefined) return;
    links.push({
      id: `l${pad2(index + 1)}`,
      userId: currentUser.id,
      sourceTaskId: source,
      targetTaskId: target,
      type: LINK_TYPES[index % LINK_TYPES.length] ?? 'RELATED',
    });
  });

  return { tasks, links };
}

const board = build();

export const mockTasks: readonly Task[] = board.tasks;
export const mockLinks: readonly TaskLink[] = board.links;

export interface TagStat {
  name: string;
  /** How many tasks carry the tag — drives which tags the filter shows first. */
  count: number;
}

/** Shape the backend's GET /tags is expected to return. */
export const mockTagStats: readonly TagStat[] = (() => {
  const counts = new Map<string, number>();
  mockTasks.forEach((task) => {
    task.tags.forEach((tag) => counts.set(tag, (counts.get(tag) ?? 0) + 1));
  });
  return [...counts.entries()]
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));
})();

export const mockTags: readonly string[] = mockTagStats
  .map((tag) => tag.name)
  .slice()
  .sort();
