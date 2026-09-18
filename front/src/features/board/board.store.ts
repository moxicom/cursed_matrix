import { create } from 'zustand';

import { QUADRANT_BY_ID } from '@/shared/config/domain';
import { pad2 } from '@/shared/lib/ascii';
import { xpForTask } from '@/shared/lib/progression';
import { currentUser, mockLinks, mockTasks } from '@/shared/mocks';
import type { LinkType, Quadrant, Task, TaskColorId, TaskLink } from '@/shared/types/domain';

export interface ToastState {
  code: string;
  detail: string;
}

interface BoardState {
  tasks: Task[];
  links: TaskLink[];
  toast: ToastState | null;
  nextId: number;

  /** Selectors */
  taskById: (id: string) => Task | undefined;
  subtasksOf: (id: string) => Task[];
  linksOf: (id: string) => TaskLink[];
  activeTaskCount: () => number;

  /** Mutations — mirror the operations the backend will own. */
  toggleDone: (id: string) => void;
  patchTask: (id: string, patch: Partial<Task>) => void;
  createTask: (quadrant: Quadrant, title: string) => void;
  createSubtask: (parentId: string, title: string) => void;
  promoteSubtask: (id: string) => void;
  moveTask: (id: string, quadrant: Quadrant, beforeId: string | null) => void;
  deleteTask: (id: string) => void;
  addLink: (sourceId: string, targetId: string) => void;
  setLinkType: (linkId: string, type: LinkType) => void;
  removeLink: (linkId: string) => void;
  setTaskColor: (id: string, color: TaskColorId) => void;
  addTag: (id: string, tag: string) => void;
  removeTag: (id: string, tag: string) => void;
  flash: (code: string, detail: string) => void;
  dismissToast: () => void;
}

const nowIso = () => new Date().toISOString();

