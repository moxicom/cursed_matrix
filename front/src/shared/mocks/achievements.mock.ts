import { ACHIEVEMENT_SEEDS } from './seed';
import type { Achievement, AchievementCategory } from '@/shared/types/domain';

const CATEGORY: Record<string, AchievementCategory> = {
  FIRST_BLOOD: 'TASKS',
  CENTURION: 'TASKS',
  XP_10K: 'XP',
  STREAK_30: 'STREAK',
  NETWORK_BUILDER: 'LINKS',
  FIREFIGHTER: 'TASKS',
  ARCHITECT: 'LEVEL',
  CARTOGRAPHER: 'EXPLORATION',
};

export const mockAchievements: readonly Achievement[] = ACHIEVEMENT_SEEDS.map((seed) => ({
  code: seed.code,
  category: CATEGORY[seed.code] ?? 'TASKS',
  description: { en: seed.descriptionEn, ru: seed.descriptionRu },
  threshold: seed.threshold,
  progress: seed.progress,
  unlockedAt: seed.progress >= seed.threshold ? new Date().toISOString() : null,
}));
