import { create } from 'zustand';

import { useUiStore } from '@/app/ui.store';
import * as tasksApi from '@/shared/api/tasks';
import { ApiError } from '@/shared/api/client';
import { QUADRANT_BY_ID } from '@/shared/config/domain';
import { TAG_MAX_LENGTH, TITLE_MAX_LENGTH } from '@/shared/config/limits';
import {
  isSubtask,
  type LinkType,
  type Quadrant,
  type Task,
  type TaskColorId,
  type TaskLink,
} from '@/shared/types/domain';

export interface ToastState {
  code: string;
  detail: string;
  /**
   * Set when the toast reports a refusal: the server's parameters, kept as
   * they arrived. The sentence is built where the language is known, so a
   * toast already on screen is right in whichever language is switched to.
   */
  params: Record<string, unknown> | null;
  tone: 'green' | 'red';
}

/**
 * How long a field waits before it is saved.
 *
 * Typing a title must not be a request per keystroke, and it must not be lost
 * either — the edit is applied here at once and written once the typing stops.
 */
const SAVE_AFTER_MS = 600;

interface BoardState {
  tasks: Task[];
  links: TaskLink[];
  tags: tasksApi.Tag[];
  toast: ToastState | null;
  /** False until the first board has arrived; screens render empty until then. */
  loaded: boolean;

  /** Selectors */
  taskById: (id: string) => Task | undefined;
  subtasksOf: (id: string) => Task[];
  linksOf: (id: string) => TaskLink[];
  activeTaskCount: () => number;

