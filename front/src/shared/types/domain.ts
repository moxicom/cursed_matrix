/**
 * Shared domain contract.
 *
 * These are the shapes the backend (back/, Golang) is expected to return, so mocks
 * in shared/mocks are typed with exactly these interfaces and swap out for real
 * responses without touching the components. All timestamps are ISO-8601 strings.
 */

export type Quadrant =
  | 'IMPORTANT_URGENT'
  | 'IMPORTANT_NOT_URGENT'
  | 'NOT_IMPORTANT_URGENT'
  | 'NOT_IMPORTANT_NOT_URGENT';

export type TaskStatus = 'ACTIVE' | 'COMPLETED';
export type CompletionSource = 'DIRECT' | 'PARENT_CASCADE';
export type LinkType = 'RELATED' | 'BLOCKS' | 'DEPENDS_ON' | 'CONNECTED_TO';
export type Language = 'EN' | 'RU';
export type PlanId = 'FREE' | 'PRO' | 'SELF_HOSTED';

export type TaskColorId = 'NONE' | 'CYAN' | 'VIOLET' | 'AMBER' | 'ROSE' | 'TEAL' | 'SLATE';

export type DeadlineState =
  | 'NONE'
  | 'FUTURE'
  | 'APPROACHING'
  | 'OVERDUE'
  | 'COMPLETED_ON_TIME'
  | 'COMPLETED_LATE';

export interface Task {
  id: string;
  userId: string;
  /** null for a regular task, parent id for a subtask. One level only. */
  parentTaskId: string | null;
  title: string;
  description: string;
  /** null on subtasks — they inherit the parent's quadrant. */
  quadrant: Quadrant | null;
  position: number;
  color: TaskColorId;
  deadlineAt: string | null;
  deadlineHasTime: boolean;
  status: TaskStatus;
  createdAt: string;
  updatedAt: string;
  completedAt: string | null;
  /** XP snapshot, frozen at completion. */
  xpAwarded: number | null;
  quadrantAtCompletion: Quadrant | null;
  completedVia: CompletionSource | null;
  tags: string[];
}

/**
 * Narrowed views of `Task`.
 *
 * The flat `Task` above can express states the domain forbids — a subtask with
 * a quadrant, or a COMPLETED task without `completedAt`. Making that
 * unrepresentable belongs in the backend contract (a discriminated union plus
 * response validation at the API boundary); until that exists, these guards let
 * the code narrow explicitly instead of guessing with `??` fallbacks.
 */
export type RegularTask = Task & { parentTaskId: null; quadrant: Quadrant };
export type Subtask = Task & { parentTaskId: string; quadrant: null };
export type CompletedTask = Task & {
  status: 'COMPLETED';
  completedAt: string;
  xpAwarded: number;
  quadrantAtCompletion: Quadrant;
};

export function isSubtask(task: Task): task is Subtask {
  return task.parentTaskId !== null;
}

export function isRegularTask(task: Task): task is RegularTask {
  return task.parentTaskId === null && task.quadrant !== null;
}

export function isCompleted(task: Task): task is CompletedTask {
  return task.status === 'COMPLETED' && task.completedAt !== null;
}

export interface TaskLink {
  id: string;
  sourceTaskId: string;
  targetTaskId: string;
  type: LinkType;
  createdAt: string;
}

export interface UserStats {
  lifetimeXp: number;
  level: number;
  currentStreak: number;
  longestStreak: number;
  tasksCreated: number;
  tasksCompleted: number;
  subtasksCompleted: number;
  linksCreated: number;
  achievementsUnlocked: number;
}

export interface User {
  id: string;
  username: string;
  /** Optional: the account is identified by its username. */
  email: string | null;
  avatarUrl: string | null;
  language: Language;
  timezone: string;
  showInLeaderboard: boolean;
  plan: PlanId;
  /** Null on a plan that does not expire. */
  planExpiresAt: string | null;
  /** Trial that has run out — every action bounces to pricing. */
  planExpired: boolean;
  createdAt: string;
  stats: UserStats;
}

export type LeaderboardPeriod = 'WEEK' | 'MONTH' | 'ALL_TIME';

export interface LeaderboardEntry {
  rank: number;
  userId: string;
  username: string;
  xp: number;
  level: number;
  currentStreak: number;
  isCurrentUser: boolean;
}

export type ActivityEventType =
  | 'TASK_CREATED'
  | 'TASK_COMPLETED'
  | 'SUBTASK_COMPLETED'
  | 'TASK_LINKED'
  | 'LEVEL_UP'
  | 'ACHIEVEMENT_UNLOCKED'
  | 'STREAK_EXTENDED'
  | 'GRAPH_OPENED';

export interface ActivityEvent {
  id: string;
  type: ActivityEventType;
  occurredAt: string;
  /** Calendar day in the user's timezone — what the heatmap buckets by. */
  localDate: string;
  taskId: string | null;
  taskTitle: string | null;
  /**
   * Parameters, never a rendered sentence: the row is written here, in
   * whichever language is selected.
   */
  detail: Record<string, unknown>;
}

export interface ActivityDay {
  date: string;
  createdCount: number;
  completedCount: number;
  totalActivity: number;
}

export type AchievementCategory = 'TASKS' | 'XP' | 'LEVEL' | 'STREAK' | 'LINKS' | 'EXPLORATION';

export interface Achievement {
  code: string;
  category: AchievementCategory;
  threshold: number;
  progress: number;
  rewardXp: number;
  unlockedAt: string | null;
}

/** A label with how many live tasks carry it, counted by the server. */
export interface TagStat {
  name: string;
  count: number;
}
