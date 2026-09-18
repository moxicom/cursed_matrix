import { QUADRANT_BY_ID, SUBTASK_XP_MULTIPLIER } from '@/shared/config/domain';
import type { Quadrant, Task } from '@/shared/types/domain';

/**
 * XP and level curve, matching the design exactly:
 *   levelOf(xp)     = floor(sqrt(xp / 45)) + 1
 *   xpForLevel(l)   = 45 * (l - 1)^2
 * The backend owns these numbers; the frontend only mirrors them for display.
 */

export function levelForXp(xp: number): number {
  return Math.max(1, Math.floor(Math.sqrt(xp / 45)) + 1);
}

export function xpForLevel(level: number): number {
  return Math.round(45 * (level - 1) ** 2);
}

export interface LevelProgress {
  level: number;
  /** XP earned inside the current level. */
  into: number;
  /** XP the current level spans. */
  span: number;
  /** XP left until the next level. */
  remaining: number;
  ratio: number;
}

export function levelProgress(xp: number): LevelProgress {
  const level = levelForXp(xp);
  const current = xpForLevel(level);
  const next = xpForLevel(level + 1);
  const span = Math.max(1, next - current);
  return {
    level,
    into: xp - current,
    span,
    remaining: next - xp,
    ratio: (xp - current) / span,
  };
}

/** XP a task would award right now. Subtasks use their parent's quadrant. */
export function xpForTask(quadrant: Quadrant, isSubtask: boolean): number {
  const base = QUADRANT_BY_ID[quadrant].xp;
  return isSubtask ? Math.round(base * SUBTASK_XP_MULTIPLIER) : base;
}

/** Effective quadrant of a task: its own, or the parent's for a subtask. */
export function effectiveQuadrant(task: Task, parent?: Task): Quadrant | null {
  return task.quadrant ?? parent?.quadrant ?? null;
}
