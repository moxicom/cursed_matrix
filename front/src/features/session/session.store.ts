import { create } from 'zustand';

import { currentUser } from '@/shared/mocks';
import type { PlanId, User } from '@/shared/types/domain';

interface SessionState {
  /** Signed out visitors only ever see the landing and pricing pages. */
  authenticated: boolean;
  user: User;
  signIn: () => void;
  signOut: () => void;
  setPlan: (plan: PlanId) => void;
  /** Simulates a trial that has run out — every app action bounces to pricing. */
  expireTrial: () => void;
  toggleLeaderboardVisibility: () => void;
}

export const useSessionStore = create<SessionState>((set) => ({
  authenticated: false,
  user: currentUser,

  signIn: () => set({ authenticated: true }),
  signOut: () => set({ authenticated: false }),
  setPlan: (plan) =>
    set((state) => ({ user: { ...state.user, plan, planExpired: false } })),
  expireTrial: () => set((state) => ({ user: { ...state.user, planExpired: true } })),
  toggleLeaderboardVisibility: () =>
    set((state) => ({
      user: { ...state.user, showInLeaderboard: !state.user.showInLeaderboard },
    })),
}));

/** True when the app is usable; false sends the user to /pricing. */
export function useHasAccess(): boolean {
  return useSessionStore((s) => s.authenticated && !s.user.planExpired);
}
