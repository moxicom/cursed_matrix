import { DEADLINE_APPROACHING_HOURS } from '@/shared/config/domain';
import { pad2 } from '@/shared/lib/ascii';
import type { DeadlineState, Task } from '@/shared/types/domain';

const MONTHS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'];

export interface DeadlineInfo {
  state: DeadlineState;
  /** Ready-to-render label: `! OVERDUE 26H`, `» T-8H`, `SEP 18 14:30`, `—`. */
  text: string;
  tone: 'none' | 'future' | 'approaching' | 'overdue' | 'completed' | 'late';
}

export function formatDeadline(date: Date, withTime = true): string {
  const day = `${MONTHS[date.getMonth()] ?? ''} ${pad2(date.getDate())}`;
  // a date-only deadline must not pretend to carry a meaningful time
  return withTime ? `${day} ${pad2(date.getHours())}:${pad2(date.getMinutes())}` : day;
}

/** Same thresholds as the design's dlInfo(): overdue < 0h, approaching < 48h. */
export function deadlineInfo(task: Task, completedLabel: string): DeadlineInfo {
  if (task.status === 'COMPLETED') {
    // a task closed after its deadline is reported as late, with the overrun
    if (task.deadlineAt !== null && task.completedAt !== null) {
      const overrunHours =
        (new Date(task.completedAt).getTime() - new Date(task.deadlineAt).getTime()) / 3_600_000;
      if (overrunHours > 0) {
        const amount =
          overrunHours < 24 ? `${Math.round(overrunHours)}H` : `${Math.round(overrunHours / 24)}D`;
        return { state: 'COMPLETED_LATE', text: `${completedLabel} +${amount}`, tone: 'late' };
      }
    }
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
  return {
    state: 'FUTURE',
    text: formatDeadline(deadline, task.deadlineHasTime),
    tone: 'future',
  };
}

export const DEADLINE_TONE_CLASS = {
  none: 'border-line-subtle text-txt-faint',
  future: 'border-line text-txt-dim',
  approaching: 'border-amber-border text-amber-deadline',
  overdue: 'border-red-border text-red-light',
  completed: 'border-green-border-soft text-green',
  late: 'border-amber-border text-amber-deadline',
} satisfies Record<DeadlineInfo['tone'], string>;
