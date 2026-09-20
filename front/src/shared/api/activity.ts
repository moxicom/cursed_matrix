import type {
  Achievement,
  ActivityDay,
  ActivityEventType,
  LeaderboardEntry,
  LeaderboardPeriod,
} from '@/shared/types/domain';

import { api } from './client';

export interface Heatmap {
  days: ActivityDay[];
  totals: { created: number; completed: number };
}

/** 365 days ending today in the user's timezone, empty ones included. */
export async function heatmap() {
  return api.get<Heatmap>('/activity/heatmap');
}

/**
 * Nullable fields arrive absent rather than as null — the contract declares
 * them nullable and not required — so every module that turns the wire shape
 * into a domain one fills them in here.
 */

export interface ActivityStats {
  createdLastYear: number;
  completedLastYear: number;
  currentStreak: number;
  longestStreak: number;
  lifetimeXp: number;
  level: number;
  activeTasks: number;
  archivedTasks: number;
}

export async function stats() {
  return api.get<ActivityStats>('/activity/stats');
}

/** One row of the history: a code with parameters, never a rendered sentence. */
export interface ActivityEntry {
  id: string;
  type: ActivityEventType;
  occurredAt: string;
  localDate: string;
  taskId: string | null;
  taskTitle: string | null;
  detail: Record<string, unknown>;
}

export interface EventPage {
  items: ActivityEntry[];
  nextCursor: string | null;
}

export async function events(cursor?: string, limit?: number) {
  const answer = await api.get<{
    items: Array<Omit<ActivityEntry, 'taskId' | 'taskTitle'> & {
      taskId?: string | null;
      taskTitle?: string | null;
    }>;
    nextCursor?: string | null;
  }>('/activity/events', { query: { cursor, limit } });

  return {
    items: answer.items.map((entry) => ({
      ...entry,
      taskId: entry.taskId ?? null,
      taskTitle: entry.taskTitle ?? null,
    })),
    nextCursor: answer.nextCursor ?? null,
  } satisfies EventPage;
}

/** Recorded once per day; the exploration achievements count the days. */
export async function graphOpened() {
  await api.post<void>('/activity/graph-opened');
}

export async function achievements() {
  const answer = await api.get<{
    items: Array<Omit<Achievement, 'unlockedAt'> & { unlockedAt?: string | null }>;
  }>('/achievements');

  return answer.items.map((item) => ({ ...item, unlockedAt: item.unlockedAt ?? null }));
}

export interface Standing {
  visible: boolean;
  rank: number | null;
  xp: number;
}

export interface Ranking {
  entries: LeaderboardEntry[];
  me: Standing;
  nextOffset: number | null;
}

export async function leaderboard(period: LeaderboardPeriod, limit = 50) {
  const answer = await api.get<{
    entries: Array<LeaderboardEntry & { avatarUrl?: string | null }>;
    me: { visible: boolean; rank?: number | null; xp: number };
    nextOffset?: number | null;
  }>('/leaderboard', { query: { period, limit } });

  return {
    entries: answer.entries,
    me: { ...answer.me, rank: answer.me.rank ?? null },
    nextOffset: answer.nextOffset ?? null,
  } satisfies Ranking;
}
