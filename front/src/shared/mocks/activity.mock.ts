import type { ActivityDay } from '@/shared/types/domain';

/** xorshift32 — the design's rnd(), so the heatmap is byte-identical across reloads. */
function seededRandom(seed: number): () => number {
  let x = seed || 1;
  return () => {
    x ^= x << 13;
    x >>>= 0;
    x ^= x >> 17;
    x ^= x << 5;
    x >>>= 0;
    return x / 4294967296;
  };
}

const DAY = 86_400_000;

function buildDays(): ActivityDay[] {
  const today = new Date();
  const end = new Date(today.getFullYear(), today.getMonth(), today.getDate());
  const random = seededRandom(4242);

  let start = new Date(end.getTime() - 364 * DAY);
  start = new Date(start.getTime() - start.getDay() * DAY); // align to Sunday

  const days: ActivityDay[] = [];
  for (let d = new Date(start); d <= end; d = new Date(d.getTime() + DAY)) {
    const weekday = d.getDay();
    const base = weekday === 0 || weekday === 6 ? 0.1 : 0.26;
    const active = random() < base;
    const completedCount = active ? 1 + Math.floor(random() * 2.2) : 0;
    const createdCount = active ? Math.floor(random() * 3.2) : 0;
    days.push({
      date: new Date(d.getTime() - d.getTimezoneOffset() * 60_000).toISOString().slice(0, 10),
      createdCount,
      completedCount,
      totalActivity: completedCount + createdCount,
    });
  }
  return days;
}

export const mockActivityDays: readonly ActivityDay[] = buildDays();

export const mockActivityTotals = mockActivityDays.reduce(
  (acc, day) => ({
    created: acc.created + day.createdCount,
    completed: acc.completed + day.completedCount,
  }),
  { created: 0, completed: 0 },
);