export const useBoardStore = create<BoardState>((set, get) => ({
  tasks: [...mockTasks],
  links: [...mockLinks],
  toast: null,
  nextId: mockTasks.length + 1,

  taskById: (id) => get().tasks.find((t) => t.id === id),
  subtasksOf: (id) =>
    get()
      .tasks.filter((t) => t.parentTaskId === id)
      .sort((a, b) => a.position - b.position),
  linksOf: (id) => get().links.filter((l) => l.sourceTaskId === id || l.targetTaskId === id),
  activeTaskCount: () => get().tasks.filter((t) => t.status === 'ACTIVE').length,

  toggleDone: (id) => {
    const task = get().taskById(id);
    if (!task) return;
    const next = task.status === 'ACTIVE';
    const stamp = nowIso();
    const parent = task.parentTaskId === null ? null : get().taskById(task.parentTaskId);
    const quadrant = task.quadrant ?? parent?.quadrant ?? 'IMPORTANT_URGENT';
    const xp = xpForTask(quadrant, task.parentTaskId !== null);

    set((state) => ({
      tasks: state.tasks.map((t) => {
        if (t.id === id) {
          return {
            ...t,
            status: next ? 'COMPLETED' : 'ACTIVE',
            completedAt: next ? stamp : null,
            xpAwarded: next ? xp : null,
            quadrantAtCompletion: next ? quadrant : null,
            completedVia: next ? 'DIRECT' : null,
          };
        }
        // completing a parent cascades into its unfinished subtasks
        if (next && t.parentTaskId === id && t.status === 'ACTIVE') {
          return {
            ...t,
            status: 'COMPLETED',
            completedAt: stamp,
            xpAwarded: xpForTask(quadrant, true),
            quadrantAtCompletion: quadrant,
            completedVia: 'PARENT_CASCADE',
          };
        }
        return t;
      }),
    }));

    get().flash(next ? 'QUEST_RESOLVED' : 'QUEST_REOPENED', `${next ? '+' : '−'}${xp} XP · ${task.title}`);
  },

  patchTask: (id, patch) =>
    set((state) => ({
      tasks: state.tasks.map((t) => (t.id === id ? { ...t, ...patch, updatedAt: nowIso() } : t)),
    })),

  createTask: (quadrant, title) => {
    const id = `t${pad2(get().nextId)}`;
    const position =
      Math.max(
        0,
        ...get()
          .tasks.filter((t) => t.parentTaskId === null && t.quadrant === quadrant)
          .map((t) => t.position),
      ) + 1024;

    set((state) => ({
      nextId: state.nextId + 1,
      tasks: [
        ...state.tasks,
        {
          id,
          userId: currentUser.id,
          parentTaskId: null,
          title,
          description: '',
          quadrant,
          position,
          color: 'NONE',
          deadlineAt: null,
          deadlineHasTime: false,
          status: 'ACTIVE',
          createdAt: nowIso(),
          updatedAt: nowIso(),
          completedAt: null,
          xpAwarded: null,
          quadrantAtCompletion: null,
          completedVia: null,
          tags: [],
        },
      ],
    }));

    get().flash(
      'QUEST_REGISTERED',
      `${QUADRANT_BY_ID[quadrant].code} · +${QUADRANT_BY_ID[quadrant].xp} XP on completion`,
    );
  },

  createSubtask: (parentId, title) => {
    const id = `t${pad2(get().nextId)}`;
    const position = Math.max(0, ...get().subtasksOf(parentId).map((t) => t.position)) + 1024;
    const parent = get().taskById(parentId);

    set((state) => ({
      nextId: state.nextId + 1,
      tasks: [
        ...state.tasks,
        {
          id,
          userId: currentUser.id,
          parentTaskId: parentId,
          title,
          description: '',
          quadrant: null,
          position,
          color: parent?.color ?? 'NONE',
          deadlineAt: null,
          deadlineHasTime: false,
          status: 'ACTIVE',
          createdAt: nowIso(),
          updatedAt: nowIso(),
          completedAt: null,
          xpAwarded: null,
          quadrantAtCompletion: null,
          completedVia: null,
          tags: [],
        },
      ],
    }));
  },

  promoteSubtask: (id) => {
    const task = get().taskById(id);
    if (!task || task.parentTaskId === null) return;
    const parent = get().taskById(task.parentTaskId);
    const quadrant = parent?.quadrant ?? 'IMPORTANT_URGENT';
    const position =
      Math.max(
        0,
        ...get()
          .tasks.filter((t) => t.parentTaskId === null && t.quadrant === quadrant)
          .map((t) => t.position),
      ) + 1024;

    get().patchTask(id, { parentTaskId: null, quadrant, position });
    get().flash('NODE_PROMOTED', `${task.title} → ${QUADRANT_BY_ID[quadrant].code}`);
  },

  moveTask: (id, quadrant, beforeId) => {
    const task = get().taskById(id);
    if (!task) return;

    const siblings = get()
      .tasks.filter((t) => t.parentTaskId === null && t.quadrant === quadrant && t.id !== id)
      .sort((a, b) => a.position - b.position);

    let position: number;
    if (beforeId === null) {
      position = (siblings[siblings.length - 1]?.position ?? 0) + 1024;
    } else {
      const index = siblings.findIndex((t) => t.id === beforeId);
      const before = siblings[index - 1]?.position ?? 0;
      const target = siblings[index]?.position ?? before + 2048;
      position = (before + target) / 2;
    }

    const previousQuadrant = task.quadrant;
    get().patchTask(id, { quadrant, position, parentTaskId: null });

    if (previousQuadrant !== null && previousQuadrant !== quadrant) {
      get().flash(
        'PRIORITY_REASSIGNED',
        `${QUADRANT_BY_ID[previousQuadrant].code} → ${QUADRANT_BY_ID[quadrant].code} · ${xpForTask(
          quadrant,
          false,
        )} XP`,
      );
    } else if (task.parentTaskId !== null) {
      get().flash('NODE_PROMOTED', `${task.title} → ${QUADRANT_BY_ID[quadrant].code}`);
    }
  },

  deleteTask: (id) => {
    set((state) => ({
      tasks: state.tasks.filter((t) => t.id !== id && t.parentTaskId !== id),
      links: state.links.filter((l) => l.sourceTaskId !== id && l.targetTaskId !== id),
    }));
    get().flash('NODE_PURGED', id.toUpperCase());
  },

  addLink: (sourceId, targetId) => {
    if (sourceId === targetId) return;
    const exists = get().links.some(
      (l) =>
        (l.sourceTaskId === sourceId && l.targetTaskId === targetId) ||
        (l.sourceTaskId === targetId && l.targetTaskId === sourceId),
    );
    if (exists) return;

    set((state) => ({
      links: [
        ...state.links,
        {
          id: `l${state.links.length + 1}`,
          userId: currentUser.id,
          sourceTaskId: sourceId,
          targetTaskId: targetId,
          type: 'RELATED',
        },
      ],
    }));
    get().flash('NODE_LINK_ESTABLISHED', `${sourceId.toUpperCase()} ◈ ${targetId.toUpperCase()}`);
  },

  setLinkType: (linkId, type) =>
    set((state) => ({
      links: state.links.map((l) => (l.id === linkId ? { ...l, type } : l)),
    })),

  removeLink: (linkId) =>
    set((state) => ({ links: state.links.filter((l) => l.id !== linkId) })),

  setTaskColor: (id, color) => get().patchTask(id, { color }),

  addTag: (id, tag) => {
    const task = get().taskById(id);
    if (!task || tag.trim() === '' || task.tags.includes(tag)) return;
    get().patchTask(id, { tags: [...task.tags, tag.trim()] });
  },

  removeTag: (id, tag) => {
    const task = get().taskById(id);
    if (!task) return;
    get().patchTask(id, { tags: task.tags.filter((t) => t !== tag) });
  },

  flash: (code, detail) => set({ toast: { code, detail } }),
  dismissToast: () => set({ toast: null }),
}));
