import { create } from 'zustand';

import { useBoardStore } from '@/features/board/board.store';
import * as authApi from '@/shared/api/auth';
import { ApiError } from '@/shared/api/client';
import type { User } from '@/shared/types/domain';

type Status = 'unknown' | 'anonymous' | 'authenticated';

interface SessionState {
  /**
   * Unknown until the first answer: the session lives in a cookie the page
   * cannot read, so the only way to learn whether there is one is to ask.
   * Routing on "anonymous" before that would bounce a signed-in user to the
   * landing page on every reload.
   */
  status: Status;
  user: User | null;
  /** The last refusal, as the server stated it: a code and its parameters. */
  error: SessionRefusal | null;
  busy: boolean;

  restore: () => Promise<void>;
  signIn: (username: string, password: string) => Promise<boolean>;
  register: (username: string, password: string, language: 'EN' | 'RU') => Promise<boolean>;
  signOut: () => Promise<void>;
  /** Rotates the token so the new plan is believed, then re-reads the account. */
  refreshAccount: () => Promise<void>;
  toggleLeaderboardVisibility: () => Promise<void>;
}

/** The signed-out shape, used on sign-out and whenever the server says 401. */
const anonymous = { status: 'anonymous' as const, user: null, busy: false };

/**
 * Empties the board along with the session.
 *
 * Without this the next person to sign in on the same tab sees the previous
 * one's tasks for as long as their first load takes — the guard only asks
 * whether there is a session, not whose data is on screen.
 */
function clearAccountData() {
  useBoardStore.getState().reset();
}

/**
 * Which settings write is the latest.
 *
 * Two flicks of the switch are two requests, and they can answer out of
 * order; only the newest one is allowed to decide what the switch shows.
 */
let settingsWrite = 0;

export interface SessionRefusal {
  code: string;
  params: Record<string, unknown>;
}

export const useSessionStore = create<SessionState>((set, get) => ({
  status: 'unknown',
  user: null,
  error: null,
  busy: false,

  restore: async () => {
    try {
      set({ user: await authApi.account(), status: 'authenticated' });
    } catch (error) {
      // A session that has merely gone stale is worth one attempt at
      // refreshing before the user is asked to sign in again.
      if (error instanceof ApiError && error.unauthenticated) {
        try {
          await authApi.refresh();
          set({ user: await authApi.account(), status: 'authenticated' });
          return;
        } catch {
          clearAccountData();
          set(anonymous);
          return;
        }
      }
      clearAccountData();
      set(anonymous);
    }
  },

  signIn: async (username, password) => {
    set({ busy: true, error: null });
    try {
      set({ user: await authApi.signIn(username, password), status: 'authenticated', busy: false });
      return true;
    } catch (error) {
      clearAccountData();
      set({ ...anonymous, error: codeOf(error) });
      return false;
    }
  },

  register: async (username, password, language) => {
    set({ busy: true, error: null });
    try {
      const user = await authApi.register(username, password, language);
      set({ user, status: 'authenticated', busy: false });
      return true;
    } catch (error) {
      clearAccountData();
      set({ ...anonymous, error: codeOf(error) });
      return false;
    }
  },

  signOut: async () => {
    try {
      await authApi.signOut();
    } finally {
      // Whatever the server said, this browser is signed out: leaving the
      // user on a board they can no longer load would be worse.
      clearAccountData();
      set({ ...anonymous, error: null });
    }
  },

  refreshAccount: async () => {
    // The plan rides in the access token, so re-reading the account without
    // rotating it would show the plan the user just left.
    try {
      await authApi.refresh();
    } catch {
      // Not fatal: the account still answers, just with the older token.
    }
    try {
      set({ user: await authApi.account(), status: 'authenticated' });
    } catch {
      set(anonymous);
    }
  },

  toggleLeaderboardVisibility: async () => {
    const current = get().user;
    if (!current) return;

    const wanted = !current.showInLeaderboard;
    // Shown at once because the switch should not lag behind the finger; the
    // server's answer replaces it, and a refusal puts it back.
    set({ user: { ...current, showInLeaderboard: wanted } });

    settingsWrite += 1;
    const write = settingsWrite;
    try {
      const saved = await authApi.updateSettings({ showInLeaderboard: wanted });
      if (write === settingsWrite) set({ user: saved });
    } catch {
      // Only the newest attempt may undo what is shown: an older one would
      // put back a value the user has already changed again.
      if (write !== settingsWrite) return;
      const shown = get().user;
      if (shown) set({ user: { ...shown, showInLeaderboard: current.showInLeaderboard } });
    }
  },
}));

function codeOf(error: unknown): SessionRefusal {
  if (error instanceof ApiError) return { code: error.code, params: error.params };
  return { code: 'REQUEST_FAILED', params: {} };
}

/** True when the app is usable; false sends the user to /pricing. */
export function useHasAccess(): boolean {
  return useSessionStore((s) => s.status === 'authenticated' && !(s.user?.planExpired ?? true));
}

/** Until this is true, routing cannot tell a visitor from a signed-in user. */
export function useSessionResolved(): boolean {
  return useSessionStore((s) => s.status !== 'unknown');
}

/**
 * The signed-in account.
 *
 * Only called from under RequireAccess, which does not render its children
 * until there is one — so this asserts rather than making every screen carry
 * a branch that cannot happen.
 */
export function useUser(): User {
  const user = useSessionStore((s) => s.user);
  if (!user) throw new Error('useUser outside a guarded route');
  return user;
}
