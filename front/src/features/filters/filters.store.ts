import { create } from 'zustand';

import type { Quadrant, Task, TaskColorId } from '@/shared/types/domain';

export type StatusFilter = 'active' | 'completed' | 'all';
export type DeadlineFilter = 'any' | 'overdue' | 'today' | 'week' | 'none';
export type TopologyFilter = 'any' | 'linked' | 'unlinked';

interface FiltersState {
  status: StatusFilter;
  tags: string[];
  colors: TaskColorId[];
  quadrants: Quadrant[];
  deadline: DeadlineFilter;
  topology: TopologyFilter;
  query: string;

  setStatus: (status: StatusFilter) => void;
  toggleTag: (tag: string) => void;
  toggleColor: (color: TaskColorId) => void;
  toggleQuadrant: (quadrant: Quadrant) => void;
  setDeadline: (deadline: DeadlineFilter) => void;
  setTopology: (topology: TopologyFilter) => void;
  setQuery: (query: string) => void;
  reset: () => void;
}

const toggle = <T,>(list: T[], value: T): T[] =>
  list.includes(value) ? list.filter((item) => item !== value) : [...list, value];

/** Shared by the board toolbar and the graph sidebar — one filter state, two views. */
export const useFiltersStore = create<FiltersState>((set) => ({
  status: 'active',
  tags: [],
  colors: [],
  quadrants: [],
  deadline: 'any',
  topology: 'any',
  query: '',

  setStatus: (status) => set({ status }),
  toggleTag: (tag) => set((s) => ({ tags: toggle(s.tags, tag) })),
  toggleColor: (color) => set((s) => ({ colors: toggle(s.colors, color) })),
  toggleQuadrant: (quadrant) => set((s) => ({ quadrants: toggle(s.quadrants, quadrant) })),
  setDeadline: (deadline) => set({ deadline }),
  setTopology: (topology) => set({ topology }),
  setQuery: (query) => set({ query }),
  reset: () =>
    set({
      status: 'active',
      tags: [],
      colors: [],
      quadrants: [],
      deadline: 'any',
      topology: 'any',
      query: '',
    }),
}));

export interface FilterCriteria {
  status: StatusFilter;
  tags: string[];
  colors: TaskColorId[];
  quadrants: Quadrant[];
  deadline: DeadlineFilter;
  topology: TopologyFilter;
  query: string;
}

/**
 * Single predicate used by board, graph and search — the same logic the backend
 * filter endpoint will implement.
 */
export function matchesFilters(
  task: Task,
  criteria: FilterCriteria,
  context: { quadrant: Quadrant | null; linkCount: number },
): boolean {
  const { status, tags, colors, quadrants, deadline, topology, query } = criteria;

  if (status === 'active' && task.status === 'COMPLETED') return false;
  if (status === 'completed' && task.status === 'ACTIVE') return false;
  if (tags.length > 0 && !tags.some((tag) => task.tags.includes(tag))) return false;
  if (colors.length > 0 && !colors.includes(task.color)) return false;
  if (quadrants.length > 0 && (context.quadrant === null || !quadrants.includes(context.quadrant))) {
    return false;
  }

  if (deadline !== 'any') {
    const due = task.deadlineAt === null ? null : new Date(task.deadlineAt);
    if (deadline === 'none' && due !== null) return false;
    if (deadline === 'overdue' && (due === null || due.getTime() >= Date.now())) return false;

    // Calendar days in the reader's own zone, which is how the server reads
    // the same two windows: "today" is a date, not the next 24 hours.
    if (deadline === 'today' || deadline === 'week') {
      if (due === null) return false;
      const start = new Date();
      start.setHours(0, 0, 0, 0);
      const end = new Date(start);
      end.setDate(end.getDate() + (deadline === 'today' ? 1 : 7));
      if (due < start || due >= end) return false;
    }
  }

  if (topology === 'linked' && context.linkCount === 0) return false;
  if (topology === 'unlinked' && context.linkCount > 0) return false;

  const q = query.trim().toLowerCase();
  if (q !== '') {
    const haystack = `${task.title} ${task.description} ${task.tags.map((t) => `#${t}`).join(' ')}`;
    if (!haystack.toLowerCase().includes(q)) return false;
  }

  return true;
}
