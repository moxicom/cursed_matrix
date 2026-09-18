import type { User } from '@/shared/types/domain';

/** Signed-in operator used across the mocked app. Matches ME_ROW in the design. */
export const currentUser: User = {
  id: 'u_4417',
  username: 'nullptr_ok',
  email: 'operator@quest.terminal',
  avatarUrl: null,
  language: 'EN',
  timezone: 'Europe/Moscow',
  showInLeaderboard: true,
  plan: 'FREE',
  planExpired: false,
  createdAt: '2024-03-01T00:00:00.000Z',
  stats: {
    lifetimeXp: 7420,
    level: 12,
    currentStreak: 18,
    longestStreak: 31,
    tasksCreated: 418,
    tasksCompleted: 312,
    subtasksCompleted: 194,
    linksCreated: 31,
    achievementsUnlocked: 2,
  },
};
