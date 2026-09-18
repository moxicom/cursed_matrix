import type { LinkType, Quadrant, TaskColorId } from '@/shared/types/domain';

/**
 * Domain constants taken from the design (QC / XPQ / COLS / COLORS / LINK_TYPES).
 * XP values and the level curve are the design's, not the ones drafted in docs/SPEC.md.
 */

export interface QuadrantMeta {
  id: Quadrant;
  /** Short display code: Q1..Q4. */
  code: string;
  accent: string;
  /** XP a task in this quadrant awards on completion. */
  xp: number;
  name: { en: string; ru: string };
  subtitle: { en: string; ru: string };
  empty: { en: string; ru: string };
}

export const QUADRANTS: readonly QuadrantMeta[] = [
  {
    id: "IMPORTANT_URGENT",
    code: "Q1",
    accent: "#d2685a",
    xp: 50,
    name: {
      en: "CRITICAL_PATH",
      ru: "КРИТИЧЕСКИЙ_ПУТЬ",
    },
    subtitle: {
      en: "IMPORTANT + URGENT",
      ru: "ВАЖНО + СРОЧНО",
    },
    empty: {
      en: "NO_CRITICAL_QUESTS",
      ru: "КРИТИЧЕСКИХ_ЗАДАЧ_НЕТ",
    },
  },
  {
    id: "IMPORTANT_NOT_URGENT",
    code: "Q2",
    accent: "#c8973c",
    xp: 35,
    name: {
      en: "SCHEDULED_OPS",
      ru: "ПЛАНОВЫЕ_ОПЕРАЦИИ",
    },
    subtitle: {
      en: "IMPORTANT · NOT URGENT",
      ru: "ВАЖНО · НЕ СРОЧНО",
    },
    empty: {
      en: "NO_SCHEDULED_QUESTS",
      ru: "ПЛАНОВЫХ_ЗАДАЧ_НЕТ",
    },
  },
  {
    id: "NOT_IMPORTANT_URGENT",
    code: "Q3",
    accent: "#6fa8c0",
    xp: 20,
    name: {
      en: "DELEGATE_QUEUE",
      ru: "ОЧЕРЕДЬ_ДЕЛЕГИРОВАНИЯ",
    },
    subtitle: {
      en: "NOT IMPORTANT · URGENT",
      ru: "НЕ ВАЖНО · СРОЧНО",
    },
    empty: {
      en: "QUEUE_EMPTY",
      ru: "ОЧЕРЕДЬ_ПУСТА",
    },
  },
  {
    id: "NOT_IMPORTANT_NOT_URGENT",
    code: "Q4",
    accent: "#7d878c",
    xp: 10,
    name: {
      en: "BACKLOG_SINK",
      ru: "ОТСТОЙНИК_БЭКЛОГА",
    },
    subtitle: {
      en: "NOT IMPORTANT · NOT URGENT",
      ru: "НЕ ВАЖНО · НЕ СРОЧНО",
    },
    empty: {
      en: "SINK_EMPTY",
      ru: "ОТСТОЙНИК_ПУСТ",
    },
  },
];

export const QUADRANT_BY_ID: Record<Quadrant, QuadrantMeta> = Object.fromEntries(
  QUADRANTS.map((q) => [q.id, q]),
) as Record<Quadrant, QuadrantMeta>;

export interface TaskColorMeta {
  id: TaskColorId;
  /** null renders as the transparent "no color" swatch. */
  hex: string | null;
  name: { en: string; ru: string };
}

export const TASK_COLORS: readonly TaskColorMeta[] = [
  {
    id: "NONE",
    hex: null,
    name: {
      en: "NONE",
      ru: "НЕТ",
    },
  },
  {
    id: "CYAN",
    hex: "#4f93ad",
    name: {
      en: "CYAN",
      ru: "ГОЛУБОЙ",
    },
  },
  {
    id: "VIOLET",
    hex: "#8f7fc4",
    name: {
      en: "VIOLET",
      ru: "ФИОЛЕТОВЫЙ",
    },
  },
  {
    id: "AMBER",
    hex: "#c8973c",
    name: {
      en: "AMBER",
      ru: "ЯНТАРНЫЙ",
    },
  },
  {
    id: "ROSE",
    hex: "#c05b6a",
    name: {
      en: "ROSE",
      ru: "РОЗОВЫЙ",
    },
  },
  {
    id: "TEAL",
    hex: "#3f9e8f",
    name: {
      en: "TEAL",
      ru: "БИРЮЗОВЫЙ",
    },
  },
  {
    id: "SLATE",
    hex: "#6b7780",
    name: {
      en: "SLATE",
      ru: "СЕРЫЙ",
    },
  },
];

export const TASK_COLOR_BY_ID: Record<TaskColorId, TaskColorMeta> = Object.fromEntries(
  TASK_COLORS.map((c) => [c.id, c]),
) as Record<TaskColorId, TaskColorMeta>;

export const LINK_TYPES: readonly LinkType[] = ["RELATED", "BLOCKS", "DEPENDS_ON", "CONNECTED_TO"];

/** Subtasks award 35% of their parent quadrant's XP. */
export const SUBTASK_XP_MULTIPLIER = 0.35;

/** Free plan quotas. Hitting either one opens the paywall. */
export const FREE_TASK_CAP = 35;
export const FREE_LINK_CAP = 25;

/** Deadline within this many hours renders as "approaching". */
export const DEADLINE_APPROACHING_HOURS = 48;
