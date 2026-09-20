import type { Language } from '@/shared/types/domain';

/**
 * What each achievement asks for, in the reader's language.
 *
 * The server sends a code and nothing else — it has no business knowing which
 * language anyone reads — so the words live here, keyed by that code.
 */
const DESCRIPTIONS: Record<string, { en: string; ru: string }> = {
  FIRST_BLOOD: { en: 'Complete your first task', ru: 'Завершите первую задачу' },
  CENTURION: { en: 'Complete 100 tasks', ru: 'Завершите 100 задач' },
  XP_10K: { en: 'Reach 10 000 lifetime XP', ru: 'Наберите 10 000 XP за всё время' },
  STREAK_30: { en: 'Keep a 30-day login streak', ru: 'Держите серию входов 30 дней' },
  NETWORK_BUILDER: { en: 'Create 50 links between tasks', ru: 'Создайте 50 связей между задачами' },
  FIREFIGHTER: { en: 'Clear 25 Q1 critical tasks', ru: 'Закройте 25 критических задач Q1' },
  ARCHITECT: { en: 'Reach level 20', ru: 'Достигните 20 уровня' },
  CARTOGRAPHER: { en: 'Open the graph on 14 separate days', ru: 'Откройте граф в 14 разных дней' },
};

/**
 * A code with no words here still shows: a release that adds an achievement
 * server-side must not leave a blank card, so the code itself is the fallback.
 */
export function achievementDescription(code: string, language: Language): string {
  const entry = DESCRIPTIONS[code];
  if (!entry) return code;
  return language === 'RU' ? entry.ru : entry.en;
}
