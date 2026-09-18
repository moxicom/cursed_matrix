import type { ActivityEvent } from '@/shared/types/domain';

const today = new Date().toISOString().slice(0, 10);
const at = (time: string) => `${today}T${time}.000Z`;

/** SYSTEM_LOG rows on the profile page. Codes stay language-independent. */
export const mockEvents: readonly ActivityEvent[] = [
  {
    id: 'e1',
    type: 'TASK_COMPLETED',
    occurredAt: at('14:22:07'),
    localDate: today,
    detail: 'Fix billing webhook duplicate charges',
    xp: 50,
  },
  {
    id: 'e2',
    type: 'SUBTASK_COMPLETED',
    occurredAt: at('14:21:55'),
    localDate: today,
    detail: 'Backfill affected invoices',
    xp: 18,
  },
  {
    id: 'e3',
    type: 'TASK_LINKED',
    occurredAt: at('11:04:31'),
    localDate: today,
    detail: 'T01 ◈ T03 · DEPENDS_ON',
    xp: null,
  },
  {
    id: 'e4',
    type: 'TASK_CREATED',
    occurredAt: at('09:47:12'),
    localDate: today,
    detail: 'Renew production TLS certificates · Q1',
    xp: null,
  },
  {
    id: 'e5',
    type: 'STREAK_EXTENDED',
    occurredAt: at('09:12:00'),
    localDate: today,
    detail: 'streak 18D',
    xp: null,
  },
  {
    id: 'e6',
    type: 'ACHIEVEMENT_UNLOCKED',
    occurredAt: at('08:58:40'),
    localDate: today,
    detail: 'FIREFIGHTER · 25/25',
    xp: null,
  },
];
