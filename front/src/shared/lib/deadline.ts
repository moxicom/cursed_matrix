import { DEADLINE_APPROACHING_HOURS } from '@/shared/config/domain';
import { pad2 } from '@/shared/lib/ascii';
import type { DeadlineState, Task } from '@/shared/types/domain';

const MONTHS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'];

export interface DeadlineInfo {
  state: DeadlineState;
  /** Ready-to-render label: `! OVERDUE 26H`, `» T-8H`, `SEP 18 14:30`, `—`. */
  text: string;
  tone: 'none' | 'future' | 'approaching' | 'overdue' | 'completed';
}

export function formatDeadline(date: Date): string {
  return `${MONTHS[date.getMonth()] ?? ''} ${pad2(date.getDate())} ${pad2(date.getHours())}:${pad2(
    date.getMinutes(),
  )}`;
}

/** Same thresholds as the design's dlInfo(): overdue < 0h, approaching < 48h. */
export function deadlineInfo(task: Task, completedLabel: string): DeadlineInfo {
  if (task.status === 'COMPLETED') {
    return { state: 'COMPLETED_ON_TIME', text: completedLabel, tone: 'completed' };
  }
  if (task.deadlineAt === null) {
    return { state: 'NONE', text: '—', tone: 'none' };
  }

  const deadline = new Date(task.deadlineAt);
  const hours = (deadline.getTime() - Date.now()) / 3_600_000;

  if (hours < 0) {
    const abs = Math.abs(hours);
    const amount = abs < 24 ? `${Math.round(abs)}H` : `${Math.round(abs / 24)}D`;
    return { state: 'OVERDUE', text: `! OVERDUE ${amount}`, tone: 'overdue' };
  }
  if (hours < DEADLINE_APPROACHING_HOURS) {
    const amount = hours < 24 ? `${Math.round(hours)}H` : `${Math.round(hours / 24)}D`;
    return { state: 'APPROACHING', text: `» T-${amount}`, tone: 'approaching' };
  }
  return { state: 'FUTURE', text: formatDeadline(deadline), tone: 'future' };
}

export const DEADLINE_TONE_CLASS = {
  none: 'border-line-subtle text-txt-faint',
  future: 'border-line text-txt-dim',
  approaching: 'border-amber-border text-amber-deadline',
  overdue: 'border-red-border text-red-light',
  completed: 'border-green-border-soft text-green',
} satisfies Record<DeadlineInfo['tone'], string>;