  /** Mutations. Each one is the server's decision applied here. */
  load: () => Promise<void>;
  /** Drops everything, including unwritten edits. Used when the session ends. */
  reset: () => void;
  toggleDone: (id: string) => void;
  patchTask: (id: string, patch: Partial<Task>) => void;
  createTask: (quadrant: Quadrant, title: string) => void;
  createSubtask: (parentId: string, title: string) => void;
  promoteSubtask: (id: string) => void;
  moveTask: (id: string, quadrant: Quadrant, beforeId: string | null) => void;
  reorderSubtask: (id: string, parentId: string, beforeId: string | null) => void;
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

/** Edits waiting to be written, and the timer that will write them. */
const pending = new Map<string, tasksApi.TaskPatch>();
const timers = new Map<string, ReturnType<typeof setTimeout>>();

/**
 * Which save is the latest for a task.
 *
 * Two saves for one task can be in flight at once — type, pause, type again on
 * a slow line — and nothing makes the answers come back in order. The older
 * one's response would put the older text back on screen, so only the newest
 * save is allowed to apply what the server returned.
 */
const saves = new Map<string, number>();

/** Forgets every unwritten edit and the timers that would write them. */
function forgetPendingEdits() {
  for (const timer of timers.values()) clearTimeout(timer);
  timers.clear();
  pending.clear();
  saves.clear();
}

export const useBoardStore = create<BoardState>((set, get) => {
  /** Replaces the tasks the server just changed, keeping the rest. */
  const merge = (changed: Task[]) =>
    set((state) => {
      const byId = new Map(changed.map((task) => [task.id, task]));
      const kept = state.tasks.map((task) => byId.get(task.id) ?? task);

      // Every answer that reports a change reports whole tasks — a move
      // includes the neighbours it shifted — so anything not already held is
      // appended rather than discarded.
      const known = new Set(kept.map((task) => task.id));
      const added = changed.filter((task) => !known.has(task.id));
      return { tasks: [...kept, ...added] };
    });

  /**
   * Reports a refusal. The code and its parameters are carried as they came;
   * turning them into a sentence is the renderer's job.
   */
  const refuse = (error: unknown) => {
    // What the plan will not allow — a quota reached, a trial run out — is
    // the server's decision, and this is where the client learns of it. A
    // toast would state it and move on; the paywall offers the way out.
    if (error instanceof ApiError && error.requiresPayment) {
      useUiStore.getState().openPaywall({ code: error.code, params: error.params });
      return;
    }
    // No answer at all — the request never completed, so there is no code to
    // report but the one we give it here.
    if (!(error instanceof ApiError)) {
      set({ toast: { code: 'REQUEST_FAILED', detail: '', params: {}, tone: 'red' } });
      return;
    }
    set({
      toast: {
        code: error.code,
        detail: error.requestId ?? '',
        params: error.params,
        tone: 'red',
      },
    });
  };

  const flush = async (id: string) => {
    const change = pending.get(id);
    pending.delete(id);
    timers.delete(id);
    if (!change) return;

    const save = (saves.get(id) ?? 0) + 1;
    saves.set(id, save);

    try {
      const saved = await tasksApi.patch(id, change);
      // A later save has already gone out, and what is on screen belongs to
      // it: applying this answer would put the older text back.
      if (saves.get(id) !== save) return;
      merge([saved]);
    } catch (error) {
      refuse(error);
      // The server did not take it, so the board must stop showing it as
      // though it had: whatever it holds now is the truth.
      void get().load();
    }
  };

  return {
    tasks: [],
    links: [],
    tags: [],
    toast: null,
    loaded: false,

    taskById: (id) => get().tasks.find((t) => t.id === id),
    subtasksOf: (id) =>
      get()
        .tasks.filter((t) => t.parentTaskId === id)
        .sort((a, b) => a.position - b.position),
    linksOf: (id) => get().links.filter((l) => l.sourceTaskId === id || l.targetTaskId === id),
    activeTaskCount: () => get().tasks.filter((t) => t.status === 'ACTIVE').length,

    reset: () => {
      // The unwritten edits go with the data: a timer that fired after a sign
      // out would write one account's text with another account's session.
      forgetPendingEdits();
      set({ tasks: [], links: [], tags: [], toast: null, loaded: false });
    },

    load: async () => {
      try {
        // Everything, completed included: the board hides them, the graph and
        // the search need them, and one call keeps the two consistent.
        const [board, tags] = await Promise.all([
          tasksApi.board({ status: 'ALL' }),
          tasksApi.tags(),
        ]);
        set({ tasks: board.tasks, links: board.links, tags, loaded: true });
        if (board.truncated) get().flash('RESULT_TRUNCATED', '');
      } catch (error) {
        set({ loaded: true });
        refuse(error);
      }
    },

    toggleDone: (id) => {
      const task = get().taskById(id);
      if (!task) return;

      const finishing = task.status === 'ACTIVE';
      void (async () => {
        try {
          const progress = finishing ? await tasksApi.complete(id) : await tasksApi.reopen(id);
          merge(progress.tasks);

          if (finishing) {
            const suffix = progress.levelUp ? ` · LVL ${progress.levelUp.toLevel}` : '';
            get().flash('NODE_RESOLVED', `+${progress.xpAwarded} XP${suffix}`);
            for (const code of progress.unlockedAchievements) get().flash('ACHIEVEMENT', code);
          } else {
            get().flash('NODE_REOPENED', `${progress.xpAwarded} XP`);
          }
        } catch (error) {
          refuse(error);
        }
      })();
    },

    patchTask: (id, patch) => {
      // Applied here first so the field does not lag behind the finger, and
      // written once the typing stops.
      set((state) => ({
        tasks: state.tasks.map((task) => (task.id === id ? { ...task, ...patch } : task)),
      }));

      const change: tasksApi.TaskPatch = { ...pending.get(id) };
      if (patch.title !== undefined) change.title = patch.title.slice(0, TITLE_MAX_LENGTH);
      if (patch.description !== undefined) change.description = patch.description;
      if (patch.color !== undefined) change.color = patch.color;
      if (patch.deadlineAt !== undefined) change.deadlineAt = patch.deadlineAt;
      if (patch.deadlineHasTime !== undefined) change.deadlineHasTime = patch.deadlineHasTime;
      if (Object.keys(change).length === 0) return;

      pending.set(id, change);
      const running = timers.get(id);
      if (running) clearTimeout(running);
      timers.set(
        id,
        setTimeout(() => void flush(id), SAVE_AFTER_MS),
      );
    },

    createTask: (quadrant, rawTitle) => {
      const title = rawTitle.slice(0, TITLE_MAX_LENGTH);
      void (async () => {
        try {
          const task = await tasksApi.create({ title, quadrant });
          set((state) => ({ tasks: [...state.tasks, task] }));
          get().flash(
            'QUEST_REGISTERED',
            `${QUADRANT_BY_ID[quadrant].code} · +${QUADRANT_BY_ID[quadrant].xp} XP on completion`,
          );
        } catch (error) {
          refuse(error);
        }
      })();
    },

    createSubtask: (parentId, rawTitle) => {
      const parent = get().taskById(parentId);
      // One level deep: the UI hides the button, and the server refuses it.
      if (!parent || isSubtask(parent)) return;

      void (async () => {
        try {
          const task = await tasksApi.createSubtask(parentId, rawTitle.slice(0, TITLE_MAX_LENGTH));
          set((state) => ({ tasks: [...state.tasks, task] }));
        } catch (error) {
          refuse(error);
        }
      })();
    },

    promoteSubtask: (id) => {
      const task = get().taskById(id);
      if (!task || task.parentTaskId === null) return;

      void (async () => {
        try {
          const answer = await tasksApi.promote(id);
          merge(answer.tasks);

          const promoted = answer.tasks.find((t) => t.id === id);
          const quadrant = promoted?.quadrant;
          if (quadrant) get().flash('NODE_PROMOTED', `${task.title} → ${QUADRANT_BY_ID[quadrant].code}`);
        } catch (error) {
          refuse(error);
        }
      })();
    },

    moveTask: (id, quadrant, beforeId) => {
      const task = get().taskById(id);
      if (!task) return;
      const from = task.quadrant;
      const wasSubtask = task.parentTaskId !== null;

      void (async () => {
        try {
          // A place, not a number: the server decides the position, which is
          // what keeps two clients dragging at once from disagreeing.
          const answer = await tasksApi.move(id, {
            targetQuadrant: quadrant,
            ...(beforeId ? { beforeTaskId: beforeId } : {}),
          });
          merge(answer.tasks);

          if (from !== null && from !== quadrant) {
            get().flash(
              'PRIORITY_REASSIGNED',
              `${QUADRANT_BY_ID[from].code} → ${QUADRANT_BY_ID[quadrant].code}`,
            );
          } else if (wasSubtask) {
            get().flash('NODE_PROMOTED', `${task.title} → ${QUADRANT_BY_ID[quadrant].code}`);
          }
        } catch (error) {
          refuse(error);
        }
      })();
    },

    reorderSubtask: (id, parentId, beforeId) => {
      void (async () => {
        try {
          const answer = await tasksApi.move(id, {
            parentTaskId: parentId,
            ...(beforeId ? { beforeTaskId: beforeId } : {}),
          });
          merge(answer.tasks);
        } catch (error) {
          refuse(error);
        }
      })();
    },

    deleteTask: (id) => {
      void (async () => {
        try {
          const { deletedIds } = await tasksApi.remove(id);
          const gone = new Set(deletedIds);
          set((state) => ({
            tasks: state.tasks.filter((task) => !gone.has(task.id)),
            // The server breaks the links of a deleted task, so they go too.
            links: state.links.filter(
              (link) => !gone.has(link.sourceTaskId) && !gone.has(link.targetTaskId),
            ),
          }));
          get().flash('NODE_PURGED', id.toUpperCase());
        } catch (error) {
          refuse(error);
        }
      })();
    },

    addLink: (sourceId, targetId) => {
      if (sourceId === targetId) return;

      void (async () => {
        try {
          const link = await tasksApi.createLink(sourceId, targetId);
          set((state) => ({ links: [...state.links, link] }));
          get().flash(
            'NODE_LINK_ESTABLISHED',
            `${sourceId.toUpperCase()} ◈ ${targetId.toUpperCase()}`,
          );
        } catch (error) {
          refuse(error);
        }
      })();
    },

    setLinkType: (linkId, type) => {
      const previous = get().links;
      set((state) => ({
        links: state.links.map((link) => (link.id === linkId ? { ...link, type } : link)),
      }));

      void (async () => {
        try {
          const updated = await tasksApi.setLinkType(linkId, type);
          set((state) => ({
            links: state.links.map((link) => (link.id === linkId ? updated : link)),
          }));
        } catch (error) {
          // The new meaning may collide with a link that already joins this
          // pair, and then the old one still stands.
          set({ links: previous });
          refuse(error);
        }
      })();
    },

    removeLink: (linkId) => {
      const previous = get().links;
      set((state) => ({ links: state.links.filter((link) => link.id !== linkId) }));

      void (async () => {
        try {
          await tasksApi.removeLink(linkId);
        } catch (error) {
          set({ links: previous });
          refuse(error);
        }
      })();
    },

    setTaskColor: (id, color) => get().patchTask(id, { color }),

    addTag: (id, raw) => {
      const task = get().taskById(id);
      const name = raw.trim().slice(0, TAG_MAX_LENGTH);
      if (!task || name === '' || task.tags.includes(name)) return;

      set((state) => ({
        tasks: state.tasks.map((t) => (t.id === id ? { ...t, tags: [...t.tags, name] } : t)),
      }));

      void (async () => {
        try {
          await tasksApi.attachTag(id, name);
          set({ tags: await tasksApi.tags() });
        } catch (error) {
          refuse(error);
          void get().load();
        }
      })();
    },

    removeTag: (id, name) => {
      const task = get().taskById(id);
      const tag = get().tags.find((entry) => entry.name === name);
      if (!task || !tag) return;

      set((state) => ({
        tasks: state.tasks.map((t) =>
          t.id === id ? { ...t, tags: t.tags.filter((entry) => entry !== name) } : t,
        ),
      }));

      void (async () => {
        try {
          await tasksApi.detachTag(id, tag.id);
          set({ tags: await tasksApi.tags() });
        } catch (error) {
          refuse(error);
          void get().load();
        }
      })();
    },

    flash: (code, detail) => set({ toast: { code, detail, params: null, tone: 'green' } }),
    dismissToast: () => set({ toast: null }),
  };
});
