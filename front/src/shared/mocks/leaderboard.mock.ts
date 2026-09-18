import { currentUser } from './user.mock';
import { LEADERBOARD_SEEDS } from './seed';
import type { LeaderboardEntry, LeaderboardPeriod } from '@/shared/types/domain';

const ME = { monthXp: 2870, weekXp: 1640, streak: currentUser.stats.currentStreak };

function xpFor(
  period: LeaderboardPeriod,
  all: number,
  month: number,
  week: number,
): number {
  if (period === 'WEEK') return week;
  if (period === 'MONTH') return month;
  return all;
}

/** Ranking is recomputed per period, exactly like the XP-transaction query would. */
export function mockLeaderboard(
  period: LeaderboardPeriod,
  includeCurrentUser: boolean,
): LeaderboardEntry[] {
  const rows = LEADERBOARD_SEEDS.map((seed) => ({
    userId: seed.username,
    username: seed.username,
    level: seed.level,
    currentStreak: seed.streak,
    xp: xpFor(period, seed.lifetimeXp, seed.periodXp[0], seed.periodXp[1]),
    isCurrentUser: false,
  }));

  if (includeCurrentUser) {
    rows.push({
      userId: currentUser.id,
      username: currentUser.username,
      level: currentUser.stats.level,
      currentStreak: ME.streak,
      xp: xpFor(period, currentUser.stats.lifetimeXp, ME.monthXp, ME.weekXp),
      isCurrentUser: true,
    });
  }

  return rows
    .sort((a, b) => b.xp - a.xp)
    .map((row, index) => ({ ...row, rank: index + 1 }));
}
