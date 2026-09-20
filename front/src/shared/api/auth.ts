import type { PlanId, User } from '@/shared/types/domain';

import { api } from './client';

/**
 * What the server answers with for the signed-in account.
 *
 * Declared here rather than reusing the domain type so that a change on the
 * server shows up as a type error in one file instead of silently reshaping
 * the app's own vocabulary.
 */
interface AccountResponse {
  id: string;
  username: string;
  email?: string | null;
  avatarUrl?: string | null;
  language: 'EN' | 'RU';
  timezone: string;
  showInLeaderboard: boolean;
  plan: PlanId;
  planExpiresAt?: string | null;
  planExpired: boolean;
  createdAt: string;
  stats: User['stats'];
}

function toUser(account: AccountResponse): User {
  return {
    id: account.id,
    username: account.username,
    email: account.email ?? null,
    avatarUrl: account.avatarUrl ?? null,
    language: account.language,
    timezone: account.timezone,
    showInLeaderboard: account.showInLeaderboard,
    plan: account.plan,
    planExpiresAt: account.planExpiresAt ?? null,
    planExpired: account.planExpired,
    createdAt: account.createdAt,
    stats: account.stats,
  };
}

/** The zone the browser is in, which the server needs for every day-based rule. */
function timezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  } catch {
    return 'UTC';
  }
}

export async function register(username: string, password: string, language: 'EN' | 'RU') {
  return toUser(
    await api.post<AccountResponse>('/auth/register', {
      body: { username, password, timezone: timezone(), language },
    }),
  );
}

export async function signIn(username: string, password: string) {
  return toUser(
    await api.post<AccountResponse>('/auth/login', {
      // Sent on every sign-in, so a travelling user stays correct.
      body: { username, password, timezone: timezone() },
    }),
  );
}

export async function signOut() {
  await api.post<void>('/auth/logout');
}

export async function signOutEverywhere() {
  await api.post<void>('/auth/logout-all');
}

/** Rotates the pair. Used when a request says the session expired. */
export async function refresh() {
  await api.post<void>('/auth/refresh');
}

export async function account() {
  return toUser(await api.get<AccountResponse>('/me'));
}

export interface SettingsChange {
  language?: 'EN' | 'RU';
  timezone?: string;
  showInLeaderboard?: boolean;
}

export async function updateSettings(change: SettingsChange) {
  return toUser(await api.patch<AccountResponse>('/me', { body: change }));
}
